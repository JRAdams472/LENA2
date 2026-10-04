package bff

import (
	"context"
	"strconv"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/graph-gophers/graphql-go"
)

// GroceryStores resolves the household's stores.
func (r *Resolver) GroceryStores(ctx context.Context) ([]*storeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	stores, err := r.GroceryService.ListStores(ctx, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	out := make([]*storeResolver, len(stores))
	for i := range stores {
		out[i] = &storeResolver{g: r.GroceryService, householdID: u.HouseholdID, store: stores[i]}
	}
	return out, nil
}

// GroceryRouteGroups resolves the authoritative walk order for a grocery
// list. Both clients render this response verbatim — ordering is
// computed server-side so web and mobile can never diverge.
func (r *Resolver) GroceryRouteGroups(ctx context.Context, args struct{ GroceryListID graphql.ID }) ([]*routeGroupResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	listID, err := parseID(string(args.GroceryListID))
	if err != nil {
		return nil, err
	}
	groups, err := r.GroceryService.RouteGroups(ctx, listID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	// Batch-load catalog children for every routed item so nested
	// item/ingredient resolvers never issue per-row queries.
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, r.IdentityService, r.UserPrefsService, u.HouseholdID, []int64{listID})
	if err != nil {
		return nil, err
	}
	out := make([]*routeGroupResolver, len(groups))
	for i := range groups {
		out[i] = &routeGroupResolver{inv: r.InventoryService, group: groups[i], catItems: gc.items, units: gc.units, ingredients: gc.ingredients, ch: gc.ch}
	}
	return out, nil
}

// CreateStore adds a household store.
func (r *Resolver) CreateStore(ctx context.Context, args struct{ Name string }) (*storeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	st, err := r.GroceryService.CreateStore(ctx, u.HouseholdID, args.Name, u.Email)
	if err != nil {
		return nil, err
	}
	return &storeResolver{g: r.GroceryService, householdID: u.HouseholdID, store: st}, nil
}

// RenameStore renames a household store.
func (r *Resolver) RenameStore(ctx context.Context, args struct {
	StoreID graphql.ID
	Name    string
}) (*storeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := parseID(string(args.StoreID))
	if err != nil {
		return nil, err
	}
	st, err := r.GroceryService.RenameStore(ctx, storeID, u.HouseholdID, args.Name, u.Email)
	if err != nil {
		return nil, err
	}
	return &storeResolver{g: r.GroceryService, householdID: u.HouseholdID, store: st}, nil
}

// DeleteStore removes a store; lists pointing at it keep their rows.
func (r *Resolver) DeleteStore(ctx context.Context, args struct{ StoreID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	storeID, err := parseID(string(args.StoreID))
	if err != nil {
		return false, err
	}
	if err := r.GroceryService.DeleteStore(ctx, storeID, u.HouseholdID); err != nil {
		return false, err
	}
	return true, nil
}

// CreateStoreAisle adds an aisle to a store at a walk-order position.
func (r *Resolver) CreateStoreAisle(ctx context.Context, args struct {
	StoreID  graphql.ID
	Name     string
	Position int32
}) (*storeAisleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := parseID(string(args.StoreID))
	if err != nil {
		return nil, err
	}
	a, err := r.GroceryService.CreateAisle(ctx, storeID, u.HouseholdID, args.Name, args.Position, u.Email)
	if err != nil {
		return nil, err
	}
	return &storeAisleResolver{aisle: a}, nil
}

// RenameStoreAisle renames an aisle.
func (r *Resolver) RenameStoreAisle(ctx context.Context, args struct {
	AisleID graphql.ID
	Name    string
}) (*storeAisleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	aisleID, err := parseID(string(args.AisleID))
	if err != nil {
		return nil, err
	}
	a, err := r.GroceryService.RenameAisle(ctx, aisleID, u.HouseholdID, args.Name, u.Email)
	if err != nil {
		return nil, err
	}
	return &storeAisleResolver{aisle: a}, nil
}

// DeleteStoreAisle removes an aisle; its assignments cascade.
func (r *Resolver) DeleteStoreAisle(ctx context.Context, args struct{ AisleID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	aisleID, err := parseID(string(args.AisleID))
	if err != nil {
		return false, err
	}
	if err := r.GroceryService.DeleteAisle(ctx, aisleID, u.HouseholdID); err != nil {
		return false, err
	}
	return true, nil
}

// ReorderStoreAisles rewrites aisle walk order from the submitted IDs.
func (r *Resolver) ReorderStoreAisles(ctx context.Context, args struct {
	StoreID  graphql.ID
	AisleIDs []graphql.ID
}) ([]*storeAisleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := parseID(string(args.StoreID))
	if err != nil {
		return nil, err
	}
	ids, err := parseIDs(args.AisleIDs)
	if err != nil {
		return nil, err
	}
	if err := r.GroceryService.ReorderAisles(ctx, storeID, u.HouseholdID, ids, u.Email); err != nil {
		return nil, err
	}
	aisles, err := r.GroceryService.ListAisles(ctx, storeID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	out := make([]*storeAisleResolver, len(aisles))
	for i := range aisles {
		out[i] = &storeAisleResolver{aisle: aisles[i]}
	}
	return out, nil
}

// AssignItemToAisle maps one item identity (item, ingredient, or manual
// name) to an aisle; aisleId null unassigns it.
func (r *Resolver) AssignItemToAisle(ctx context.Context, args struct {
	StoreID        graphql.ID
	AisleID        *graphql.ID
	ItemID         *graphql.ID
	IngredientID   *graphql.ID
	ManualItemName *string
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	storeID, err := parseID(string(args.StoreID))
	if err != nil {
		return false, err
	}
	aisleID, err := optionalID(args.AisleID)
	if err != nil {
		return false, err
	}
	itemID, err := optionalID(args.ItemID)
	if err != nil {
		return false, err
	}
	ingredientID, err := optionalID(args.IngredientID)
	if err != nil {
		return false, err
	}
	id := grocery.RouteIdentity{ItemID: itemID, IngredientID: ingredientID, ManualName: derefString(args.ManualItemName)}
	if itemID == nil && ingredientID == nil && id.ManualName == "" {
		return false, badInputf("assignItemToAisle requires itemId, ingredientId or manualItemName")
	}
	if err := r.GroceryService.AssignToAisle(ctx, storeID, u.HouseholdID, id, aisleID, u.Email); err != nil {
		return false, err
	}
	return true, nil
}

// SetGroceryListStore selects which store routes a grocery list.
func (r *Resolver) SetGroceryListStore(ctx context.Context, args struct {
	GroceryListID graphql.ID
	StoreID       *graphql.ID
}) (*groceryListResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	listID, err := parseID(string(args.GroceryListID))
	if err != nil {
		return nil, err
	}
	storeID, err := optionalID(args.StoreID)
	if err != nil {
		return nil, err
	}
	var list grocery.GroceryList
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		// The service verifies store ownership inside the tx.
		if err := r.GroceryService.SetGroceryListStore(ctx, listID, u.HouseholdID, storeID, u.Email); err != nil {
			return err
		}
		var err error
		list, err = r.GroceryService.GetGroceryListByID(ctx, listID, u.HouseholdID)
		return err
	}); err != nil {
		return nil, err
	}
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, r.IdentityService, r.UserPrefsService, u.HouseholdID, []int64{listID})
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, householdID: u.HouseholdID, list: list, items: gc.itemsByList[listID], catItems: gc.items, units: gc.units, ingredients: gc.ingredients, ch: gc.ch}, nil
}

// ReorderGroceryListItems persists the user's arrangement: every entry
// gets a manual rank matching its display position, and entries carrying
// aisleId also move the item into that aisle — all inside one unit of
// work.
func (r *Resolver) ReorderGroceryListItems(ctx context.Context, args struct {
	GroceryListID graphql.ID
	Entries       []groceryReorderEntryInput
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	listID, err := parseID(string(args.GroceryListID))
	if err != nil {
		return false, err
	}
	entries := make([]grocery.ReorderEntry, 0, len(args.Entries))
	for _, e := range args.Entries {
		itemID, err := parseID(string(e.GroceryListItemID))
		if err != nil {
			return false, err
		}
		aisleID, err := optionalID(e.AisleID)
		if err != nil {
			return false, err
		}
		entries = append(entries, grocery.ReorderEntry{GroceryListItemID: itemID, AisleID: aisleID})
	}
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		return r.GroceryService.ReorderListItems(ctx, listID, u.HouseholdID, entries, u.Email)
	}); err != nil {
		return false, err
	}
	return true, nil
}

