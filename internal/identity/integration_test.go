package identity_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

func TestIntegrationUserLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
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
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}
