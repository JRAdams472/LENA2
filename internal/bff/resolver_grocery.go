package bff

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
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
	// usualItems maps ingredient_id -> usual brand item_id for the
	// household, used by the GroceryListItem.usualBrand field.
	usualItems map[int64]int64
	ch         *itemChildren
}

// loadGroceryChildren batch-loads list items and every catalog row they
// reference — items, units, ingredients, usual brands, and per-item
// children.
func loadGroceryChildren(ctx context.Context, g GroceryService, inv ItemReader, id IdentityService, up UserPrefsService, householdID int64, listIDs []int64) (*groceryChildren, error) {
	gc := &groceryChildren{
		itemsByList: make(map[int64][]grocery.GroceryListItem),
		items:       make(map[int64]inventory.Item),
		units:       make(map[int64]inventory.Unit),
		ingredients: make(map[int64]inventory.Ingredient),
		usualItems:  make(map[int64]int64),
	}
	if len(listIDs) == 0 {
		return gc, nil
	}
	listItems, err := g.ListGroceryListItemsByLists(ctx, listIDs, householdID)
	if err != nil {
		return nil, err
	}
	for _, it := range listItems {
		gc.itemsByList[it.GroceryListID] = append(gc.itemsByList[it.GroceryListID], it)
	}
	ingredientIDs := distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.IngredientID })
	// Usual brands for ingredient lines join the catalog load so the
	// usualBrand resolver never queries per line.
	if len(ingredientIDs) > 0 {
		usuals, err := inv.GetUsualItemsForIngredients(ctx, householdID, ingredientIDs)
		if err != nil {
			return nil, err
		}
		for ingredientID, u := range usuals {
			gc.usualItems[ingredientID] = u.ItemID
		}
	}
	itemIDSet := make(map[int64]bool)
	for _, id := range distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.ItemID }) {
		itemIDSet[id] = true
	}
	for _, id := range gc.usualItems {
		itemIDSet[id] = true
	}
	itemIDs := make([]int64, 0, len(itemIDSet))
	for id := range itemIDSet {
		itemIDs = append(itemIDs, id)
	}
	gc.items, err = loadItems(ctx, inv, itemIDs)
	if err != nil {
		return nil, err
	}
	gc.units, err = loadUnits(ctx, inv, distinctIDs(listItems, func(it grocery.GroceryListItem) *int64 { return it.UnitID }))
	if err != nil {
		return nil, err
	}
	gc.ingredients, err = loadIngredients(ctx, inv, ingredientIDs)
	if err != nil {
		return nil, err
	}
	itemList := make([]inventory.Item, 0, len(gc.items))
	for _, it := range gc.items {
		itemList = append(itemList, it)
	}
	gc.ch, err = loadItemChildren(ctx, inv, id, up, itemList, householdID, ingredientIDs)
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
	list, err := r.GroceryService.GetGroceryListByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	// Single-list reads preload the same children as the list page so
	// nested resolvers never fall back to per-row queries.
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, r.IdentityService, r.UserPrefsService, u.HouseholdID, []int64{id})
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, householdID: u.HouseholdID, list: list, items: gc.itemsByList[id], catItems: gc.items, units: gc.units, ingredients: gc.ingredients, usualItems: gc.usualItems, ch: gc.ch}, nil
}

// GroceryLists resolves the current household's grocery lists.
func (r *Resolver) GroceryLists(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*groceryListPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)
	lists, err := r.GroceryService.ListGroceryLists(ctx, u.HouseholdID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.GroceryService.CountGroceryLists(ctx, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, r.IdentityService, r.UserPrefsService, u.HouseholdID, distinctIDs(lists, func(l grocery.GroceryList) *int64 { return &l.GroceryListID }))
	if err != nil {
		return nil, err
	}
	return &groceryListPageResolver{g: r.GroceryService, inv: r.InventoryService, householdID: u.HouseholdID, lists: lists, itemsByList: gc.itemsByList, items: gc.items, units: gc.units, ingredients: gc.ingredients, usualItems: gc.usualItems, ch: gc.ch, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
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
	// to it — otherwise the mutation is an existence oracle on
	// users' plan IDs via the FK error.
	if _, err := r.MealPlanService.GetMealPlanByID(ctx, mealPlanID, u.HouseholdID); err != nil {
		return nil, err
	}

	var list grocery.GroceryList
	var needs []groceryNeed
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		list, needs, err = r.generateGroceryListTx(ctx, u, mealPlanID)
		return err
	}); err != nil {
		return nil, err
	}
	r.recordGroceryNeedEvents(u, needs)
	// Preload the freshly generated list's children so nested resolvers
	// never fall back to per-row queries.
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, r.IdentityService, r.UserPrefsService, u.HouseholdID, []int64{list.GroceryListID})
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, householdID: u.HouseholdID, list: list, items: gc.itemsByList[list.GroceryListID], catItems: gc.items, units: gc.units, ingredients: gc.ingredients, usualItems: gc.usualItems, ch: gc.ch}, nil
}

