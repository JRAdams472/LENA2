package identity_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func TestIntegrationUserLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)

	svc := identity.NewService(pool)

	userA, err := svc.UpsertUser(ctx, "google", "sub-123", "alice@example.com", "Alice")
	require.NoError(t, err)
	require.NotZero(t, userA.UserID)
	assert.Equal(t, "google", userA.Provider)
	assert.Equal(t, "sub-123", userA.ExternalSubject)
	assert.Equal(t, "alice@example.com", userA.Email)
	assert.Equal(t, "Alice", userA.DisplayName)
	assert.True(t, userA.IsActive)

	userA2, err := svc.UpsertUser(ctx, "google", "sub-123", "alice.new@example.com", "Alice Updated")
	require.NoError(t, err)
	assert.Equal(t, userA.UserID, userA2.UserID)
	assert.Equal(t, "alice.new@example.com", userA2.Email)
	assert.Equal(t, "Alice Updated", userA2.DisplayName)

	userB, err := svc.UpsertUser(ctx, "google", "sub-456", "bob@example.com", "Bob")
	require.NoError(t, err)
	assert.NotEqual(t, userA.UserID, userB.UserID)

	got, err := svc.GetByID(ctx, userA.UserID)
	require.NoError(t, err)
	assert.Equal(t, userA.UserID, got.UserID)
	assert.Equal(t, "alice.new@example.com", got.Email)
	assert.Equal(t, "Alice Updated", got.DisplayName)

	_, err = svc.GetByID(ctx, 99999999)
	require.Error(t, err)
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
}

// TestIntegrationHouseholdFields covers the household columns added in
// migration 0028: conditional household assignment, searchability, and the
// invite-candidate search.
func TestIntegrationHouseholdFields(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)

	svc := identity.NewService(pool)
	hsvc := household.NewService(pool)

	caller := testutil.MustUser(ctx, t, pool, "hh-caller@example.com")
	mate := testutil.MustUser(ctx, t, pool, "hh-mate@example.com")
	candidate := testutil.MustUser(ctx, t, pool, "hh-candidate@example.com")
	optedOut := testutil.MustUser(ctx, t, pool, "hh-hidden@example.com")

	// MustUser assigns each user a default household (household_id = user_id,
	// the 0028 backfill convention). Since migration 0044, searchability is
	// opt-in: users default to NOT searchable.
	got, err := svc.GetByID(ctx, caller)
	require.NoError(t, err)
	require.NotNil(t, got.HouseholdID)
	assert.Equal(t, caller, *got.HouseholdID)
	assert.False(t, got.IsSearchable)
	callerH := *got.HouseholdID

	mateGot, err := svc.GetByID(ctx, mate)
	require.NoError(t, err)
	require.NotNil(t, mateGot.HouseholdID)

	// Assign both caller and mate to one household, guarding on their
	// current membership so a stale expected value loses with a conflict.
	// Member reads now come from household.household_member (LEN-26), so
	// each pointer move mirrors a membership swap like the resolver does.
	hh, err := hsvc.CreateHousehold(ctx, "hh-caller@example.com")
	require.NoError(t, err)
	require.NoError(t, svc.SetUserHousehold(ctx, caller, hh.HouseholdID, identity.HouseholdRoleOwner, &callerH))
	_, err = hsvc.JoinHousehold(ctx, hh.HouseholdID, caller, identity.HouseholdRoleOwner, "hh-caller@example.com")
	require.NoError(t, err)
	require.NoError(t, hsvc.RemoveMembership(ctx, callerH, caller))
	require.NoError(t, svc.SetUserHousehold(ctx, mate, hh.HouseholdID, identity.HouseholdRoleMember, mateGot.HouseholdID))
	_, err = hsvc.JoinHousehold(ctx, hh.HouseholdID, mate, identity.HouseholdRoleMember, "hh-caller@example.com")
	require.NoError(t, err)
	require.NoError(t, hsvc.RemoveMembership(ctx, *mateGot.HouseholdID, mate))

	// A stale expected value loses the race with a conflict.
	wrong := int64(999)
	err = svc.SetUserHousehold(ctx, caller, hh.HouseholdID, identity.HouseholdRoleOwner, &wrong)
	assert.ErrorIs(t, err, domainerr.ErrConflict)

	got, err = svc.GetByID(ctx, caller)
	require.NoError(t, err)
	require.NotNil(t, got.HouseholdID)
	assert.Equal(t, hh.HouseholdID, *got.HouseholdID)

	// Opt the candidate in; opt out one user explicitly; deactivate another.
	require.NoError(t, svc.SetUserSearchable(ctx, candidate, true, "hh-candidate@example.com"))
	require.NoError(t, svc.SetUserSearchable(ctx, optedOut, false, "hh-hidden@example.com"))
	inactive := testutil.MustUser(ctx, t, pool, "hh-inactive@example.com")
	_, err = pool.Exec(ctx, "UPDATE identity.users SET is_active = false WHERE user_id = $1", inactive)
	require.NoError(t, err)

	// Search: only the candidate matches — mate shares the caller's
	// household, optedOut opted out, inactive is disabled.
	users, err := svc.SearchUsers(ctx, "hh-", caller, hh.HouseholdID, 20)
	require.NoError(t, err)
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.UserID)
	}
	assert.Contains(t, ids, candidate)
	assert.NotContains(t, ids, mate)
	assert.NotContains(t, ids, optedOut)
	assert.NotContains(t, ids, inactive)
	assert.NotContains(t, ids, caller)

	// Member + batch lookups.
	members, err := svc.ListUsersByHousehold(ctx, hh.HouseholdID)
	require.NoError(t, err)
	assert.Len(t, members, 2)

	batch, err := svc.ListUsersByIDs(ctx, []int64{caller, optedOut})
	require.NoError(t, err)
	assert.Len(t, batch, 2)

	// GetByID on a backfilled row still maps the household pointer.
	got, err = svc.GetByID(ctx, mate)
	require.NoError(t, err)
	require.NotNil(t, got.HouseholdID)
}

