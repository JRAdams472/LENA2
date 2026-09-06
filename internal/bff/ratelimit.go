package bff

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// GraphQLRateLimiter returns a middleware that rate-limits /graphql
// requests per authenticated user, falling back to the client IP when no
// user is in context (e.g. before authentication). perMinute <= 0 disables
// limiting. The limiter must run after the authenticator so the user is
// already present in the request context.
func GraphQLRateLimiter(perMinute, burst int) echo.MiddlewareFunc {
	if perMinute <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	store := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(float64(perMinute) / 60.0),
		Burst:     burst,
		ExpiresIn: 10 * time.Minute,
	})
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c echo.Context) (string, error) {
			if u, ok := currentuser.FromContext(c.Request().Context()); ok {
				return "user:" + strconv.FormatInt(u.UserID, 10), nil
			}
			return "ip:" + c.RealIP(), nil
		},
	})
}
