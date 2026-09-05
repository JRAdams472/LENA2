package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPoolInvalidURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantMsg string
	}{
		{"not a url", "not-a-database-url", "invalid database url"},
		{"bad scheme", "http://localhost:5432/db", "invalid database url"},
		// Empty DSN parses to env/defaults; failure then happens at ping.
		{"empty", "", "failed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			pool, err := NewPool(ctx, tc.url)
			require.Error(t, err)
			assert.Nil(t, pool)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestNewPoolUnreachableHost(t *testing.T) {
	// Valid DSN pointing at a closed port; Ping must fail without a live DB.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, "postgres://user:pass@127.0.0.1:1/db?connect_timeout=1")
	require.Error(t, err)
	assert.Nil(t, pool)
}
