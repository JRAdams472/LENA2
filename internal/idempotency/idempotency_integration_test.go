package idempotency_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/idempotency"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func newStore(t *testing.T, ctx context.Context, cfg idempotency.Config) (*idempotency.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := testutil.SharedTestDB(t, ctx)
	require.NoError(t, err)
	return idempotency.NewStore(pool, cfg), pool
}

func TestIntegrationClaimCompleteReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{})
	uid := testutil.MustUser(ctx, t, pool, "idem-a@example.com")
	hash := idempotency.Hash([]byte(`{"query":"mutation { a }"}`))

	stored, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	assert.Nil(t, stored, "first request claims the key")

	require.NoError(t, store.Complete(ctx, uid, "k1", 200, []byte(`{"data":{"ok":true}}`)))

	stored, err = store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	require.NotNil(t, stored, "identical retry replays")
	assert.Equal(t, 200, stored.Status)
	assert.JSONEq(t, `{"data":{"ok":true}}`, string(stored.Body))
}

func TestIntegrationKeyReusedDifferentPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{})
	uid := testutil.MustUser(ctx, t, pool, "idem-b@example.com")

	_, err := store.Begin(ctx, uid, "k1", idempotency.Hash([]byte("a")))
	require.NoError(t, err)
	require.NoError(t, store.Complete(ctx, uid, "k1", 200, []byte(`{}`)))

	_, err = store.Begin(ctx, uid, "k1", idempotency.Hash([]byte("b")))
	assert.ErrorIs(t, err, idempotency.ErrKeyReused)
}

func TestIntegrationUserIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{})
	uid1 := testutil.MustUser(ctx, t, pool, "idem-c1@example.com")
	uid2 := testutil.MustUser(ctx, t, pool, "idem-c2@example.com")
	hash := idempotency.Hash([]byte("same body"))

	_, err := store.Begin(ctx, uid1, "shared-key", hash)
	require.NoError(t, err)
	require.NoError(t, store.Complete(ctx, uid1, "shared-key", 200, []byte(`{"data":{"u":1}}`)))

	// A second user with the same key claims independently — never replays
	// another user's response.
	stored, err := store.Begin(ctx, uid2, "shared-key", hash)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestIntegrationInFlightWaitsThenReplays(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{
		WaitTimeout:  5 * time.Second,
		PollInterval: 20 * time.Millisecond,
	})
	uid := testutil.MustUser(ctx, t, pool, "idem-d@example.com")
	hash := idempotency.Hash([]byte("body"))

	_, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(150 * time.Millisecond)
		_ = store.Complete(ctx, uid, "k1", 200, []byte(`{"data":{"slow":true}}`))
	}()

	stored, err := store.Begin(ctx, uid, "k1", hash)
	wg.Wait()
	require.NoError(t, err)
	require.NotNil(t, stored, "duplicate waits for the in-flight twin then replays")
	assert.JSONEq(t, `{"data":{"slow":true}}`, string(stored.Body))
}

func TestIntegrationInFlightTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{
		WaitTimeout:  200 * time.Millisecond,
		PollInterval: 20 * time.Millisecond,
		InFlightTTL:  time.Hour,
	})
	uid := testutil.MustUser(ctx, t, pool, "idem-e@example.com")
	hash := idempotency.Hash([]byte("body"))

	_, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)

	_, err = store.Begin(ctx, uid, "k1", hash)
	assert.ErrorIs(t, err, idempotency.ErrInFlight)
}

func TestIntegrationStaleInFlightReclaimed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{
		InFlightTTL: 50 * time.Millisecond,
	})
	uid := testutil.MustUser(ctx, t, pool, "idem-f@example.com")
	hash := idempotency.Hash([]byte("body"))

	_, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)

	// Backdate the claim past the in-flight TTL to simulate a crashed twin.
	_, err = pool.Exec(ctx,
		`UPDATE platform.idempotency_key SET created_at = now() - interval '1 hour' WHERE user_id = $1 AND key = 'k1'`, uid)
	require.NoError(t, err)

	stored, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	assert.Nil(t, stored, "stale in-progress rows are reclaimable")
}

func TestIntegrationExpiredKeyReclaimed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{})
	uid := testutil.MustUser(ctx, t, pool, "idem-g@example.com")
	hash := idempotency.Hash([]byte("body"))

	_, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	require.NoError(t, store.Complete(ctx, uid, "k1", 200, []byte(`{}`)))

	// Force expiry: the claim upsert must take the row over atomically.
	_, err = pool.Exec(ctx,
		`UPDATE platform.idempotency_key SET expires_at = now() - interval '1 second' WHERE user_id = $1 AND key = 'k1'`, uid)
	require.NoError(t, err)

	stored, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	assert.Nil(t, stored, "expired keys are reclaimable")
}

func TestIntegrationAbandonReleasesClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newStore(t, ctx, idempotency.Config{})
	uid := testutil.MustUser(ctx, t, pool, "idem-h@example.com")
	hash := idempotency.Hash([]byte("body"))

	_, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	require.NoError(t, store.Abandon(ctx, uid, "k1"))

	stored, err := store.Begin(ctx, uid, "k1", hash)
	require.NoError(t, err)
	assert.Nil(t, stored, "abandoned claims are immediately re-claimable")
}

func TestHashAndAutoKey(t *testing.T) {
	h1 := idempotency.Hash([]byte(`{"a":1}`))
	h2 := idempotency.Hash([]byte(`{"a":1}`))
	h3 := idempotency.Hash([]byte(`{"a":2}`))
	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Equal(t, 32, len(h1))

	k := idempotency.AutoKey(h1)
	assert.Contains(t, k, idempotency.AutoKeyPrefix)
	assert.Len(t, k, len(idempotency.AutoKeyPrefix)+64)
}

func TestMain(m *testing.M) {
	os.Exit(testutil.SharedDBTestMain(m))
}