// generateGroceryListTx is the transactional core of GenerateGroceryList:
// resolve or create the plan's list, expand it into needs, subtract pantry
// stock and write the generated lines.
func (r *Resolver) generateGroceryListTx(ctx context.Context, u currentuser.User, mealPlanID int64) (grocery.GroceryList, []groceryNeed, error) {
	// Regenerate-in-place: a second generation for the same plan replaces
	// the latest list's generated lines instead of stacking up a duplicate
	// list.
	list, regenerate, err := r.planListTarget(ctx, u, mealPlanID)
	if err != nil {
		return grocery.GroceryList{}, nil, err
	}
	needs, err := r.planGroceryNeeds(ctx, mealPlanID, u.HouseholdID)
	if err != nil {
		return grocery.GroceryList{}, nil, err
	}
	if len(needs) == 0 {
		if regenerate {
			// The plan now contributes nothing — still clear the
			// previously generated lines so the list reflects it.
			_, err = r.GroceryService.ReplaceGeneratedItems(ctx, list.GroceryListID, u.HouseholdID, nil, u.Email)
			return list, nil, err
		}
		return list, nil, nil
	}
	if err := r.writeGeneratedLines(ctx, list, u, needs, regenerate); err != nil {
		return grocery.GroceryList{}, nil, err
	}
	return list, needs, nil
}

// planListTarget returns the plan's existing latest list (in-place
// regeneration) or creates a fresh one linked to the plan.
func (r *Resolver) planListTarget(ctx context.Context, u currentuser.User, mealPlanID int64) (grocery.GroceryList, bool, error) {
	existing, err := r.GroceryService.GetLatestGroceryListByPlan(ctx, mealPlanID, u.HouseholdID)
	switch {
	case err == nil:
		return existing, true, nil
	case errors.Is(err, domainerr.ErrNotFound):
		created, cerr := r.GroceryService.CreateGroceryList(ctx, u.HouseholdID, &mealPlanID, u.Email)
		return created, false, cerr
	default:
		return grocery.GroceryList{}, false, err
	}
}

// planGroceryNeeds expands the plan's slots into aggregated buy needs.
func (r *Resolver) planGroceryNeeds(ctx context.Context, mealPlanID, householdID int64) ([]groceryNeed, error) {
	slots, err := r.MealPlanService.ListMealSlotsForPlan(ctx, mealPlanID, householdID)
	if err != nil {
		return nil, err
	}
	slotItems, err := r.MealPlanService.ListMealSlotItemsByPlan(ctx, mealPlanID, householdID)
	if err != nil {
		return nil, err
	}
	recipes, recipeItems, err := r.planRecipes(ctx, householdID, slots)
	if err != nil {
		return nil, err
	}
	return aggregateGroceryNeeds(slots, slotItems, recipes, recipeItems), nil
}

// writeGeneratedLines subtracts pantry stock from needs — each entry
// converted into the need's unit when the kinds allow — and writes the
// remaining quantities as generated lines, replacing the list's prior
// generated lines on regeneration or appending on a fresh list.
func (r *Resolver) writeGeneratedLines(ctx context.Context, list grocery.GroceryList, u currentuser.User, needs []groceryNeed, regenerate bool) error {
	stock, err := r.householdPantryStock(ctx, u.HouseholdID)
	if err != nil {
		return err
	}
	unitSet := make(map[int64]bool)
	for _, n := range needs {
		unitSet[n.unitID] = true
	}
	for _, e := range stock {
		unitSet[e.unitID] = true
	}
	unitsByID, err := loadUnits(ctx, r.InventoryService, sortedIDs(unitSet))
	if err != nil {
		return err
	}
	lines := groceryNeedLines(list.GroceryListID, needs, stock, unitsByID)
	if regenerate {
		_, err = r.GroceryService.ReplaceGeneratedItems(ctx, list.GroceryListID, u.HouseholdID, lines, u.Email)
	} else {
		_, err = r.GroceryService.AddGroceryListItems(ctx, lines, u.HouseholdID, u.Email)
	}
	return err
}

// recordGroceryNeedEvents logs a grocery signal per generated line's
// entity — ingredient first, else the preferred brand item.
func (r *Resolver) recordGroceryNeedEvents(u currentuser.User, needs []groceryNeed) {
	// Generated lines are planned usage — each entity on the list is a
	// grocery signal for ranking.
	for _, n := range needs {
		e := analytics.Event{EventType: analytics.EventGroceryItemAdded}
		switch {
		case n.ingredientID != nil:
			e.EntityType = analytics.EntityIngredient
			e.EntityID = *n.ingredientID
		case n.itemID != nil:
			e.EntityType = analytics.EntityItem
			e.EntityID = *n.itemID
		default:
			continue
		}
		r.recordEventAsync(u.UserID, u.Email, e)
	}
}

