package bff

import (
	"context"
	"strconv"
	"time"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/graph-gophers/graphql-go"
)

// UserBottles resolves the current household's wine cellar. Each row's
// isFavorite flag stays per-user: favorites were split out of the shared
// holding table so household members keep personal favorites.
func (r *Resolver) UserBottles(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userBottlePageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)
	bottles, err := r.UserPrefsService.ListHouseholdBottles(ctx, u.HouseholdID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.UserPrefsService.CountHouseholdBottles(ctx, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	bottleIDs := distinctIDs(bottles, func(b userprefs.HouseholdBottle) *int64 { return &b.BottleID })
	favorites, err := r.UserPrefsService.ListBottleFavorites(ctx, u.UserID, bottleIDs)
	if err != nil {
		return nil, err
	}
	bc, err := loadBottleChildren(ctx, r.WineService, bottleIDs, true)
	if err != nil {
		return nil, err
	}
	return &userBottlePageResolver{wine: r.WineService, bottles: bottles, favorites: favorites, bc: bc, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// UserItems resolves the current household's pantry items with the current
// user's per-item favorite flags.
func (r *Resolver) UserItems(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userItemPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)
	items, err := r.UserPrefsService.ListHouseholdItems(ctx, u.HouseholdID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.UserPrefsService.CountHouseholdItems(ctx, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	itemIDs := distinctIDs(items, func(it userprefs.HouseholdItem) *int64 { return &it.ItemID })
	favorites, err := r.UserPrefsService.ListItemFavorites(ctx, u.UserID, itemIDs)
	if err != nil {
		return nil, err
	}
	itemsByID, err := loadItems(ctx, r.InventoryService, itemIDs)
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
	return &userItemPageResolver{inv: r.InventoryService, items: items, favorites: favorites, itemsByID: itemsByID, ch: ch, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// AdjustUserItem updates the quantity and purchase date of the household's
// pantry item.
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
	existing, err := findHouseholdItem(ctx, r.UserPrefsService, u.HouseholdID, itemID)
	if err != nil {
		return nil, err
	}
	var minQty *float64
	var expiresAt *time.Time
	notes := ""
	if existing != nil {
		minQty = existing.MinQty
		expiresAt = existing.ExpiresAt
		notes = existing.Notes
	}
	var purchaseAt *time.Time
	if args.PurchaseAt != nil {
		purchaseAt = &args.PurchaseAt.Time
	}
	updated, err := r.UserPrefsService.UpsertHouseholdItem(ctx, userprefs.HouseholdItem{
		HouseholdID: u.HouseholdID,
		ItemID:      itemID,
		CurrentQty:  args.Quantity,
		MinQty:      minQty,
		PurchaseAt:  purchaseAt,
		ExpiresAt:   expiresAt,
		Notes:       notes,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	isFav, err := r.UserPrefsService.GetItemFavorite(ctx, u.UserID, itemID)
	if err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: updated, isFavorite: isFav}, nil
}

// SetItemFavorite toggles the current user's favorite flag for a pantry
// item. Favorites are personal, but a household holding row is still
// created when absent so the favorite surfaces in the pantry list (the
// client's favorite filter reads holdings). The two writes share one unit
// of work.
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
	existing, err := findHouseholdItem(ctx, r.UserPrefsService, u.HouseholdID, itemID)
	if err != nil {
		return nil, err
	}
	item := existing
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.UserPrefsService.SetItemFavorite(ctx, u.UserID, itemID, args.IsFavorite, u.Email); err != nil {
			return err
		}
		if item == nil {
			created, err := r.UserPrefsService.UpsertHouseholdItem(ctx, userprefs.HouseholdItem{
				HouseholdID: u.HouseholdID,
				ItemID:      itemID,
				CurrentQty:  0,
			}, u.Email)
			if err != nil {
				return err
			}
			item = &created
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: *item, isFavorite: args.IsFavorite}, nil
}

// DeleteUserItem removes the household's pantry row by catalog item ID.
func (r *Resolver) DeleteUserItem(ctx context.Context, args struct{ ItemID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	existing, err := findHouseholdItem(ctx, r.UserPrefsService, u.HouseholdID, itemID)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, nil
	}
	if err := r.UserPrefsService.DeleteHouseholdItem(ctx, existing.HouseholdItemID, u.HouseholdID); err != nil {
		return false, err
	}
	return true, nil
}

// IncrementUserItem adds or removes a delta from the household's pantry
// stock for a single catalog item. The adjustment is performed by a single
// atomic upsert; the row is deleted when the resulting quantity is clamped
// to 0.
func (r *Resolver) IncrementUserItem(ctx context.Context, args struct {
	ItemID graphql.ID
	Delta  float64
}) (*userItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	if args.Delta == 0 {
		return nil, badInputf("delta cannot be zero")
	}

	// The adjustment and the conditional delete run inside one unit of
	// work; the ctx-carried transaction joins the domain service calls.
	var result *userprefs.HouseholdItem
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		adjusted, err := r.UserPrefsService.AdjustHouseholdItemQuantity(ctx, u.HouseholdID, itemID, args.Delta, u.Email)
		if err != nil {
			return err
		}
		if adjusted.CurrentQty == 0 {
			return r.UserPrefsService.DeleteHouseholdItem(ctx, adjusted.HouseholdItemID, u.HouseholdID)
		}
		result = &adjusted
		return nil
	}); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	isFav, err := r.UserPrefsService.GetItemFavorite(ctx, u.UserID, itemID)
	if err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: *result, isFavorite: isFav}, nil
}

// AdjustUserBottle updates the quantity of the household's wine cellar
// holding.
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
	existing, err := findHouseholdBottle(ctx, r.UserPrefsService, u.HouseholdID, bottleID)
	if err != nil {
		return nil, err
	}
	var bottleNum *int32
	var purchaseAt *time.Time
	var purchasePrice *float64
	var storageTemp *float64
	location := ""
	notes := ""
	if existing != nil {
		bottleNum = existing.BottleNumber
		purchaseAt = existing.PurchaseAt
		purchasePrice = existing.PurchasePrice
		storageTemp = existing.StorageTemp
		location = existing.Location
		notes = existing.Notes
	}
	updated, err := r.UserPrefsService.UpsertHouseholdBottle(ctx, userprefs.HouseholdBottle{
		HouseholdID:   u.HouseholdID,
		BottleID:      bottleID,
		BottleNumber:  bottleNum,
		Quantity:      args.Quantity,
		PurchaseAt:    purchaseAt,
		PurchasePrice: purchasePrice,
		StorageTemp:   storageTemp,
		Location:      location,
		Notes:         notes,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	isFav, err := r.UserPrefsService.GetBottleFavorite(ctx, u.UserID, bottleID)
	if err != nil {
		return nil, err
	}
	return &userBottleResolver{wine: r.WineService, bottle: updated, isFavorite: isFav}, nil
}

// SetBottleFavorite toggles the current user's favorite flag for a bottle.
// Like SetItemFavorite, a household holding row is created when absent so
// the favorite remains visible in the cellar list; both writes share one
// unit of work.
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
	existing, err := findHouseholdBottle(ctx, r.UserPrefsService, u.HouseholdID, bottleID)
	if err != nil {
		return nil, err
	}
	bottle := existing
	if err := r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.UserPrefsService.SetBottleFavorite(ctx, u.UserID, bottleID, args.IsFavorite, u.Email); err != nil {
			return err
		}
		if bottle == nil {
			created, err := r.UserPrefsService.UpsertHouseholdBottle(ctx, userprefs.HouseholdBottle{
				HouseholdID: u.HouseholdID,
				BottleID:    bottleID,
				Quantity:    0,
			}, u.Email)
			if err != nil {
				return err
			}
			bottle = &created
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &userBottleResolver{wine: r.WineService, bottle: *bottle, isFavorite: args.IsFavorite}, nil
}

func findHouseholdItem(ctx context.Context, svc UserPrefsService, householdID, itemID int64) (*userprefs.HouseholdItem, error) {
	return svc.GetHouseholdItemByItem(ctx, householdID, itemID)
}

func findHouseholdBottle(ctx context.Context, svc UserPrefsService, householdID, bottleID int64) (*userprefs.HouseholdBottle, error) {
	return svc.GetHouseholdBottleByBottle(ctx, householdID, bottleID)
}

// userItemResolver resolves UserItem fields. When items is non-nil the
// batch-loaded catalog rows are used instead of a per-item service call.
type userItemResolver struct {
	inv        ItemReader
	item       userprefs.HouseholdItem
	isFavorite bool
	items      map[int64]inventory.Item
	ch         *itemChildren
}

func (r *userItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.HouseholdItemID, 10))
}

func (r *userItemResolver) CurrentQty() float64 { return r.item.CurrentQty }

func (r *userItemResolver) MinQty() *float64 { return r.item.MinQty }

func (r *userItemResolver) Notes() *string { return nilIfEmpty(r.item.Notes) }

func (r *userItemResolver) IsFavorite() bool { return r.isFavorite }

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
	inv       ItemReader
	items     []userprefs.HouseholdItem
	favorites map[int64]bool
	itemsByID map[int64]inventory.Item
	ch        *itemChildren
	page      int32
	pageSize  int32
	total     int32
}

