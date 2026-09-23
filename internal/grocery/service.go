// Package grocery owns per-user grocery lists and their items. Meal-plan
// expansion and pantry-stock subtraction are composed by the BFF, which
// is the only layer allowed to read across domains.
package grocery

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/grocery/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Service provides grocery list operations.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates a grocery Service using the given connection pool. The
// querier resolves a ctx-carried transaction first (see dbtx.ContextExecer)
// so calls made inside a UnitOfWork join that transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "grocery")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "grocery"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "grocery")) }
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
		return fmt.Errorf("grocery: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// GroceryList is a user's generated shopping list.
type GroceryList struct {
	GroceryListID int64
	UserID        int64
	MealPlanID    *int64
	GeneratedAt   time.Time
}

// CreateGroceryList creates an empty grocery list for a user.
func (s *Service) CreateGroceryList(ctx context.Context, userID int64, mealPlanID *int64, by string) (GroceryList, error) {
	row, err := s.q.CreateGroceryList(ctx, sqlc.CreateGroceryListParams{
		UserID:     userID,
		MealPlanID: optInt8(mealPlanID),
		CreatedBy:  by,
		UpdatedBy:  textOrNull(by),
	})
	if err != nil {
		return GroceryList{}, fmt.Errorf("create grocery list: %w", err)
	}
	return toGroceryList(row), nil
}

// GetGroceryListByID returns a grocery list owned by the user.
func (s *Service) GetGroceryListByID(ctx context.Context, groceryListID, userID int64) (GroceryList, error) {
	row, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: groceryListID, UserID: userID})
	if err != nil {
		return GroceryList{}, fmt.Errorf("get grocery list: %w", domainerr.FromStorage(err))
	}
	return toGroceryList(row), nil
}

// ListGroceryLists returns a user's grocery lists.
func (s *Service) ListGroceryLists(ctx context.Context, userID int64, limit, offset int32) ([]GroceryList, error) {
	rows, err := s.q.ListGroceryLists(ctx, sqlc.ListGroceryListsParams{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list grocery lists: %w", err)
	}
	out := make([]GroceryList, len(rows))
	for i := range rows {
		out[i] = toGroceryList(rows[i])
	}
	return out, nil
}

// CountGroceryLists returns the total number of lists owned by the user.
func (s *Service) CountGroceryLists(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountGroceryLists(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count grocery lists: %w", err)
	}
	return n, nil
}

// DeleteGroceryList removes a grocery list owned by the user.
func (s *Service) DeleteGroceryList(ctx context.Context, groceryListID, userID int64) error {
	return s.q.DeleteGroceryList(ctx, sqlc.DeleteGroceryListParams{GroceryListID: groceryListID, UserID: userID})
}

// GroceryListItem is a single item on a shopping list. ItemID is the
// branded catalog item; IngredientID optionally points at the
// brand-agnostic ingredient abstraction.
type GroceryListItem struct {
	GroceryListItemID int64
	GroceryListID     int64
	ItemID            *int64
	IngredientID      *int64
	ManualItemName    string
	QuantityNeeded    float64
	UnitID            *int64
	Source            string
	IsChecked         bool
}

// AddGroceryListItem adds an item to a grocery list owned by the user;
// the ownership check fails with the same not-found error as
// GetGroceryListByID.
func (s *Service) AddGroceryListItem(ctx context.Context, arg GroceryListItem, userID int64, by string) (GroceryListItem, error) {
	if _, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: arg.GroceryListID, UserID: userID}); err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", domainerr.FromStorage(err))
	}
	qty, err := numericFromFloat64(arg.QuantityNeeded)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	row, err := s.q.AddGroceryListItem(ctx, sqlc.AddGroceryListItemParams{
		GroceryListID:  arg.GroceryListID,
		ItemID:         optInt8(arg.ItemID),
		IngredientID:   optInt8(arg.IngredientID),
		ManualItemName: textOrNull(arg.ManualItemName),
		QuantityNeeded: qty,
		UnitID:         optInt8(arg.UnitID),
		Source:         arg.Source,
		IsChecked:      arg.IsChecked,
		CreatedBy:      by,
		UpdatedBy:      textOrNull(by),
	})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	return gli, nil
}

