// Package analytics owns user-behavior logging and the materialized
// selection counts used for frequency-based ranking and future
// recommendation systems.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

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

// Reason values stored in analytics.recipe_recommendation.
// ReasonCollaborativeFiltering is reserved for a future external
// recommender job that writes into the same table.
const (
	ReasonIngredientOverlap      = "ingredient_overlap"
	ReasonRatingRecency          = "rating_recency"
	ReasonCollaborativeFiltering = "collaborative_filtering"
)

// OverlapMinScore is the minimum Jaccard similarity required to store an
// ingredient-overlap recommendation.
const OverlapMinScore = 0.3

// Recommendation is one scored recipe suggestion for a user.
type Recommendation struct {
	RecommendationID int64
	UserID           int64
	RecipeID         int64
	Reason           string
	Score            float64
}

// runInTx executes fn inside a transaction when a pool is configured; it is a
// test seam that falls back to the default querier when the service was
// constructed without a pool (unit-test doubles).
func (s *Service) runInTx(ctx context.Context, fn func(*Service) error) error {
	if s.pool == nil {
		return fn(s)
	}
	return s.InTx(ctx, fn)
}

// ComputeIngredientOverlapSuggestions scores a newly created recipe against
// every user's meal-plan history and upserts ingredient-overlap
// recommendations for users whose best similarity clears OverlapMinScore.
// It returns the number of recommendation rows written.
func (s *Service) ComputeIngredientOverlapSuggestions(ctx context.Context, newRecipeID int64) (int, error) {
	minScore, err := numericFromFloat64(OverlapMinScore)
	if err != nil {
		return 0, fmt.Errorf("compute ingredient overlap: %w", err)
	}
	rows, err := s.q.IngredientOverlapScores(ctx, sqlc.IngredientOverlapScoresParams{
		RecipeID: newRecipeID,
		MinScore: minScore,
	})
	if err != nil {
		return 0, fmt.Errorf("compute ingredient overlap: %w", err)
	}

	written := 0
	err = s.runInTx(ctx, func(tx *Service) error {
		for _, r := range rows {
			score, err := numericFromFloat64(r.Score)
			if err != nil {
				return fmt.Errorf("compute ingredient overlap: %w", err)
			}
			if err := tx.q.UpsertRecipeRecommendation(ctx, sqlc.UpsertRecipeRecommendationParams{
				UserID:   r.UserID,
				RecipeID: newRecipeID,
				Reason:   ReasonIngredientOverlap,
				Score:    score,
			}); err != nil {
				return fmt.Errorf("upsert recipe recommendation: %w", err)
			}
			written++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return written, nil
}

// ListRecipeRecommendations returns a user's stored recommendations for one
// reason, highest score first.
func (s *Service) ListRecipeRecommendations(ctx context.Context, userID int64, reason string, limit int32) ([]Recommendation, error) {
	rows, err := s.q.ListRecipeRecommendations(ctx, sqlc.ListRecipeRecommendationsParams{
		UserID: userID,
		Reason: reason,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list recipe recommendations: %w", err)
	}
	out := make([]Recommendation, len(rows))
	for i := range rows {
		score, err := rows[i].Score.Float64Value()
		if err != nil {
			return nil, fmt.Errorf("list recipe recommendations: %w", err)
		}
		out[i] = Recommendation{
			RecommendationID: rows[i].RecommendationID,
			UserID:           rows[i].UserID,
			RecipeID:         rows[i].RecipeID,
			Reason:           rows[i].Reason,
			Score:            score.Float64,
		}
	}
	return out, nil
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
