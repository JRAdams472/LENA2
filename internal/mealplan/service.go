// Package mealplan owns per-user meal plans, their slots and any
// slot-level item overrides. Recipe and inventory details are resolved
// by the BFF, not joined in SQL.
package mealplan

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/mealplan/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
)

// Service provides meal planning operations.
type Service struct {
	q sqlc.Querier
}

// NewService creates a mealplan Service using the given connection pool.
func NewService(pool dbtx.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

// MealPlan is a user's weekly plan.
type MealPlan struct {
	MealPlanID         int64
	UserID             int64
	Name               string
	WeekStartDate      time.Time
	WeekStartDayOfWeek int16
	IsActive           bool
}

// CreateMealPlan creates a new weekly plan for the user.
func (s *Service) CreateMealPlan(ctx context.Context, arg MealPlan, by string) (MealPlan, error) {
	row, err := s.q.CreateMealPlan(ctx, sqlc.CreateMealPlanParams{
		UserID:             arg.UserID,
		Name:               arg.Name,
		WeekStartDate:      pgtype.Date{Time: arg.WeekStartDate, Valid: true},
		WeekStartDayOfWeek: arg.WeekStartDayOfWeek,
		IsActive:           arg.IsActive,
		CreatedBy:          by,
		UpdatedBy:          textOrNull(by),
	})
	if err != nil {
		return MealPlan{}, fmt.Errorf("create meal plan: %w", err)
	}
	return toMealPlan(row), nil
}

// GetMealPlanByID returns a plan owned by the user.
func (s *Service) GetMealPlanByID(ctx context.Context, mealPlanID, userID int64) (MealPlan, error) {
	row, err := s.q.GetMealPlanByID(ctx, sqlc.GetMealPlanByIDParams{MealPlanID: mealPlanID, UserID: userID})
	if err != nil {
		return MealPlan{}, fmt.Errorf("get meal plan: %w", err)
	}
	return toMealPlan(row), nil
}

// ListMealPlans returns a user's plans ordered by week.
func (s *Service) ListMealPlans(ctx context.Context, userID int64, limit, offset int32) ([]MealPlan, error) {
	rows, err := s.q.ListMealPlans(ctx, sqlc.ListMealPlansParams{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list meal plans: %w", err)
	}
	out := make([]MealPlan, len(rows))
	for i := range rows {
		out[i] = toMealPlan(rows[i])
	}
	return out, nil
}

// CountMealPlans returns the total number of plans owned by the user.
func (s *Service) CountMealPlans(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountMealPlans(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count meal plans: %w", err)
	}
	return n, nil
}

// UpdateMealPlan modifies a user's plan.
func (s *Service) UpdateMealPlan(ctx context.Context, mealPlanID, userID int64, arg MealPlan, by string) error {
	return s.q.UpdateMealPlan(ctx, sqlc.UpdateMealPlanParams{
		MealPlanID:         mealPlanID,
		UserID:             userID,
		Name:               arg.Name,
		WeekStartDate:      pgtype.Date{Time: arg.WeekStartDate, Valid: true},
		WeekStartDayOfWeek: arg.WeekStartDayOfWeek,
		IsActive:           arg.IsActive,
		UpdatedBy:          textOrNull(by),
	})
}

// DeleteMealPlan removes a plan owned by the user.
func (s *Service) DeleteMealPlan(ctx context.Context, mealPlanID, userID int64) error {
	return s.q.DeleteMealPlan(ctx, sqlc.DeleteMealPlanParams{MealPlanID: mealPlanID, UserID: userID})
}

// MealSlot is a single meal within a plan.
type MealSlot struct {
	SlotID          int64
	MealPlanID      int64
	DayOfWeek       int16
	MealType        string
	RecipeID        *int64
	Servings        *int32
	ReplacementNote string
}

// AddMealSlot adds a slot to a plan.
func (s *Service) AddMealSlot(ctx context.Context, arg MealSlot, by string) (MealSlot, error) {
	row, err := s.q.AddMealSlot(ctx, sqlc.AddMealSlotParams{
		MealPlanID:      arg.MealPlanID,
		DayOfWeek:       arg.DayOfWeek,
		MealType:        arg.MealType,
		RecipeID:        optInt8(arg.RecipeID),
		Servings:        optInt4(arg.Servings),
		ReplacementNote: textOrNull(arg.ReplacementNote),
		CreatedBy:       by,
		UpdatedBy:       textOrNull(by),
	})
	if err != nil {
		return MealSlot{}, fmt.Errorf("add meal slot: %w", err)
	}
	return toMealSlot(row), nil
}

// GetMealSlotByID returns a slot by id.
func (s *Service) GetMealSlotByID(ctx context.Context, slotID int64) (MealSlot, error) {
	row, err := s.q.GetMealSlotByID(ctx, slotID)
	if err != nil {
		return MealSlot{}, fmt.Errorf("get meal slot: %w", err)
	}
	return toMealSlot(row), nil
}

// ListMealSlotsForPlan returns all slots for a plan, ordered by day and type.
func (s *Service) ListMealSlotsForPlan(ctx context.Context, mealPlanID int64) ([]MealSlot, error) {
	rows, err := s.q.ListMealSlotsForPlan(ctx, mealPlanID)
	if err != nil {
		return nil, fmt.Errorf("list meal slots: %w", err)
	}
	out := make([]MealSlot, len(rows))
	for i := range rows {
		out[i] = toMealSlot(rows[i])
	}
	return out, nil
}

// UpdateMealSlot updates a slot's recipe, servings or note.
func (s *Service) UpdateMealSlot(ctx context.Context, slotID int64, arg MealSlot, by string) error {
	return s.q.UpdateMealSlot(ctx, sqlc.UpdateMealSlotParams{
		SlotID:          slotID,
		DayOfWeek:       arg.DayOfWeek,
		MealType:        arg.MealType,
		RecipeID:        optInt8(arg.RecipeID),
		Servings:        optInt4(arg.Servings),
		ReplacementNote: textOrNull(arg.ReplacementNote),
		UpdatedBy:       textOrNull(by),
	})
}

// DeleteMealSlot removes a slot.
func (s *Service) DeleteMealSlot(ctx context.Context, slotID int64) error {
	return s.q.DeleteMealSlot(ctx, slotID)
}

// MealSlotItem is an item override attached to a slot.
type MealSlotItem struct {
	SlotItemID   int64
	SlotID       int64
	ItemID       *int64
	Quantity     float64
	Unit         string
	IsFromRecipe bool
}

// AddMealSlotItem adds an item override to a slot.
func (s *Service) AddMealSlotItem(ctx context.Context, arg MealSlotItem, by string) (MealSlotItem, error) {
	qty, err := numericFromFloat64(arg.Quantity)
	if err != nil {
		return MealSlotItem{}, fmt.Errorf("add meal slot item: %w", err)
	}
	row, err := s.q.AddMealSlotItem(ctx, sqlc.AddMealSlotItemParams{
		SlotID:       arg.SlotID,
		ItemID:       optInt8(arg.ItemID),
		Quantity:     qty,
		Unit:         arg.Unit,
		IsFromRecipe: arg.IsFromRecipe,
		CreatedBy:    by,
		UpdatedBy:    textOrNull(by),
	})
	if err != nil {
		return MealSlotItem{}, fmt.Errorf("add meal slot item: %w", err)
	}
	msi, err := toMealSlotItem(row)
	if err != nil {
		return MealSlotItem{}, fmt.Errorf("add meal slot item: %w", err)
	}
	return msi, nil
}

// ListMealSlotItems returns all item overrides for a slot.
func (s *Service) ListMealSlotItems(ctx context.Context, slotID int64) ([]MealSlotItem, error) {
	rows, err := s.q.ListMealSlotItems(ctx, slotID)
	if err != nil {
		return nil, fmt.Errorf("list meal slot items: %w", err)
	}
	return toMealSlotItems(rows)
}

// ListMealSlotItemsByPlan returns all item overrides across every slot of a
// plan in a single query.
func (s *Service) ListMealSlotItemsByPlan(ctx context.Context, mealPlanID int64) ([]MealSlotItem, error) {
	rows, err := s.q.ListMealSlotItemsByPlan(ctx, mealPlanID)
	if err != nil {
		return nil, fmt.Errorf("list meal slot items by plan: %w", err)
	}
	return toMealSlotItems(rows)
}

func toMealSlotItems(rows []sqlc.MealplanMealSlotItem) ([]MealSlotItem, error) {
	out := make([]MealSlotItem, len(rows))
	for i := range rows {
		msi, err := toMealSlotItem(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = msi
	}
	return out, nil
}

// DeleteMealSlotItem removes an item override.
func (s *Service) DeleteMealSlotItem(ctx context.Context, slotItemID int64) error {
	return s.q.DeleteMealSlotItem(ctx, slotItemID)
}

func toMealPlan(row sqlc.MealplanMealPlan) MealPlan {
	return MealPlan{
		MealPlanID:         row.MealPlanID,
		UserID:             row.UserID,
		Name:               row.Name,
		WeekStartDate:      row.WeekStartDate.Time,
		WeekStartDayOfWeek: row.WeekStartDayOfWeek,
		IsActive:           row.IsActive,
	}
}

func toMealSlot(row sqlc.MealplanMealSlot) MealSlot {
	ms := MealSlot{
		SlotID:     row.SlotID,
		MealPlanID: row.MealPlanID,
		DayOfWeek:  row.DayOfWeek,
		MealType:   row.MealType,
		Servings:   nil,
	}
	if row.RecipeID.Valid {
		v := row.RecipeID.Int64
		ms.RecipeID = &v
	}
	if row.Servings.Valid {
		v := row.Servings.Int32
		ms.Servings = &v
	}
	ms.ReplacementNote = row.ReplacementNote.String
	return ms
}

func toMealSlotItem(row sqlc.MealplanMealSlotItem) (MealSlotItem, error) {
	msi := MealSlotItem{
		SlotItemID:   row.SlotItemID,
		SlotID:       row.SlotID,
		Unit:         row.Unit,
		IsFromRecipe: row.IsFromRecipe,
	}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		msi.ItemID = &v
	}
	if row.Quantity.Valid {
		f8, err := row.Quantity.Float64Value()
		if err != nil {
			return MealSlotItem{}, fmt.Errorf("slot item %d quantity: %w", row.SlotItemID, err)
		}
		msi.Quantity = f8.Float64
	}
	return msi, nil
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

func optInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func optInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
