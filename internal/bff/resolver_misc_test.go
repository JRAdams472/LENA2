package bff

import (
	"context"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
