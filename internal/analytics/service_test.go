package analytics

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/analytics/sqlc"
	"github.com/JRAdams472/LENA2/internal/analytics/sqlc/mock"
)

var errBoom = errors.New("boom")

func mustNum(f float64) pgtype.Numeric {
	n, err := numericFromFloat64(f)
	if err != nil {
		panic(err)
	}
	return n
}

func newTestService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	ctrl := gomock.NewController(t)
	q := mock.NewMockQuerier(ctrl)
	return &Service{q: q}, q
}

func TestRecordEvent_Validation(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)

	err := s.RecordEvent(ctx, Event{EventType: "item_selected"}, "test")
	assert.ErrorContains(t, err, "user_id is required")

	err = s.RecordEvent(ctx, Event{UserID: 1}, "test")
	assert.ErrorContains(t, err, "event_type is required")
}

func TestGetUserSelectionCounts(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)

	q.EXPECT().GetUserSelectionCounts(ctx, sqlc.GetUserSelectionCountsParams{
		UserID:     1,
		EntityType: EntityItem,
		EntityIds:  []int64{10, 20},
	}).Return([]sqlc.AnalyticsUserSelectionCount{
		{EntityType: EntityItem, EntityID: 10, UserID: 1, SelectCount: 3},
		{EntityType: EntityItem, EntityID: 20, UserID: 1, SelectCount: 1},
	}, nil)

	counts, err := s.GetUserSelectionCounts(ctx, 1, EntityItem, []int64{10, 20})
	require.NoError(t, err)
	require.Len(t, counts, 2)
	assert.Equal(t, int64(3), counts[0].SelectCount)
	assert.Equal(t, int64(10), counts[0].EntityID)
}

func TestGetUserSelectionCounts_Error(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)

	q.EXPECT().GetUserSelectionCounts(ctx, gomock.Any()).Return(nil, errBoom)

	_, err := s.GetUserSelectionCounts(ctx, 1, EntityItem, []int64{10})
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "get user selection counts")
}

func TestGetGlobalSelectionCounts(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)

	q.EXPECT().GetGlobalSelectionCounts(ctx, sqlc.GetGlobalSelectionCountsParams{
		EntityType: EntityBrand,
		EntityIds:  []int64{5},
	}).Return([]sqlc.AnalyticsGlobalSelectionCount{
		{EntityType: EntityBrand, EntityID: 5, SelectCount: 42},
	}, nil)

	counts, err := s.GetGlobalSelectionCounts(ctx, EntityBrand, []int64{5})
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, int64(42), counts[0].SelectCount)
}

func TestTopUserSelections(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)

	q.EXPECT().TopUserSelections(ctx, sqlc.TopUserSelectionsParams{
		UserID:     1,
		EntityType: EntityRecipe,
		Limit:      5,
	}).Return([]sqlc.AnalyticsUserSelectionCount{
		{EntityType: EntityRecipe, EntityID: 7, UserID: 1, SelectCount: 9},
	}, nil)

	counts, err := s.TopUserSelections(ctx, 1, EntityRecipe, 5)
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, int64(7), counts[0].EntityID)
}

func TestTopGlobalSelections(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)

	q.EXPECT().TopGlobalSelections(ctx, sqlc.TopGlobalSelectionsParams{
		EntityType: EntityItem,
		Limit:      10,
	}).Return([]sqlc.AnalyticsGlobalSelectionCount{
		{EntityType: EntityItem, EntityID: 3, SelectCount: 100},
	}, nil)

	counts, err := s.TopGlobalSelections(ctx, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, int64(3), counts[0].EntityID)
}

func TestComputeIngredientOverlapSuggestions(t *testing.T) {
	ctx := context.Background()

	t.Run("upserts one row per qualifying user", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().IngredientOverlapScores(gomock.Any(), sqlc.IngredientOverlapScoresParams{
			RecipeID: 7,
			MinScore: mustNum(OverlapMinScore),
		}).Return([]sqlc.IngredientOverlapScoresRow{
			{UserID: 1, Score: 0.5},
			{UserID: 2, Score: 0.75},
		}, nil)
		q.EXPECT().UpsertRecipeRecommendation(gomock.Any(), sqlc.UpsertRecipeRecommendationParams{
			UserID: 1, RecipeID: 7, Reason: ReasonIngredientOverlap, Score: mustNum(0.5),
		}).Return(nil)
		q.EXPECT().UpsertRecipeRecommendation(gomock.Any(), sqlc.UpsertRecipeRecommendationParams{
			UserID: 2, RecipeID: 7, Reason: ReasonIngredientOverlap, Score: mustNum(0.75),
		}).Return(nil)

		n, err := s.ComputeIngredientOverlapSuggestions(ctx, 7)
		require.NoError(t, err)
		assert.Equal(t, 2, n)
	})

	t.Run("no qualifying users writes nothing", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().IngredientOverlapScores(gomock.Any(), gomock.Any()).
			Return([]sqlc.IngredientOverlapScoresRow{}, nil)

		n, err := s.ComputeIngredientOverlapSuggestions(ctx, 7)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("query error propagates", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().IngredientOverlapScores(gomock.Any(), gomock.Any()).Return(nil, errBoom)

		_, err := s.ComputeIngredientOverlapSuggestions(ctx, 7)
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorContains(t, err, "compute ingredient overlap")
	})

	t.Run("upsert error stops after partial writes", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().IngredientOverlapScores(gomock.Any(), gomock.Any()).
			Return([]sqlc.IngredientOverlapScoresRow{
				{UserID: 1, Score: 0.5},
				{UserID: 2, Score: 0.75},
			}, nil)
		q.EXPECT().UpsertRecipeRecommendation(gomock.Any(), gomock.Any()).Return(errBoom)

		n, err := s.ComputeIngredientOverlapSuggestions(ctx, 7)
		assert.ErrorIs(t, err, errBoom)
		assert.Equal(t, 0, n)
	})
}

func TestListRecipeRecommendations(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps rows", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().ListRecipeRecommendations(gomock.Any(), sqlc.ListRecipeRecommendationsParams{
			UserID: 3, Reason: ReasonIngredientOverlap, Limit: 10,
		}).Return([]sqlc.AnalyticsRecipeRecommendation{
			{RecommendationID: 5, UserID: 3, RecipeID: 7, Reason: ReasonIngredientOverlap, Score: mustNum(0.8)},
		}, nil)

		recs, err := s.ListRecipeRecommendations(ctx, 3, ReasonIngredientOverlap, 10)
		require.NoError(t, err)
		require.Len(t, recs, 1)
		assert.Equal(t, int64(5), recs[0].RecommendationID)
		assert.Equal(t, int64(7), recs[0].RecipeID)
		assert.Equal(t, ReasonIngredientOverlap, recs[0].Reason)
		assert.InDelta(t, 0.8, recs[0].Score, 1e-9)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().ListRecipeRecommendations(gomock.Any(), gomock.Any()).Return(nil, errBoom)

		_, err := s.ListRecipeRecommendations(ctx, 3, ReasonIngredientOverlap, 10)
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorContains(t, err, "list recipe recommendations")
	})
}
