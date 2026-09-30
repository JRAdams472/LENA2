package bff

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Account-link endpoints attach verified provider credentials to the
// signed-in user. Linking requires step-up authentication: the bearer
// must be a fresh provider credential, not a LENA session token — a
// stolen ~15-minute access token must not be able to attach an attacker's
// provider identity to the victim's account.

// linkVerifier verifies a provider credential and returns the provider
// identity it proves. *Authenticator implements it via verifyOIDCToken;
// non-OIDC providers plug in here in later phases.
type linkVerifier interface {
	VerifyProviderCredential(ctx context.Context, credential string) (issuer, subject, email, name string, err error)
}

// linkStore is the identity.Service subset the link handler needs.
type linkStore interface {
	LinkLogin(ctx context.Context, userID int64, provider, subject, email, displayName string) error
	UnlinkLogin(ctx context.Context, userID int64, provider string) error
	ListLogins(ctx context.Context, userID int64) ([]identity.Login, error)
}

// LinkHandler serves the linked-identity endpoints.
type LinkHandler struct {
	verifier linkVerifier
	identity linkStore
}

// NewLinkHandler builds the handler.
func NewLinkHandler(v linkVerifier, id linkStore) *LinkHandler {
	return &LinkHandler{verifier: v, identity: id}
}

// RegisterRoutes mounts the endpoints behind the authenticator
// middleware; link additionally rejects session-authenticated requests.
func (h *LinkHandler) RegisterRoutes(e *echo.Echo, authMW, ipLimit echo.MiddlewareFunc) {
	g := e.Group("/auth", middleware.BodyLimit("8K"))
	g.GET("/identities", h.List, ipLimit, authMW)
	g.POST("/link", h.Link, ipLimit, authMW)
	g.DELETE("/link", h.Unlink, ipLimit, authMW)
}

type loginResponse struct {
	Provider    string     `json:"provider"`
	Email       string     `json:"email,omitempty"`
	DisplayName string     `json:"displayName,omitempty"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func toLoginResponses(logins []identity.Login) []loginResponse {
	out := make([]loginResponse, 0, len(logins))
	for _, l := range logins {
		out = append(out, loginResponse{
			Provider:    l.Provider,
			Email:       l.Email,
			DisplayName: l.DisplayName,
			LastLoginAt: l.LastLoginAt,
			CreatedAt:   l.CreatedAt,
		})
	}
	return out
}

func (h *LinkHandler) currentUser(c echo.Context) (currentuser.User, error) {
	u, ok := currentuser.FromContext(c.Request().Context())
	if !ok {
		return currentuser.User{}, echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	return u, nil
}

// List returns the caller's linked provider identities.
func (h *LinkHandler) List(c echo.Context) error {
	u, err := h.currentUser(c)
	if err != nil {
		return err
	}
	logins, err := h.identity.ListLogins(c.Request().Context(), u.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "list logins failed")
	}
	return c.JSON(http.StatusOK, map[string]any{"identities": toLoginResponses(logins)})
}

type linkRequest struct {
	Credential string `json:"credential"`
}

// Link verifies the supplied provider credential and binds it to the
// caller. Session-authenticated requests are rejected — linking is a
// step-up operation that demands a fresh provider credential.
func (h *LinkHandler) Link(c echo.Context) error {
	if isSessionAuth(c.Request().Context()) {
		return echo.NewHTTPError(http.StatusForbidden, "provider credential required to link sign-ins")
	}
	u, err := h.currentUser(c)
	if err != nil {
		return err
	}
	var req linkRequest
	if c.Request().ContentLength == 0 || c.Bind(&req) != nil || req.Credential == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "credential is required")
	}
	issuer, subject, email, name, err := h.verifier.VerifyProviderCredential(c.Request().Context(), req.Credential)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credential")
	}
	if err := h.identity.LinkLogin(c.Request().Context(), u.UserID, issuer, subject, email, name); err != nil {
		if errors.Is(err, identity.ErrLoginTaken) {
			return echo.NewHTTPError(http.StatusConflict, "this sign-in is already linked to a different account")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "link failed")
	}
	return h.List(c)
}

type unlinkRequest struct {
	Provider string `json:"provider"`
}

// Unlink removes a provider login from the caller's account. The last
// remaining sign-in method cannot be removed.
func (h *LinkHandler) Unlink(c echo.Context) error {
	u, err := h.currentUser(c)
	if err != nil {
		return err
	}
	var req unlinkRequest
	if c.Request().ContentLength == 0 || c.Bind(&req) != nil || req.Provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}
	if err := h.identity.UnlinkLogin(c.Request().Context(), u.UserID, req.Provider); err != nil {
		switch {
		case errors.Is(err, identity.ErrLastLogin):
			return echo.NewHTTPError(http.StatusConflict, "cannot remove your last sign-in method")
		case errors.Is(err, identity.ErrPrimaryLogin):
			return echo.NewHTTPError(http.StatusConflict, "cannot remove your primary sign-in")
		case errors.Is(err, domainerr.ErrNotFound):
			return echo.NewHTTPError(http.StatusNotFound, "no sign-in for that provider")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "unlink failed")
	}
	return c.NoContent(http.StatusNoContent)
}
