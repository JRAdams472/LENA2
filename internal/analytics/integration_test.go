package analytics

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

const itBy = "analytics-integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool), pool
}

func TestIntegrationRecordEventAndCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)
	userID := testenv.MustUser(ctx, t, pool, "analytics-a@example.com")

	err := svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventItemSelected,
		EntityType: EntityItem,
		EntityID:   1,
	}, itBy)
	require.NoError(t, err)

	err = svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventItemSelected,
		EntityType: EntityItem,
		EntityID:   1,
	}, itBy)
	require.NoError(t, err)

	err = svc.RecordEvent(ctx, Event{
		UserID:     userID,
		EventType:  EventBrandSelected,
		EntityType: EntityBrand,
		EntityID:   2,
	}, itBy)
	require.NoError(t, err)

	err = svc.RecordEvent(ctx, Event{
		UserID:    userID,
		EventType: EventItemSearched,
		EntityType: EntityItem,
		SearchTerm: "milk",
	}, itBy)
	require.NoError(t, err)

	counts, err := svc.GetUserSelectionCounts(ctx, userID, EntityItem, []int64{1})
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, int64(2), counts[0].SelectCount)

	global, err := svc.GetGlobalSelectionCounts(ctx, EntityBrand, []int64{2})
	require.NoError(t, err)
	require.Len(t, global, 1)
	assert.Equal(t, int64(1), global[0].SelectCount)

	top, err := svc.TopUserSelections(ctx, userID, EntityItem, 10)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, int64(1), top[0].EntityID)
}
