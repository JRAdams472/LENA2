package bff

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/graph-gophers/graphql-go"
)

// groceryChildren holds batch-loaded rows for one or more grocery lists so
// nested field resolvers never issue a query per row.
type groceryChildren struct {
	itemsByList map[int64][]grocery.GroceryListItem
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
}

// loadGroceryChildren batch-loads list items and every catalog row they
// reference — items, units, ingredients, and per-item children.
func loadGroceryChildren(ctx context.Context, g GroceryService, inv ItemReader, userID int64, listIDs []int64) (*groceryChildren, error) {
	gc := &groceryChildren{
		itemsByList: make(map[int64][]grocery.GroceryListItem),
		items:       make(map[int64]inventory.Item),
		units:       make(map[int64]inventory.Unit),
		ingredients: make(map[int64]inventory.Ingredient),
	}
	if len(listIDs) == 0 {
		return gc, nil
	}
	listItems, err := g.ListGroceryListItemsByLists(ctx, listIDs, userID)
	if err != nil {
		return nil, err
	}
	for _, it := range listItems {
		gc.itemsByList[it.GroceryListID] = append(gc.itemsByList[it.GroceryListID], it)
	}
	gc.items, err = loadItems(ctx, inv, distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.ItemID }))
	if err != nil {
		return nil, err
	}
	gc.units, err = loadUnits(ctx, inv, distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.UnitID }))
	if err != nil {
		return nil, err
	}
	gc.ingredients, err = loadIngredients(ctx, inv, distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.IngredientID }))
	if err != nil {
		return nil, err
	}
	itemList := make([]inventory.Item, 0, len(gc.items))
	for _, it := range gc.items {
		itemList = append(itemList, it)
	}
	gc.ch, err = loadItemChildren(ctx, inv, itemList)
	if err != nil {
		return nil, err
	}
	return gc, nil
}

// GroceryList resolves a single grocery list by ID.
func (r *Resolver) GroceryList(ctx context.Context, args struct{ ID graphql.ID }) (*groceryListResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	list, err := r.GroceryService.GetGroceryListByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	// Single-list reads preload the same children as the list page so
	// nested resolvers never fall back to per-row queries.
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, u.UserID, []int64{id})
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, userID: u.UserID, list: list, items: gc.itemsByList[id], catItems: gc.items, units: gc.units, ingredients: gc.ingredients, ch: gc.ch}, nil
}

// GroceryLists resolves the current user's grocery lists.
func (r *Resolver) GroceryLists(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*groceryListPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)
	lists, err := r.GroceryService.ListGroceryLists(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.GroceryService.CountGroceryLists(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, u.UserID, distinctIDs(lists, func(l grocery.GroceryList) *int64 { return &l.GroceryListID }))
	if err != nil {
		return nil, err
	}
	return &groceryListPageResolver{g: r.GroceryService, inv: r.InventoryService, userID: u.UserID, lists: lists, itemsByList: gc.itemsByList, items: gc.items, units: gc.units, ingredients: gc.ingredients, ch: gc.ch, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// GenerateGroceryList generates a grocery list from a meal plan. The BFF
// is the only layer allowed to read across mealplan, recipe, userprefs and
// inventory, so the whole expansion — slots to recipe items, per-item
// totals, pantry-stock subtraction, and the list write — is composed here
// inside one unit of work.
func (r *Resolver) GenerateGroceryList(ctx context.Context, args struct{ MealPlanID graphql.ID }) (*groceryListResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.MealPlanID))
	if err != nil {
		return nil, err
	}
	// Verify the plan belongs to the caller before creating a list linked
	// to it — otherwise the mutation is an existence oracle on other
	// users' plan IDs via the FK error.
	if _, err := r.MealPlanService.GetMealPlanByID(ctx, mealPlanID, u.UserID); err != nil {
		return nil, err
	}

	var list grocery.GroceryList
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		created, err := r.GroceryService.CreateGroceryList(ctx, u.UserID, &mealPlanID, u.Email)
		if err != nil {
			return err
		}
		list = created

		slots, err := r.MealPlanService.ListMealSlotsForPlan(ctx, mealPlanID, u.UserID)
		if err != nil {
			return err
		}
		slotItems, err := r.MealPlanService.ListMealSlotItemsByPlan(ctx, mealPlanID, u.UserID)
		if err != nil {
			return err
		}

		recipes, recipeItems, err := r.planRecipes(ctx, slots)
		if err != nil {
			return err
		}
		needs := aggregateGroceryNeeds(slots, slotItems, recipes, recipeItems)
		if len(needs) == 0 {
			return nil
		}

		stock, err := r.userItemStock(ctx, u.UserID)
		if err != nil {
			return err
		}
		itemIDs := distinctIDs(needs, func(n groceryNeed) *int64 { return &n.itemID })
		itemsByID, err := loadItems(ctx, r.InventoryService, itemIDs)
		if err != nil {
			return err
		}
		unitSet := make(map[int64]bool)
		for _, n := range needs {
			unitSet[n.unitID] = true
		}
		for _, it := range itemsByID {
			unitSet[it.UnitID] = true
		}
		unitIDs := make([]int64, 0, len(unitSet))
		for id := range unitSet {
			unitIDs = append(unitIDs, id)
		}
		unitsByID, err := loadUnits(ctx, r.InventoryService, unitIDs)
		if err != nil {
			return err
		}

		lines := groceryNeedLines(list.GroceryListID, needs, stock, itemsByID, unitsByID)
		_, err = r.GroceryService.AddGroceryListItems(ctx, lines, u.UserID, u.Email)
		return err
	}); err != nil {
		return nil, err
	}
	// Preload the freshly generated list's children so nested resolvers
	// never fall back to per-row queries.
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, u.UserID, []int64{list.GroceryListID})
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, userID: u.UserID, list: list, items: gc.itemsByList[list.GroceryListID], catItems: gc.items, units: gc.units, ingredients: gc.ingredients, ch: gc.ch}, nil
}

