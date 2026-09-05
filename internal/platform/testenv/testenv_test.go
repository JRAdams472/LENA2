package testenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

func TestWithUserPopulatesContext(t *testing.T) {
	ctx := WithUser(context.Background(), 42, "user@example.com")

	u, ok := currentuser.FromContext(ctx)
	require.True(t, ok, "expected a currentuser.User in context")
	assert.Equal(t, int64(42), u.UserID)
	assert.Equal(t, "test-provider", u.Provider)
	assert.Equal(t, "user@example.com", u.Email)
}

func TestWithUserEmptyContext(t *testing.T) {
	_, ok := currentuser.FromContext(context.Background())
	assert.False(t, ok, "expected no currentuser.User in a bare context")
}
