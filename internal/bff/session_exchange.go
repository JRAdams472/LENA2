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

// identityProvisioner turns a verified provider identity into a LENA
// user (upsert, household backfill, ban check, admin bootstrap).
type identityProvisioner interface {
	ProvisionUser(ctx context.Context, provider, subject, email, name string, emailVerified bool) (currentuser.User, error)
}

// ProviderSessionHandler serves code-exchange sign-ins for OAuth2
// providers (Discord, Microsoft, Facebook). There is no bearer — the
// authorization code is the credential, verified server-side against
// the provider's token endpoint with the configured client secret.
type ProviderSessionHandler struct {
	providers map[string]CodeVerifier
	provision identityProvisioner
	sessions  sessionIssuer
}

// NewProviderSessionHandler builds the handler; providers may be empty.
// Provider names not in the map answer 503.
func NewProviderSessionHandler(providers map[string]CodeVerifier, provision identityProvisioner, sessions sessionIssuer) *ProviderSessionHandler {
	return &ProviderSessionHandler{providers: providers, provision: provision, sessions: sessions}
}

// RegisterRoutes mounts the code-exchange sign-in endpoint. Static
// siblings (/auth/session/refresh, /revoke) take precedence over the
// :provider parameter in echo's router.
func (h *ProviderSessionHandler) RegisterRoutes(e *echo.Echo, mw ...echo.MiddlewareFunc) {
	g := e.Group("/auth/session", middleware.BodyLimit("8K"))
	g.POST("/:provider", h.CreateSession, mw...)
}

type providerSessionRequest struct {
	Code   string `json:"code"`
	Nonce  string `json:"nonce"`
	Device string `json:"device"`
}

// CreateSession exchanges a provider authorization code for a LENA
// session: verify → provision (upsert/household/ban/admin) → issue.
func (h *ProviderSessionHandler) CreateSession(c echo.Context) error {
	name := c.Param("provider")
	verifier, ok := h.providers[name]
	if !ok || verifier == nil || h.sessions == nil || !h.sessions.Enabled() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sign-in with "+name+" is not configured")
	}
	var req providerSessionRequest
	if c.Request().ContentLength == 0 || c.Bind(&req) != nil || req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code is required")
	}
	if utf8Len(req.Device) > maxDeviceLen {
		return echo.NewHTTPError(http.StatusBadRequest, "device label too long")
	}

	ctx := c.Request().Context()
	ident, err := verifier.verify(ctx, req.Code, req.Nonce)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid "+name+" credential")
	}
	u, err := h.provision.ProvisionUser(ctx, ident.provider, ident.subject, ident.email, ident.name, ident.emailVerified)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid "+name+" credential")
	}
	iss, err := h.sessions.Issue(ctx, u.UserID, req.Device)
	if err != nil {
		if errors.Is(err, session.ErrUnavailable) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "sessions are not configured")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "session error")
	}
	setRefreshCookie(c, iss)
	return c.JSON(http.StatusOK, toSessionResponse(iss))
}

func utf8Len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
