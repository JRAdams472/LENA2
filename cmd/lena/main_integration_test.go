package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/logger"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	defer cleanup()

	issuer := testenv.NewTestIssuer(t)

	cfg := config.Config{
		DatabaseURL:        pool.Config().ConnString(),
		Port:               "0",
		CORSAllowedOrigins: "*",
		AuthIssuers:        issuer.URL,
		AuthAudiences:      testenv.TestAudience,
		LogLevel:           "error",
	}

	log := logger.New(cfg.LogLevel)
	e := newServer(cfg, pool, log, nil)
	srv := httptest.NewServer(e)
	defer srv.Close()

	t.Run("health returns 200", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/health")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"status":"ok"`)
	})

	t.Run("ready returns 200", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/ready")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("graphql without token returns 401", func(t *testing.T) {
		req := graphQLRequest(t, srv.URL, "", `{ me { email } }`, nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("graphql with valid token returns user", func(t *testing.T) {
		token := issuer.Token(t, "int-test-user", "int@example.com", "Integration User")
		req := graphQLRequest(t, srv.URL, token, `{ me { email } }`, nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var gr graphQLResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&gr))

		var data struct {
			Me struct {
				Email string `json:"email"`
			} `json:"me"`
		}
		require.NoError(t, json.Unmarshal(gr.Data, &data))
		assert.Equal(t, "int@example.com", data.Me.Email)
	})
}

type graphQLResponse struct {
	Data json.RawMessage `json:"data"`
}

func graphQLRequest(t *testing.T, baseURL, token, query string, vars map[string]any) *http.Request {
	payload, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": vars,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/graphql", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}
