package bff

import (
	"context"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/session"
)

// Session endpoints live outside GraphQL on purpose: refreshSession must
// be reachable when the access token is already expired, and the GraphQL
// route rejects unauthenticated requests in middleware. The refresh token
// is itself the credential for refresh/revoke — no bearer required.

// sessionIssuer is the subset of *session.Service the endpoints need;
// an interface so tests can substitute a fake.
type sessionIssuer interface {
	Enabled() bool
	Issue(ctx context.Context, userID int64, device string) (session.Issued, error)
	Refresh(ctx context.Context, refreshToken, device string) (session.Issued, error)
	Revoke(ctx context.Context, refreshToken string) error
}

// SessionHandler serves the session credential endpoints.
type SessionHandler struct {
	sessions sessionIssuer
}

// NewSessionHandler builds the handler; sessions may be nil (endpoints
// report 503 when session issuing is not configured).
func NewSessionHandler(sessions sessionIssuer) *SessionHandler {
	return &SessionHandler{sessions: sessions}
}

// RegisterRoutes mounts the session endpoints. Create runs
// behind the authenticator middleware (the bearer credential — a provider
// ID token — is what gets exchanged); refresh/revoke are unauthenticated
// but IP-rate-limited because the refresh token is the credential.
func (h *SessionHandler) RegisterRoutes(e *echo.Echo, authMW, ipLimit echo.MiddlewareFunc) {
	g := e.Group("/auth/session", middleware.BodyLimit("8K"))
	g.POST("", h.Create, ipLimit, authMW)
	g.POST("/refresh", h.Refresh, ipLimit)
	g.POST("/revoke", h.Revoke, ipLimit)
}

type sessionCreateRequest struct {
	Device string `json:"device"`
}

type sessionTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
	Device       string `json:"device"`
}

type sessionResponse struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// maxDeviceLen mirrors identity.session.device VARCHAR(200).
const maxDeviceLen = 200

// Create exchanges the caller's provider credential (validated by the
// auth middleware) for a LENA session. Requests authenticated by a LENA
// access token are rejected — a stolen short-lived token must not be
// able to mint fresh refresh tokens.
func (h *SessionHandler) Create(c echo.Context) error {
	if h.sessions == nil || !h.sessions.Enabled() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sessions are not configured")
	}
	ctx := c.Request().Context()
	if isSessionAuth(ctx) {
		return echo.NewHTTPError(http.StatusForbidden, "session tokens cannot create sessions")
	}
	u, ok := currentuser.FromContext(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	var req sessionCreateRequest
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
	}
	if utf8.RuneCountInString(req.Device) > maxDeviceLen {
		return echo.NewHTTPError(http.StatusBadRequest, "device label too long")
	}
	iss, err := h.sessions.Issue(ctx, u.UserID, req.Device)
	if err != nil {
		return mapSessionError(err)
	}
	return c.JSON(http.StatusOK, toSessionResponse(iss))
}

// Refresh rotates a refresh token into a new credential pair. The refresh
// token is the credential — the caller's access token is expected to be
// expired, so this endpoint is intentionally unauthenticated.
func (h *SessionHandler) Refresh(c echo.Context) error {
	if h.sessions == nil || !h.sessions.Enabled() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sessions are not configured")
	}
	var req sessionTokenRequest
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "refreshToken is required")
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refreshToken is required")
	}
	if utf8.RuneCountInString(req.Device) > maxDeviceLen {
		return echo.NewHTTPError(http.StatusBadRequest, "device label too long")
	}
	iss, err := h.sessions.Refresh(c.Request().Context(), req.RefreshToken, req.Device)
	if err != nil {
		return mapSessionError(err)
	}
	return c.JSON(http.StatusOK, toSessionResponse(iss))
}

// Revoke ends a session (sign-out). The refresh token is the credential.
func (h *SessionHandler) Revoke(c echo.Context) error {
	if h.sessions == nil || !h.sessions.Enabled() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sessions are not configured")
	}
	var req sessionTokenRequest
	if c.Request().ContentLength == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "refreshToken is required")
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refreshToken is required")
	}
	if err := h.sessions.Revoke(c.Request().Context(), req.RefreshToken); err != nil {
		return mapSessionError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func toSessionResponse(iss session.Issued) sessionResponse {
	return sessionResponse{
		AccessToken:  iss.AccessToken,
		RefreshToken: iss.RefreshToken,
		ExpiresAt:    iss.ExpiresAt,
	}
}

// mapSessionError translates service sentinels into HTTP statuses. Reuse
// reports 401 like an invalid token — the response deliberately does not
// distinguish "theft detected" to avoid oracle behavior.
func mapSessionError(err error) error {
	switch {
	case errors.Is(err, session.ErrInvalidSession), errors.Is(err, session.ErrSessionReuse):
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid session")
	case errors.Is(err, session.ErrUnavailable):
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sessions are not configured")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "session error")
	}
}
