// Package mealplan owns household meal plans, their slots and any
// slot-level item overrides. Recipe and inventory details are resolved
// by the BFF, not joined in SQL.
package mealplan

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/mealplan/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Service provides meal planning operations.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates a mealplan Service using the given connection pool.
// The querier resolves a ctx-carried transaction first (see
// dbtx.ContextExecer) so calls made inside a UnitOfWork join that
// transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "mealplan")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "mealplan"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "mealplan")) }
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
		return fmt.Errorf("mealplan: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// MealPlan is a household's weekly plan.
type MealPlan struct {
	MealPlanID         int64
	HouseholdID        int64
	Name               string
	WeekStartDate      time.Time
	WeekStartDayOfWeek int16
	IsActive           bool
}

// CreateMealPlan creates a new weekly plan for the household.
func (s *Service) CreateMealPlan(ctx context.Context, arg MealPlan, by string) (MealPlan, error) {
	row, err := s.q.CreateMealPlan(ctx, sqlc.CreateMealPlanParams{
		HouseholdID:        arg.HouseholdID,
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

// GetMealPlanByID returns a plan owned by the household.
func (s *Service) GetMealPlanByID(ctx context.Context, mealPlanID, householdID int64) (MealPlan, error) {
	row, err := s.q.GetMealPlanByID(ctx, sqlc.GetMealPlanByIDParams{MealPlanID: mealPlanID, HouseholdID: householdID})
	if err != nil {
		return MealPlan{}, fmt.Errorf("get meal plan: %w", domainerr.FromStorage(err))
	}
	return toMealPlan(row), nil
}

// ListMealPlans returns a household's plans ordered by week.
func (s *Service) ListMealPlans(ctx context.Context, householdID int64, limit, offset int32) ([]MealPlan, error) {
	rows, err := s.q.ListMealPlans(ctx, sqlc.ListMealPlansParams{HouseholdID: householdID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list meal plans: %w", err)
	}
	out := make([]MealPlan, len(rows))
	for i := range rows {
		out[i] = toMealPlan(rows[i])
	}
	return out, nil
}

// CountMealPlans returns the total number of plans owned by the household.
func (s *Service) CountMealPlans(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountMealPlans(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("count meal plans: %w", err)
	}
	return n, nil
}

// UpdateMealPlan modifies a household's plan.
func (s *Service) UpdateMealPlan(ctx context.Context, mealPlanID, householdID int64, arg MealPlan, by string) error {
	return s.q.UpdateMealPlan(ctx, sqlc.UpdateMealPlanParams{
		MealPlanID:         mealPlanID,
		HouseholdID:        householdID,
		Name:               arg.Name,
		WeekStartDate:      pgtype.Date{Time: arg.WeekStartDate, Valid: true},
		WeekStartDayOfWeek: arg.WeekStartDayOfWeek,
		IsActive:           arg.IsActive,
		UpdatedBy:          textOrNull(by),
	})
}

// DeleteMealPlan removes a plan owned by the household.
func (s *Service) DeleteMealPlan(ctx context.Context, mealPlanID, householdID int64) error {
	return s.q.DeleteMealPlan(ctx, sqlc.DeleteMealPlanParams{MealPlanID: mealPlanID, HouseholdID: householdID})
}

// ReassignHousehold repoints every meal plan owned by the source household
// to the target household as part of an invite-accept merge. Callers run it
// inside the merge unit of work so the reassignment commits atomically with
// the household switch; a source with no plans is not an error.
func (s *Service) ReassignHousehold(ctx context.Context, fromHouseholdID, toHouseholdID int64, by string) error {
	if fromHouseholdID == toHouseholdID {
		return nil
	}
	if err := s.q.ReassignMealPlansToHousehold(ctx, sqlc.ReassignMealPlansToHouseholdParams{
		ToHouseholdID:   toHouseholdID,
		UpdatedBy:       by,
		FromHouseholdID: fromHouseholdID,
	}); err != nil {
		return fmt.Errorf("reassign meal plans: %w", domainerr.FromStorage(err))
	}
	return nil
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

// AddMealSlot adds a slot to a plan owned by the household; the ownership
// check fails with the same not-found error as GetMealPlanByID.
func (s *Service) AddMealSlot(ctx context.Context, arg MealSlot, householdID int64, by string) (MealSlot, error) {
	if _, err := s.q.GetMealPlanByID(ctx, sqlc.GetMealPlanByIDParams{MealPlanID: arg.MealPlanID, HouseholdID: householdID}); err != nil {
		return MealSlot{}, fmt.Errorf("add meal slot: %w", domainerr.FromStorage(err))
	}
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

// GetMealSlotByID returns a slot belonging to the household's plan.
func (s *Service) GetMealSlotByID(ctx context.Context, slotID, householdID int64) (MealSlot, error) {
	row, err := s.q.GetMealSlotByID(ctx, sqlc.GetMealSlotByIDParams{SlotID: slotID, HouseholdID: householdID})
	if err != nil {
		return MealSlot{}, fmt.Errorf("get meal slot: %w", domainerr.FromStorage(err))
	}
	return toMealSlot(row), nil
}

// ListMealSlotsForPlan returns all slots for a plan owned by the household,
// ordered by day and type.
func (s *Service) ListMealSlotsForPlan(ctx context.Context, mealPlanID, householdID int64) ([]MealSlot, error) {
	rows, err := s.q.ListMealSlotsForPlan(ctx, sqlc.ListMealSlotsForPlanParams{MealPlanID: mealPlanID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list meal slots: %w", err)
	}
	out := make([]MealSlot, len(rows))
	for i := range rows {
		out[i] = toMealSlot(rows[i])
	}
	return out, nil
}

// ListMealSlotsByPlans returns all slots for a set of plans owned by the
// household in a single query.
func (s *Service) ListMealSlotsByPlans(ctx context.Context, mealPlanIDs []int64, householdID int64) ([]MealSlot, error) {
	rows, err := s.q.ListMealSlotsByPlans(ctx, sqlc.ListMealSlotsByPlansParams{MealPlanIds: mealPlanIDs, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list meal slots by plans: %w", err)
	}
	out := make([]MealSlot, len(rows))
	for i := range rows {
		out[i] = toMealSlot(rows[i])
	}
	return out, nil
}

// UpdateMealSlot updates a slot's recipe, servings or note on a plan
// owned by the household.
func (s *Service) UpdateMealSlot(ctx context.Context, slotID, householdID int64, arg MealSlot, by string) error {
	return s.q.UpdateMealSlot(ctx, sqlc.UpdateMealSlotParams{
		SlotID:          slotID,
		HouseholdID:     householdID,
		DayOfWeek:       arg.DayOfWeek,
		MealType:        arg.MealType,
		RecipeID:        optInt8(arg.RecipeID),
		Servings:        optInt4(arg.Servings),
		ReplacementNote: textOrNull(arg.ReplacementNote),
		UpdatedBy:       textOrNull(by),
	})
}

// DeleteMealSlot removes a slot from a plan owned by the household.
func (s *Service) DeleteMealSlot(ctx context.Context, slotID, householdID int64) error {
	return s.q.DeleteMealSlot(ctx, sqlc.DeleteMealSlotParams{SlotID: slotID, HouseholdID: householdID})
}

// MealSlotItem is an item override attached to a slot.
type MealSlotItem struct {
	SlotItemID   int64
	SlotID       int64
	ItemID       *int64
	IngredientID *int64
	Quantity     float64
	UnitID       int64
	IsFromRecipe bool
}

// AddMealSlotItem adds an item override to a slot on a plan owned by the
// household; the ownership check fails with the same not-found error as
// GetMealSlotByID.
func (s *Service) AddMealSlotItem(ctx context.Context, arg MealSlotItem, householdID int64, by string) (MealSlotItem, error) {
	if _, err := s.q.GetMealSlotByID(ctx, sqlc.GetMealSlotByIDParams{SlotID: arg.SlotID, HouseholdID: householdID}); err != nil {
		return MealSlotItem{}, fmt.Errorf("add meal slot item: %w", domainerr.FromStorage(err))
	}
	qty, err := numericFromFloat64(arg.Quantity)
	if err != nil {
		return MealSlotItem{}, fmt.Errorf("add meal slot item: %w", err)
	}
	row, err := s.q.AddMealSlotItem(ctx, sqlc.AddMealSlotItemParams{
		SlotID:       arg.SlotID,
		ItemID:       optInt8(arg.ItemID),
		IngredientID: optInt8(arg.IngredientID),
		Quantity:     qty,
		UnitID:       arg.UnitID,
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

// ListMealSlotItems returns all item overrides for a slot on a plan owned
// by the household.
func (s *Service) ListMealSlotItems(ctx context.Context, slotID, householdID int64) ([]MealSlotItem, error) {
	rows, err := s.q.ListMealSlotItems(ctx, sqlc.ListMealSlotItemsParams{SlotID: slotID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list meal slot items: %w", err)
	}
	return toMealSlotItems(rows)
}

// ListMealSlotItemsByPlan returns all item overrides across every slot of
// a plan owned by the household in a single query.
func (s *Service) ListMealSlotItemsByPlan(ctx context.Context, mealPlanID, householdID int64) ([]MealSlotItem, error) {
	rows, err := s.q.ListMealSlotItemsByPlan(ctx, sqlc.ListMealSlotItemsByPlanParams{MealPlanID: mealPlanID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list meal slot items by plan: %w", err)
	}
	return toMealSlotItems(rows)
}

// ListMealSlotItemsByPlans returns all item overrides across every slot
// of a set of plans owned by the household in a single query.
func (s *Service) ListMealSlotItemsByPlans(ctx context.Context, mealPlanIDs []int64, householdID int64) ([]MealSlotItem, error) {
	rows, err := s.q.ListMealSlotItemsByPlans(ctx, sqlc.ListMealSlotItemsByPlansParams{MealPlanIds: mealPlanIDs, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list meal slot items by plans: %w", err)
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

// DeleteMealSlotItem removes an item override from a plan owned by the
// household.
func (s *Service) DeleteMealSlotItem(ctx context.Context, slotItemID, householdID int64) error {
	return s.q.DeleteMealSlotItem(ctx, sqlc.DeleteMealSlotItemParams{SlotItemID: slotItemID, HouseholdID: householdID})
}

// LastPlannedDates returns, for one household, the most recent plan week in
// which each recipe appeared. Recipes absent from the map were never
// planned. The BFF combines this with recipe ratings for recency scoring.
func (s *Service) LastPlannedDates(ctx context.Context, householdID int64, recipeIDs []int64) (map[int64]time.Time, error) {
	rows, err := s.q.ListLastPlannedDates(ctx, sqlc.ListLastPlannedDatesParams{HouseholdID: householdID, RecipeIds: recipeIDs})
	if err != nil {
		return nil, fmt.Errorf("list last planned dates: %w", err)
	}
	out := make(map[int64]time.Time, len(rows))
	for _, r := range rows {
		if r.RecipeID.Valid {
			out[r.RecipeID.Int64] = r.LastPlanned.Time
		}
	}
	return out, nil
}

func toMealPlan(row sqlc.MealplanMealPlan) MealPlan {
	return MealPlan{
		MealPlanID:         row.MealPlanID,
		HouseholdID:        row.HouseholdID,
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
		UnitID:       row.UnitID,
		IsFromRecipe: row.IsFromRecipe,
	}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		msi.ItemID = &v
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		msi.IngredientID = &v
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
