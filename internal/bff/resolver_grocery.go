package bff

import (
	"context"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/graph-gophers/graphql-go"
	"strconv"
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
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, list: list}, nil
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
	return &groceryListPageResolver{g: r.GroceryService, inv: r.InventoryService, lists: lists, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// GenerateGroceryList generates a grocery list from a meal plan.
func (r *Resolver) GenerateGroceryList(ctx context.Context, args struct{ MealPlanID graphql.ID }) (*groceryListResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.MealPlanID))
	if err != nil {
		return nil, err
	}
	list, err := r.GroceryService.Generate(ctx, u.UserID, mealPlanID, u.Email)
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, list: list}, nil
}

// ToggleGroceryItemChecked flips the checked state of a grocery list item.
func (r *Resolver) ToggleGroceryItemChecked(ctx context.Context, args struct{ GroceryListItemID graphql.ID }) (*groceryListItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return nil, err
	}
	it, err := r.GroceryService.GetGroceryListItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	it.IsChecked = !it.IsChecked
	if err := r.GroceryService.UpdateGroceryListItem(ctx, id, it, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.GroceryService.GetGroceryListItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: updated}, nil
}

// DeleteGroceryItem removes an item from a grocery list.
func (r *Resolver) DeleteGroceryItem(ctx context.Context, args struct{ GroceryListItemID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return false, err
	}
	if err := r.GroceryService.DeleteGroceryListItem(ctx, id); err != nil {
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
	it, err := r.GroceryService.AddGroceryListItem(ctx, grocery.GroceryListItem{
		GroceryListID:  groceryListID,
		ItemID:         itemID,
		ManualItemName: derefString(args.Input.ManualItemName),
		QuantityNeeded: args.Input.Quantity,
		UnitOfMeasure:  args.Input.Unit,
		Source:         "manual",
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: it}, nil
}

// groceryListResolver resolves GroceryList fields.
type groceryListResolver struct {
	g    GroceryService
	inv  InventoryService
	list grocery.GroceryList
}

func (r *groceryListResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.list.GroceryListID, 10))
}

func (r *groceryListResolver) GeneratedAt() graphql.Time {
	return graphql.Time{Time: r.list.GeneratedAt}
}

func (r *groceryListResolver) Items(ctx context.Context) ([]*groceryListItemResolver, error) {
	items, err := r.g.ListGroceryListItems(ctx, r.list.GroceryListID)
	if err != nil {
		return nil, err
	}
	out := make([]*groceryListItemResolver, len(items))
	for i := range items {
		out[i] = &groceryListItemResolver{inv: r.inv, item: items[i]}
	}
	return out, nil
}

// groceryListItemResolver resolves GroceryListItem fields.
type groceryListItemResolver struct {
	inv  InventoryService
	item grocery.GroceryListItem
}

func (r *groceryListItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.GroceryListItemID, 10))
}

func (r *groceryListItemResolver) ManualItemName() *string { return nilIfEmpty(r.item.ManualItemName) }

func (r *groceryListItemResolver) QuantityNeeded() float64 { return r.item.QuantityNeeded }

func (r *groceryListItemResolver) UnitOfMeasure() *string { return nilIfEmpty(r.item.UnitOfMeasure) }

func (r *groceryListItemResolver) Source() string { return r.item.Source }

func (r *groceryListItemResolver) IsChecked() bool { return r.item.IsChecked }

func (r *groceryListItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.item.ItemID == nil {
		return nil, nil
	}
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type groceryListPageResolver struct {
	g        GroceryService
	inv      InventoryService
	lists    []grocery.GroceryList
	page     int32
	pageSize int32
	total    int32
}

func (r *groceryListPageResolver) Items() []*groceryListResolver {
	out := make([]*groceryListResolver, len(r.lists))
	for i := range r.lists {
		out[i] = &groceryListResolver{g: r.g, inv: r.inv, list: r.lists[i]}
	}
	return out
}

func (r *groceryListPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type addGroceryItemInput struct {
	GroceryListID  graphql.ID
	ItemID         *graphql.ID
	ManualItemName *string
	Quantity       float64
	Unit           string
}
