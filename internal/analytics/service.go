// Package analytics owns user-behavior logging and the materialized
// selection counts used for frequency-based ranking and future
// recommendation systems.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/analytics/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
)

// Service provides interaction tracking and selection-count aggregation.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
}

// NewService creates an analytics Service using the given connection pool.
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

// EntityType values identify the class of catalog object being tracked.
const (
	EntityItem   = "item"
	EntityBrand  = "brand"
	EntityRecipe = "recipe"
)

// EventType values classify the kind of interaction being recorded.
const (
	EventItemSelected   = "item_selected"
	EventBrandSelected  = "brand_selected"
	EventRecipeSelected = "recipe_selected"
	EventItemSearched   = "item_searched"
	EventRecipeSearched = "recipe_searched"
	EventRecipeCreated  = "recipe_created"
	EventMenuAdd        = "menu_add"
	EventRatingGiven    = "rating_given"
)

// Weights are based on the signal-value table from docs/updates.md.
var eventWeights = map[string]int16{
	EventItemSelected:   1,
	EventBrandSelected:  1,
	EventRecipeSelected: 1,
	EventItemSearched:   2,
	EventRecipeSearched: 2,
	EventRecipeCreated:  4,
	EventMenuAdd:        4,
	EventRatingGiven:    5,
}

// Event is one user interaction to record.
type Event struct {
	UserID     int64
	EventType  string
	EntityType string
	EntityID   int64
	SearchTerm string
}

func eventWeight(eventType string) int16 {
	if w, ok := eventWeights[eventType]; ok {
		return w
	}
	return 1
}

// RecordEvent writes an interaction to the event log and, when the event
// targets a catalog entity, increments the per-user and global selection
// counts in the same transaction.
func (s *Service) RecordEvent(ctx context.Context, e Event, by string) error {
	if e.UserID == 0 {
		return fmt.Errorf("record event: user_id is required")
	}
	if e.EventType == "" {
		return fmt.Errorf("record event: event_type is required")
	}

	metadata, err := metadataJSON(by)
	if err != nil {
		return fmt.Errorf("record event: %w", err)
	}

	return s.InTx(ctx, func(tx *Service) error {
		if err := tx.q.InsertInteractionEvent(ctx, sqlc.InsertInteractionEventParams{
			UserID:     pgtype.Int8{Int64: e.UserID, Valid: true},
			EventType:  e.EventType,
			EntityType: pgtype.Text{String: e.EntityType, Valid: e.EntityType != ""},
			EntityID:   pgtype.Int8{Int64: e.EntityID, Valid: e.EntityID != 0},
			SearchTerm: pgtype.Text{String: e.SearchTerm, Valid: e.SearchTerm != ""},
			Weight:     eventWeight(e.EventType),
			Metadata:   metadata,
		}); err != nil {
			return fmt.Errorf("insert interaction event: %w", err)
		}

		if e.EntityType == "" || e.EntityID == 0 {
			return nil
		}

		if err := tx.q.UpsertUserSelectionCount(ctx, sqlc.UpsertUserSelectionCountParams{
			EntityType: e.EntityType,
			EntityID:   e.EntityID,
			UserID:     e.UserID,
		}); err != nil {
			return fmt.Errorf("upsert user selection count: %w", err)
		}

		if err := tx.q.UpsertGlobalSelectionCount(ctx, sqlc.UpsertGlobalSelectionCountParams{
			EntityType: e.EntityType,
			EntityID:   e.EntityID,
		}); err != nil {
			return fmt.Errorf("upsert global selection count: %w", err)
		}
		return nil
	})
}

func metadataJSON(by string) ([]byte, error) {
	m := map[string]string{"created_by": by}
	return json.Marshal(m)
}

// SelectionCount is a single entity's usage frequency for one user and/or
// the whole user base.
type SelectionCount struct {
	EntityType  string
	EntityID    int64
	UserID      int64
	SelectCount int64
}

// GetUserSelectionCounts returns the per-user selection counts for a set of
// entities of a given type.
func (s *Service) GetUserSelectionCounts(ctx context.Context, userID int64, entityType string, entityIDs []int64) ([]SelectionCount, error) {
	rows, err := s.q.GetUserSelectionCounts(ctx, sqlc.GetUserSelectionCountsParams{
		UserID:     userID,
		EntityType: entityType,
		EntityIds:  entityIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get user selection counts: %w", err)
	}
	out := make([]SelectionCount, len(rows))
	for i, r := range rows {
		out[i] = SelectionCount{
			EntityType:  r.EntityType,
			EntityID:    r.EntityID,
			UserID:      r.UserID,
			SelectCount: r.SelectCount,
		}
	}
	return out, nil
}

// GetGlobalSelectionCounts returns the global selection counts for a set of
// entities of a given type.
func (s *Service) GetGlobalSelectionCounts(ctx context.Context, entityType string, entityIDs []int64) ([]SelectionCount, error) {
	rows, err := s.q.GetGlobalSelectionCounts(ctx, sqlc.GetGlobalSelectionCountsParams{
		EntityType: entityType,
		EntityIds:  entityIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get global selection counts: %w", err)
	}
	out := make([]SelectionCount, len(rows))
	for i, r := range rows {
		out[i] = SelectionCount{
			EntityType:  r.EntityType,
			EntityID:    r.EntityID,
			SelectCount: r.SelectCount,
		}
	}
	return out, nil
}

// TopUserSelections returns the most frequently selected entities for a user.
func (s *Service) TopUserSelections(ctx context.Context, userID int64, entityType string, limit int32) ([]SelectionCount, error) {
	rows, err := s.q.TopUserSelections(ctx, sqlc.TopUserSelectionsParams{
		UserID:     userID,
		EntityType: entityType,
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("top user selections: %w", err)
	}
	out := make([]SelectionCount, len(rows))
	for i, r := range rows {
		out[i] = SelectionCount{
			EntityType:  r.EntityType,
			EntityID:    r.EntityID,
			UserID:      r.UserID,
			SelectCount: r.SelectCount,
		}
	}
	return out, nil
}

// TopGlobalSelections returns the most frequently selected entities across
// all users.
func (s *Service) TopGlobalSelections(ctx context.Context, entityType string, limit int32) ([]SelectionCount, error) {
	rows, err := s.q.TopGlobalSelections(ctx, sqlc.TopGlobalSelectionsParams{
		EntityType: entityType,
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("top global selections: %w", err)
	}
	out := make([]SelectionCount, len(rows))
	for i, r := range rows {
		out[i] = SelectionCount{
			EntityType:  r.EntityType,
			EntityID:    r.EntityID,
			SelectCount: r.SelectCount,
		}
	}
	return out, nil
}
