// Package userprefs owns per-user state: pantry stock, wine cellar
// holdings, and recipe favorites. It touches tables in the inventory,
// wine and recipe schemas but only the per-user tables belonging to its
// own module.
package userprefs

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/userprefs/sqlc"
)

// Service provides per-user preference and holding operations.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
}

// NewService creates a userprefs Service using the given connection pool.
func NewService(pool dbtx.Pool) *Service {
	return &Service{q: sqlc.New(pool), pool: pool}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	return &Service{q: sqlc.New(tx), pool: s.pool}
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// UserItem is one pantry row for a user.
type UserItem struct {
	UserItemID int64
	UserID     int64
	ItemID     int64
	CurrentQty float64
	MinQty     *float64
	PurchaseAt *time.Time
	ExpiresAt  *time.Time
	Notes      string
	IsFavorite bool
}

// UpsertUserItem creates or updates a user's pantry item.
func (s *Service) UpsertUserItem(ctx context.Context, arg UserItem, by string) (UserItem, error) {
	currentQty, err := numericFromFloat64(arg.CurrentQty)
	if err != nil {
		return UserItem{}, fmt.Errorf("upsert user item: %w", err)
	}
	minQty, err := optNumeric(arg.MinQty)
	if err != nil {
		return UserItem{}, fmt.Errorf("upsert user item: %w", err)
	}
	row, err := s.q.UpsertUserItem(ctx, sqlc.UpsertUserItemParams{
		UserID:     arg.UserID,
		ItemID:     arg.ItemID,
		CurrentQty: currentQty,
		MinQty:     minQty,
		PurchaseAt: optTimestamptz(arg.PurchaseAt),
		ExpiresAt:  optTimestamptz(arg.ExpiresAt),
		Notes:      textOrNull(arg.Notes),
		IsFavorite: arg.IsFavorite,
		CreatedBy:  by,
		UpdatedBy:  textOrNull(by),
	})
	if err != nil {
		return UserItem{}, fmt.Errorf("upsert user item: %w", err)
	}
	ui, err := toUserItem(row)
	if err != nil {
		return UserItem{}, fmt.Errorf("upsert user item: %w", err)
	}
	return ui, nil
}

// GetUserItemByID returns a pantry item owned by the user.
func (s *Service) GetUserItemByID(ctx context.Context, userItemID, userID int64) (UserItem, error) {
	row, err := s.q.GetUserItemByID(ctx, sqlc.GetUserItemByIDParams{UserItemID: userItemID, UserID: userID})
	if err != nil {
		return UserItem{}, fmt.Errorf("get user item: %w", err)
	}
	ui, err := toUserItem(row)
	if err != nil {
		return UserItem{}, fmt.Errorf("get user item: %w", err)
	}
	return ui, nil
}

// ListUserItems returns a user's pantry items.
func (s *Service) ListUserItems(ctx context.Context, userID int64, limit, offset int32) ([]UserItem, error) {
	rows, err := s.q.ListUserItems(ctx, sqlc.ListUserItemsParams{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list user items: %w", err)
	}
	out := make([]UserItem, len(rows))
	for i := range rows {
		ui, err := toUserItem(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list user items: %w", err)
		}
		out[i] = ui
	}
	return out, nil
}

// CountUserItems returns the total number of pantry items owned by the user.
func (s *Service) CountUserItems(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountUserItems(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count user items: %w", err)
	}
	return n, nil
}

// DeleteUserItem removes a pantry item owned by the user.
func (s *Service) DeleteUserItem(ctx context.Context, userItemID, userID int64) error {
	return s.q.DeleteUserItem(ctx, sqlc.DeleteUserItemParams{UserItemID: userItemID, UserID: userID})
}

// UserBottle is one cellar holding for a user.
type UserBottle struct {
	UserBottleID  int64
	UserID        int64
	BottleID      int64
	BottleNumber  *int32
	Quantity      int32
	PurchaseAt    *time.Time
	PurchasePrice *float64
	StorageTemp   *float64
	Location      string
	Notes         string
	IsFavorite    bool
}

// UpsertUserBottle creates or updates a user's wine holding.
func (s *Service) UpsertUserBottle(ctx context.Context, arg UserBottle, by string) (UserBottle, error) {
	price, err := optNumeric(arg.PurchasePrice)
	if err != nil {
		return UserBottle{}, fmt.Errorf("upsert user bottle: %w", err)
	}
	temp, err := optNumeric(arg.StorageTemp)
	if err != nil {
		return UserBottle{}, fmt.Errorf("upsert user bottle: %w", err)
	}
	row, err := s.q.UpsertUserBottle(ctx, sqlc.UpsertUserBottleParams{
		UserID:        arg.UserID,
		BottleID:      arg.BottleID,
		BottleNumber:  optInt4(arg.BottleNumber),
		Quantity:      arg.Quantity,
		PurchaseAt:    optTimestamptz(arg.PurchaseAt),
		PurchasePrice: price,
		StorageTemp:   temp,
		Location:      textOrNull(arg.Location),
		Notes:         textOrNull(arg.Notes),
		IsFavorite:    arg.IsFavorite,
		CreatedBy:     by,
		UpdatedBy:     textOrNull(by),
	})
	if err != nil {
		return UserBottle{}, fmt.Errorf("upsert user bottle: %w", err)
	}
	ub, err := toUserBottle(row)
	if err != nil {
		return UserBottle{}, fmt.Errorf("upsert user bottle: %w", err)
	}
	return ub, nil
}

// GetUserBottleByID returns a cellar holding owned by the user.
func (s *Service) GetUserBottleByID(ctx context.Context, userBottleID, userID int64) (UserBottle, error) {
	row, err := s.q.GetUserBottleByID(ctx, sqlc.GetUserBottleByIDParams{UserBottleID: userBottleID, UserID: userID})
	if err != nil {
		return UserBottle{}, fmt.Errorf("get user bottle: %w", err)
	}
	ub, err := toUserBottle(row)
	if err != nil {
		return UserBottle{}, fmt.Errorf("get user bottle: %w", err)
	}
	return ub, nil
}

// ListUserBottles returns a user's cellar holdings.
func (s *Service) ListUserBottles(ctx context.Context, userID int64, limit, offset int32) ([]UserBottle, error) {
	rows, err := s.q.ListUserBottles(ctx, sqlc.ListUserBottlesParams{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list user bottles: %w", err)
	}
	out := make([]UserBottle, len(rows))
	for i := range rows {
		ub, err := toUserBottle(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list user bottles: %w", err)
		}
		out[i] = ub
	}
	return out, nil
}

// CountUserBottles returns the total number of cellar holdings owned by the user.
func (s *Service) CountUserBottles(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountUserBottles(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count user bottles: %w", err)
	}
	return n, nil
}

// DeleteUserBottle removes a cellar holding owned by the user.
func (s *Service) DeleteUserBottle(ctx context.Context, userBottleID, userID int64) error {
	return s.q.DeleteUserBottle(ctx, sqlc.DeleteUserBottleParams{UserBottleID: userBottleID, UserID: userID})
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

// GetRecipeFavorite returns a user's favorite preference for a recipe.
func (s *Service) GetRecipeFavorite(ctx context.Context, userID, recipeID int64) (RecipeFavorite, error) {
	row, err := s.q.GetRecipeFavorite(ctx, sqlc.GetRecipeFavoriteParams{UserID: userID, RecipeID: recipeID})
	if err != nil {
		return RecipeFavorite{}, fmt.Errorf("get recipe favorite: %w", err)
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

func toUserItem(row sqlc.InventoryUserItem) (UserItem, error) {
	ui := UserItem{
		UserItemID: row.UserItemID,
		UserID:     row.UserID,
		ItemID:     row.ItemID,
		Notes:      row.Notes.String,
		IsFavorite: row.IsFavorite,
	}
	if row.CurrentQty.Valid {
		f8, err := row.CurrentQty.Float64Value()
		if err != nil {
			return UserItem{}, fmt.Errorf("user item %d current_qty: %w", row.UserItemID, err)
		}
		ui.CurrentQty = f8.Float64
	}
	if row.MinQty.Valid {
		f8, err := row.MinQty.Float64Value()
		if err != nil {
			return UserItem{}, fmt.Errorf("user item %d min_qty: %w", row.UserItemID, err)
		}
		v := f8.Float64
		ui.MinQty = &v
	}
	if row.PurchaseAt.Valid {
		v := row.PurchaseAt.Time
		ui.PurchaseAt = &v
	}
	if row.ExpiresAt.Valid {
		v := row.ExpiresAt.Time
		ui.ExpiresAt = &v
	}
	return ui, nil
}

func toUserBottle(row sqlc.WineUserBottle) (UserBottle, error) {
	ub := UserBottle{
		UserBottleID: row.UserBottleID,
		UserID:       row.UserID,
		BottleID:     row.BottleID,
		Quantity:     row.Quantity,
		Location:     row.Location.String,
		Notes:        row.Notes.String,
		IsFavorite:   row.IsFavorite,
	}
	if row.BottleNumber.Valid {
		v := row.BottleNumber.Int32
		ub.BottleNumber = &v
	}
	if row.PurchaseAt.Valid {
		v := row.PurchaseAt.Time
		ub.PurchaseAt = &v
	}
	if row.PurchasePrice.Valid {
		f8, err := row.PurchasePrice.Float64Value()
		if err != nil {
			return UserBottle{}, fmt.Errorf("user bottle %d purchase_price: %w", row.UserBottleID, err)
		}
		v := f8.Float64
		ub.PurchasePrice = &v
	}
	if row.StorageTemp.Valid {
		f8, err := row.StorageTemp.Float64Value()
		if err != nil {
			return UserBottle{}, fmt.Errorf("user bottle %d storage_temp: %w", row.UserBottleID, err)
		}
		v := f8.Float64
		ub.StorageTemp = &v
	}
	return ub, nil
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