// planLine is one expanded plan contribution: a quantity of a catalog
// item in a specific unit.
type planLine struct {
	itemID int64
	unitID int64
	qty    float64
}

// expandPlanLines expands a plan's slots into item quantity lines.
// Explicit slot items always count; a slot item flagged is_from_recipe
// replaces the recipe's own line for that item, and recipe contributions
// scale by the slot's servings ratio. Shared by grocery generation and
// nutrition aggregation.
func expandPlanLines(slots []mealplan.MealSlot, slotItems []mealplan.MealSlotItem, recipes []recipe.Recipe, recipeItems []recipe.RecipeItem) []planLine {
	itemsBySlot := make(map[int64][]mealplan.MealSlotItem)
	for _, si := range slotItems {
		itemsBySlot[si.SlotID] = append(itemsBySlot[si.SlotID], si)
	}
	recipesByID := make(map[int64]recipe.Recipe, len(recipes))
	for _, rec := range recipes {
		recipesByID[rec.RecipeID] = rec
	}
	itemsByRecipe := make(map[int64][]recipe.RecipeItem)
	for _, ri := range recipeItems {
		itemsByRecipe[ri.RecipeID] = append(itemsByRecipe[ri.RecipeID], ri)
	}

	var lines []planLine
	for _, slot := range slots {
		overridden := make(map[int64]bool)
		for _, si := range itemsBySlot[slot.SlotID] {
			if si.ItemID == nil {
				continue
			}
			lines = append(lines, planLine{itemID: *si.ItemID, unitID: si.UnitID, qty: si.Quantity})
			if si.IsFromRecipe {
				overridden[*si.ItemID] = true
			}
		}
		if slot.RecipeID == nil {
			continue
		}
		rec, ok := recipesByID[*slot.RecipeID]
		if !ok || rec.Servings == nil || *rec.Servings <= 0 {
			continue
		}
		scale := 1.0
		if slot.Servings != nil {
			scale = float64(*slot.Servings) / float64(*rec.Servings)
		}
		for _, ri := range itemsByRecipe[rec.RecipeID] {
			if overridden[ri.ItemID] {
				continue
			}
			lines = append(lines, planLine{itemID: ri.ItemID, unitID: ri.UnitID, qty: ri.Quantity * scale})
		}
	}
	return lines
}

// groceryNeed is a unit-resolved quantity of a catalog item to buy.
type groceryNeed struct {
	itemID int64
	unitID int64
	qty    float64
}

// aggregateGroceryNeeds groups expanded plan lines into per-(item, unit)
// totals in deterministic order.
func aggregateGroceryNeeds(slots []mealplan.MealSlot, slotItems []mealplan.MealSlotItem, recipes []recipe.Recipe, recipeItems []recipe.RecipeItem) []groceryNeed {
	type key struct{ itemID, unitID int64 }
	totals := make(map[key]float64)
	for _, l := range expandPlanLines(slots, slotItems, recipes, recipeItems) {
		totals[key{l.itemID, l.unitID}] += l.qty
	}

	keys := make([]key, 0, len(totals))
	for k := range totals {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].itemID != keys[j].itemID {
			return keys[i].itemID < keys[j].itemID
		}
		return keys[i].unitID < keys[j].unitID
	})
	out := make([]groceryNeed, 0, len(keys))
	for _, k := range keys {
		out = append(out, groceryNeed{itemID: k.itemID, unitID: k.unitID, qty: totals[k]})
	}
	return out
}

