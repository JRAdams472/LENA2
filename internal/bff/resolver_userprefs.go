package bff

import (
	"context"
	"strconv"
	"time"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/graph-gophers/graphql-go"
)

// UserBottles resolves the current user's wine cellar.
func (r *Resolver) UserBottles(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userBottlePageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	bottles, err := r.UserPrefsService.ListUserBottles(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.UserPrefsService.CountUserBottles(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	bc, err := loadBottleChildren(ctx, r.WineService, distinctIDs(bottles, func(b userprefs.UserBottle) *int64 { return &b.BottleID }), true)
	if err != nil {
		return nil, err
	}
	return &userBottlePageResolver{wine: r.WineService, bottles: bottles, bc: bc, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// UserItems resolves the current user's pantry items.
func (r *Resolver) UserItems(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userItemPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	items, err := r.UserPrefsService.ListUserItems(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.UserPrefsService.CountUserItems(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	itemsByID, err := loadItems(ctx, r.InventoryService, distinctIDs(items, func(it userprefs.UserItem) *int64 { return &it.ItemID }))
	if err != nil {
		return nil, err
	}
	itemList := make([]inventory.Item, 0, len(itemsByID))
	for _, it := range itemsByID {
		itemList = append(itemList, it)
	}
	ch, err := loadItemChildren(ctx, r.InventoryService, itemList)
	if err != nil {
		return nil, err
	}
	return &userItemPageResolver{inv: r.InventoryService, items: items, itemsByID: itemsByID, ch: ch, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// AdjustUserItem updates the quantity and purchase date of a user's pantry item.
func (r *Resolver) AdjustUserItem(ctx context.Context, args struct {
	ItemID     graphql.ID
	Quantity   float64
	PurchaseAt *graphql.Time
}) (*userItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserItem(ctx, r.UserPrefsService, u.UserID, itemID)
	if err != nil {
		return nil, err
	}
	isFav := false
	var minQty *float64
	var expiresAt *time.Time
	notes := ""
	if existing != nil {
		isFav = existing.IsFavorite
		minQty = existing.MinQty
		expiresAt = existing.ExpiresAt
		notes = existing.Notes
	}
	var purchaseAt *time.Time
	if args.PurchaseAt != nil {
		purchaseAt = &args.PurchaseAt.Time
	}
	updated, err := r.UserPrefsService.UpsertUserItem(ctx, userprefs.UserItem{
		UserID:     u.UserID,
		ItemID:     itemID,
		CurrentQty: args.Quantity,
		MinQty:     minQty,
		PurchaseAt: purchaseAt,
		ExpiresAt:  expiresAt,
		Notes:      notes,
		IsFavorite: isFav,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: updated}, nil
}

// SetItemFavorite toggles the favorite flag for a user's pantry item.
func (r *Resolver) SetItemFavorite(ctx context.Context, args struct {
	ItemID     graphql.ID
	IsFavorite bool
}) (*userItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserItem(ctx, r.UserPrefsService, u.UserID, itemID)
	if err != nil {
		return nil, err
	}
	currentQty := 0.0
	var minQty *float64
	var purchaseAt *time.Time
	var expiresAt *time.Time
	notes := ""
	if existing != nil {
		currentQty = existing.CurrentQty
		minQty = existing.MinQty
		purchaseAt = existing.PurchaseAt
		expiresAt = existing.ExpiresAt
		notes = existing.Notes
	}
	updated, err := r.UserPrefsService.UpsertUserItem(ctx, userprefs.UserItem{
		UserID:     u.UserID,
		ItemID:     itemID,
		CurrentQty: currentQty,
		MinQty:     minQty,
		PurchaseAt: purchaseAt,
		ExpiresAt:  expiresAt,
		Notes:      notes,
		IsFavorite: args.IsFavorite,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: updated}, nil
}

// DeleteUserItem removes a user's pantry item by catalog item ID.
func (r *Resolver) DeleteUserItem(ctx context.Context, args struct{ ItemID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	existing, err := findUserItem(ctx, r.UserPrefsService, u.UserID, itemID)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, nil
	}
	if err := r.UserPrefsService.DeleteUserItem(ctx, existing.UserItemID, u.UserID); err != nil {
		return false, err
	}
	return true, nil
}

// AdjustUserBottle updates the quantity of a user's wine cellar holding.
func (r *Resolver) AdjustUserBottle(ctx context.Context, args struct {
	BottleID graphql.ID
	Quantity int32
}) (*userBottleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	bottleID, err := parseID(string(args.BottleID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserBottle(ctx, r.UserPrefsService, u.UserID, bottleID)
	if err != nil {
		return nil, err
	}
	var bottleNum *int32
	var purchaseAt *time.Time
	var purchasePrice *float64
	var storageTemp *float64
	location := ""
	notes := ""
	isFav := false
	if existing != nil {
		bottleNum = existing.BottleNumber
		purchaseAt = existing.PurchaseAt
		purchasePrice = existing.PurchasePrice
		storageTemp = existing.StorageTemp
		location = existing.Location
		notes = existing.Notes
		isFav = existing.IsFavorite
	}
	updated, err := r.UserPrefsService.UpsertUserBottle(ctx, userprefs.UserBottle{
		UserID:        u.UserID,
		BottleID:      bottleID,
		BottleNumber:  bottleNum,
		Quantity:      args.Quantity,
		PurchaseAt:    purchaseAt,
		PurchasePrice: purchasePrice,
		StorageTemp:   storageTemp,
		Location:      location,
		Notes:         notes,
		IsFavorite:    isFav,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userBottleResolver{wine: r.WineService, bottle: updated}, nil
}

// SetBottleFavorite toggles the favorite flag for a user's wine holding.
func (r *Resolver) SetBottleFavorite(ctx context.Context, args struct {
	BottleID   graphql.ID
	IsFavorite bool
}) (*userBottleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	bottleID, err := parseID(string(args.BottleID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserBottle(ctx, r.UserPrefsService, u.UserID, bottleID)
	if err != nil {
		return nil, err
	}
	quantity := int32(0)
	var bottleNum *int32
	var purchaseAt *time.Time
	var purchasePrice *float64
	var storageTemp *float64
	location := ""
	notes := ""
	if existing != nil {
		quantity = existing.Quantity
		bottleNum = existing.BottleNumber
		purchaseAt = existing.PurchaseAt
		purchasePrice = existing.PurchasePrice
		storageTemp = existing.StorageTemp
		location = existing.Location
		notes = existing.Notes
	}
	updated, err := r.UserPrefsService.UpsertUserBottle(ctx, userprefs.UserBottle{
		UserID:        u.UserID,
		BottleID:      bottleID,
		BottleNumber:  bottleNum,
		Quantity:      quantity,
		PurchaseAt:    purchaseAt,
		PurchasePrice: purchasePrice,
		StorageTemp:   storageTemp,
		Location:      location,
		Notes:         notes,
		IsFavorite:    args.IsFavorite,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userBottleResolver{wine: r.WineService, bottle: updated}, nil
}

func findUserItem(ctx context.Context, svc UserPrefsService, userID, itemID int64) (*userprefs.UserItem, error) {
	items, err := svc.ListUserItems(ctx, userID, 100_000, 0)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ItemID == itemID {
			return &items[i], nil
		}
	}
	return nil, nil
}

func findUserBottle(ctx context.Context, svc UserPrefsService, userID, bottleID int64) (*userprefs.UserBottle, error) {
	bottles, err := svc.ListUserBottles(ctx, userID, 100_000, 0)
	if err != nil {
		return nil, err
	}
	for i := range bottles {
		if bottles[i].BottleID == bottleID {
			return &bottles[i], nil
		}
	}
	return nil, nil
}

// userItemResolver resolves UserItem fields. When items is non-nil the
// batch-loaded catalog rows are used instead of a per-item service call.
type userItemResolver struct {
	inv   InventoryService
	item  userprefs.UserItem
	items map[int64]inventory.Item
	ch    *itemChildren
}

func (r *userItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.UserItemID, 10))
}

func (r *userItemResolver) CurrentQty() float64 { return r.item.CurrentQty }

func (r *userItemResolver) MinQty() *float64 { return r.item.MinQty }

func (r *userItemResolver) Notes() *string { return nilIfEmpty(r.item.Notes) }

func (r *userItemResolver) IsFavorite() bool { return r.item.IsFavorite }

func (r *userItemResolver) PurchaseAt() *graphql.Time { return timeToGraphQL(r.item.PurchaseAt) }

func (r *userItemResolver) ExpiresAt() *graphql.Time { return timeToGraphQL(r.item.ExpiresAt) }

func (r *userItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.items != nil {
		it, ok := r.items[r.item.ItemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch}, nil
	}
	it, err := r.inv.GetItemByID(ctx, r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type userItemPageResolver struct {
	inv       InventoryService
	items     []userprefs.UserItem
	itemsByID map[int64]inventory.Item
	ch        *itemChildren
	page      int32
	pageSize  int32
	total     int32
}

func (r *userItemPageResolver) Items() []*userItemResolver {
	out := make([]*userItemResolver, len(r.items))
	for i := range r.items {
		out[i] = &userItemResolver{inv: r.inv, item: r.items[i], items: r.itemsByID, ch: r.ch}
	}
	return out
}

func (r *userItemPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// userBottleResolver resolves UserBottle fields. When bc is non-nil the
// batch-loaded bottle rows are used instead of a per-bottle service call.
type userBottleResolver struct {
	wine   WineService
	bottle userprefs.UserBottle
	bc     *bottleChildren
}

func (r *userBottleResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.bottle.UserBottleID, 10))
}

func (r *userBottleResolver) BottleNumber() *int32 { return r.bottle.BottleNumber }

func (r *userBottleResolver) Quantity() int32 { return r.bottle.Quantity }

func (r *userBottleResolver) PurchaseAt() *graphql.Time { return timeToGraphQL(r.bottle.PurchaseAt) }

func (r *userBottleResolver) PurchasePrice() *float64 { return r.bottle.PurchasePrice }

func (r *userBottleResolver) StorageTemp() *float64 { return r.bottle.StorageTemp }

func (r *userBottleResolver) Location() *string { return nilIfEmpty(r.bottle.Location) }

func (r *userBottleResolver) Notes() *string { return nilIfEmpty(r.bottle.Notes) }

func (r *userBottleResolver) IsFavorite() bool { return r.bottle.IsFavorite }

func (r *userBottleResolver) Bottle(ctx context.Context) (*bottleResolver, error) {
	if r.bc != nil {
		b, ok := r.bc.bottles[r.bottle.BottleID]
		if !ok {
			return nil, nil
		}
		return &bottleResolver{wine: r.wine, b: b, bc: r.bc}, nil
	}
	b, err := r.wine.GetBottleByID(ctx, r.bottle.BottleID)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{wine: r.wine, b: b}, nil
}

type userBottlePageResolver struct {
	wine     WineService
	bottles  []userprefs.UserBottle
	bc       *bottleChildren
	page     int32
	pageSize int32
	total    int32
}

func (r *userBottlePageResolver) Items() []*userBottleResolver {
	out := make([]*userBottleResolver, len(r.bottles))
	for i := range r.bottles {
		out[i] = &userBottleResolver{wine: r.wine, bottle: r.bottles[i], bc: r.bc}
	}
	return out
}

func (r *userBottlePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}
