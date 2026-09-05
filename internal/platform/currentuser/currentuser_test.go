package currentuser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithUserRoundTrip(t *testing.T) {
	u := User{
		UserID:          42,
		Provider:        "google",
		ExternalSubject: "sub-123",
		Email:           "user@example.com",
		DisplayName:     "Test User",
	}

	ctx := WithUser(context.Background(), u)

	got, ok := FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, u, got)
}

func TestFromContextEmpty(t *testing.T) {
	u, ok := FromContext(context.Background())
	assert.False(t, ok)
	assert.Equal(t, User{}, u)
}

func TestWithUserDoesNotMutateParent(t *testing.T) {
	parent := context.Background()
	ctx := WithUser(parent, User{UserID: 1})

	_, ok := FromContext(parent)
	assert.False(t, ok)

	got, ok := FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, int64(1), got.UserID)
}
