package bff

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/graph-gophers/graphql-go"
)

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
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, userID: u.UserID, list: list}, nil
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
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	lists, err := r.GroceryService.ListGroceryLists(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.GroceryService.CountGroceryLists(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	listIDs := distinctIDs(lists, func(l grocery.GroceryList) *int64 { return &l.GroceryListID })
	itemsByList := make(map[int64][]grocery.GroceryListItem)
	var items map[int64]inventory.Item
	var units map[int64]inventory.Unit
	var ch *itemChildren
	if len(listIDs) > 0 {
		listItems, err := r.GroceryService.ListGroceryListItemsByLists(ctx, listIDs, u.UserID)
		if err != nil {
			return nil, err
		}
		for _, it := range listItems {
			itemsByList[it.GroceryListID] = append(itemsByList[it.GroceryListID], it)
		}
		items, err = loadItems(ctx, r.InventoryService, distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.ItemID }))
		if err != nil {
			return nil, err
		}
		units, err = loadUnits(ctx, r.InventoryService, distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.UnitID }))
		if err != nil {
			return nil, err
		}
		itemList := make([]inventory.Item, 0, len(items))
		for _, it := range items {
			itemList = append(itemList, it)
		}
		ch, err = loadItemChildren(ctx, r.InventoryService, itemList)
		if err != nil {
			return nil, err
		}
	}
	return &groceryListPageResolver{g: r.GroceryService, inv: r.InventoryService, userID: u.UserID, lists: lists, itemsByList: itemsByList, items: items, units: units, ch: ch, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
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

		recipeIDs := distinctIDs(slots, func(s mealplan.MealSlot) *int64 { return s.RecipeID })
		var recipes []recipe.Recipe
		var recipeItems []recipe.RecipeItem
		if len(recipeIDs) > 0 {
			recipes, err = r.RecipeService.GetRecipesByIDs(ctx, recipeIDs)
			if err != nil {
				return err
			}
			recipeItems, err = r.RecipeService.ListRecipeItemsByRecipes(ctx, recipeIDs)
			if err != nil {
				return err
			}
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
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, userID: u.UserID, list: list}, nil
}

// groceryNeed is a unit-resolved quantity of a catalog item to buy.
type groceryNeed struct {
	itemID int64
	unitID int64
	qty    float64
}

// aggregateGroceryNeeds expands a plan's slots into per-(item, unit)
// quantities. Explicit slot items always count; a slot item flagged
// is_from_recipe replaces the recipe's own line for that item, and recipe
// contributions scale by the slot's servings ratio.
func aggregateGroceryNeeds(slots []mealplan.MealSlot, slotItems []mealplan.MealSlotItem, recipes []recipe.Recipe, recipeItems []recipe.RecipeItem) []groceryNeed {
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

	type key struct{ itemID, unitID int64 }
	totals := make(map[key]float64)
	add := func(itemID, unitID int64, qty float64) {
		totals[key{itemID, unitID}] += qty
	}

	for _, slot := range slots {
		overridden := make(map[int64]bool)
		for _, si := range itemsBySlot[slot.SlotID] {
			if si.ItemID == nil {
				continue
			}
			add(*si.ItemID, si.UnitID, si.Quantity)
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
			add(ri.ItemID, ri.UnitID, ri.Quantity*scale)
		}
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
	groceryListID, err := parseID(string(args.Input.GroceryListID))
	if err != nil {
		return nil, err
	}
	var itemID *int64
	if args.Input.ItemID != nil {
		parsed, err := parseID(string(*args.Input.ItemID))
		if err != nil {
			return nil, err
		}
		itemID = &parsed
	}
	ingredientID, err := optionalID(args.Input.IngredientID)
	if err != nil {
		return nil, err
	}
	manualName := derefString(args.Input.ManualItemName)
	if itemID == nil && ingredientID == nil && strings.TrimSpace(manualName) == "" {
		return nil, badInputf("grocery item requires an itemId, ingredientId or manualItemName")
	}
	if args.Input.Quantity <= 0 {
		return nil, badInputf("quantity must be positive")
	}
	// The unit is optional for grocery items; an empty string means unset.
	var unitID *int64
	if u := strings.TrimSpace(args.Input.Unit); u != "" {
		id, err := resolveUnitID(ctx, r.InventoryService, u)
		if err != nil {
			return nil, err
		}
		unitID = &id
	}
	it, err := r.GroceryService.AddGroceryListItem(ctx, grocery.GroceryListItem{
		GroceryListID:  groceryListID,
		ItemID:         itemID,
		IngredientID:   ingredientID,
		ManualItemName: manualName,
		QuantityNeeded: args.Input.Quantity,
		UnitID:         unitID,
		Source:         "manual",
	}, u.UserID, u.Email)
	if err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: it}, nil
}

// groceryListResolver resolves GroceryList fields. When items is non-nil
// the batch-loaded rows and catalog lookups are used instead of per-list
// service calls.
type groceryListResolver struct {
	g        GroceryService
	inv      InventoryService
	userID   int64
	list     grocery.GroceryList
	items    []grocery.GroceryListItem
	catItems map[int64]inventory.Item
	units    map[int64]inventory.Unit
	ch       *itemChildren
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
		var err error
		items, err = r.g.ListGroceryListItems(ctx, r.list.GroceryListID, r.userID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*groceryListItemResolver, len(items))
	for i := range items {
		out[i] = &groceryListItemResolver{inv: r.inv, item: items[i], items: r.catItems, units: r.units, ch: r.ch}
	}
	return out, nil
}

// groceryListItemResolver resolves GroceryListItem fields.
type groceryListItemResolver struct {
	inv   InventoryService
	item  grocery.GroceryListItem
	items map[int64]inventory.Item
	units map[int64]inventory.Unit
	ch    *itemChildren
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
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type groceryListPageResolver struct {
	g           GroceryService
	inv         InventoryService
	userID      int64
	lists       []grocery.GroceryList
	itemsByList map[int64][]grocery.GroceryListItem
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ch          *itemChildren
	page        int32
	pageSize    int32
	total       int32
}

func (r *groceryListPageResolver) Items() []*groceryListResolver {
	out := make([]*groceryListResolver, len(r.lists))
	for i := range r.lists {
		out[i] = &groceryListResolver{g: r.g, inv: r.inv, userID: r.userID, list: r.lists[i], items: r.itemsByList[r.lists[i].GroceryListID], catItems: r.items, units: r.units, ch: r.ch}
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