// planLine is one expanded plan contribution: a quantity in a specific
// unit keyed by ingredient when the recipe/slot line carries one, with
// itemID retained as the preferred-brand hint.
type planLine struct {
	itemID       *int64
	ingredientID *int64
	unitID       int64
	qty          float64
}

// lineKey identifies the thing a plan line contributes: the ingredient
// when present, else the branded item.
func lineKey(l planLine) int64 {
	if l.ingredientID != nil {
		return -*l.ingredientID
	}
	if l.itemID != nil {
		return *l.itemID
	}
	return 0
}

// expandPlanLines expands a plan's slots into quantity lines. Explicit
// slot items always count; a slot item flagged is_from_recipe replaces
// the recipe's own line for that ingredient (or item when brand-only),
// and recipe contributions scale by the slot's servings ratio. Shared by
// grocery generation and nutrition aggregation.
// planExpansion indexes the plan's inputs for expansion: slot items by
// slot, recipes by ID, and recipe items by recipe.
type planExpansion struct {
	itemsBySlot   map[int64][]mealplan.MealSlotItem
	recipesByID   map[int64]recipe.Recipe
	itemsByRecipe map[int64][]recipe.RecipeItem
}

func newPlanExpansion(slotItems []mealplan.MealSlotItem, recipes []recipe.Recipe, recipeItems []recipe.RecipeItem) planExpansion {
	e := planExpansion{
		itemsBySlot:   make(map[int64][]mealplan.MealSlotItem),
		recipesByID:   make(map[int64]recipe.Recipe, len(recipes)),
		itemsByRecipe: make(map[int64][]recipe.RecipeItem),
	}
	for _, si := range slotItems {
		e.itemsBySlot[si.SlotID] = append(e.itemsBySlot[si.SlotID], si)
	}
	for _, rec := range recipes {
		e.recipesByID[rec.RecipeID] = rec
	}
	for _, ri := range recipeItems {
		e.itemsByRecipe[ri.RecipeID] = append(e.itemsByRecipe[ri.RecipeID], ri)
	}
	return e
}

// expandSlotItems returns the slot's explicit lines plus the recipe-line
// keys those lines override (is_from_recipe slot items replace the
// recipe's own line for that ingredient or item).
func expandSlotItems(items []mealplan.MealSlotItem) (lines []planLine, overridden map[int64]bool) {
	overridden = make(map[int64]bool)
	for _, si := range items {
		l := planLine{itemID: si.ItemID, ingredientID: si.IngredientID, unitID: si.UnitID, qty: si.Quantity}
		if l.itemID == nil && l.ingredientID == nil {
			continue
		}
		lines = append(lines, l)
		if si.IsFromRecipe {
			// Suppress the recipe line by both keys: a brand-only
			// override still covers an ingredient-keyed recipe line
			// naming the same preferred item, and vice versa.
			overridden[lineKey(l)] = true
			if l.itemID != nil {
				overridden[*l.itemID] = true
			}
			if l.ingredientID != nil {
				overridden[-*l.ingredientID] = true
			}
		}
	}
	return lines, overridden
}