// userItemStock returns the user's on-hand quantity per item, expressed in
// each item's canonical unit.
func (r *Resolver) userItemStock(ctx context.Context, userID int64) (map[int64]float64, error) {
	stock := make(map[int64]float64)
	const page int32 = 1000
	for offset := int32(0); ; offset += page {
		rows, err := r.UserPrefsService.ListUserItems(ctx, userID, page, offset)
		if err != nil {
			return nil, err
		}
		for _, ui := range rows {
			stock[ui.ItemID] += ui.CurrentQty
		}
		if len(rows) < int(page) {
			return stock, nil
		}
	}
}

// groceryNeedLines subtracts pantry stock — converted into each need's
// unit when the kinds allow — and returns the remaining quantities as
// grocery list rows. Fully covered needs produce no line.
func groceryNeedLines(listID int64, needs []groceryNeed, stock map[int64]float64, items map[int64]inventory.Item, units map[int64]inventory.Unit) []grocery.GroceryListItem {
	out := make([]grocery.GroceryListItem, 0, len(needs))
	for _, n := range needs {
		remaining := n.qty
		if onHand := stock[n.itemID]; onHand > 0 {
			if it, ok := items[n.itemID]; ok {
				from, okFrom := units[it.UnitID]
				to, okTo := units[n.unitID]
				if okFrom && okTo {
					if converted, ok := inventory.ConvertQuantity(onHand, from, to); ok {
						remaining -= converted
					}
				}
			}
		}
		if remaining <= 0 {
			continue
		}
		itemID := n.itemID
		unitID := n.unitID
		out = append(out, grocery.GroceryListItem{
			GroceryListID:  listID,
			ItemID:         &itemID,
			QuantityNeeded: remaining,
			UnitID:         &unitID,
			Source:         "mealplan",
		})
	}
	return out
}

// ToggleGroceryItemChecked flips the checked state of a grocery list item.
// The flip and the pantry quantity adjustment run inside one unit of work
// so pantry stock always matches the list state; the ctx-carried
// transaction joins both domain services automatically.
func (r *Resolver) ToggleGroceryItemChecked(ctx context.Context, args struct{ GroceryListItemID graphql.ID }) (*groceryListItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return nil, err
	}

	var updated grocery.GroceryListItem
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		toggled, err := r.GroceryService.ToggleGroceryListItemChecked(ctx, id, u.UserID, u.Email)
		if err != nil {
			return err
		}
		if toggled.ItemID != nil {
			delta := toggled.QuantityNeeded
			if !toggled.IsChecked {
				delta = -delta
			}
			if _, err := r.UserPrefsService.AdjustUserItemQuantity(ctx, u.UserID, *toggled.ItemID, delta, u.Email); err != nil {
				return err
			}
		}
		updated = toggled
		return nil
	}); err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: updated}, nil
}

// DeleteGroceryItem removes an item from a grocery list.
func (r *Resolver) DeleteGroceryItem(ctx context.Context, args struct{ GroceryListItemID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return false, err
	}
	if err := r.GroceryService.DeleteGroceryListItem(ctx, id, u.UserID); err != nil {
		return false, err
	}
	return true, nil
}

// AddGroceryItem adds a manual or catalog item to a grocery list.
func (r *Resolver) AddGroceryItem(ctx context.Context, args struct{ Input addGroceryItemInput }) (*groceryListItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	item, err := r.parseGroceryItemInput(ctx, args.Input)
	if err != nil {
		return nil, err
	}
	it, err := r.GroceryService.AddGroceryListItem(ctx, item, u.UserID, u.Email)
	if err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: it}, nil
}

// parseGroceryItemInput validates a grocery-item input and converts it to
// the service type, resolving the optional unit name to a unit ID.
func (r *Resolver) parseGroceryItemInput(ctx context.Context, in addGroceryItemInput) (grocery.GroceryListItem, error) {
	groceryListID, err := parseID(string(in.GroceryListID))
	if err != nil {
		return grocery.GroceryListItem{}, err
	}
	itemID, err := optionalID(in.ItemID)
	if err != nil {
		return grocery.GroceryListItem{}, err
	}
	ingredientID, err := optionalID(in.IngredientID)
	if err != nil {
		return grocery.GroceryListItem{}, err
	}
	manualName := derefString(in.ManualItemName)
	if itemID == nil && ingredientID == nil && strings.TrimSpace(manualName) == "" {
		return grocery.GroceryListItem{}, badInputf("grocery item requires an itemId, ingredientId or manualItemName")
	}
	if in.Quantity <= 0 {
		return grocery.GroceryListItem{}, badInputf("quantity must be positive")
	}
	// The unit is optional for grocery items; an empty string means unset.
	var unitID *int64
	if u := strings.TrimSpace(in.Unit); u != "" {
		id, err := resolveUnitID(ctx, r.InventoryService, u)
		if err != nil {
			return grocery.GroceryListItem{}, err
		}
		unitID = &id
	}
	return grocery.GroceryListItem{
		GroceryListID:  groceryListID,
		ItemID:         itemID,
		IngredientID:   ingredientID,
		ManualItemName: manualName,
		QuantityNeeded: in.Quantity,
		UnitID:         unitID,
		Source:         "manual",
	}, nil
}

