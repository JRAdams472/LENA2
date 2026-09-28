package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/analytics/sqlc"
	"github.com/JRAdams472/LENA2/internal/analytics/sqlc/mock"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx/dbtxtest"
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
	return &Service{
		q:    q,
		pool: &dbtxtest.Pool{Tx: &dbtxtest.Tx{}},
		newQ: func(pgx.Tx) sqlc.Querier { return q },
	}, q
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

func TestRecordView(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts event without selection count upserts", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().InsertInteractionEvent(gomock.Any(), gomock.Any()).Return(nil)
		// RecordView must NOT call UpsertUserSelectionCount or
		// UpsertGlobalSelectionCount — views are not picks.

		err := s.RecordView(ctx, Event{
			UserID: 1, EventType: "recipe_viewed", EntityType: "recipe", EntityID: 7,
		}, "viewer")
		require.NoError(t, err)
	})

	t.Run("validation", func(t *testing.T) {
		s, _ := newTestService(t)
		err := s.RecordView(ctx, Event{EventType: "recipe_viewed"}, "test")
		assert.ErrorContains(t, err, "user_id is required")
		err = s.RecordView(ctx, Event{UserID: 1}, "test")
		assert.ErrorContains(t, err, "event_type is required")
	})
}

func TestRecipeEngagementSets(t *testing.T) {
	ctx := context.Background()

	t.Run("maps rows into engagement sets", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().HouseholdUsedRecipeIDs(gomock.Any(), int64(7)).
			Return([]sqlc.HouseholdUsedRecipeIDsRow{
				{RecipeID: pgtype.Int8{Int64: 9, Valid: true}, Hits: 5},
				{RecipeID: pgtype.Int8{Int64: 3, Valid: true}, Hits: 2},
				{RecipeID: pgtype.Int8{}},
			}, nil)
		q.EXPECT().UserViewedRecipeIDs(gomock.Any(), int64(1)).
			Return([]sqlc.UserViewedRecipeIDsRow{
				{RecipeID: pgtype.Int8{Int64: 4, Valid: true}, Hits: 3},
			}, nil)
		q.EXPECT().UserRecipeSearchTerms(gomock.Any(), int64(1)).
			Return([]pgtype.Text{
				{String: "taco", Valid: true},
				{String: "", Valid: false},
			}, nil)

		eng, err := s.RecipeEngagementSets(ctx, 1, 7)
		require.NoError(t, err)
		assert.Equal(t, []int64{9, 3}, eng.UsedIDs)
		assert.Equal(t, []int64{4}, eng.ViewedIDs)
		assert.Equal(t, []string{"taco"}, eng.SearchTerms)
	})

	t.Run("query error propagates", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().HouseholdUsedRecipeIDs(gomock.Any(), gomock.Any()).Return(nil, errBoom)

		_, err := s.RecipeEngagementSets(ctx, 1, 7)
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorContains(t, err, "engagement used")
	})
}

func TestDecayScores(t *testing.T) {
	ctx := context.Background()

	t.Run("rebuilds all three scopes with default half-life", func(t *testing.T) {
		s, q := newTestService(t)
		gomock.InOrder(
			q.EXPECT().ClearSelectionScores(gomock.Any()).Return(nil),
			q.EXPECT().RebuildUserSelectionScores(gomock.Any(), 90.0).Return(nil),
			q.EXPECT().RebuildHouseholdSelectionScores(gomock.Any(), 90.0).Return(nil),
			q.EXPECT().RebuildGlobalSelectionScores(gomock.Any(), 90.0).Return(nil),
		)

		require.NoError(t, s.DecayScores(ctx))
	})

	t.Run("honors configured half-life", func(t *testing.T) {
		s, q := newTestService(t)
		s.cfg = Config{HalfLifeDays: 30}
		q.EXPECT().ClearSelectionScores(gomock.Any()).Return(nil)
		q.EXPECT().RebuildUserSelectionScores(gomock.Any(), 30.0).Return(nil)
		q.EXPECT().RebuildHouseholdSelectionScores(gomock.Any(), 30.0).Return(nil)
		q.EXPECT().RebuildGlobalSelectionScores(gomock.Any(), 30.0).Return(nil)

		require.NoError(t, s.DecayScores(ctx))
	})

	t.Run("clear failure aborts before rebuilds", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().ClearSelectionScores(gomock.Any()).Return(errBoom)

		err := s.DecayScores(ctx)
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorContains(t, err, "clear selection scores")
	})

	t.Run("rebuild failure wraps the scope name", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().ClearSelectionScores(gomock.Any()).Return(nil)
		q.EXPECT().RebuildUserSelectionScores(gomock.Any(), gomock.Any()).Return(nil)
		q.EXPECT().RebuildHouseholdSelectionScores(gomock.Any(), gomock.Any()).Return(errBoom)

		err := s.DecayScores(ctx)
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorContains(t, err, "household scores")
	})
}

