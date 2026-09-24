// Package userprefs owns shared household holdings (pantry stock, wine
// cellar) and per-user preferences (recipe favorites plus item and bottle
// favorites). All tables live in the userprefs schema.
package userprefs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/userprefs/sqlc"
)

// Service provides household holding and per-user preference operations.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates a userprefs Service using the given connection pool.
// The querier resolves a ctx-carried transaction first (see
// dbtx.ContextExecer) so calls made inside a UnitOfWork join that
// transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "userprefs")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "userprefs"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "userprefs")) }
	}
	c := *s
	c.q = newQ(tx)
	c.tx = tx
	c.newQ = newQ
	return &c
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise. If the service is already bound to a transaction, or
// ctx already carries a UnitOfWork transaction, fn runs in that transaction
// instead of starting a new one.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	if s.tx != nil || dbtx.HasTx(ctx) {
		return fn(s)
	}
	if s.pool == nil {
		return fmt.Errorf("userprefs: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// HouseholdItem is one pantry row shared by a household. Item favorites are
// per-user and live separately; they are not part of this type.
type HouseholdItem struct {
	HouseholdItemID int64
	HouseholdID     int64
	ItemID          int64
	CurrentQty      float64
	MinQty          *float64
	PurchaseAt      *time.Time
	ExpiresAt       *time.Time
	Notes           string
}

// UpsertHouseholdItem creates or updates the household's pantry row for a
// catalog item.
func (s *Service) UpsertHouseholdItem(ctx context.Context, arg HouseholdItem, by string) (HouseholdItem, error) {
	currentQty, err := numericFromFloat64(arg.CurrentQty)
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("upsert household item: %w", err)
	}
	minQty, err := optNumeric(arg.MinQty)
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("upsert household item: %w", err)
	}
	row, err := s.q.UpsertHouseholdItem(ctx, sqlc.UpsertHouseholdItemParams{
		HouseholdID: arg.HouseholdID,
		ItemID:      arg.ItemID,
		CurrentQty:  currentQty,
		MinQty:      minQty,
		PurchaseAt:  optTimestamptz(arg.PurchaseAt),
		ExpiresAt:   optTimestamptz(arg.ExpiresAt),
		Notes:       textOrNull(arg.Notes),
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("upsert household item: %w", err)
	}
	hi, err := toHouseholdItem(row)
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("upsert household item: %w", err)
	}
	return hi, nil
}

// AdjustHouseholdItemQuantity atomically adds delta to the household's
// pantry stock for itemID, clamping at 0, and creates the row if it does
// not exist.
func (s *Service) AdjustHouseholdItemQuantity(ctx context.Context, householdID, itemID int64, delta float64, by string) (HouseholdItem, error) {
	d, err := numericFromFloat64(delta)
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("adjust household item quantity: %w", err)
	}
	row, err := s.q.AdjustHouseholdItemQuantity(ctx, sqlc.AdjustHouseholdItemQuantityParams{
		HouseholdID: householdID,
		ItemID:      itemID,
		CreatedBy:   by,
		Delta:       d,
	})
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("adjust household item quantity: %w", domainerr.FromStorage(err))
	}
	return toHouseholdItem(row)
}

// GetHouseholdItemByID returns a pantry row owned by the household.
func (s *Service) GetHouseholdItemByID(ctx context.Context, householdItemID, householdID int64) (HouseholdItem, error) {
	row, err := s.q.GetHouseholdItemByID(ctx, sqlc.GetHouseholdItemByIDParams{HouseholdItemID: householdItemID, HouseholdID: householdID})
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("get household item: %w", domainerr.FromStorage(err))
	}
	hi, err := toHouseholdItem(row)
	if err != nil {
		return HouseholdItem{}, fmt.Errorf("get household item: %w", err)
	}
	return hi, nil
}

