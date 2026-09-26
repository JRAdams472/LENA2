// Package event owns household food events (parties/gatherings) and the
// recipes scheduled into them. Recipe details are resolved by the BFF,
// not joined in SQL.
package event

import (
	"context"
	"fmt"
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
// within an event.
type EventRecipe struct {
	EventRecipeID int64
	FoodEventID   int64
	RecipeID      *int64
	MealType      string
	TargetTime    time.Time
	Servings      *int32
	Notes         string
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