// ListGroceryListItems returns all items on a list owned by the user.
func (s *Service) ListGroceryListItems(ctx context.Context, groceryListID, userID int64) ([]GroceryListItem, error) {
	rows, err := s.q.ListGroceryListItems(ctx, sqlc.ListGroceryListItemsParams{GroceryListID: groceryListID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("list grocery list items: %w", err)
	}
	out := make([]GroceryListItem, len(rows))
	for i := range rows {
		gli, err := toGroceryListItem(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list grocery list items: %w", err)
		}
		out[i] = gli
	}
	return out, nil
}

// ListGroceryListItemsByLists returns all items across a set of lists
// owned by the user in a single query.
func (s *Service) ListGroceryListItemsByLists(ctx context.Context, groceryListIDs []int64, userID int64) ([]GroceryListItem, error) {
	rows, err := s.q.ListGroceryListItemsByLists(ctx, sqlc.ListGroceryListItemsByListsParams{GroceryListIds: groceryListIDs, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("list grocery list items by lists: %w", err)
	}
	out := make([]GroceryListItem, len(rows))
	for i := range rows {
		gli, err := toGroceryListItem(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list grocery list items by lists: %w", err)
		}
		out[i] = gli
	}
	return out, nil
}

// GetGroceryListItemByID returns an item on a list owned by the user.
func (s *Service) GetGroceryListItemByID(ctx context.Context, groceryListItemID, userID int64) (GroceryListItem, error) {
	row, err := s.q.GetGroceryListItemByID(ctx, sqlc.GetGroceryListItemByIDParams{GroceryListItemID: groceryListItemID, UserID: userID})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("get grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("get grocery list item: %w", err)
	}
	return gli, nil
}

// UpdateGroceryListItem modifies an item on a list owned by the user.
func (s *Service) UpdateGroceryListItem(ctx context.Context, groceryListItemID, userID int64, arg GroceryListItem, by string) error {
	qty, err := numericFromFloat64(arg.QuantityNeeded)
	if err != nil {
		return fmt.Errorf("update grocery list item: %w", err)
	}
	return s.q.UpdateGroceryListItem(ctx, sqlc.UpdateGroceryListItemParams{
		GroceryListItemID: groceryListItemID,
		UserID:            userID,
		ItemID:            optInt8(arg.ItemID),
		IngredientID:      optInt8(arg.IngredientID),
		ManualItemName:    textOrNull(arg.ManualItemName),
		QuantityNeeded:    qty,
		UnitID:            optInt8(arg.UnitID),
		Source:            arg.Source,
		IsChecked:         arg.IsChecked,
		UpdatedBy:         textOrNull(by),
	})
}

// DeleteGroceryListItem removes an item from a list owned by the user.
func (s *Service) DeleteGroceryListItem(ctx context.Context, groceryListItemID, userID int64) error {
	return s.q.DeleteGroceryListItem(ctx, sqlc.DeleteGroceryListItemParams{GroceryListItemID: groceryListItemID, UserID: userID})
}

// ToggleGroceryListItemChecked flips the checked state of an item and
// returns the post-toggle row in one atomic statement.
func (s *Service) ToggleGroceryListItemChecked(ctx context.Context, groceryListItemID, userID int64, by string) (GroceryListItem, error) {
	row, err := s.q.ToggleGroceryListItemChecked(ctx, sqlc.ToggleGroceryListItemCheckedParams{
		GroceryListItemID: groceryListItemID,
		UserID:            userID,
		UpdatedBy:         textOrNull(by),
	})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("toggle grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("toggle grocery list item: %w", err)
	}
	return gli, nil
}

// AddGroceryListItems inserts a batch of items onto a single list owned
// by the user. All rows share one GroceryListID; callers composing the
// batch with other writes wrap the call in a unit of work, which the
// ctx-carried transaction joins automatically.
func (s *Service) AddGroceryListItems(ctx context.Context, items []GroceryListItem, userID int64, by string) ([]GroceryListItem, error) {
	if len(items) == 0 {
		return nil, nil
	}
	listID := items[0].GroceryListID
	for _, it := range items[1:] {
		if it.GroceryListID != listID {
			return nil, fmt.Errorf("add grocery list items: mixed list ids %d and %d", listID, it.GroceryListID)
		}
	}
	if _, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: listID, UserID: userID}); err != nil {
		return nil, fmt.Errorf("add grocery list items: %w", domainerr.FromStorage(err))
	}
	out := make([]GroceryListItem, 0, len(items))
	for _, it := range items {
		qty, err := numericFromFloat64(it.QuantityNeeded)
		if err != nil {
			return nil, fmt.Errorf("add grocery list items: %w", err)
		}
		row, err := s.q.AddGroceryListItem(ctx, sqlc.AddGroceryListItemParams{
			GroceryListID:  it.GroceryListID,
			ItemID:         optInt8(it.ItemID),
			IngredientID:   optInt8(it.IngredientID),
			ManualItemName: textOrNull(it.ManualItemName),
			QuantityNeeded: qty,
			UnitID:         optInt8(it.UnitID),
			Source:         it.Source,
			IsChecked:      it.IsChecked,
			CreatedBy:      by,
			UpdatedBy:      textOrNull(by),
		})
		if err != nil {
			return nil, fmt.Errorf("add grocery list items: %w", err)
		}
		gli, err := toGroceryListItem(row)
		if err != nil {
			return nil, fmt.Errorf("add grocery list items: %w", err)
		}
		out = append(out, gli)
	}
	return out, nil
}

func toGroceryList(row sqlc.GroceryGroceryList) GroceryList {
	gl := GroceryList{
		GroceryListID: row.GroceryListID,
		UserID:        row.UserID,
		GeneratedAt:   row.GeneratedAt,
	}
	if row.MealPlanID.Valid {
		v := row.MealPlanID.Int64
		gl.MealPlanID = &v
	}
	return gl
}

func toGroceryListItem(row sqlc.GroceryGroceryListItem) (GroceryListItem, error) {
	gli := GroceryListItem{
		GroceryListItemID: row.GroceryListItemID,
		GroceryListID:     row.GroceryListID,
		ManualItemName:    row.ManualItemName.String,
		Source:            row.Source,
		IsChecked:         row.IsChecked,
	}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		gli.ItemID = &v
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		gli.IngredientID = &v
	}
	if row.UnitID.Valid {
		v := row.UnitID.Int64
		gli.UnitID = &v
	}
	if row.QuantityNeeded.Valid {
		f8, err := row.QuantityNeeded.Float64Value()
		if err != nil {
			return GroceryListItem{}, fmt.Errorf("grocery list item %d quantity: %w", row.GroceryListItemID, err)
		}
		gli.QuantityNeeded = f8.Float64
	}
	return gli, nil
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

func optInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
