package bff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDiscord(t *testing.T, exchangeStatus int, meStatus int) (*DiscordVerifier, *httptest.Server, *string) {
	t.Helper()
	var gotCode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/token":
			require.NoError(t, r.ParseForm())
			gotCode = r.Form.Get("code")
			assert.Equal(t, "cid", r.Form.Get("client_id"))
			assert.Equal(t, "secret", r.Form.Get("client_secret"))
			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, "http://localhost/cb", r.Form.Get("redirect_uri"))
			w.WriteHeader(exchangeStatus)
			if exchangeStatus == http.StatusOK {
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-1"})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			}
		case "/users/@me":
			assert.Equal(t, "Bearer at-1", r.Header.Get("Authorization"))
			w.WriteHeader(meStatus)
			if meStatus == http.StatusOK {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "123456789", "username": "cord", "global_name": "Cord",
					"email": "d@cord.com", "verified": true,
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"message": "401: Unauthorized"})
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	d := NewDiscordVerifier("cid", "secret", "http://localhost/cb")
	d.base = srv.URL
	return d, srv, &gotCode
}

func TestDiscordVerify(t *testing.T) {
	d, _, gotCode := newTestDiscord(t, http.StatusOK, http.StatusOK)
	id, err := d.verify(t.Context(), "auth-code", "")
	require.NoError(t, err)
	assert.Equal(t, "auth-code", *gotCode)
	assert.Equal(t, "123456789", id.subject)
	assert.Equal(t, "d@cord.com", id.email)
	assert.True(t, id.emailVerified)
	assert.Equal(t, "Cord", id.name)
}

func TestDiscordVerify_ExchangeRejected(t *testing.T) {
	d, _, _ := newTestDiscord(t, http.StatusBadRequest, http.StatusOK)
	_, err := d.verify(t.Context(), "used-code", "")
	assert.ErrorIs(t, err, errDiscordCredential)
}

func TestDiscordVerify_MeRejected(t *testing.T) {
	d, _, _ := newTestDiscord(t, http.StatusOK, http.StatusUnauthorized)
	_, err := d.verify(t.Context(), "code", "")
	assert.ErrorIs(t, err, errDiscordCredential)
}

func TestNewDiscordVerifier_Unconfigured(t *testing.T) {
	assert.Nil(t, NewDiscordVerifier("", "secret", "cb"))
	assert.Nil(t, NewDiscordVerifier("id", "", "cb"))
	assert.NotNil(t, NewDiscordVerifier("id", "secret", "cb"))
}
