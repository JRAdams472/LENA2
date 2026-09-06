package analytics

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/analytics/sqlc"
	"github.com/JRAdams472/LENA2/internal/analytics/sqlc/mock"
)

var errBoom = errors.New("boom")

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
