package bff

import (
	"context"
	"errors"
	"fmt"
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
	ResolveLoginUserID(ctx context.Context, provider, subject string) (int64, error)
}

// LinkHandler serves the linked-identity endpoints.
type LinkHandler struct {
	verifier linkVerifier
	discord  codeVerifier
	identity linkStore
}

// NewLinkHandler builds the handler. discord may be nil — requests naming
// that provider then fail verification.
func NewLinkHandler(v linkVerifier, discord codeVerifier, id linkStore) *LinkHandler {
	return &LinkHandler{verifier: v, discord: discord, identity: id}
}

// verifyCredential verifies a provider credential by name: an empty
// provider means an OIDC ID token; "discord" means an OAuth2
// authorization code.
func (h *LinkHandler) verifyCredential(ctx context.Context, provider, credential string) (prov, subject, email, name string, err error) {
	switch provider {
	case "", "oidc":
		return h.verifier.VerifyProviderCredential(ctx, credential)
	case DiscordProvider:
		if h.discord == nil {
			return "", "", "", "", errDiscordCredential
		}
		me, err := h.discord.verify(ctx, credential)
		if err != nil {
			return "", "", "", "", err
		}
		return DiscordProvider, me.subject, me.email, me.name, nil
	default:
		return "", "", "", "", fmt.Errorf("unknown provider %q", provider)
	}
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
	// Provider names the credential type being linked: "" for an OIDC
	// ID token, "discord" for an OAuth2 authorization code.
	Provider   string `json:"provider"`
	Credential string `json:"credential"`
	// CurrentProvider/CurrentCredential provide step-up authentication
	// when the bearer is a LENA session token: a fresh provider credential
	// that must resolve to the caller's own account, so a stolen access
	// token alone cannot attach an attacker's identity.
	CurrentProvider   string `json:"currentProvider"`
	CurrentCredential string `json:"currentCredential"`
}

// Link verifies the supplied provider credential and binds it to the
// caller. A provider-credential bearer is already fresh proof of the
// account; a session bearer must be accompanied by currentCredential.
func (h *LinkHandler) Link(c echo.Context) error {
	u, err := h.currentUser(c)
	if err != nil {
		return err
	}
	var req linkRequest
	if c.Request().ContentLength == 0 || c.Bind(&req) != nil || req.Credential == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "credential is required")
	}
	ctx := c.Request().Context()
	if isSessionAuth(ctx) {
		if req.CurrentCredential == "" {
			return echo.NewHTTPError(http.StatusForbidden, "fresh provider credential required to link sign-ins")
		}
		cp, cs, _, _, err := h.verifyCredential(ctx, req.CurrentProvider, req.CurrentCredential)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid current credential")
		}
		ownerID, err := h.identity.ResolveLoginUserID(ctx, cp, cs)
		if err != nil || ownerID != u.UserID {
			return echo.NewHTTPError(http.StatusForbidden, "current credential does not match this account")
		}
	}
	prov, subject, email, name, err := h.verifyCredential(ctx, req.Provider, req.Credential)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credential")
	}
	if err := h.identity.LinkLogin(ctx, u.UserID, prov, subject, email, name); err != nil {
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
