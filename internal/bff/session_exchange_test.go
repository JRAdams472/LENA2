package bff

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/session"
)

type fakeCodeVerifier struct {
	id  providerIdentity
	err error

	gotCode     string
	gotNonce    string
	gotVerifier string
}

func (f *fakeCodeVerifier) verify(_ context.Context, code, nonce, codeVerifier string) (providerIdentity, error) {
	f.gotCode, f.gotNonce, f.gotVerifier = code, nonce, codeVerifier
	return f.id, f.err
}

type fakeProvisioner struct {
	user currentuser.User
	err  error

	provider, subject, email, name string
	verified                       bool
}

func (f *fakeProvisioner) ProvisionUser(_ context.Context, provider, subject, email, name string, emailVerified bool) (currentuser.User, error) {
	f.provider, f.subject, f.email, f.name, f.verified = provider, subject, email, name, emailVerified
	return f.user, f.err
}

func exchangeCtx(t *testing.T, provider, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/session/"+provider, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues(provider)
	return c, rec
}

func TestProviderCreateSession(t *testing.T) {
	dv := &fakeCodeVerifier{id: providerIdentity{
		provider: "discord", subject: "123456789",
		email: "d@cord.com", emailVerified: true, name: "Cord",
	}}
	prov := &fakeProvisioner{user: currentuser.User{UserID: 7}}
	sess := &fakeSessionIssuer{enabled: true, issued: session.Issued{
		AccessToken: "acc", RefreshToken: "ref", ExpiresAt: time.Now().Add(time.Hour),
	}}
	h := NewProviderSessionHandler(map[string]CodeVerifier{"discord": dv}, prov, sess)

	c, rec := exchangeCtx(t, "discord", `{"code":"abc","nonce":"n1","device":"web","codeVerifier":"v1"}`)
	require.NoError(t, h.CreateSession(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "abc", dv.gotCode)
	assert.Equal(t, "n1", dv.gotNonce)
	assert.Equal(t, "v1", dv.gotVerifier)
	assert.Equal(t, "discord", prov.provider)
	assert.Equal(t, "123456789", prov.subject)
	assert.Equal(t, "d@cord.com", prov.email)
	assert.True(t, prov.verified)
	assert.Equal(t, int64(7), sess.issueUID)
	assert.Contains(t, rec.Body.String(), "ref")
}

func TestProviderCreateSession_UnknownProvider(t *testing.T) {
	h := NewProviderSessionHandler(map[string]CodeVerifier{"discord": &fakeCodeVerifier{}},
		&fakeProvisioner{}, &fakeSessionIssuer{enabled: true})
	c, _ := exchangeCtx(t, "microsoft", `{"code":"abc","codeVerifier":"v1"}`)
	err := h.CreateSession(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.Code)
}

func TestProviderCreateSession_Disabled(t *testing.T) {
	// Provider absent from the registry (not configured) → 503.
	h := NewProviderSessionHandler(nil, &fakeProvisioner{}, &fakeSessionIssuer{enabled: true})
	c, _ := exchangeCtx(t, "discord", `{"code":"abc","codeVerifier":"v1"}`)
	err := h.CreateSession(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.Code)

	// Sessions disabled → 503 even with the provider configured.
	h = NewProviderSessionHandler(map[string]CodeVerifier{"discord": &fakeCodeVerifier{}},
		&fakeProvisioner{}, &fakeSessionIssuer{enabled: false})
	c, _ = exchangeCtx(t, "discord", `{"code":"abc","codeVerifier":"v1"}`)
	err = h.CreateSession(c)
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.Code)
}

func TestProviderCreateSession_Errors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		verifyErr error
		provErr   error
		issueErr  error
		code      int
	}{
		{"missing code", `{}`, nil, nil, nil, http.StatusBadRequest},
		{"missing verifier", `{"code":"x"}`, nil, nil, nil, http.StatusBadRequest},
		{"bad code", `{"code":"x","codeVerifier":"v1"}`, errDiscordCredential, nil, nil, http.StatusUnauthorized},
		{"provision failure", `{"code":"x","codeVerifier":"v1"}`, nil, errors.New("banned"), nil, http.StatusUnauthorized},
		{"issue failure", `{"code":"x","codeVerifier":"v1"}`, nil, nil, errors.New("db"), http.StatusInternalServerError},
		{"sessions unavailable", `{"code":"x","codeVerifier":"v1"}`, nil, nil, session.ErrUnavailable, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dv := &fakeCodeVerifier{id: providerIdentity{provider: "discord", subject: "s"}, err: tc.verifyErr}
			prov := &fakeProvisioner{user: currentuser.User{UserID: 1}, err: tc.provErr}
			sess := &fakeSessionIssuer{enabled: true, issueErr: tc.issueErr}
			h := NewProviderSessionHandler(map[string]CodeVerifier{"discord": dv}, prov, sess)
			c, _ := exchangeCtx(t, "discord", tc.body)
			err := h.CreateSession(c)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tc.code, he.Code)
		})
	}
}