// GetHouseholdItemByItem returns the household's pantry row for a catalog
// item, or nil when no such row exists.
func (s *Service) GetHouseholdItemByItem(ctx context.Context, householdID, itemID int64) (*HouseholdItem, error) {
	row, err := s.q.GetHouseholdItemByItem(ctx, sqlc.GetHouseholdItemByItemParams{HouseholdID: householdID, ItemID: itemID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get household item: %w", err)
	}
	hi, err := toHouseholdItem(row)
	if err != nil {
		return nil, fmt.Errorf("get household item: %w", err)
	}
	return &hi, nil
}

// ListHouseholdItems returns a household's pantry items.
func (s *Service) ListHouseholdItems(ctx context.Context, householdID int64, limit, offset int32) ([]HouseholdItem, error) {
	rows, err := s.q.ListHouseholdItems(ctx, sqlc.ListHouseholdItemsParams{HouseholdID: householdID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list household items: %w", err)
	}
	out := make([]HouseholdItem, len(rows))
	for i := range rows {
		hi, err := toHouseholdItem(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list household items: %w", err)
		}
		out[i] = hi
	}
	return out, nil
}

// CountHouseholdItems returns the number of pantry rows the household holds.
func (s *Service) CountHouseholdItems(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountHouseholdItems(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("count household items: %w", err)
	}
	return n, nil
}

// DeleteHouseholdItem removes a pantry row owned by the household. Like the
// grocery delete paths, a missing or foreign-owned row is a no-op: deletes
// are idempotent so a stale client cannot distinguish them anyway.
func (s *Service) DeleteHouseholdItem(ctx context.Context, householdItemID, householdID int64) error {
	_, err := s.q.DeleteHouseholdItem(ctx, sqlc.DeleteHouseholdItemParams{HouseholdItemID: householdItemID, HouseholdID: householdID})
	return err
}

// HouseholdBottle is one cellar holding shared by a household. Bottle
// favorites are per-user and live separately.
type HouseholdBottle struct {
	HouseholdBottleID int64
	HouseholdID       int64
	BottleID          int64
	BottleNumber      *int32
	Quantity          int32
	PurchaseAt        *time.Time
	PurchasePrice     *float64
	StorageTemp       *float64
	Location          string
	Notes             string
}

// UpsertHouseholdBottle creates or updates the household's cellar holding.
func (s *Service) UpsertHouseholdBottle(ctx context.Context, arg HouseholdBottle, by string) (HouseholdBottle, error) {
	price, err := optNumeric(arg.PurchasePrice)
	if err != nil {
		return HouseholdBottle{}, fmt.Errorf("upsert household bottle: %w", err)
	}
	temp, err := optNumeric(arg.StorageTemp)
	if err != nil {
		return HouseholdBottle{}, fmt.Errorf("upsert household bottle: %w", err)
	}
	row, err := s.q.UpsertHouseholdBottle(ctx, sqlc.UpsertHouseholdBottleParams{
		HouseholdID:   arg.HouseholdID,
		BottleID:      arg.BottleID,
		BottleNumber:  optInt4(arg.BottleNumber),
		Quantity:      arg.Quantity,
		PurchaseAt:    optTimestamptz(arg.PurchaseAt),
		PurchasePrice: price,
		StorageTemp:   temp,
		Location:      textOrNull(arg.Location),
		Notes:         textOrNull(arg.Notes),
		CreatedBy:     by,
		UpdatedBy:     textOrNull(by),
	})
	if err != nil {
		return HouseholdBottle{}, fmt.Errorf("upsert household bottle: %w", err)
	}
	hb, err := toHouseholdBottle(row)
	if err != nil {
		return HouseholdBottle{}, fmt.Errorf("upsert household bottle: %w", err)
	}
	return hb, nil
}

// GetHouseholdBottleByID returns a cellar holding owned by the household.
func (s *Service) GetHouseholdBottleByID(ctx context.Context, householdBottleID, householdID int64) (HouseholdBottle, error) {
	row, err := s.q.GetHouseholdBottleByID(ctx, sqlc.GetHouseholdBottleByIDParams{HouseholdBottleID: householdBottleID, HouseholdID: householdID})
	if err != nil {
		return HouseholdBottle{}, fmt.Errorf("get household bottle: %w", domainerr.FromStorage(err))
	}
	hb, err := toHouseholdBottle(row)
	if err != nil {
		return HouseholdBottle{}, fmt.Errorf("get household bottle: %w", err)
	}
	return hb, nil
}

// GetHouseholdBottleByBottle returns the household's cellar holding for a
// bottle, or nil when no such row exists.
func (s *Service) GetHouseholdBottleByBottle(ctx context.Context, householdID, bottleID int64) (*HouseholdBottle, error) {
	row, err := s.q.GetHouseholdBottleByBottle(ctx, sqlc.GetHouseholdBottleByBottleParams{HouseholdID: householdID, BottleID: bottleID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get household bottle: %w", err)
	}
	hb, err := toHouseholdBottle(row)
	if err != nil {
		return nil, fmt.Errorf("get household bottle: %w", err)
	}
	return &hb, nil
}

// ListHouseholdBottles returns a household's cellar holdings.
func (s *Service) ListHouseholdBottles(ctx context.Context, householdID int64, limit, offset int32) ([]HouseholdBottle, error) {
	rows, err := s.q.ListHouseholdBottles(ctx, sqlc.ListHouseholdBottlesParams{HouseholdID: householdID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list household bottles: %w", err)
	}
	out := make([]HouseholdBottle, len(rows))
	for i := range rows {
		hb, err := toHouseholdBottle(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list household bottles: %w", err)
		}
		out[i] = hb
	}
	return out, nil
}

// CountHouseholdBottles returns the number of cellar rows the household holds.
func (s *Service) CountHouseholdBottles(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountHouseholdBottles(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("count household bottles: %w", err)
	}
	return n, nil
}

// DeleteHouseholdBottle removes a cellar holding owned by the household.
// Missing or foreign-owned rows are a no-op (idempotent delete).
func (s *Service) DeleteHouseholdBottle(ctx context.Context, householdBottleID, householdID int64) error {
	_, err := s.q.DeleteHouseholdBottle(ctx, sqlc.DeleteHouseholdBottleParams{HouseholdBottleID: householdBottleID, HouseholdID: householdID})
	return err
}

// MergeHouseholdStock folds the source household's pantry and cellar rows
// into the target household, as part of an invite-accept merge. Rows whose
// item/bottle already exists at the target sum quantities and keep the most
// recent non-null dates; non-colliding rows are repointed. Callers must run
// this inside the invite-accept unit of work so the reassignment commits
// atomically with the household switch. Meal plans and grocery lists are
// reassigned by their own domains.
func (s *Service) MergeHouseholdStock(ctx context.Context, fromHouseholdID, toHouseholdID int64, by string) error {
	if fromHouseholdID == toHouseholdID {
		return nil
	}
	mergeItems := sqlc.MergeHouseholdItemConflictsParams{FromHouseholdID: fromHouseholdID, ToHouseholdID: toHouseholdID, UpdatedBy: by}
	if err := s.q.MergeHouseholdItemConflicts(ctx, mergeItems); err != nil {
		return fmt.Errorf("merge household stock: %w", domainerr.FromStorage(err))
	}
	moveItems := sqlc.ReassignHouseholdItemsParams{FromHouseholdID: fromHouseholdID, ToHouseholdID: toHouseholdID, UpdatedBy: by}
	if err := s.q.ReassignHouseholdItems(ctx, moveItems); err != nil {
		return fmt.Errorf("merge household stock: %w", domainerr.FromStorage(err))
	}
	delItems := sqlc.DeleteMergedHouseholdItemsParams{FromHouseholdID: fromHouseholdID, ToHouseholdID: toHouseholdID}
	if err := s.q.DeleteMergedHouseholdItems(ctx, delItems); err != nil {
		return fmt.Errorf("merge household stock: %w", domainerr.FromStorage(err))
	}
	mergeBottles := sqlc.MergeHouseholdBottleConflictsParams{FromHouseholdID: fromHouseholdID, ToHouseholdID: toHouseholdID, UpdatedBy: by}
	if err := s.q.MergeHouseholdBottleConflicts(ctx, mergeBottles); err != nil {
		return fmt.Errorf("merge household stock: %w", domainerr.FromStorage(err))
	}
	moveBottles := sqlc.ReassignHouseholdBottlesParams{FromHouseholdID: fromHouseholdID, ToHouseholdID: toHouseholdID, UpdatedBy: by}
	if err := s.q.ReassignHouseholdBottles(ctx, moveBottles); err != nil {
		return fmt.Errorf("merge household stock: %w", domainerr.FromStorage(err))
	}
	delBottles := sqlc.DeleteMergedHouseholdBottlesParams{FromHouseholdID: fromHouseholdID, ToHouseholdID: toHouseholdID}
	if err := s.q.DeleteMergedHouseholdBottles(ctx, delBottles); err != nil {
		return fmt.Errorf("merge household stock: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ItemFavorite is a user's personal favorite flag for a catalog item. It is
// independent of whether the household stocks the item.
type ItemFavorite struct {
	UserID     int64
	ItemID     int64
	IsFavorite bool
}

// SetItemFavorite creates or updates a user's item favorite.
func (s *Service) SetItemFavorite(ctx context.Context, userID, itemID int64, isFavorite bool, by string) (ItemFavorite, error) {
	row, err := s.q.SetUserItemFavorite(ctx, sqlc.SetUserItemFavoriteParams{
		UserID:     userID,
		ItemID:     itemID,
		IsFavorite: isFavorite,
		CreatedBy:  by,
	})
	if err != nil {
		return ItemFavorite{}, fmt.Errorf("set item favorite: %w", domainerr.FromStorage(err))
	}
	return ItemFavorite{UserID: row.UserID, ItemID: row.ItemID, IsFavorite: row.IsFavorite}, nil
}

// GetItemFavorite reports whether the user favorited the item; a missing
// preference row means not favorited.
func (s *Service) GetItemFavorite(ctx context.Context, userID, itemID int64) (bool, error) {
	favs, err := s.ListItemFavorites(ctx, userID, []int64{itemID})
	if err != nil {
		return false, err
	}
	return favs[itemID], nil
}

// ListItemFavorites returns the user's favorite flags for a set of items in
// one query, keyed by item ID. Items without a preference row map to false.
func (s *Service) ListItemFavorites(ctx context.Context, userID int64, itemIDs []int64) (map[int64]bool, error) {
	rows, err := s.q.ListUserItemFavorites(ctx, sqlc.ListUserItemFavoritesParams{UserID: userID, ItemIds: itemIDs})
	if err != nil {
		return nil, fmt.Errorf("list item favorites: %w", err)
	}
	out := make(map[int64]bool, len(rows))
	for _, r := range rows {
		out[r.ItemID] = r.IsFavorite
	}
	return out, nil
}

// DeleteItemFavorite removes a user's item favorite; a missing row is a
// no-op.
func (s *Service) DeleteItemFavorite(ctx context.Context, userID, itemID int64) error {
	return s.q.DeleteUserItemFavorite(ctx, sqlc.DeleteUserItemFavoriteParams{UserID: userID, ItemID: itemID})
}

// BottleFavorite is a user's personal favorite flag for a bottle.
type BottleFavorite struct {
	UserID     int64
	BottleID   int64
	IsFavorite bool
}

// SetBottleFavorite creates or updates a user's bottle favorite.
func (s *Service) SetBottleFavorite(ctx context.Context, userID, bottleID int64, isFavorite bool, by string) (BottleFavorite, error) {
	row, err := s.q.SetUserBottleFavorite(ctx, sqlc.SetUserBottleFavoriteParams{
		UserID:     userID,
		BottleID:   bottleID,
		IsFavorite: isFavorite,
		CreatedBy:  by,
	})
	if err != nil {
		return BottleFavorite{}, fmt.Errorf("set bottle favorite: %w", domainerr.FromStorage(err))
	}
	return BottleFavorite{UserID: row.UserID, BottleID: row.BottleID, IsFavorite: row.IsFavorite}, nil
}

// GetBottleFavorite reports whether the user favorited the bottle; a missing
// preference row means not favorited.
func (s *Service) GetBottleFavorite(ctx context.Context, userID, bottleID int64) (bool, error) {
	favs, err := s.ListBottleFavorites(ctx, userID, []int64{bottleID})
	if err != nil {
		return false, err
	}
	return favs[bottleID], nil
}

// ListBottleFavorites returns the user's favorite flags for a set of bottles
// in one query, keyed by bottle ID. Bottles without a preference row map to
// false.
func (s *Service) ListBottleFavorites(ctx context.Context, userID int64, bottleIDs []int64) (map[int64]bool, error) {
	rows, err := s.q.ListUserBottleFavorites(ctx, sqlc.ListUserBottleFavoritesParams{UserID: userID, BottleIds: bottleIDs})
	if err != nil {
		return nil, fmt.Errorf("list bottle favorites: %w", err)
	}
	out := make(map[int64]bool, len(rows))
	for _, r := range rows {
		out[r.BottleID] = r.IsFavorite
	}
	return out, nil
}

// DeleteBottleFavorite removes a user's bottle favorite; a missing row is a
// no-op.
func (s *Service) DeleteBottleFavorite(ctx context.Context, userID, bottleID int64) error {
	return s.q.DeleteUserBottleFavorite(ctx, sqlc.DeleteUserBottleFavoriteParams{UserID: userID, BottleID: bottleID})
}

// RecipeFavorite is a user's favorite flag for a recipe.
type RecipeFavorite struct {
	UserID     int64
	RecipeID   int64
	IsFavorite bool
}

// SetRecipeFavorite creates or updates a user's recipe favorite.
func (s *Service) SetRecipeFavorite(ctx context.Context, userID, recipeID int64, isFavorite bool, by string) (RecipeFavorite, error) {
	row, err := s.q.UpsertRecipeFavorite(ctx, sqlc.UpsertRecipeFavoriteParams{
		UserID:     userID,
		RecipeID:   recipeID,
		IsFavorite: isFavorite,
		CreatedBy:  by,
		UpdatedBy:  textOrNull(by),
	})
	if err != nil {
		return RecipeFavorite{}, fmt.Errorf("set recipe favorite: %w", err)
	}
	return RecipeFavorite{UserID: row.UserID, RecipeID: row.RecipeID, IsFavorite: row.IsFavorite}, nil
}

// GetRecipeFavorite returns a user's favorite preference for a recipe,
// or domainerr.ErrNotFound if no preference exists yet.
func (s *Service) GetRecipeFavorite(ctx context.Context, userID, recipeID int64) (RecipeFavorite, error) {
	row, err := s.q.GetRecipeFavorite(ctx, sqlc.GetRecipeFavoriteParams{UserID: userID, RecipeID: recipeID})
	if err != nil {
		return RecipeFavorite{}, fmt.Errorf("get recipe favorite: %w", domainerr.FromStorage(err))
	}
	return RecipeFavorite{UserID: row.UserID, RecipeID: row.RecipeID, IsFavorite: row.IsFavorite}, nil
}

// ListRecipeFavorites returns a user's favorite flags for a set of recipes
// in a single query.
func (s *Service) ListRecipeFavorites(ctx context.Context, userID int64, recipeIDs []int64) ([]RecipeFavorite, error) {
	rows, err := s.q.ListRecipeFavorites(ctx, sqlc.ListRecipeFavoritesParams{UserID: userID, RecipeIds: recipeIDs})
	if err != nil {
		return nil, fmt.Errorf("list recipe favorites: %w", err)
	}
	out := make([]RecipeFavorite, len(rows))
	for i := range rows {
		out[i] = RecipeFavorite{UserID: rows[i].UserID, RecipeID: rows[i].RecipeID, IsFavorite: rows[i].IsFavorite}
	}
	return out, nil
}

// DeleteRecipeFavorite removes a user's recipe favorite.
func (s *Service) DeleteRecipeFavorite(ctx context.Context, userID, recipeID int64) error {
	return s.q.DeleteRecipeFavorite(ctx, sqlc.DeleteRecipeFavoriteParams{UserID: userID, RecipeID: recipeID})
}

func toHouseholdItem(row sqlc.UserprefsHouseholdItem) (HouseholdItem, error) {
	hi := HouseholdItem{
		HouseholdItemID: row.HouseholdItemID,
		HouseholdID:     row.HouseholdID,
		ItemID:          row.ItemID,
		Notes:           row.Notes.String,
	}
	if row.CurrentQty.Valid {
		f8, err := row.CurrentQty.Float64Value()
		if err != nil {
			return HouseholdItem{}, fmt.Errorf("household item %d current_qty: %w", row.HouseholdItemID, err)
		}
		hi.CurrentQty = f8.Float64
	}
	if row.MinQty.Valid {
		f8, err := row.MinQty.Float64Value()
		if err != nil {
			return HouseholdItem{}, fmt.Errorf("household item %d min_qty: %w", row.HouseholdItemID, err)
		}
		v := f8.Float64
		hi.MinQty = &v
	}
	if row.PurchaseAt.Valid {
		v := row.PurchaseAt.Time
		hi.PurchaseAt = &v
	}
	if row.ExpiresAt.Valid {
		v := row.ExpiresAt.Time
		hi.ExpiresAt = &v
	}
	return hi, nil
}

func toHouseholdBottle(row sqlc.UserprefsHouseholdBottle) (HouseholdBottle, error) {
	hb := HouseholdBottle{
		HouseholdBottleID: row.HouseholdBottleID,
		HouseholdID:       row.HouseholdID,
		BottleID:          row.BottleID,
		Quantity:          row.Quantity,
		Location:          row.Location.String,
		Notes:             row.Notes.String,
	}
	if row.BottleNumber.Valid {
		v := row.BottleNumber.Int32
		hb.BottleNumber = &v
	}
	if row.PurchaseAt.Valid {
		v := row.PurchaseAt.Time
		hb.PurchaseAt = &v
	}
	if row.PurchasePrice.Valid {
		f8, err := row.PurchasePrice.Float64Value()
		if err != nil {
			return HouseholdBottle{}, fmt.Errorf("household bottle %d purchase_price: %w", row.HouseholdBottleID, err)
		}
		v := f8.Float64
		hb.PurchasePrice = &v
	}
	if row.StorageTemp.Valid {
		f8, err := row.StorageTemp.Float64Value()
		if err != nil {
			return HouseholdBottle{}, fmt.Errorf("household bottle %d storage_temp: %w", row.HouseholdBottleID, err)
		}
		v := f8.Float64
		hb.StorageTemp = &v
	}
	return hb, nil
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func numericFromFloat64(f float64) (pgtype.Numeric, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: value is not finite", f)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: %w", f, err)
	}
	n.Valid = true
	return n, nil
}

func optNumeric(f *float64) (pgtype.Numeric, error) {
	if f == nil {
		return pgtype.Numeric{}, nil
	}
	return numericFromFloat64(*f)
}

func optInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func optTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