// expandSlotRecipeLines scales the slot's recipe lines by the servings
// ratio and drops lines overridden by explicit slot items.
func (e planExpansion) expandSlotRecipeLines(slot mealplan.MealSlot, overridden map[int64]bool) []planLine {
	if slot.RecipeID == nil {
		return nil
	}
	rec, ok := e.recipesByID[*slot.RecipeID]
	if !ok || rec.Servings == nil || *rec.Servings <= 0 {
		return nil
	}
	scale := 1.0
	if slot.Servings != nil {
		scale = float64(*slot.Servings) / float64(*rec.Servings)
	}
	var lines []planLine
	for _, ri := range e.itemsByRecipe[rec.RecipeID] {
		l := planLine{itemID: ri.ItemID, ingredientID: ri.IngredientID, unitID: ri.UnitID, qty: ri.Quantity * scale}
		if l.itemID == nil && l.ingredientID == nil {
			continue
		}
		if overridden[lineKey(l)] || (l.itemID != nil && overridden[*l.itemID]) {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func expandPlanLines(slots []mealplan.MealSlot, slotItems []mealplan.MealSlotItem, recipes []recipe.Recipe, recipeItems []recipe.RecipeItem) []planLine {
	e := newPlanExpansion(slotItems, recipes, recipeItems)
	var lines []planLine
	for _, slot := range slots {
		slotLines, overridden := expandSlotItems(e.itemsBySlot[slot.SlotID])
		lines = append(lines, slotLines...)
		lines = append(lines, e.expandSlotRecipeLines(slot, overridden)...)
	}
	return lines
}

// groceryNeed is a unit-resolved quantity to buy, keyed by ingredient when
// the contributing lines carried one; itemID is the preferred-brand hint.
type groceryNeed struct {
	ingredientID *int64
	itemID       *int64
	unitID       int64
	qty          float64
}

// aggregateGroceryNeeds groups expanded plan lines into per-(ingredient
// or item, unit) totals in deterministic order. Ingredient-keyed lines
// merge regardless of preferred brand; the first preferred brand seen
// rides along as the line's item hint.
// groceryNeedKey identifies one aggregated need bucket: ingredient-keyed
// when any contributing line carried an ingredient (ingredientID != 0),
// else brand-only.
type groceryNeedKey struct {
	ingredientID int64
	itemID       int64
	unitID       int64
}

// groceryNeedKeyFor maps a plan line onto its aggregation bucket.
func groceryNeedKeyFor(l planLine) groceryNeedKey {
	if l.ingredientID != nil {
		return groceryNeedKey{ingredientID: *l.ingredientID, unitID: l.unitID}
	}
	return groceryNeedKey{itemID: *l.itemID, unitID: l.unitID}
}

// accumulateGroceryNeeds folds expanded lines into per-bucket totals.
// Ingredient-keyed lines merge regardless of preferred brand; the first
// preferred brand seen rides along as the line's item hint.
func accumulateGroceryNeeds(lines []planLine) (map[groceryNeedKey]float64, map[groceryNeedKey]*int64) {
	totals := make(map[groceryNeedKey]float64)
	preferred := make(map[groceryNeedKey]*int64)
	for _, l := range lines {
		k := groceryNeedKeyFor(l)
		if l.ingredientID != nil && l.itemID != nil {
			if _, ok := preferred[k]; !ok {
				v := *l.itemID
				preferred[k] = &v
			}
		}
		totals[k] += l.qty
	}
	return totals, preferred
}

// sortGroceryNeedKeys orders the buckets deterministically: ingredient,
// then item, then unit.
func sortGroceryNeedKeys(keys []groceryNeedKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ingredientID != keys[j].ingredientID {
			return keys[i].ingredientID < keys[j].ingredientID
		}
		if keys[i].itemID != keys[j].itemID {
			return keys[i].itemID < keys[j].itemID
		}
		return keys[i].unitID < keys[j].unitID
	})
}

// groceryNeedFor materializes one need from a bucket.
func groceryNeedFor(k groceryNeedKey, totals map[groceryNeedKey]float64, preferred map[groceryNeedKey]*int64) groceryNeed {
	n := groceryNeed{unitID: k.unitID, qty: totals[k]}
	if k.ingredientID != 0 {
		v := k.ingredientID
		n.ingredientID = &v
		n.itemID = preferred[k]
	} else {
		v := k.itemID
		n.itemID = &v
	}
	return n
}

func aggregateGroceryNeeds(slots []mealplan.MealSlot, slotItems []mealplan.MealSlotItem, recipes []recipe.Recipe, recipeItems []recipe.RecipeItem) []groceryNeed {
	totals, preferred := accumulateGroceryNeeds(expandPlanLines(slots, slotItems, recipes, recipeItems))
	keys := make([]groceryNeedKey, 0, len(totals))
	for k := range totals {
		keys = append(keys, k)
	}
	sortGroceryNeedKeys(keys)
	out := make([]groceryNeed, 0, len(keys))
	for _, k := range keys {
		out = append(out, groceryNeedFor(k, totals, preferred))
	}
	return out
}

// pantryStockEntry is one stocked household item: quantity in the item's
// canonical unit plus its resolved (override-aware) ingredient, which may
// be nil for unlinked items.
type pantryStockEntry struct {
	itemID       int64
	ingredientID *int64
	unitID       int64
	qty          float64
}

// householdPantryStock returns the household's stocked items with each
// item's canonical unit and resolved ingredient so needs can be covered
// by either an exact brand match or any brand of the same ingredient.
func (r *Resolver) householdPantryStock(ctx context.Context, householdID int64) ([]pantryStockEntry, error) {
	byItem := make(map[int64]float64)
	const page int32 = 1000
	for offset := int32(0); ; offset += page {
		rows, err := r.UserPrefsService.ListHouseholdItems(ctx, householdID, page, offset)
		if err != nil {
			return nil, err
		}
		for _, ui := range rows {
			byItem[ui.ItemID] += ui.CurrentQty
		}
		if len(rows) < int(page) {
			break
		}
	}
	itemIDs := make([]int64, 0, len(byItem))
	for id := range byItem {
		itemIDs = append(itemIDs, id)
	}
	if len(itemIDs) == 0 {
		return nil, nil
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
	items, err := r.InventoryService.GetItemsByIDs(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	resolved, err := r.InventoryService.ResolveItemIngredients(ctx, householdID, itemIDs)
	if err != nil {
		return nil, err
	}
	out := make([]pantryStockEntry, 0, len(items))
	for _, it := range items {
		out = append(out, pantryStockEntry{
			itemID:       it.ItemID,
			ingredientID: resolved[it.ItemID],
			unitID:       it.UnitID,
			qty:          byItem[it.ItemID],
		})
	}
	return out, nil
}

// groceryNeedLines subtracts pantry stock — each entry converted into the
// need's unit when the kinds allow — and returns the remaining quantities
// as grocery list rows. Ingredient needs draw from every stocked brand
// resolving to that ingredient; brand-only needs match the item itself.
// Fully covered needs produce no line.
// stockCoversNeed reports whether a pantry entry can satisfy the need:
// ingredient needs draw from every stocked brand resolving to that
// ingredient; brand-only needs match the item itself.
func stockCoversNeed(n groceryNeed, e pantryStockEntry) bool {
	if n.ingredientID != nil {
		return e.ingredientID != nil && *e.ingredientID == *n.ingredientID
	}
	return n.itemID != nil && e.itemID == *n.itemID
}

// pantryCover subtracts matching stock — each entry converted into the
// need's unit when the kinds allow — and returns the uncovered remainder.
func pantryCover(n groceryNeed, stock []pantryStockEntry, units map[int64]inventory.Unit) float64 {
	remaining := n.qty
	for _, e := range stock {
		if remaining <= 0 {
			break
		}
		if !stockCoversNeed(n, e) {
			continue
		}
		from, okFrom := units[e.unitID]
		to, okTo := units[n.unitID]
		if !okFrom || !okTo {
			continue
		}
		if converted, ok := inventory.ConvertQuantity(e.qty, from, to); ok {
			remaining -= converted
		}
	}
	return remaining
}

func groceryNeedLines(listID int64, needs []groceryNeed, stock []pantryStockEntry, units map[int64]inventory.Unit) []grocery.GroceryListItem {
	out := make([]grocery.GroceryListItem, 0, len(needs))
	for _, n := range needs {
		remaining := pantryCover(n, stock, units)
		if remaining <= 0 {
			continue
		}
		unitID := n.unitID
		out = append(out, grocery.GroceryListItem{
			GroceryListID:  listID,
			ItemID:         n.itemID,
			IngredientID:   n.ingredientID,
			QuantityNeeded: remaining,
			UnitID:         &unitID,
			Source:         "mealplan",
		})
	}
	return out
}

// checkCreditItem picks the branded item a check-off credits: the line's
// bound item when present, else the household's usual brand for the line's
// ingredient. Nil when nothing applies (first-time ingredient line with
// no usual — the client should offer checkGroceryItemWithBrand).
func (r *Resolver) checkCreditItem(ctx context.Context, householdID int64, it grocery.GroceryListItem) (*int64, error) {
	if it.ItemID != nil {
		return it.ItemID, nil
	}
	if it.IngredientID == nil {
		return nil, nil
	}
	usual, err := r.InventoryService.GetUsualItemForIngredient(ctx, householdID, *it.IngredientID)
	if err != nil {
		return nil, err
	}
	if usual == nil {
		return nil, nil
	}
	return &usual.ItemID, nil
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
		toggled, err := r.GroceryService.ToggleGroceryListItemChecked(ctx, id, u.HouseholdID, u.Email)
		if err != nil {
			return err
		}
		creditID, err := r.checkCreditItem(ctx, u.HouseholdID, toggled)
		if err != nil {
			return err
		}
		if creditID != nil {
			delta := toggled.QuantityNeeded
			if !toggled.IsChecked {
				delta = -delta
			}
			if _, err := r.UserPrefsService.AdjustHouseholdItemQuantity(ctx, u.HouseholdID, *creditID, delta, u.Email); err != nil {
				return err
			}
			// A successful usual-brand check-off refreshes the record.
			if toggled.IsChecked && toggled.IngredientID != nil && toggled.ItemID == nil {
				if err := r.InventoryService.SetUsualItemForIngredient(ctx, u.HouseholdID, *toggled.IngredientID, *creditID, u.Email); err != nil {
					return err
				}
			}
		}
		updated = toggled
		return nil
	}); err != nil {
		return nil, err
	}
	// A check-off is real purchase intent (unchecking records nothing).
	if updated.IsChecked {
		r.recordEventAsync(u.UserID, u.Email, groceryEntityEvent(analytics.EventGroceryItemChecked, updated))
	}
	return &groceryListItemResolver{inv: r.InventoryService, householdID: u.HouseholdID, item: updated, as: r.allergySrc(u)}, nil
}

// CheckGroceryItemWithBrand is the first-time brand pick for an ingredient
// line: binds the chosen item to the line, checks it off, credits pantry
// stock, and records the item as the household's usual brand so future
// check-offs of this ingredient auto-credit.
func (r *Resolver) CheckGroceryItemWithBrand(ctx context.Context, args struct {
	GroceryListItemID graphql.ID
	ItemID            graphql.ID
}) (*groceryListItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	listItemID, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	// The brand must be a real catalog item before it's bound or credited.
	if _, err := r.InventoryService.GetItemByID(ctx, itemID); err != nil {
		return nil, err
	}

	var updated grocery.GroceryListItem
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		line, err := r.GroceryService.GetGroceryListItemByID(ctx, listItemID, u.HouseholdID)
		if err != nil {
			return err
		}
		line.ItemID = &itemID
		if err := r.GroceryService.UpdateGroceryListItem(ctx, listItemID, u.HouseholdID, line, u.Email); err != nil {
			return err
		}
		if !line.IsChecked {
			toggled, err := r.GroceryService.ToggleGroceryListItemChecked(ctx, listItemID, u.HouseholdID, u.Email)
			if err != nil {
				return err
			}
			line = toggled
			if _, err := r.UserPrefsService.AdjustHouseholdItemQuantity(ctx, u.HouseholdID, itemID, line.QuantityNeeded, u.Email); err != nil {
				return err
			}
		}
		if line.IngredientID != nil {
			if err := r.InventoryService.SetUsualItemForIngredient(ctx, u.HouseholdID, *line.IngredientID, itemID, u.Email); err != nil {
				return err
			}
		}
		updated = line
		return nil
	}); err != nil {
		return nil, err
	}
	if updated.IsChecked {
		r.recordEventAsync(u.UserID, u.Email, groceryEntityEvent(analytics.EventGroceryItemChecked, updated))
	}
	return &groceryListItemResolver{inv: r.InventoryService, householdID: u.HouseholdID, item: updated, as: r.allergySrc(u)}, nil
}

