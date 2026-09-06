package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setRequiredEnv sets the minimum env vars needed for Load to succeed.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("LENA_DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LENA_GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("LENA_AUTH_AUDIENCES", "aud1,aud2")
}

func TestLoadDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "https://accounts.google.com", cfg.AuthIssuers)
	assert.Equal(t, "http://localhost", cfg.CORSAllowedOrigins)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DatabaseURL)
	assert.Equal(t, "client-id", cfg.GoogleClientID)
	assert.Equal(t, "aud1,aud2", cfg.AuthAudiences)
	assert.Empty(t, cfg.AdminEmails)
}

// TestLoadOptionalGoogleClientID asserts GOOGLE_CLIENT_ID is optional: it is
// only consumed by clients, not by server-side audience validation.
func TestLoadOptionalGoogleClientID(t *testing.T) {
	t.Setenv("LENA_DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LENA_AUTH_AUDIENCES", "aud1")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.GoogleClientID)
}

func TestLoadOverrides(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LENA_PORT", "9090")
	t.Setenv("LENA_LOG_LEVEL", "debug")
	t.Setenv("LENA_AUTH_ISSUERS", "https://issuer.example.com")
	t.Setenv("LENA_CORS_ALLOWED_ORIGINS", "https://app.example.com")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "https://issuer.example.com", cfg.AuthIssuers)
	assert.Equal(t, "https://app.example.com", cfg.CORSAllowedOrigins)
}

func TestLoadMissingRequired(t *testing.T) {
	cases := []struct {
		name    string
		envVars map[string]string
	}{
		{"missing all", map[string]string{}},
		{"missing database url", map[string]string{
			"LENA_AUTH_AUDIENCES": "aud",
		}},
		{"missing auth audiences", map[string]string{
			"LENA_DATABASE_URL":     "postgres://x",
			"LENA_GOOGLE_CLIENT_ID": "id",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}
			cfg, err := Load()
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "failed to load config")
		})
	}
}
