// Package event owns household food events (parties/gatherings) and the
// recipes scheduled into them. Recipe details are resolved by the BFF,
// not joined in SQL.
package event

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/event/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Service provides food-event operations.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates an event Service using the given connection pool.
// The querier resolves a ctx-carried transaction first (see
// dbtx.ContextExecer) so calls made inside a UnitOfWork join that
// transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "event")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "event"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "event")) }
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
		return fmt.Errorf("event: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// FoodEvent is a household's gathering grouping recipes by serve time.
type FoodEvent struct {
	FoodEventID            int64
	HouseholdID            int64
	Name                   string
	EventDate              time.Time
	SlotGranularityMinutes int16
	IsActive               bool
}

// CreateFoodEvent creates a new event for the household.
func (s *Service) CreateFoodEvent(ctx context.Context, arg FoodEvent, by string) (FoodEvent, error) {
	row, err := s.q.CreateFoodEvent(ctx, sqlc.CreateFoodEventParams{
		HouseholdID:            arg.HouseholdID,
		Name:                   arg.Name,
		EventDate:              pgtype.Date{Time: arg.EventDate, Valid: true},
		SlotGranularityMinutes: arg.SlotGranularityMinutes,
		IsActive:               arg.IsActive,
		CreatedBy:              by,
		UpdatedBy:              textOrNull(by),
	})
	if err != nil {
		return FoodEvent{}, fmt.Errorf("create food event: %w", err)
	}
	return toFoodEvent(row), nil
}

// GetFoodEventByID returns an event owned by the household.
func (s *Service) GetFoodEventByID(ctx context.Context, foodEventID, householdID int64) (FoodEvent, error) {
	row, err := s.q.GetFoodEventByID(ctx, sqlc.GetFoodEventByIDParams{FoodEventID: foodEventID, HouseholdID: householdID})
	if err != nil {
		return FoodEvent{}, fmt.Errorf("get food event: %w", domainerr.FromStorage(err))
	}
	return toFoodEvent(row), nil
}

// ListFoodEvents returns a household's events ordered by date descending.
func (s *Service) ListFoodEvents(ctx context.Context, householdID int64, limit, offset int32) ([]FoodEvent, error) {
	rows, err := s.q.ListFoodEvents(ctx, sqlc.ListFoodEventsParams{HouseholdID: householdID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list food events: %w", err)
	}
	out := make([]FoodEvent, len(rows))
	for i := range rows {
		out[i] = toFoodEvent(rows[i])
	}
	return out, nil
}

// CountFoodEvents returns the total number of events owned by the household.
func (s *Service) CountFoodEvents(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountFoodEvents(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("count food events: %w", err)
	}
	return n, nil
}

// UpdateFoodEvent modifies an event owned by the household.
func (s *Service) UpdateFoodEvent(ctx context.Context, foodEventID, householdID int64, arg FoodEvent, by string) error {
	return s.q.UpdateFoodEvent(ctx, sqlc.UpdateFoodEventParams{
		FoodEventID:            foodEventID,
		HouseholdID:            householdID,
		Name:                   arg.Name,
		EventDate:              pgtype.Date{Time: arg.EventDate, Valid: true},
		SlotGranularityMinutes: arg.SlotGranularityMinutes,
		IsActive:               arg.IsActive,
		UpdatedBy:              textOrNull(by),
	})
}

// DeleteFoodEvent removes an event owned by the household.
func (s *Service) DeleteFoodEvent(ctx context.Context, foodEventID, householdID int64) error {
	return s.q.DeleteFoodEvent(ctx, sqlc.DeleteFoodEventParams{FoodEventID: foodEventID, HouseholdID: householdID})
}

// ReassignHousehold repoints every food event owned by the source household
// to the target household as part of an invite-accept merge. Callers run it
// inside the merge unit of work so the reassignment commits atomically with
// the household switch; a source with no events is not an error.
func (s *Service) ReassignHousehold(ctx context.Context, fromHouseholdID, toHouseholdID int64, by string) error {
	if fromHouseholdID == toHouseholdID {
		return nil
	}
	if err := s.q.ReassignFoodEventsToHousehold(ctx, sqlc.ReassignFoodEventsToHouseholdParams{
		ToHouseholdID:   toHouseholdID,
		UpdatedBy:       by,
		FromHouseholdID: fromHouseholdID,
	}); err != nil {
		return fmt.Errorf("reassign food events: %w", domainerr.FromStorage(err))
	}
	return nil
}

// EventRecipe is a recipe scheduled to be served at an absolute time
// within an event. BaseServings freezes the linked recipe's servings at
// materialize time — the scaling denominator for ingredient quantities.
type EventRecipe struct {
	EventRecipeID int64
	FoodEventID   int64
	RecipeID      *int64
	MealType      string
	TargetTime    time.Time
	Servings      *int32
	BaseServings  *int32
	Notes         string
}

// ScalingFactor returns servings ÷ base_servings, or 1 when either side
// is unset — the multiplier applied to snapshot item quantities.
func (er EventRecipe) ScalingFactor() float64 {
	if er.Servings == nil || er.BaseServings == nil || *er.BaseServings <= 0 {
		return 1
	}
	return float64(*er.Servings) / float64(*er.BaseServings)
}

// AddEventRecipe adds a recipe slot to an event owned by the household;
// the ownership check fails with the same not-found error as
// GetFoodEventByID.
func (s *Service) AddEventRecipe(ctx context.Context, arg EventRecipe, householdID int64, by string) (EventRecipe, error) {
	if _, err := s.q.GetFoodEventByID(ctx, sqlc.GetFoodEventByIDParams{FoodEventID: arg.FoodEventID, HouseholdID: householdID}); err != nil {
		return EventRecipe{}, fmt.Errorf("add event recipe: %w", domainerr.FromStorage(err))
	}
	row, err := s.q.AddEventRecipe(ctx, sqlc.AddEventRecipeParams{
		FoodEventID: arg.FoodEventID,
		RecipeID:    optInt8(arg.RecipeID),
		MealType:    arg.MealType,
		TargetTime:  arg.TargetTime,
		Servings:    optInt4(arg.Servings),
		Notes:       textOrNull(arg.Notes),
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return EventRecipe{}, fmt.Errorf("add event recipe: %w", err)
	}
	return toEventRecipe(row), nil
}

// GetEventRecipeByID returns a recipe slot belonging to the household's
// event.
func (s *Service) GetEventRecipeByID(ctx context.Context, eventRecipeID, householdID int64) (EventRecipe, error) {
	row, err := s.q.GetEventRecipeByID(ctx, sqlc.GetEventRecipeByIDParams{EventRecipeID: eventRecipeID, HouseholdID: householdID})
	if err != nil {
		return EventRecipe{}, fmt.Errorf("get event recipe: %w", domainerr.FromStorage(err))
	}
	return toEventRecipe(row), nil
}

// ListEventRecipesForEvent returns all recipe slots for an event owned by
// the household, ordered by target time.
func (s *Service) ListEventRecipesForEvent(ctx context.Context, foodEventID, householdID int64) ([]EventRecipe, error) {
	rows, err := s.q.ListEventRecipesForEvent(ctx, sqlc.ListEventRecipesForEventParams{FoodEventID: foodEventID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list event recipes: %w", err)
	}
	out := make([]EventRecipe, len(rows))
	for i := range rows {
		out[i] = toEventRecipe(rows[i])
	}
	return out, nil
}

// ListEventRecipesByEvents returns all recipe slots for a set of events
// owned by the household in a single query.
func (s *Service) ListEventRecipesByEvents(ctx context.Context, foodEventIDs []int64, householdID int64) ([]EventRecipe, error) {
	rows, err := s.q.ListEventRecipesByEvents(ctx, sqlc.ListEventRecipesByEventsParams{FoodEventIds: foodEventIDs, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list event recipes by events: %w", err)
	}
	out := make([]EventRecipe, len(rows))
	for i := range rows {
		out[i] = toEventRecipe(rows[i])
	}
	return out, nil
}

// UpdateEventRecipe updates a recipe slot on an event owned by the
// household.
func (s *Service) UpdateEventRecipe(ctx context.Context, eventRecipeID, householdID int64, arg EventRecipe, by string) error {
	return s.q.UpdateEventRecipe(ctx, sqlc.UpdateEventRecipeParams{
		EventRecipeID: eventRecipeID,
		HouseholdID:   householdID,
		RecipeID:      optInt8(arg.RecipeID),
		MealType:      arg.MealType,
		TargetTime:    arg.TargetTime,
		Servings:      optInt4(arg.Servings),
		Notes:         textOrNull(arg.Notes),
		UpdatedBy:     textOrNull(by),
	})
}

// DeleteEventRecipe removes a recipe slot from an event owned by the
// household.
func (s *Service) DeleteEventRecipe(ctx context.Context, eventRecipeID, householdID int64) error {
	return s.q.DeleteEventRecipe(ctx, sqlc.DeleteEventRecipeParams{EventRecipeID: eventRecipeID, HouseholdID: householdID})
}

// EventRecipeStep is one step in an event slot's recipe snapshot. The
// snapshot — not the shared recipe.recipe_step rows — is what the
// timeline schedules and what event-context edits mutate, so adjusting a
// dish for one event never alters the original recipe.
type EventRecipeStep struct {
	EventRecipeStepID   int64
	EventRecipeID       int64
	StepNumber          int32
	Instruction         string
	DurationMinutes     *int32
	StepType            string
	IsPassive           bool
	DependsOnStepNumber *int32
	Appliance           string
}

// ListEventRecipeStepsForEvents batch-loads the snapshot steps for every
// slot of the given household-owned events.
func (s *Service) ListEventRecipeStepsForEvents(ctx context.Context, foodEventIDs []int64, householdID int64) ([]EventRecipeStep, error) {
	rows, err := s.q.ListEventRecipeStepsForEvents(ctx, sqlc.ListEventRecipeStepsForEventsParams{
		FoodEventIds: foodEventIDs,
		HouseholdID:  householdID,
	})
	if err != nil {
		return nil, fmt.Errorf("list event recipe steps: %w", err)
	}
	out := make([]EventRecipeStep, len(rows))
	for i := range rows {
		out[i] = toEventRecipeStep(rows[i])
	}
	return out, nil
}

// ListEventRecipeSteps returns one slot's snapshot steps, ordered.
func (s *Service) ListEventRecipeSteps(ctx context.Context, eventRecipeID, householdID int64) ([]EventRecipeStep, error) {
	rows, err := s.q.ListEventRecipeSteps(ctx, sqlc.ListEventRecipeStepsParams{
		EventRecipeID: eventRecipeID,
		HouseholdID:   householdID,
	})
	if err != nil {
		return nil, fmt.Errorf("list event recipe steps: %w", err)
	}
	out := make([]EventRecipeStep, len(rows))
	for i := range rows {
		out[i] = toEventRecipeStep(rows[i])
	}
	return out, nil
}

// GetEventRecipeStepByID returns a snapshot step owned by the household.
func (s *Service) GetEventRecipeStepByID(ctx context.Context, eventRecipeStepID, householdID int64) (EventRecipeStep, error) {
	row, err := s.q.GetEventRecipeStepByID(ctx, sqlc.GetEventRecipeStepByIDParams{
		EventRecipeStepID: eventRecipeStepID,
		HouseholdID:       householdID,
	})
	if err != nil {
		return EventRecipeStep{}, fmt.Errorf("get event recipe step: %w", domainerr.FromStorage(err))
	}
	return toEventRecipeStep(row), nil
}

// ReplaceEventRecipeSteps rewrites a slot's whole snapshot — used to
// materialize a recipe's steps when a slot is linked, and to re-sync.
func (s *Service) ReplaceEventRecipeSteps(ctx context.Context, eventRecipeID, householdID int64, steps []EventRecipeStep, by string) error {
	if _, err := s.q.GetEventRecipeByID(ctx, sqlc.GetEventRecipeByIDParams{EventRecipeID: eventRecipeID, HouseholdID: householdID}); err != nil {
		return fmt.Errorf("replace event recipe steps: %w", domainerr.FromStorage(err))
	}
	if err := s.q.DeleteEventRecipeSteps(ctx, sqlc.DeleteEventRecipeStepsParams{EventRecipeID: eventRecipeID, HouseholdID: householdID}); err != nil {
		return fmt.Errorf("replace event recipe steps: %w", err)
	}
	for _, st := range steps {
		if _, err := s.q.AddEventRecipeStep(ctx, sqlc.AddEventRecipeStepParams{
			EventRecipeID:       eventRecipeID,
			StepNumber:          st.StepNumber,
			Instruction:         st.Instruction,
			DurationMinutes:     optInt4(st.DurationMinutes),
			StepType:            textOrNull(st.StepType),
			IsPassive:           st.IsPassive,
			DependsOnStepNumber: optInt4(st.DependsOnStepNumber),
			Appliance:           textOrNull(st.Appliance),
			CreatedBy:           by,
			UpdatedBy:           textOrNull(by),
		}); err != nil {
			return fmt.Errorf("replace event recipe steps: %w", err)
		}
	}
	return nil
}

// AddEventRecipeStep appends a step to a slot's snapshot with the next
// step number.
func (s *Service) AddEventRecipeStep(ctx context.Context, arg EventRecipeStep, householdID int64, by string) (EventRecipeStep, error) {
	if _, err := s.q.GetEventRecipeByID(ctx, sqlc.GetEventRecipeByIDParams{EventRecipeID: arg.EventRecipeID, HouseholdID: householdID}); err != nil {
		return EventRecipeStep{}, fmt.Errorf("add event recipe step: %w", domainerr.FromStorage(err))
	}
	existing, err := s.ListEventRecipeSteps(ctx, arg.EventRecipeID, householdID)
	if err != nil {
		return EventRecipeStep{}, err
	}
	next := int32(1)
	for _, st := range existing {
		if st.StepNumber >= next {
			next = st.StepNumber + 1
		}
	}
	row, err := s.q.AddEventRecipeStep(ctx, sqlc.AddEventRecipeStepParams{
		EventRecipeID:       arg.EventRecipeID,
		StepNumber:          next,
		Instruction:         arg.Instruction,
		DurationMinutes:     optInt4(arg.DurationMinutes),
		StepType:            textOrNull(arg.StepType),
		IsPassive:           arg.IsPassive,
		DependsOnStepNumber: optInt4(arg.DependsOnStepNumber),
		Appliance:           textOrNull(arg.Appliance),
		CreatedBy:           by,
		UpdatedBy:           textOrNull(by),
	})
	if err != nil {
		return EventRecipeStep{}, fmt.Errorf("add event recipe step: %w", err)
	}
	return toEventRecipeStep(row), nil
}

// UpdateEventRecipeStep edits a snapshot step in place; step_number is
// the row's identity and does not change.
func (s *Service) UpdateEventRecipeStep(ctx context.Context, eventRecipeStepID, householdID int64, arg EventRecipeStep, by string) error {
	return s.q.UpdateEventRecipeStep(ctx, sqlc.UpdateEventRecipeStepParams{
		EventRecipeStepID:   eventRecipeStepID,
		HouseholdID:         householdID,
		Instruction:         arg.Instruction,
		DurationMinutes:     optInt4(arg.DurationMinutes),
		StepType:            textOrNull(arg.StepType),
		IsPassive:           arg.IsPassive,
		DependsOnStepNumber: optInt4(arg.DependsOnStepNumber),
		Appliance:           textOrNull(arg.Appliance),
		UpdatedBy:           textOrNull(by),
	})
}

// DeleteEventRecipeStep removes a snapshot step owned by the household.
func (s *Service) DeleteEventRecipeStep(ctx context.Context, eventRecipeStepID, householdID int64) error {
	return s.q.DeleteEventRecipeStep(ctx, sqlc.DeleteEventRecipeStepParams{
		EventRecipeStepID: eventRecipeStepID,
		HouseholdID:       householdID,
	})
}

// EventRecipeItem is one ingredient in a slot's snapshot. Quantity is the
// recipe's per-base-servings amount; callers multiply it by the slot's
// ScalingFactor to get the event amount.
type EventRecipeItem struct {
	EventRecipeItemID int64
	EventRecipeID     int64
	ItemID            int64
	IngredientID      *int64
	Quantity          float64
	UnitID            int64
	SectionName       string
	DisplayOrder      int32
	Notes             string
	IsOptional        bool
}

// ListEventRecipeItemsForEvents batch-loads the item snapshots for every
// slot of the given household-owned events.
func (s *Service) ListEventRecipeItemsForEvents(ctx context.Context, foodEventIDs []int64, householdID int64) ([]EventRecipeItem, error) {
	rows, err := s.q.ListEventRecipeItemsForEvents(ctx, sqlc.ListEventRecipeItemsForEventsParams{
		FoodEventIds: foodEventIDs,
		HouseholdID:  householdID,
	})
	if err != nil {
		return nil, fmt.Errorf("list event recipe items: %w", err)
	}
	out := make([]EventRecipeItem, len(rows))
	for i := range rows {
		out[i] = toEventRecipeItem(rows[i])
	}
	return out, nil
}

// ListEventRecipeItems returns one slot's snapshot items, ordered.
func (s *Service) ListEventRecipeItems(ctx context.Context, eventRecipeID, householdID int64) ([]EventRecipeItem, error) {
	rows, err := s.q.ListEventRecipeItems(ctx, sqlc.ListEventRecipeItemsParams{
		EventRecipeID: eventRecipeID,
		HouseholdID:   householdID,
	})
	if err != nil {
		return nil, fmt.Errorf("list event recipe items: %w", err)
	}
	out := make([]EventRecipeItem, len(rows))
	for i := range rows {
		out[i] = toEventRecipeItem(rows[i])
	}
	return out, nil
}

// GetEventRecipeItemByID returns a snapshot item owned by the household.
func (s *Service) GetEventRecipeItemByID(ctx context.Context, eventRecipeItemID, householdID int64) (EventRecipeItem, error) {
	row, err := s.q.GetEventRecipeItemByID(ctx, sqlc.GetEventRecipeItemByIDParams{
		EventRecipeItemID: eventRecipeItemID,
		HouseholdID:       householdID,
	})
	if err != nil {
		return EventRecipeItem{}, fmt.Errorf("get event recipe item: %w", domainerr.FromStorage(err))
	}
	return toEventRecipeItem(row), nil
}

// ReplaceEventRecipeItems rewrites a slot's item snapshot and freezes
// baseServings as the scaling denominator. Pass a nil baseServings to
// clear the denominator (e.g. free-form slot or recipe without servings).
func (s *Service) ReplaceEventRecipeItems(ctx context.Context, eventRecipeID, householdID int64, items []EventRecipeItem, baseServings *int32, by string) error {
	if _, err := s.q.GetEventRecipeByID(ctx, sqlc.GetEventRecipeByIDParams{EventRecipeID: eventRecipeID, HouseholdID: householdID}); err != nil {
		return fmt.Errorf("replace event recipe items: %w", domainerr.FromStorage(err))
	}
	if err := s.q.DeleteEventRecipeItems(ctx, sqlc.DeleteEventRecipeItemsParams{EventRecipeID: eventRecipeID, HouseholdID: householdID}); err != nil {
		return fmt.Errorf("replace event recipe items: %w", err)
	}
	for _, it := range items {
		qty, err := numericFromFloat64(it.Quantity)
		if err != nil {
			return fmt.Errorf("replace event recipe items: %w", err)
		}
		if _, err := s.q.AddEventRecipeItem(ctx, sqlc.AddEventRecipeItemParams{
			EventRecipeID: eventRecipeID,
			ItemID:        it.ItemID,
			IngredientID:  optInt8(it.IngredientID),
			Quantity:      qty,
			UnitID:        it.UnitID,
			SectionName:   textOrNull(it.SectionName),
			DisplayOrder:  it.DisplayOrder,
			Notes:         textOrNull(it.Notes),
			IsOptional:    it.IsOptional,
			CreatedBy:     by,
			UpdatedBy:     textOrNull(by),
		}); err != nil {
			return fmt.Errorf("replace event recipe items: %w", err)
		}
	}
	if err := s.q.SetEventRecipeBaseServings(ctx, sqlc.SetEventRecipeBaseServingsParams{
		EventRecipeID: eventRecipeID,
		HouseholdID:   householdID,
		BaseServings:  optInt4(baseServings),
		UpdatedBy:     textOrNull(by),
	}); err != nil {
		return fmt.Errorf("replace event recipe items: %w", err)
	}
	return nil
}

// AddEventRecipeItem appends an ingredient to a slot's snapshot.
func (s *Service) AddEventRecipeItem(ctx context.Context, arg EventRecipeItem, householdID int64, by string) (EventRecipeItem, error) {
	if _, err := s.q.GetEventRecipeByID(ctx, sqlc.GetEventRecipeByIDParams{EventRecipeID: arg.EventRecipeID, HouseholdID: householdID}); err != nil {
		return EventRecipeItem{}, fmt.Errorf("add event recipe item: %w", domainerr.FromStorage(err))
	}
	qty, err := numericFromFloat64(arg.Quantity)
	if err != nil {
		return EventRecipeItem{}, fmt.Errorf("add event recipe item: %w", err)
	}
	row, err := s.q.AddEventRecipeItem(ctx, sqlc.AddEventRecipeItemParams{
		EventRecipeID: arg.EventRecipeID,
		ItemID:        arg.ItemID,
		IngredientID:  optInt8(arg.IngredientID),
		Quantity:      qty,
		UnitID:        arg.UnitID,
		SectionName:   textOrNull(arg.SectionName),
		DisplayOrder:  arg.DisplayOrder,
		Notes:         textOrNull(arg.Notes),
		IsOptional:    arg.IsOptional,
		CreatedBy:     by,
		UpdatedBy:     textOrNull(by),
	})
	if err != nil {
		return EventRecipeItem{}, fmt.Errorf("add event recipe item: %w", err)
	}
	return toEventRecipeItem(row), nil
}

// UpdateEventRecipeItem edits a snapshot item in place.
func (s *Service) UpdateEventRecipeItem(ctx context.Context, eventRecipeItemID, householdID int64, arg EventRecipeItem, by string) error {
	qty, err := numericFromFloat64(arg.Quantity)
	if err != nil {
		return fmt.Errorf("update event recipe item: %w", err)
	}
	return s.q.UpdateEventRecipeItem(ctx, sqlc.UpdateEventRecipeItemParams{
		EventRecipeItemID: eventRecipeItemID,
		HouseholdID:       householdID,
		ItemID:            arg.ItemID,
		IngredientID:      optInt8(arg.IngredientID),
		Quantity:          qty,
		UnitID:            arg.UnitID,
		SectionName:       textOrNull(arg.SectionName),
		DisplayOrder:      arg.DisplayOrder,
		Notes:             textOrNull(arg.Notes),
		IsOptional:        arg.IsOptional,
		UpdatedBy:         textOrNull(by),
	})
}

// DeleteEventRecipeItem removes a snapshot item owned by the household.
func (s *Service) DeleteEventRecipeItem(ctx context.Context, eventRecipeItemID, householdID int64) error {
	return s.q.DeleteEventRecipeItem(ctx, sqlc.DeleteEventRecipeItemParams{
		EventRecipeItemID: eventRecipeItemID,
		HouseholdID:       householdID,
	})
}

func toEventRecipeItem(row sqlc.EventEventRecipeItem) EventRecipeItem {
	it := EventRecipeItem{
		EventRecipeItemID: row.EventRecipeItemID,
		EventRecipeID:     row.EventRecipeID,
		ItemID:            row.ItemID,
		UnitID:            row.UnitID,
		SectionName:       row.SectionName.String,
		DisplayOrder:      row.DisplayOrder,
		Notes:             row.Notes.String,
		IsOptional:        row.IsOptional,
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		it.IngredientID = &v
	}
	if f, err := row.Quantity.Float64Value(); err == nil {
		it.Quantity = f.Float64
	}
	return it
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

func toEventRecipeStep(row sqlc.EventEventRecipeStep) EventRecipeStep {
	st := EventRecipeStep{
		EventRecipeStepID: row.EventRecipeStepID,
		EventRecipeID:     row.EventRecipeID,
		StepNumber:        row.StepNumber,
		Instruction:       row.Instruction,
		StepType:          row.StepType.String,
		IsPassive:         row.IsPassive,
		Appliance:         row.Appliance.String,
	}
	if row.DurationMinutes.Valid {
		v := row.DurationMinutes.Int32
		st.DurationMinutes = &v
	}
	if row.DependsOnStepNumber.Valid {
		v := row.DependsOnStepNumber.Int32
		st.DependsOnStepNumber = &v
	}
	return st
}

func toFoodEvent(row sqlc.EventFoodEvent) FoodEvent {
	return FoodEvent{
		FoodEventID:            row.FoodEventID,
		HouseholdID:            row.HouseholdID,
		Name:                   row.Name,
		EventDate:              row.EventDate.Time,
		SlotGranularityMinutes: row.SlotGranularityMinutes,
		IsActive:               row.IsActive,
	}
}

func toEventRecipe(row sqlc.EventEventRecipe) EventRecipe {
	er := EventRecipe{
		EventRecipeID: row.EventRecipeID,
		FoodEventID:   row.FoodEventID,
		MealType:      row.MealType,
		TargetTime:    row.TargetTime,
	}
	if row.RecipeID.Valid {
		v := row.RecipeID.Int64
		er.RecipeID = &v
	}
	if row.Servings.Valid {
		v := row.Servings.Int32
		er.Servings = &v
	}
	if row.BaseServings.Valid {
		v := row.BaseServings.Int32
		er.BaseServings = &v
	}
	er.Notes = row.Notes.String
	return er
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
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
