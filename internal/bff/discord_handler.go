package bff

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/session"
)

// DiscordProvider is the opaque provider key stored on users/logins for
// Discord identities. Unlike OIDC providers there is no issuer URL — the
// string is just a discriminator.
const DiscordProvider = "discord"

// codeVerifier exchanges an OAuth2 authorization code for the provider
// identity it proves.
type codeVerifier interface {
	verify(ctx context.Context, code string) (discordIdentity, error)
}

// identityProvisioner turns a verified provider identity into a LENA user
// (upsert, household backfill, ban check, admin bootstrap).
type identityProvisioner interface {
	ProvisionUser(ctx context.Context, provider, subject, email, name string, emailVerified bool) (currentuser.User, error)
}

// DiscordHandler serves the Discord code-exchange sign-in. There is no
// bearer — the authorization code is the credential, verified server-side
// against Discord's token endpoint with the configured client secret.
type DiscordHandler struct {
	discord   codeVerifier
	provision identityProvisioner
	sessions  sessionIssuer
}

// NewDiscordHandler builds the handler; discord may be nil (disabled).
func NewDiscordHandler(discord codeVerifier, provision identityProvisioner, sessions sessionIssuer) *DiscordHandler {
	return &DiscordHandler{discord: discord, provision: provision, sessions: sessions}
}

// RegisterRoutes mounts the Discord sign-in endpoint.
func (h *DiscordHandler) RegisterRoutes(e *echo.Echo, ipLimit echo.MiddlewareFunc) {
	g := e.Group("/auth/session/discord", middleware.BodyLimit("8K"))
	g.POST("", h.CreateSession, ipLimit)
}

type discordSessionRequest struct {
	Code   string `json:"code"`
	Device string `json:"device"`
}

// CreateSession exchanges a Discord authorization code for a LENA
// session: verify → provision (upsert/household/ban/admin) → issue.
func (h *DiscordHandler) CreateSession(c echo.Context) error {
	if h.discord == nil || h.sessions == nil || !h.sessions.Enabled() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "discord sign-in is not configured")
	}
	var req discordSessionRequest
	if c.Request().ContentLength == 0 || c.Bind(&req) != nil || req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code is required")
	}
	if utf8Len(req.Device) > maxDeviceLen {
		return echo.NewHTTPError(http.StatusBadRequest, "device label too long")
	}

	ctx := c.Request().Context()
	me, err := h.discord.verify(ctx, req.Code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid discord credential")
	}
	u, err := h.provision.ProvisionUser(ctx, DiscordProvider, me.subject, me.email, me.name, me.emailVerified)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid discord credential")
	}
	iss, err := h.sessions.Issue(ctx, u.UserID, req.Device)
	if err != nil {
		if errors.Is(err, session.ErrUnavailable) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "sessions are not configured")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "session error")
	}
	return c.JSON(http.StatusOK, toSessionResponse(iss))
}

func utf8Len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