// groceryEntityEvent maps a grocery line to an analytics event: catalog
// item and ingredient IDs become entity references; a manual line records
// its name as a search term so repeat manual adds still build signal.
func groceryEntityEvent(eventType string, it grocery.GroceryListItem) analytics.Event {
	e := analytics.Event{EventType: eventType}
	switch {
	case it.ItemID != nil:
		e.EntityType = analytics.EntityItem
		e.EntityID = *it.ItemID
	case it.IngredientID != nil:
		e.EntityType = analytics.EntityIngredient
		e.EntityID = *it.IngredientID
	default:
		e.SearchTerm = it.ManualItemName
	}
	return e
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
	if err := r.GroceryService.DeleteGroceryListItem(ctx, id, u.HouseholdID); err != nil {
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
	it, err := r.GroceryService.AddGroceryListItem(ctx, item, u.HouseholdID, u.Email)
	if err != nil {
		return nil, err
	}
	r.recordEventAsync(u.UserID, u.Email, groceryEntityEvent(analytics.EventGroceryItemAdded, it))
	return &groceryListItemResolver{inv: r.InventoryService, householdID: u.HouseholdID, item: it, as: r.allergySrc(u)}, nil
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
	source := strings.ToLower(strings.TrimSpace(derefString(in.Source)))
	switch source {
	case "":
		source = "manual"
	case "mealplan", "recipe", "pantry", "manual":
	default:
		return grocery.GroceryListItem{}, badInputf("unknown grocery item source %q", derefString(in.Source))
	}
	return grocery.GroceryListItem{
		GroceryListID:  groceryListID,
		ItemID:         itemID,
		IngredientID:   ingredientID,
		ManualItemName: manualName,
		QuantityNeeded: in.Quantity,
		UnitID:         unitID,
		Source:         source,
	}, nil
}

// groceryListResolver resolves GroceryList fields. When items is non-nil
// the batch-loaded rows and catalog lookups are used instead of per-list
// service calls.
type groceryListResolver struct {
	g           GroceryService
	inv         ItemReader
	householdID int64
	list        grocery.GroceryList
	items       []grocery.GroceryListItem
	catItems    map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	usualItems  map[int64]int64
	ch          *itemChildren
}

func (r *groceryListResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.list.GroceryListID, 10))
}

