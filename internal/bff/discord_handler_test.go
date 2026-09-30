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
	id  discordIdentity
	err error

	gotCode string
}

func (f *fakeCodeVerifier) verify(_ context.Context, code string) (discordIdentity, error) {
	f.gotCode = code
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

func discordCtx(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/session/discord", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestDiscordCreateSession(t *testing.T) {
	dv := &fakeCodeVerifier{id: discordIdentity{
		subject: "123456789", email: "d@cord.com", emailVerified: true, name: "Cord",
	}}
	prov := &fakeProvisioner{user: currentuser.User{UserID: 7}}
	sess := &fakeSessionIssuer{enabled: true, issued: session.Issued{
		AccessToken: "acc", RefreshToken: "ref", ExpiresAt: time.Now().Add(time.Hour),
	}}
	h := NewDiscordHandler(dv, prov, sess)

	c, rec := discordCtx(t, `{"code":"abc","device":"web"}`)
	require.NoError(t, h.CreateSession(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "abc", dv.gotCode)
	assert.Equal(t, "discord", prov.provider)
	assert.Equal(t, "123456789", prov.subject)
	assert.Equal(t, "d@cord.com", prov.email)
	assert.True(t, prov.verified)
	assert.Equal(t, int64(7), sess.issueUID)
	assert.Contains(t, rec.Body.String(), "ref")
}

func TestDiscordCreateSession_Disabled(t *testing.T) {
	// No verifier (provider not configured) → 503.
	h := NewDiscordHandler(nil, &fakeProvisioner{}, &fakeSessionIssuer{enabled: true})
	c, _ := discordCtx(t, `{"code":"abc"}`)
	err := h.CreateSession(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.Code)

	// Sessions disabled → 503 even with the provider configured.
	h = NewDiscordHandler(&fakeCodeVerifier{}, &fakeProvisioner{}, &fakeSessionIssuer{enabled: false})
	c, _ = discordCtx(t, `{"code":"abc"}`)
	err = h.CreateSession(c)
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.Code)
}

func TestDiscordCreateSession_Errors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		verifyErr error
		provErr   error
		issueErr  error
		code      int
	}{
		{"missing code", `{}`, nil, nil, nil, http.StatusBadRequest},
		{"bad code", `{"code":"x"}`, errDiscordCredential, nil, nil, http.StatusUnauthorized},
		{"provision failure", `{"code":"x"}`, nil, errors.New("banned"), nil, http.StatusUnauthorized},
		{"issue failure", `{"code":"x"}`, nil, nil, errors.New("db"), http.StatusInternalServerError},
		{"sessions unavailable", `{"code":"x"}`, nil, nil, session.ErrUnavailable, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dv := &fakeCodeVerifier{id: discordIdentity{subject: "s"}, err: tc.verifyErr}
			prov := &fakeProvisioner{user: currentuser.User{UserID: 1}, err: tc.provErr}
			sess := &fakeSessionIssuer{enabled: true, issueErr: tc.issueErr}
			h := NewDiscordHandler(dv, prov, sess)
			c, _ := discordCtx(t, tc.body)
			err := h.CreateSession(c)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tc.code, he.Code)
		})
	}
}
