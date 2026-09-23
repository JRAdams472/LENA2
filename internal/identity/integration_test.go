package identity_test

import (
	"context"
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
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

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
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	svc := identity.NewService(pool)
	hsvc := household.NewService(pool)

	caller := testutil.MustUser(ctx, t, pool, "hh-caller@example.com")
	mate := testutil.MustUser(ctx, t, pool, "hh-mate@example.com")
	candidate := testutil.MustUser(ctx, t, pool, "hh-candidate@example.com")
	optedOut := testutil.MustUser(ctx, t, pool, "hh-hidden@example.com")

	// New users have no household and default to searchable.
	got, err := svc.GetByID(ctx, caller)
	require.NoError(t, err)
	assert.Nil(t, got.HouseholdID)
	assert.True(t, got.IsSearchable)

	// Assign both caller and mate to one household.
	hh, err := hsvc.CreateHousehold(ctx, "hh-caller@example.com")
	require.NoError(t, err)
	require.NoError(t, svc.SetUserHousehold(ctx, caller, hh.HouseholdID, nil))
	require.NoError(t, svc.SetUserHousehold(ctx, mate, hh.HouseholdID, nil))

	// A stale expected value loses the race with a conflict.
	wrong := int64(999)
	err = svc.SetUserHousehold(ctx, caller, hh.HouseholdID, &wrong)
	assert.ErrorIs(t, err, domainerr.ErrConflict)

	got, err = svc.GetByID(ctx, caller)
	require.NoError(t, err)
	require.NotNil(t, got.HouseholdID)
	assert.Equal(t, hh.HouseholdID, *got.HouseholdID)

	// Opt out one user; deactivate another.
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
