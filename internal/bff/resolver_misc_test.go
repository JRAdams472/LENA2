package bff

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

func TestMisc_Me_Unauthorized(t *testing.T) {
	r := &Resolver{}
	res, err := r.Me(context.Background())
	assert.Nil(t, res)
	assert.EqualError(t, err, "unauthorized")
}

func TestMisc_Me_Happy(t *testing.T) {
	r := &Resolver{}
	ctx := testenv.WithUser(context.Background(), 42, "me@example.com")

	res, err := r.Me(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("42"), res.ID())
	assert.Equal(t, "me@example.com", res.Email())
	assert.Nil(t, res.DisplayName()) // testenv user has no display name
}

func TestMisc_Me_DisplayName(t *testing.T) {
	r := &Resolver{}
	ctx := currentuser.WithUser(context.Background(), currentuser.User{
		UserID: 43, Email: "named@example.com", DisplayName: "Named User",
	})

	res, err := r.Me(ctx)
	require.NoError(t, err)
	require.NotNil(t, res.DisplayName())
	assert.Equal(t, "Named User", *res.DisplayName())
}

func TestMisc_UserFromContext(t *testing.T) {
	u, err := userFromContext(context.Background())
	assert.Error(t, err)
	assert.Equal(t, currentuser.User{}, u)

	ctx := testenv.WithUser(context.Background(), 42, "me@example.com")
	u, err = userFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(42), u.UserID)
	assert.Equal(t, "me@example.com", u.Email)
}

func TestMisc_ParseID(t *testing.T) {
	v, err := parseID("123")
	require.NoError(t, err)
	assert.Equal(t, int64(123), v)

	v, err = parseID("-5")
	require.NoError(t, err)
	assert.Equal(t, int64(-5), v)

	_, err = parseID("abc")
	assert.Error(t, err)

	_, err = parseID("")
	assert.Error(t, err)
}

func TestMisc_OptionalID(t *testing.T) {
	v, err := optionalID(nil)
	require.NoError(t, err)
	assert.Nil(t, v)

	id := graphql.ID("77")
	v, err = optionalID(&id)
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int64(77), *v)

	bad := graphql.ID("nope")
	v, err = optionalID(&bad)
	assert.Nil(t, v)
	assert.Error(t, err)
}

func TestMisc_NilIfEmpty(t *testing.T) {
	assert.Nil(t, nilIfEmpty(""))
	got := nilIfEmpty("hi")
	require.NotNil(t, got)
	assert.Equal(t, "hi", *got)
}

func TestMisc_Int32Ptr(t *testing.T) {
	got := int32Ptr(0)
	require.NotNil(t, got)
	assert.Equal(t, int32(0), *got)
	got = int32Ptr(7)
	require.NotNil(t, got)
	assert.Equal(t, int32(7), *got)
}

func TestMisc_Clamp(t *testing.T) {
	assert.Equal(t, int32(1), clamp(0, 1, 100))
	assert.Equal(t, int32(1), clamp(-50, 1, 100))
	assert.Equal(t, int32(50), clamp(50, 1, 100))
	assert.Equal(t, int32(100), clamp(101, 1, 100))
}

func TestMisc_TimeToGraphQL(t *testing.T) {
	assert.Nil(t, timeToGraphQL(nil))
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	got := timeToGraphQL(&now)
	require.NotNil(t, got)
	assert.True(t, got.Equal(now))
}

func TestMisc_DerefString(t *testing.T) {
	assert.Equal(t, "", derefString(nil))
	s := "hello"
	assert.Equal(t, "hello", derefString(&s))
}

func TestMisc_Int32Value(t *testing.T) {
	assert.Equal(t, int32(0), int32Value(nil))
	v := int32(9)
	assert.Equal(t, int32(9), int32Value(&v))
}

func TestMisc_BoolValue(t *testing.T) {
	assert.False(t, boolValue(nil))
	v := true
	assert.True(t, boolValue(&v))
}

func TestMisc_CheckedInt16(t *testing.T) {
	v, err := checkedInt16(3, "rating", 1, 5)
	require.NoError(t, err)
	assert.Equal(t, int16(3), v)

	for _, bad := range []int32{0, 6, -1, 65537, math.MinInt32} {
		_, err := checkedInt16(bad, "rating", 1, 5)
		assert.ErrorContains(t, err, "rating must be between 1 and 5", "input %d", bad)
	}
}

func TestMisc_CheckedInt16Ptr(t *testing.T) {
	got, err := checkedInt16Ptr(nil, "acidity", 1, 5)
	require.NoError(t, err)
	assert.Nil(t, got)

	in := int32(4)
	got, err = checkedInt16Ptr(&in, "acidity", 1, 5)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int16(4), *got)

	bad := int32(40000)
	got, err = checkedInt16Ptr(&bad, "acidity", 1, 5)
	assert.Nil(t, got)
	assert.Error(t, err)
}

func TestMisc_UnitName_PreloadedMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockInventoryService(ctrl)

	// A preloaded map that lacks the requested id must surface an error,
	// not silently render an empty unit name.
	units := map[int64]inventory.Unit{3: {UnitID: 3, Name: "cup"}}
	_, err := unitName(context.Background(), inv, units, 99)
	assert.ErrorContains(t, err, "unit 99 missing from preloaded set")

	// Hit and lazy paths unchanged.
	name, err := unitName(context.Background(), inv, units, 3)
	require.NoError(t, err)
	assert.Equal(t, "cup", name)
}

func TestResolver_AsyncWorkerBoundsAndDrains(t *testing.T) {
	r := &Resolver{}

	var running, maxSeen atomic.Int32

	// Saturate the worker: every submitted task blocks until its context
	// is cancelled by Shutdown.
	for i := 0; i < asyncWorkerCap; i++ {
		r.runAsync("blocked", time.Minute, func(ctx context.Context) error {
			cur := running.Add(1)
			for {
				if m := maxSeen.Load(); cur > m && !maxSeen.CompareAndSwap(m, cur) {
					continue
				}
				break
			}
			defer running.Add(-1)
			<-ctx.Done()
			return nil
		})
	}
	// Give the goroutines a moment to start.
	time.Sleep(50 * time.Millisecond)

	// The (cap+1)th task must be dropped, not queued.
	var dropped atomic.Int32
	for i := 0; i < asyncWorkerCap; i++ {
		r.runAsync("dropped", time.Second, func(context.Context) error {
			dropped.Add(1)
			return nil
		})
	}
	assert.Zero(t, dropped.Load())
	assert.LessOrEqual(t, maxSeen.Load(), int32(asyncWorkerCap))

	// Shutdown cancels in-flight tasks and returns once they exit.
	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, r.Shutdown(shCtx))
}

func TestResolver_ShutdownOnUnusedResolver(t *testing.T) {
	r := &Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, r.Shutdown(ctx))
}