// groceryListResolver resolves GroceryList fields. When items is non-nil
// the batch-loaded rows and catalog lookups are used instead of per-list
// service calls.
type groceryListResolver struct {
	g           GroceryService
	inv         ItemReader
	userID      int64
	list        grocery.GroceryList
	items       []grocery.GroceryListItem
	catItems    map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
}

func (r *groceryListResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.list.GroceryListID, 10))
}

func (r *groceryListResolver) GeneratedAt() graphql.Time {
	return graphql.Time{Time: r.list.GeneratedAt}
}

func (r *groceryListResolver) Items(ctx context.Context) ([]*groceryListItemResolver, error) {
	var items []grocery.GroceryListItem
	if r.ch != nil {
		items = r.items
	} else {
		slog.Default().Warn("groceryList.items missed preload; lazy-loading", "grocery_list_id", r.list.GroceryListID)
		var err error
		items, err = r.g.ListGroceryListItems(ctx, r.list.GroceryListID, r.userID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*groceryListItemResolver, len(items))
	for i := range items {
		out[i] = &groceryListItemResolver{inv: r.inv, item: items[i], items: r.catItems, units: r.units, ingredients: r.ingredients, ch: r.ch}
	}
	return out, nil
}

// groceryListItemResolver resolves GroceryListItem fields.
type groceryListItemResolver struct {
	inv         ItemReader
	item        grocery.GroceryListItem
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
}

func (r *groceryListItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.GroceryListItemID, 10))
}

func (r *groceryListItemResolver) ManualItemName() *string { return nilIfEmpty(r.item.ManualItemName) }

func (r *groceryListItemResolver) QuantityNeeded() float64 { return r.item.QuantityNeeded }

func (r *groceryListItemResolver) UnitOfMeasure(ctx context.Context) (*string, error) {
	return unitNamePtr(ctx, r.inv, r.units, r.item.UnitID)
}

func (r *groceryListItemResolver) Source() string { return r.item.Source }

func (r *groceryListItemResolver) IsChecked() bool { return r.item.IsChecked }

// Ingredient resolves the brand-agnostic ingredient linked to this grocery
// list item, when set. Scaffolding only — nothing populates ingredient_id
// yet.
func (r *groceryListItemResolver) Ingredient(ctx context.Context) (*ingredientResolver, error) {
	if r.item.IngredientID == nil {
		return nil, nil
	}
	if r.ingredients != nil {
		in, ok := r.ingredients[*r.item.IngredientID]
		if !ok {
			return nil, nil
		}
		return &ingredientResolver{inv: r.inv, in: in}, nil
	}
	slog.Default().Warn("groceryListItem.ingredient missed preload; lazy-loading", "ingredient_id", *r.item.IngredientID)
	in, err := r.inv.GetIngredientByID(ctx, *r.item.IngredientID)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.inv, in: in}, nil
}

func (r *groceryListItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.item.ItemID == nil {
		return nil, nil
	}
	if r.items != nil {
		it, ok := r.items[*r.item.ItemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch}, nil
	}
	slog.Default().Warn("groceryListItem.item missed preload; lazy-loading", "item_id", *r.item.ItemID)
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type groceryListPageResolver struct {
	g           GroceryService
	inv         ItemReader
	userID      int64
	lists       []grocery.GroceryList
	itemsByList map[int64][]grocery.GroceryListItem
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
	page        int32
	pageSize    int32
	total       int32
}

func (r *groceryListPageResolver) Items() []*groceryListResolver {
	out := make([]*groceryListResolver, len(r.lists))
	for i := range r.lists {
		out[i] = &groceryListResolver{g: r.g, inv: r.inv, userID: r.userID, list: r.lists[i], items: r.itemsByList[r.lists[i].GroceryListID], catItems: r.items, units: r.units, ingredients: r.ingredients, ch: r.ch}
	}
	return out
}

func (r *groceryListPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type addGroceryItemInput struct {
	GroceryListID  graphql.ID
	ItemID         *graphql.ID
	IngredientID   *graphql.ID
	ManualItemName *string
	Quantity       float64
	Unit           string
}
