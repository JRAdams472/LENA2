package bff

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// newRateLimiter builds an Echo rate-limiting middleware keyed by the
// given identifier extractor. perMinute <= 0 disables limiting.
func newRateLimiter(perMinute, burst int, extract middleware.Extractor) echo.MiddlewareFunc {
	if perMinute <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	store := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(float64(perMinute) / 60.0),
		Burst:     burst,
		ExpiresIn: 10 * time.Minute,
	})
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store:               store,
		IdentifierExtractor: extract,
	})
}

// IPRateLimiter returns a middleware that rate-limits requests per client
// IP. It is wired before the authenticator so unauthenticated traffic —
// including garbage-token floods that force JWKS refreshes — is throttled
// before it can reach auth or the resolver. RealIP relies on the echo
// IPExtractor configured on the server (X-Forwarded-For behind Caddy).
func IPRateLimiter(perMinute, burst int) echo.MiddlewareFunc {
	return newRateLimiter(perMinute, burst, func(c echo.Context) (string, error) {
		return "ip:" + c.RealIP(), nil
	})
}

// GraphQLRateLimiter returns a middleware that rate-limits /graphql
// requests per authenticated user. perMinute <= 0 disables limiting. The
// limiter is wired after the authenticator so the user is already present
// in the request context; the ip: fallback below is only a safety net for
// misordered wiring or standalone reuse — in the normal path
// unauthenticated requests are already throttled by IPRateLimiter and
// rejected with 401 before reaching this middleware.
func GraphQLRateLimiter(perMinute, burst int) echo.MiddlewareFunc {
	return newRateLimiter(perMinute, burst, func(c echo.Context) (string, error) {
		if u, ok := currentuser.FromContext(c.Request().Context()); ok {
			return "user:" + strconv.FormatInt(u.UserID, 10), nil
		}
		return "ip:" + c.RealIP(), nil
	})
}