// TestIntegrationAdminGuardSerialization proves the last-active-admin
// invariant under concurrency. Two admins demoting each other at once must
// not both succeed — without the advisory lock, each conditional update
// sees the other's snapshot under READ COMMITTED and both pass the
// "other admins exist" check, leaving zero admins.
func TestIntegrationAdminGuardSerialization(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)

	svc := identity.NewService(pool)
	a := testutil.MustUser(ctx, t, pool, "guard-a@example.com")
	b := testutil.MustUser(ctx, t, pool, "guard-b@example.com")
	require.NoError(t, svc.SetUserRole(ctx, a, identity.RoleAdmin))
	require.NoError(t, svc.SetUserRole(ctx, b, identity.RoleAdmin))

	// Concurrent cross-demotion: exactly one may succeed.
	type outcome struct {
		target int64
		err    error
	}
	res := make(chan outcome, 2)
	go func() { res <- outcome{b, svc.AdminSetRole(ctx, a, b, identity.RoleMember)} }()
	go func() { res <- outcome{a, svc.AdminSetRole(ctx, b, a, identity.RoleMember)} }()
	outcomes := []outcome{<-res, <-res}

	var survivor, other int64
	succeeded := 0
	for _, o := range outcomes {
		if o.err == nil {
			succeeded++
			other = o.target
			continue
		}
		assert.ErrorIs(t, o.err, domainerr.ErrLastAdmin)
		survivor = o.target
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent last-admin demotion may succeed")

	// The survivor is the last active admin — both delete and deactivate
	// must be refused.
	err = svc.DeleteUser(ctx, survivor)
	assert.ErrorIs(t, err, domainerr.ErrLastAdmin)

	err = svc.AdminSetActive(ctx, other, survivor, false, "guard@example.com")
	assert.ErrorIs(t, err, domainerr.ErrLastAdmin)

	got, err := svc.GetByID(ctx, survivor)
	require.NoError(t, err)
	assert.Equal(t, identity.RoleAdmin, got.Role)
	assert.True(t, got.IsActive)
}

func TestMain(m *testing.M) {
	os.Exit(testutil.SharedDBTestMain(m))
}