func (r *userItemPageResolver) Items() []*userItemResolver {
	out := make([]*userItemResolver, len(r.items))
	for i := range r.items {
		out[i] = &userItemResolver{inv: r.inv, item: r.items[i], isFavorite: r.favorites[r.items[i].ItemID], items: r.itemsByID, ch: r.ch}
	}
	return out
}

func (r *userItemPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// userBottleResolver resolves UserBottle fields. When bc is non-nil the
// batch-loaded bottle rows are used instead of a per-bottle service call.
type userBottleResolver struct {
	wine       BottleReader
	bottle     userprefs.HouseholdBottle
	isFavorite bool
	bc         *bottleChildren
}

func (r *userBottleResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.bottle.HouseholdBottleID, 10))
}

func (r *userBottleResolver) BottleNumber() *int32 { return r.bottle.BottleNumber }

func (r *userBottleResolver) Quantity() int32 { return r.bottle.Quantity }

func (r *userBottleResolver) PurchaseAt() *graphql.Time { return timeToGraphQL(r.bottle.PurchaseAt) }

func (r *userBottleResolver) PurchasePrice() *float64 { return r.bottle.PurchasePrice }

func (r *userBottleResolver) StorageTemp() *float64 { return r.bottle.StorageTemp }

func (r *userBottleResolver) Location() *string { return nilIfEmpty(r.bottle.Location) }

func (r *userBottleResolver) Notes() *string { return nilIfEmpty(r.bottle.Notes) }

func (r *userBottleResolver) IsFavorite() bool { return r.isFavorite }

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
	wine      BottleReader
	bottles   []userprefs.HouseholdBottle
	favorites map[int64]bool
	bc        *bottleChildren
	page      int32
	pageSize  int32
	total     int32
}

func (r *userBottlePageResolver) Items() []*userBottleResolver {
	out := make([]*userBottleResolver, len(r.bottles))
	for i := range r.bottles {
		out[i] = &userBottleResolver{wine: r.wine, bottle: r.bottles[i], isFavorite: r.favorites[r.bottles[i].BottleID], bc: r.bc}
	}
	return out
}

func (r *userBottlePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}
