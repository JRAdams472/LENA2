package household_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func newService(t *testing.T, ctx context.Context) (*household.Service, *pgxpool.Pool, func()) {
	t.Helper()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	return household.NewService(pool), pool, cleanup
}

// TestIntegrationInviteLifecycle walks the happy path: household create,
// invite, both list views, accept transition.
func TestIntegrationInviteLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool, cleanup := newService(t, ctx)
	t.Cleanup(cleanup)

	inviter := testutil.MustUser(ctx, t, pool, "hh-inviter@example.com")
	target := testutil.MustUser(ctx, t, pool, "hh-target@example.com")

	h, err := svc.CreateHousehold(ctx, "hh-inviter@example.com")
	require.NoError(t, err)
	require.NotZero(t, h.HouseholdID)

	got, err := svc.GetHouseholdByID(ctx, h.HouseholdID)
	require.NoError(t, err)
	assert.Equal(t, h.HouseholdID, got.HouseholdID)

	inv, err := svc.CreateInvite(ctx, inviter, target, h.HouseholdID, "hh-inviter@example.com")
	require.NoError(t, err)
	assert.Equal(t, household.StatusPending, inv.Status)

	pending, err := svc.ListPendingInvitesForUser(ctx, target)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, inv.InviteID, pending[0].InviteID)

	sent, err := svc.ListSentInvitesForUser(ctx, inviter)
	require.NoError(t, err)
	require.Len(t, sent, 1)

	out, err := svc.TransitionInvite(ctx, inv.InviteID, household.StatusAccepted, "hh-target@example.com")
	require.NoError(t, err)
	assert.Equal(t, household.StatusAccepted, out.Status)

	// The concluded invite leaves both pending lists.
	pending, err = svc.ListPendingInvitesForUser(ctx, target)
	require.NoError(t, err)
	assert.Empty(t, pending)
	sent, err = svc.ListSentInvitesForUser(ctx, inviter)
	require.NoError(t, err)
	assert.Empty(t, sent)
}

// TestIntegrationInviteGuards pins the concurrency contracts: duplicate
// pending invites conflict, and a concluded invite cannot transition again.
func TestIntegrationInviteGuards(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool, cleanup := newService(t, ctx)
	t.Cleanup(cleanup)

	inviter := testutil.MustUser(ctx, t, pool, "hh-guard-a@example.com")
	target := testutil.MustUser(ctx, t, pool, "hh-guard-b@example.com")

	h, err := svc.CreateHousehold(ctx, "hh-guard-a@example.com")
	require.NoError(t, err)

	inv, err := svc.CreateInvite(ctx, inviter, target, h.HouseholdID, "hh-guard-a@example.com")
	require.NoError(t, err)

	// A second pending invite between the same pair hits the partial
	// unique index -> ErrConflict.
	_, err = svc.CreateInvite(ctx, inviter, target, h.HouseholdID, "hh-guard-a@example.com")
	assert.ErrorIs(t, err, domainerr.ErrConflict)

	// Conclude it; a second transition loses the status race -> ErrConflict.
	_, err = svc.TransitionInvite(ctx, inv.InviteID, household.StatusDeclined, "hh-guard-b@example.com")
	require.NoError(t, err)
	_, err = svc.TransitionInvite(ctx, inv.InviteID, household.StatusAccepted, "hh-guard-b@example.com")
	assert.ErrorIs(t, err, domainerr.ErrConflict)

	// Re-inviting after a concluded invite is allowed (no unique violation).
	inv2, err := svc.CreateInvite(ctx, inviter, target, h.HouseholdID, "hh-guard-a@example.com")
	require.NoError(t, err)
	assert.Equal(t, household.StatusPending, inv2.Status)

	_, err = svc.GetInviteByID(ctx, 999999)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}
