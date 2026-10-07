package household_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func newService(t *testing.T, ctx context.Context) (*household.Service, *pgxpool.Pool) {
	t.Helper()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)
	return household.NewService(pool), pool
}

// TestIntegrationInviteLifecycle walks the happy path: household create,
// invite, both list views, accept transition.
func TestIntegrationInviteLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newService(t, ctx)

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

	sent, err := svc.ListSentInvitesForUser(ctx, inviter, h.HouseholdID)
	require.NoError(t, err)
	require.Len(t, sent, 1)

	out, err := svc.TransitionInvite(ctx, inv.InviteID, household.StatusAccepted, "hh-target@example.com")
	require.NoError(t, err)
	assert.Equal(t, household.StatusAccepted, out.Status)

	// The concluded invite leaves both pending lists.
	pending, err = svc.ListPendingInvitesForUser(ctx, target)
	require.NoError(t, err)
	assert.Empty(t, pending)
	sent, err = svc.ListSentInvitesForUser(ctx, inviter, h.HouseholdID)
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
	svc, pool := newService(t, ctx)

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

// TestIntegrationMembership covers the LEN-26 membership layer: the
// MustUser bootstrap mirrors a membership row, a user can hold a second
// membership, member reads scope per household, and SetActiveHousehold
// syncs the pointer role from the membership row.
func TestIntegrationMembership(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newService(t, ctx)
	isvc := identity.NewService(pool)

	// Bootstrap backfill-equivalence: MustUser's active pointer must have a
	// matching membership row (mirrors migration 0053's backfill).
	user := testutil.MustUser(ctx, t, pool, "mem-user@example.com")
	u, err := isvc.GetByID(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, u.HouseholdID)
	homeID := *u.HouseholdID

	m, err := svc.GetMembership(ctx, homeID, user)
	require.NoError(t, err)
	assert.Equal(t, identity.HouseholdRoleOwner, m.Role)

	mine, err := svc.ListMyHouseholds(ctx, user)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, homeID, mine[0].Household.HouseholdID)

	// Dual membership: join a second household without moving the pointer.
	second, err := svc.CreateHousehold(ctx, "mem-user@example.com")
	require.NoError(t, err)
	_, err = svc.JoinHousehold(ctx, second.HouseholdID, user, identity.HouseholdRoleMember, "mem-user@example.com")
	require.NoError(t, err)

	mine, err = svc.ListMyHouseholds(ctx, user)
	require.NoError(t, err)
	require.Len(t, mine, 2)
	roles := map[int64]string{mine[0].Household.HouseholdID: mine[0].Role, mine[1].Household.HouseholdID: mine[1].Role}
	assert.Equal(t, map[int64]string{homeID: "owner", second.HouseholdID: "member"}, roles)

	// Isolation: the user shows up in both member lists; a stranger shows
	// up in neither.
	stranger := testutil.MustUser(ctx, t, pool, "mem-stranger@example.com")
	for _, hhID := range []int64{homeID, second.HouseholdID} {
		ids := memberIDs(t, isvc, ctx, hhID)
		assert.Contains(t, ids, user)
		assert.NotContains(t, ids, stranger)
	}
	n, err := isvc.CountUsersByHousehold(ctx, second.HouseholdID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Active-household switch: pointer moves and household_role syncs from
	// the membership row (member in `second`, owner in `home`).
	require.NoError(t, isvc.SetActiveHousehold(ctx, user, second.HouseholdID, "mem-user@example.com"))
	u, err = isvc.GetByID(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, u.HouseholdID)
	assert.Equal(t, second.HouseholdID, *u.HouseholdID)
	assert.Equal(t, identity.HouseholdRoleMember, u.HouseholdRole)

	// Switching to a household the user does not belong to conflicts.
	err = isvc.SetActiveHousehold(ctx, user, 99999999, "mem-user@example.com")
	assert.ErrorIs(t, err, domainerr.ErrConflict)

	// Removing the second membership shrinks the switcher back to one.
	require.NoError(t, svc.RemoveMembership(ctx, second.HouseholdID, user))
	mine, err = svc.ListMyHouseholds(ctx, user)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.ErrorIs(t, svc.RemoveMembership(ctx, second.HouseholdID, user), domainerr.ErrNotFound)
}

func memberIDs(t *testing.T, isvc *identity.Service, ctx context.Context, householdID int64) []int64 {
	t.Helper()
	users, err := isvc.ListUsersByHousehold(ctx, householdID)
	require.NoError(t, err)
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.UserID)
	}
	return ids
}

func TestMain(m *testing.M) {
	os.Exit(testutil.SharedDBTestMain(m))
}