func TestTopScores(t *testing.T) {
	ctx := context.Background()
	s, q := newTestService(t)

	now := time.Now()
	q.EXPECT().TopSelectionScores(ctx, sqlc.TopSelectionScoresParams{
		ScopeType: ScopeHousehold, ScopeID: 7, EntityType: EntityItem, Limit: 50,
	}).Return([]sqlc.TopSelectionScoresRow{
		{EntityID: 42, Score: 3.5, EventCount: 4, LastSelectedAt: pgtype.Timestamptz{Time: now, Valid: true}},
	}, nil)

	scores, err := s.TopScores(ctx, ScopeHousehold, 7, EntityItem, 50)
	require.NoError(t, err)
	require.Len(t, scores, 1)
	assert.Equal(t, int64(42), scores[0].EntityID)
	assert.InDelta(t, 3.5, scores[0].Score, 1e-9)
	assert.Equal(t, int64(4), scores[0].EventCount)
}

func TestEntityEngagementSets(t *testing.T) {
	ctx := context.Background()

	t.Run("assembles all five signal sets", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().TopSelectionScores(gomock.Any(), sqlc.TopSelectionScoresParams{
			ScopeType: ScopeUser, ScopeID: 1, EntityType: EntityItem, Limit: engagementLimit,
		}).Return([]sqlc.TopSelectionScoresRow{{EntityID: 10, Score: 2.5}}, nil)
		q.EXPECT().TopSelectionScores(gomock.Any(), sqlc.TopSelectionScoresParams{
			ScopeType: ScopeHousehold, ScopeID: 7, EntityType: EntityItem, Limit: engagementLimit,
		}).Return([]sqlc.TopSelectionScoresRow{{EntityID: 20, Score: 8}}, nil)
		q.EXPECT().TopSelectionScores(gomock.Any(), sqlc.TopSelectionScoresParams{
			ScopeType: ScopeGlobal, ScopeID: 0, EntityType: EntityItem, Limit: engagementLimit,
		}).Return([]sqlc.TopSelectionScoresRow{{EntityID: 30, Score: 100}}, nil)
		q.EXPECT().UserViewedEntityIDs(gomock.Any(), sqlc.UserViewedEntityIDsParams{
			EntityType: EntityItem, UserID: 1, Limit: engagementLimit,
		}).Return([]sqlc.UserViewedEntityIDsRow{
			{EntityID: pgtype.Int8{Int64: 40, Valid: true}, Hits: 2},
			{EntityID: pgtype.Int8{}},
		}, nil)
		q.EXPECT().UserEntitySearchTerms(gomock.Any(), sqlc.UserEntitySearchTermsParams{
			EntityType: EntityItem, UserID: 1,
		}).Return([]pgtype.Text{{String: "milk", Valid: true}}, nil)

		eng, err := s.EntityEngagementSets(ctx, 1, 7, EntityItem)
		require.NoError(t, err)
		assert.Equal(t, []int64{10}, eng.PersonalIDs)
		assert.Equal(t, []int64{20}, eng.HouseholdIDs)
		assert.Equal(t, []int64{30}, eng.GlobalIDs)
		assert.Equal(t, []int64{40}, eng.ViewedIDs)
		assert.Equal(t, []string{"milk"}, eng.SearchTerms)
	})

	t.Run("household scope skipped without household", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().TopSelectionScores(gomock.Any(), gomock.Any()).
			Return([]sqlc.TopSelectionScoresRow{}, nil).Times(2)
		q.EXPECT().UserViewedEntityIDs(gomock.Any(), gomock.Any()).
			Return([]sqlc.UserViewedEntityIDsRow{}, nil)
		q.EXPECT().UserEntitySearchTerms(gomock.Any(), gomock.Any()).
			Return([]pgtype.Text{}, nil)

		eng, err := s.EntityEngagementSets(ctx, 1, 0, EntityBottle)
		require.NoError(t, err)
		assert.Empty(t, eng.HouseholdIDs)
	})

	t.Run("error propagates", func(t *testing.T) {
		s, q := newTestService(t)
		q.EXPECT().TopSelectionScores(gomock.Any(), gomock.Any()).Return(nil, errBoom)

		_, err := s.EntityEngagementSets(ctx, 1, 7, EntityItem)
		assert.ErrorIs(t, err, errBoom)
		assert.ErrorContains(t, err, "engagement personal")
	})
}

func TestEventWeights_NewSignals(t *testing.T) {
	for _, et := range []string{
		EventBottleSelected, EventBottleViewed, EventBottleSearched, EventBottleCreated,
		EventIngredientSelected, EventIngredientSearched, EventIngredientViewed,
		EventPantryItemAdded, EventPantryItemAdjusted,
		EventGroceryItemAdded, EventGroceryItemChecked,
		EventBrandSearched, EventItemViewed, EventBrandViewed,
	} {
		assert.NotZero(t, eventWeight(et), "event %s has no weight", et)
	}
	assert.Equal(t, int16(5), eventWeight(EventRatingGiven))
	assert.Greater(t, eventWeight(EventGroceryItemChecked), eventWeight(EventItemSelected))
}
