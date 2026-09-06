// Package grocery owns per-user grocery lists and their items. It may
// read a meal plan (same domain) and explicitly calls userprefs to see
// per-user stock, but it never joins to inventory.item or user_item in SQL.
package grocery

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/grocery/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
)

// Service provides grocery list operations.
type Service struct {
	q sqlc.Querier
}

// NewService creates a grocery Service using the given connection pool.
func NewService(pool dbtx.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
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
		return GroceryList{}, fmt.Errorf("get grocery list: %w", err)
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

// GroceryListItem is a single item on a shopping list.
type GroceryListItem struct {
	GroceryListItemID int64
	GroceryListID     int64
	ItemID            *int64
	ManualItemName    string
	QuantityNeeded    float64
	UnitOfMeasure     string
	Source            string
	IsChecked         bool
}

// AddGroceryListItem adds an item to a grocery list.
func (s *Service) AddGroceryListItem(ctx context.Context, arg GroceryListItem, by string) (GroceryListItem, error) {
	qty, err := numericFromFloat64(arg.QuantityNeeded)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	row, err := s.q.AddGroceryListItem(ctx, sqlc.AddGroceryListItemParams{
		GroceryListID:  arg.GroceryListID,
		ItemID:         optInt8(arg.ItemID),
		ManualItemName: textOrNull(arg.ManualItemName),
		QuantityNeeded: qty,
		UnitOfMeasure:  textOrNull(arg.UnitOfMeasure),
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

// ListGroceryListItems returns all items on a list.
func (s *Service) ListGroceryListItems(ctx context.Context, groceryListID int64) ([]GroceryListItem, error) {
	rows, err := s.q.ListGroceryListItems(ctx, groceryListID)
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

// ListGroceryListItemsByLists returns all items across a set of lists in a
// single query.
func (s *Service) ListGroceryListItemsByLists(ctx context.Context, groceryListIDs []int64) ([]GroceryListItem, error) {
	rows, err := s.q.ListGroceryListItemsByLists(ctx, groceryListIDs)
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

// GetGroceryListItemByID returns an item by its primary key.
func (s *Service) GetGroceryListItemByID(ctx context.Context, groceryListItemID int64) (GroceryListItem, error) {
	row, err := s.q.GetGroceryListItemByID(ctx, groceryListItemID)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("get grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("get grocery list item: %w", err)
	}
	return gli, nil
}

// UpdateGroceryListItem modifies an item.
func (s *Service) UpdateGroceryListItem(ctx context.Context, groceryListItemID int64, arg GroceryListItem, by string) error {
	qty, err := numericFromFloat64(arg.QuantityNeeded)
	if err != nil {
		return fmt.Errorf("update grocery list item: %w", err)
	}
	return s.q.UpdateGroceryListItem(ctx, sqlc.UpdateGroceryListItemParams{
		GroceryListItemID: groceryListItemID,
		ItemID:            optInt8(arg.ItemID),
		ManualItemName:    textOrNull(arg.ManualItemName),
		QuantityNeeded:    qty,
		UnitOfMeasure:     textOrNull(arg.UnitOfMeasure),
		Source:            arg.Source,
		IsChecked:         arg.IsChecked,
		UpdatedBy:         textOrNull(by),
	})
}

// DeleteGroceryListItem removes an item.
func (s *Service) DeleteGroceryListItem(ctx context.Context, groceryListItemID int64) error {
	return s.q.DeleteGroceryListItem(ctx, groceryListItemID)
}

// Generate creates a new grocery list and seeds it from a meal plan.
// Actual item totals and pantry subtraction are calculated in Go so the
// SQL stays free of business logic.
func (s *Service) Generate(ctx context.Context, userID int64, mealPlanID int64, by string) (GroceryList, error) {
	// Phase 4 provides the list container only; full aggregation against
	// recipe items and user stock is left for the BFF/resolver layer.
	return s.CreateGroceryList(ctx, userID, &mealPlanID, by)
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
		UnitOfMeasure:     row.UnitOfMeasure.String,
		Source:            row.Source,
		IsChecked:         row.IsChecked,
	}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		gli.ItemID = &v
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
