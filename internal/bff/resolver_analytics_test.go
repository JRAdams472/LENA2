package bff

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

const analyticsTestEmail = "analytics@example.com"

func analyticsCtx() context.Context {
	return testenv.WithUser(context.Background(), 7, analyticsTestEmail)
}

func newAnalyticsMock(t *testing.T) *mock.MockAnalyticsService {
	t.Helper()
	return mock.NewMockAnalyticsService(gomock.NewController(t))
}

func TestResolver_RecordSelection(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		an := newAnalyticsMock(t)
		an.EXPECT().RecordEvent(gomock.Any(), analytics.Event{
			UserID:     7,
			EventType:  analytics.EventItemSelected,
			EntityType: analytics.EntityItem,
			EntityID:   42,
		}, analyticsTestEmail).Return(nil)

		r := &Resolver{AnalyticsService: an}
		ok, err := r.RecordSelection(analyticsCtx(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: analytics.EntityItem, EntityID: "42"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{AnalyticsService: newAnalyticsMock(t)}
		_, err := r.RecordSelection(context.Background(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: analytics.EntityItem, EntityID: "42"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		r := &Resolver{AnalyticsService: newAnalyticsMock(t)}
		_, err := r.RecordSelection(analyticsCtx(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: analytics.EntityItem, EntityID: "abc"})
		require.Error(t, err)
	})

	t.Run("brand selection", func(t *testing.T) {
		an := newAnalyticsMock(t)
		an.EXPECT().RecordEvent(gomock.Any(), analytics.Event{
			UserID:     7,
			EventType:  analytics.EventBrandSelected,
			EntityType: analytics.EntityBrand,
			EntityID:   3,
		}, analyticsTestEmail).Return(nil)

		r := &Resolver{AnalyticsService: an}
		ok, err := r.RecordSelection(analyticsCtx(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: analytics.EntityBrand, EntityID: "3"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_RecordSearch(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		an := newAnalyticsMock(t)
		an.EXPECT().RecordEvent(gomock.Any(), analytics.Event{
			UserID:     7,
			EventType:  analytics.EventItemSearched,
			EntityType: analytics.EntityItem,
			SearchTerm: "milk",
		}, analyticsTestEmail).Return(nil)

		r := &Resolver{AnalyticsService: an}
		ok, err := r.RecordSearch(analyticsCtx(), struct {
			EntityType string
			Term       string
		}{EntityType: analytics.EntityItem, Term: "milk"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := &Resolver{AnalyticsService: newAnalyticsMock(t)}
		_, err := r.RecordSearch(context.Background(), struct {
			EntityType string
			Term       string
		}{EntityType: analytics.EntityItem, Term: "milk"})
		require.ErrorContains(t, err, "unauthorized")
	})
}