func (r *groceryListResolver) GeneratedAt() graphql.Time {
	return graphqlTime(r.list.GeneratedAt)
}

// Store resolves the store selected to route this list.
func (r *groceryListResolver) Store(ctx context.Context) (*storeResolver, error) {
	if r.list.StoreID == nil {
		return nil, nil
	}
	st, err := r.g.GetStoreByID(ctx, *r.list.StoreID, r.householdID)
	if err != nil {
		if errors.Is(err, domainerr.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &storeResolver{g: r.g, householdID: r.householdID, store: st}, nil
}

func (r *groceryListResolver) Items(ctx context.Context) ([]*groceryListItemResolver, error) {
	var items []grocery.GroceryListItem
	if r.ch != nil {
		items = r.items
	} else {
		slog.Default().Warn("groceryList.items missed preload; lazy-loading", "grocery_list_id", r.list.GroceryListID)
		var err error
		items, err = r.g.ListGroceryListItems(ctx, r.list.GroceryListID, r.householdID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*groceryListItemResolver, len(items))
	for i := range items {
		out[i] = &groceryListItemResolver{inv: r.inv, householdID: r.householdID, item: items[i], items: r.catItems, units: r.units, ingredients: r.ingredients, usualItems: r.usualItems, ch: r.ch, as: asOfItemChildren(r.ch)}
	}
	return out, nil
}

// groceryListItemResolver resolves GroceryListItem fields.
type groceryListItemResolver struct {
	inv         ItemReader
	householdID int64
	item        grocery.GroceryListItem
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	usualItems  map[int64]int64
	ch          *itemChildren
	as          *allergySource
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
		return &ingredientResolver{inv: r.inv, in: in, as: r.lazySource()}, nil
	}
	slog.Default().Warn("groceryListItem.ingredient missed preload; lazy-loading", "ingredient_id", *r.item.IngredientID)
	in, err := r.inv.GetIngredientByID(ctx, *r.item.IngredientID)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.inv, in: in, as: r.lazySource()}, nil
}

func (r *groceryListItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	return r.catalogItem(ctx, r.item.ItemID)
}

// UsualBrand resolves the household's usual brand for this line's
// ingredient — the item check-offs credit automatically once recorded.
func (r *groceryListItemResolver) UsualBrand(ctx context.Context) (*itemResolver, error) {
	if r.item.IngredientID == nil {
		return nil, nil
	}
	var itemID *int64
	if r.usualItems != nil {
		if id, ok := r.usualItems[*r.item.IngredientID]; ok {
			itemID = &id
		}
	} else {
		slog.Default().Warn("groceryListItem.usualBrand missed preload; lazy-loading", "ingredient_id", *r.item.IngredientID)
		householdID := r.householdID
		if householdID == 0 {
			u, ok := currentuser.FromContext(ctx)
			if !ok {
				return nil, errForbidden()
			}
			householdID = u.HouseholdID
		}
		usual, err := r.inv.GetUsualItemForIngredient(ctx, householdID, *r.item.IngredientID)
		if err != nil {
			return nil, err
		}
		if usual == nil {
			return nil, nil
		}
		itemID = &usual.ItemID
	}
	return r.catalogItem(ctx, itemID)
}

// lazySource prefers the preloaded source and falls back to the resolver's
// own for children built lazily.
func (r *groceryListItemResolver) lazySource() *allergySource {
	return firstSource(asOfItemChildren(r.ch), r.as)
}

// catalogItem renders an item ID through the preloaded maps, falling back
// to a lazy fetch when the preload missed it.
func (r *groceryListItemResolver) catalogItem(ctx context.Context, itemID *int64) (*itemResolver, error) {
	if itemID == nil {
		return nil, nil
	}
	if r.items != nil {
		it, ok := r.items[*itemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch, as: r.lazySource()}, nil
	}
	slog.Default().Warn("groceryListItem.item missed preload; lazy-loading", "item_id", *itemID)
	it, err := r.inv.GetItemByID(ctx, *itemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it, as: r.lazySource()}, nil
}

// allergenCtx returns the preloaded context or builds one for this line.
func (r *groceryListItemResolver) allergenCtx(ctx context.Context) (*allergyContext, error) {
	if r.ch != nil && r.ch.ac != nil {
		return r.ch.ac, nil
	}
	var ingredientIDs, itemIDs []int64
	if r.item.IngredientID != nil {
		ingredientIDs = append(ingredientIDs, *r.item.IngredientID)
	}
	if r.item.ItemID != nil {
		itemIDs = append(itemIDs, *r.item.ItemID)
	}
	return lazyAllergenCtx(ctx, r.lazySource(), ingredientIDs, itemIDs)
}

func (r *groceryListItemResolver) allergenSet(ctx context.Context) (*allergyContext, map[int64]string, error) {
	ac, err := r.allergenCtx(ctx)
	if err != nil {
		return nil, nil, err
	}
	return ac, ac.lineSet(r.item.IngredientID, r.item.ItemID), nil
}

// Allergens resolves the line's merged flags: its ingredient's flags
// unioned with the bound item's product-level flags.
func (r *groceryListItemResolver) Allergens(ctx context.Context) ([]*allergenFlagResolver, error) {
	ac, set, err := r.allergenSet(ctx)
	if err != nil {
		return nil, err
	}
	return ac.flagResolvers(set), nil
}

// AllergyWarnings resolves which household members conflict with this
// grocery line.
func (r *groceryListItemResolver) AllergyWarnings(ctx context.Context) ([]*allergyWarningResolver, error) {
	ac, set, err := r.allergenSet(ctx)
	if err != nil {
		return nil, err
	}
	return ac.warningResolvers(set), nil
}

type groceryListPageResolver struct {
	g           GroceryService
	inv         ItemReader
	householdID int64
	lists       []grocery.GroceryList
	itemsByList map[int64][]grocery.GroceryListItem
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	usualItems  map[int64]int64
	ch          *itemChildren
	page        int32
	pageSize    int32
	total       int32
}

func (r *groceryListPageResolver) Items() []*groceryListResolver {
	out := make([]*groceryListResolver, len(r.lists))
	for i := range r.lists {
		out[i] = &groceryListResolver{g: r.g, inv: r.inv, householdID: r.householdID, list: r.lists[i], items: r.itemsByList[r.lists[i].GroceryListID], catItems: r.items, units: r.units, ingredients: r.ingredients, usualItems: r.usualItems, ch: r.ch}
	}
	return out
}

func (r *groceryListPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// addGroceryItemInput mirrors AddGroceryItemInput. Source is optional
// provenance (defaults to "manual"); the UI tags restock adds as "pantry".
type addGroceryItemInput struct {
	GroceryListID  graphql.ID
	ItemID         *graphql.ID
	IngredientID   *graphql.ID
	ManualItemName *string
	Quantity       float64
	Unit           string
	Source         *string
}

// SuggestedRestockItems surfaces pantry items that have fallen to or below
// their minimum quantity, ranked by household engagement and excluding
// anything already on the latest grocery list. Engagement comes from the
// synchronous household selection counts so an item added moments ago can
// appear immediately (the decayed rollup only refreshes on the decay job).
func (r *Resolver) SuggestedRestockItems(ctx context.Context, args struct {
	Limit int32
}) ([]*itemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	limit := clamp(args.Limit, 1, 50)

	candidateIDs, err := r.restockCandidates(ctx, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	engaged := r.engagementRanked(ctx, u.HouseholdID, candidateIDs)
	if int(limit) < len(engaged) {
		engaged = engaged[:int(limit)]
	}
	if len(engaged) == 0 {
		return []*itemResolver{}, nil
	}
	return r.approvedItemResolvers(ctx, u, engaged)
}

// lowStockItemIDs returns pantry items at or below their minimum quantity.
func (r *Resolver) lowStockItemIDs(ctx context.Context, householdID int64) ([]int64, error) {
	pantry, err := r.UserPrefsService.ListHouseholdItems(ctx, householdID, 5000, 0)
	if err != nil {
		return nil, err
	}
	var lowIDs []int64
	for _, hi := range pantry {
		if hi.MinQty != nil && hi.CurrentQty <= *hi.MinQty {
			lowIDs = append(lowIDs, hi.ItemID)
		}
	}
	return lowIDs, nil
}

// restockCandidates drops low-stock items already on the household's
// latest grocery list — the latest list is "active", so anything on it is
// being shopped for and isn't a suggestion.
func (r *Resolver) restockCandidates(ctx context.Context, householdID int64) ([]int64, error) {
	lowIDs, err := r.lowStockItemIDs(ctx, householdID)
	if err != nil || len(lowIDs) == 0 {
		return nil, err
	}
	lists, err := r.GroceryService.ListGroceryLists(ctx, householdID, 1, 0)
	if err != nil {
		return nil, err
	}
	onList := map[int64]bool{}
	if len(lists) > 0 {
		listItems, err := r.GroceryService.ListGroceryListItems(ctx, lists[0].GroceryListID, householdID)
		if err != nil {
			return nil, err
		}
		for _, li := range listItems {
			if li.ItemID != nil {
				onList[*li.ItemID] = true
			}
		}
	}
	var candidateIDs []int64
	for _, id := range lowIDs {
		if !onList[id] {
			candidateIDs = append(candidateIDs, id)
		}
	}
	return candidateIDs, nil
}

// engagementRanked keeps candidates with household selection signal,
// sorted by signal strength with an ID tiebreak. An analytics failure
// degrades to an empty result rather than failing the query.
func (r *Resolver) engagementRanked(ctx context.Context, householdID int64, candidateIDs []int64) []int64 {
	if len(candidateIDs) == 0 {
		return nil
	}
	counts := map[int64]int64{}
	if r.AnalyticsService != nil {
		if c, err := r.AnalyticsService.HouseholdSelectionCounts(ctx, householdID, analytics.EntityItem, candidateIDs); err == nil {
			counts = c
		}
	}
	var engaged []int64
	for _, id := range candidateIDs {
		if counts[id] > 0 {
			engaged = append(engaged, id)
		}
	}
	sort.Slice(engaged, func(i, j int) bool {
		if counts[engaged[i]] != counts[engaged[j]] {
			return counts[engaged[i]] > counts[engaged[j]]
		}
		return engaged[i] < engaged[j]
	})
	return engaged
}

// approvedItemResolvers loads the candidates and keeps only approved
// catalog rows — pending/rejected submissions aren't restock suggestions.
func (r *Resolver) approvedItemResolvers(ctx context.Context, u currentuser.User, ids []int64) ([]*itemResolver, error) {
	items, err := r.InventoryService.GetItemsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]inventory.Item, len(items))
	for _, it := range items {
		byID[it.ItemID] = it
	}
	out := make([]*itemResolver, 0, len(ids))
	for _, id := range ids {
		if it, ok := byID[id]; ok && it.Status == inventory.ItemStatusApproved {
			out = append(out, &itemResolver{inv: r.InventoryService, it: it, as: r.allergySrc(u)})
		}
	}
	return out, nil
}