// groceryReorderEntryInput mirrors GroceryReorderEntryInput.
type groceryReorderEntryInput struct {
	GroceryListItemID graphql.ID
	AisleID           *graphql.ID
}

// storeResolver resolves Store fields.
type storeResolver struct {
	g           GroceryService
	householdID int64
	store       grocery.Store
}

func (r *storeResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.store.StoreID, 10))
}

func (r *storeResolver) Name() string { return r.store.Name }

func (r *storeResolver) Aisles(ctx context.Context) ([]*storeAisleResolver, error) {
	aisles, err := r.g.ListAisles(ctx, r.store.StoreID, r.householdID)
	if err != nil {
		return nil, err
	}
	out := make([]*storeAisleResolver, len(aisles))
	for i := range aisles {
		out[i] = &storeAisleResolver{aisle: aisles[i]}
	}
	return out, nil
}

// storeAisleResolver resolves StoreAisle fields.
type storeAisleResolver struct {
	aisle grocery.StoreAisle
}

func (r *storeAisleResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.aisle.AisleID, 10))
}

func (r *storeAisleResolver) Name() string    { return r.aisle.Name }
func (r *storeAisleResolver) Position() int32 { return r.aisle.Position }

// routeGroupResolver resolves GroceryRouteGroup fields.
type routeGroupResolver struct {
	inv         ItemReader
	group       grocery.RouteGroup
	catItems    map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
}

func (r *routeGroupResolver) Aisle() *storeAisleResolver {
	if r.group.Aisle == nil {
		return nil
	}
	return &storeAisleResolver{aisle: *r.group.Aisle}
}

func (r *routeGroupResolver) Items() []*routeItemResolver {
	out := make([]*routeItemResolver, len(r.group.Items))
	for i := range r.group.Items {
		out[i] = &routeItemResolver{
			inv: r.inv, suggested: r.group.Items[i].Suggested, item: r.group.Items[i].Item,
			items: r.catItems, units: r.units, ingredients: r.ingredients, ch: r.ch,
		}
	}
	return out
}

// routeItemResolver resolves GroceryRouteItem fields.
type routeItemResolver struct {
	inv         ItemReader
	item        grocery.GroceryListItem
	suggested   bool
	items       map[int64]inventory.Item
	units       map[int64]inventory.Unit
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
}

func (r *routeItemResolver) Item() *groceryListItemResolver {
	return &groceryListItemResolver{inv: r.inv, item: r.item, items: r.items, units: r.units, ingredients: r.ingredients, ch: r.ch, as: asOfItemChildren(r.ch)}
}

func (r *routeItemResolver) Suggested() bool { return r.suggested }
