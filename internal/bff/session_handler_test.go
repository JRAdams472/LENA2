package bff

import (
	"context"
	"encoding/json"
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

type fakeSessionIssuer struct {
	enabled    bool
	issued     session.Issued
	issueErr   error
	refreshErr error
	revokeErr  error
	gotDevice  string
	gotToken   string
	issueUID   int64
}

func (f *fakeSessionIssuer) Enabled() bool { return f.enabled }

func (f *fakeSessionIssuer) Issue(_ context.Context, userID int64, device string) (session.Issued, error) {
	f.issueUID = userID
	f.gotDevice = device
	return f.issued, f.issueErr
}

func (f *fakeSessionIssuer) Refresh(_ context.Context, refreshToken, device string) (session.Issued, error) {
	f.gotToken = refreshToken
	f.gotDevice = device
	return f.issued, f.refreshErr
}

func (f *fakeSessionIssuer) Revoke(_ context.Context, refreshToken string) error {
	f.gotToken = refreshToken
	return f.revokeErr
}

func sessionCtx(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/session", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestSessionCreate(t *testing.T) {
	exp := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	fake := &fakeSessionIssuer{
		enabled: true,
		issued:  session.Issued{AccessToken: "acc", RefreshToken: "ref", ExpiresAt: exp},
	}
	h := NewSessionHandler(fake)
	c, rec := sessionCtx(t, `{"device":"pixel"}`)
	ctx := currentuser.WithUser(c.Request().Context(), currentuser.User{UserID: 42})
	c.SetRequest(c.Request().WithContext(ctx))

	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(42), fake.issueUID)
	assert.Equal(t, "pixel", fake.gotDevice)
	var resp sessionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "acc", resp.AccessToken)
	assert.Equal(t, "ref", resp.RefreshToken)
	assert.True(t, exp.Equal(resp.ExpiresAt))
}

func TestSessionCreate_RejectsSessionToken(t *testing.T) {
	fake := &fakeSessionIssuer{enabled: true}
	h := NewSessionHandler(fake)
	c, _ := sessionCtx(t, `{}`)
	ctx := currentuser.WithUser(c.Request().Context(), currentuser.User{UserID: 42})
	c.SetRequest(c.Request().WithContext(withSessionAuth(ctx)))

	err := h.Create(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	assert.Zero(t, fake.issueUID)
}

func TestSessionCreate_Disabled(t *testing.T) {
	h := NewSessionHandler(&fakeSessionIssuer{enabled: false})
	c, _ := sessionCtx(t, `{}`)
	err := h.Create(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.Code)
}

func TestSessionCreate_NoUser(t *testing.T) {
	fake := &fakeSessionIssuer{enabled: true}
	h := NewSessionHandler(fake)
	c, _ := sessionCtx(t, `{}`)
	err := h.Create(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestSessionCreate_DeviceTooLong(t *testing.T) {
	fake := &fakeSessionIssuer{enabled: true}
	h := NewSessionHandler(fake)
	c, _ := sessionCtx(t, `{"device":"`+strings.Repeat("x", 201)+`"}`)
	ctx := currentuser.WithUser(c.Request().Context(), currentuser.User{UserID: 1})
	c.SetRequest(c.Request().WithContext(ctx))
	err := h.Create(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSessionRefresh(t *testing.T) {
	fake := &fakeSessionIssuer{
		enabled: true,
		issued:  session.Issued{AccessToken: "new-acc", RefreshToken: "new-ref", ExpiresAt: time.Now()},
	}
	h := NewSessionHandler(fake)
	c, rec := sessionCtx(t, `{"refreshToken":"old","device":"web"}`)

	require.NoError(t, h.Refresh(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "old", fake.gotToken)
	assert.Equal(t, "web", fake.gotDevice)
	var resp sessionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "new-ref", resp.RefreshToken)
}

func TestSessionRefresh_Errors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"unknown token", session.ErrInvalidSession, http.StatusUnauthorized},
		{"reuse detected", session.ErrSessionReuse, http.StatusUnauthorized},
		{"disabled", session.ErrUnavailable, http.StatusServiceUnavailable},
		{"internal", errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeSessionIssuer{enabled: true, refreshErr: tc.err}
			h := NewSessionHandler(fake)
			c, _ := sessionCtx(t, `{"refreshToken":"x"}`)
			err := h.Refresh(c)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tc.code, he.Code)
		})
	}
}

func TestSessionRefresh_MissingToken(t *testing.T) {
	h := NewSessionHandler(&fakeSessionIssuer{enabled: true})
	c, _ := sessionCtx(t, `{}`)
	err := h.Refresh(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSessionRevoke(t *testing.T) {
	fake := &fakeSessionIssuer{enabled: true}
	h := NewSessionHandler(fake)
	c, rec := sessionCtx(t, `{"refreshToken":"bye"}`)

	require.NoError(t, h.Revoke(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "bye", fake.gotToken)
}

func TestSessionRevoke_Unknown(t *testing.T) {
	fake := &fakeSessionIssuer{enabled: true, revokeErr: session.ErrInvalidSession}
	h := NewSessionHandler(fake)
	c, _ := sessionCtx(t, `{"refreshToken":"x"}`)
	err := h.Revoke(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}
