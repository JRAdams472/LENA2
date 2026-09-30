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

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

type fakeLinkVerifier struct {
	issuer, subject, email, name string
	err                          error
	gotCredential                string
}

func (f *fakeLinkVerifier) VerifyProviderCredential(_ context.Context, credential string) (string, string, string, string, error) {
	f.gotCredential = credential
	return f.issuer, f.subject, f.email, f.name, f.err
}

type fakeLinkStore struct {
	linkErr    error
	unlinkErr  error
	logins     []identity.Login
	listErr    error
	resolveID  int64
	resolveErr error

	linkUID  int64
	linkProv string
	linkSub  string
	unlUID   int64
	unlProv  string
}

func (f *fakeLinkStore) LinkLogin(_ context.Context, userID int64, provider, subject, _, _ string) error {
	f.linkUID, f.linkProv, f.linkSub = userID, provider, subject
	return f.linkErr
}

func (f *fakeLinkStore) UnlinkLogin(_ context.Context, userID int64, provider string) error {
	f.unlUID, f.unlProv = userID, provider
	return f.unlinkErr
}

func (f *fakeLinkStore) ListLogins(_ context.Context, _ int64) ([]identity.Login, error) {
	return f.logins, f.listErr
}

func (f *fakeLinkStore) ResolveLoginUserID(_ context.Context, _, _ string) (int64, error) {
	return f.resolveID, f.resolveErr
}

func linkCtx(t *testing.T, method, target, body string, session bool) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	ctx := currentuser.WithUser(req.Context(), currentuser.User{UserID: 42})
	if session {
		ctx = withSessionAuth(ctx)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req.WithContext(ctx), rec)
	return c, rec
}

func TestLinkIdentities_List(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	store := &fakeLinkStore{logins: []identity.Login{
		{Provider: "https://accounts.google.com", Email: "a@b.com", CreatedAt: now},
		{Provider: "discord", Email: "x@y.com"},
	}}
	h := NewLinkHandler(&fakeLinkVerifier{}, nil, store)
	c, rec := linkCtx(t, http.MethodGet, "/auth/identities", "", false)

	require.NoError(t, h.List(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "discord")
	assert.Contains(t, rec.Body.String(), "a@b.com")
	// Subjects are never exposed.
	assert.NotContains(t, rec.Body.String(), "sub")
}

func TestLinkIdentities_List_NoUser(t *testing.T) {
	h := NewLinkHandler(&fakeLinkVerifier{}, nil, &fakeLinkStore{})
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/identities", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	err := h.List(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusUnauthorized, he.Code)
}

func TestLink_Link(t *testing.T) {
	store := &fakeLinkStore{}
	v := &fakeLinkVerifier{issuer: "https://www.facebook.com", subject: "fb-1", email: "f@b.com", name: "FB"}
	h := NewLinkHandler(v, nil, store)
	c, rec := linkCtx(t, http.MethodPost, "/auth/link", `{"credential":"tok"}`, false)

	require.NoError(t, h.Link(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tok", v.gotCredential)
	assert.Equal(t, int64(42), store.linkUID)
	assert.Equal(t, "https://www.facebook.com", store.linkProv)
	assert.Equal(t, "fb-1", store.linkSub)
}

func TestLink_RejectsSessionAuthWithoutStepUp(t *testing.T) {
	store := &fakeLinkStore{}
	h := NewLinkHandler(&fakeLinkVerifier{}, nil, store)
	c, _ := linkCtx(t, http.MethodPost, "/auth/link", `{"credential":"tok"}`, true)

	err := h.Link(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	assert.Zero(t, store.linkUID)
}

func TestLink_SessionAuthWithStepUp(t *testing.T) {
	// A session bearer + a fresh provider credential that resolves to the
	// same account is allowed — that's the step-up proof.
	store := &fakeLinkStore{resolveID: 42}
	v := &fakeLinkVerifier{issuer: "google", subject: "sub-42"}
	h := NewLinkHandler(v, nil, store)
	c, rec := linkCtx(t, http.MethodPost, "/auth/link",
		`{"credential":"new-tok","currentCredential":"cur-tok"}`, true)

	require.NoError(t, h.Link(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(42), store.linkUID)
	assert.Equal(t, "sub-42", store.linkSub)
}

func TestLink_SessionAuthStepUpWrongAccount(t *testing.T) {
	// The step-up credential resolving to a different account is rejected.
	store := &fakeLinkStore{resolveID: 99}
	v := &fakeLinkVerifier{issuer: "google", subject: "sub-42"}
	h := NewLinkHandler(v, nil, store)
	c, _ := linkCtx(t, http.MethodPost, "/auth/link",
		`{"credential":"new-tok","currentCredential":"cur-tok"}`, true)

	err := h.Link(c)
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.Code)
	assert.Zero(t, store.linkUID)
}

func TestLink_Errors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		verifyErr error
		linkErr   error
		code      int
	}{
		{"missing credential", `{}`, nil, nil, http.StatusBadRequest},
		{"bad provider token", `{"credential":"x"}`, errors.New("bad sig"), nil, http.StatusUnauthorized},
		{"already linked elsewhere", `{"credential":"x"}`, nil, identity.ErrLoginTaken, http.StatusConflict},
		{"store error", `{"credential":"x"}`, nil, errors.New("db"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeLinkStore{linkErr: tc.linkErr}
			v := &fakeLinkVerifier{err: tc.verifyErr}
			h := NewLinkHandler(v, nil, store)
			c, _ := linkCtx(t, http.MethodPost, "/auth/link", tc.body, false)
			err := h.Link(c)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tc.code, he.Code)
		})
	}
}

func TestLink_Unlink(t *testing.T) {
	store := &fakeLinkStore{}
	h := NewLinkHandler(&fakeLinkVerifier{}, nil, store)
	c, rec := linkCtx(t, http.MethodDelete, "/auth/link", `{"provider":"discord"}`, false)

	require.NoError(t, h.Unlink(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, int64(42), store.unlUID)
	assert.Equal(t, "discord", store.unlProv)
}

func TestLink_Unlink_Errors(t *testing.T) {
	cases := []struct {
		name string
		body string
		err  error
		code int
	}{
		{"missing provider", `{}`, nil, http.StatusBadRequest},
		{"last login", `{"provider":"google"}`, identity.ErrLastLogin, http.StatusConflict},
		{"primary login", `{"provider":"google"}`, identity.ErrPrimaryLogin, http.StatusConflict},
		{"unknown provider", `{"provider":"x"}`, domainerr.ErrNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeLinkStore{unlinkErr: tc.err}
			h := NewLinkHandler(&fakeLinkVerifier{}, nil, store)
			c, _ := linkCtx(t, http.MethodDelete, "/auth/link", tc.body, false)
			err := h.Unlink(c)
			var he *echo.HTTPError
			require.ErrorAs(t, err, &he)
			assert.Equal(t, tc.code, he.Code)
		})
	}
}
