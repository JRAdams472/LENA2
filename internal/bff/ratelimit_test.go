package bff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

func withUser(userID int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(testenv.WithUser(c.Request().Context(), userID, "t@example.com")))
			return next(c)
		}
	}
}

func TestGraphQLRateLimiter_LimitsPerUser(t *testing.T) {
	e := echo.New()
	e.Use(withUser(7))
	e.Use(GraphQLRateLimiter(60, 2))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	codes := make([]int, 3)
	for i := range codes {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil))
		codes[i] = rec.Code
	}
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}, codes)
}

func TestGraphQLRateLimiter_UsersAreIndependent(t *testing.T) {
	e := echo.New()
	var userID int64 = 7
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(testenv.WithUser(c.Request().Context(), userID, "t@example.com")))
			return next(c)
		}
	})
	e.Use(GraphQLRateLimiter(60, 1))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	do := func() int {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil))
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusTooManyRequests, do())
	userID = 8
	assert.Equal(t, http.StatusOK, do())
}

// A user in context must be rate-limited by user id, not by IP: two
// requests from different IPs for the same user share one bucket. This
// guards the intended middleware ordering (authenticator before limiter).
func TestGraphQLRateLimiter_IdentifiesByUserNotIP(t *testing.T) {
	e := echo.New()
	e.Use(withUser(7))
	e.Use(GraphQLRateLimiter(60, 1))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	do := func(ip string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil)
		req.RemoteAddr = ip + ":1234"
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do("10.0.0.1"))
	assert.Equal(t, http.StatusTooManyRequests, do("10.0.0.2"))
}

// The IP limiter runs before the authenticator, so unauthenticated floods
// are throttled by client IP before reaching auth.
func TestIPRateLimiter_LimitsPerIP(t *testing.T) {
	e := echo.New()
	e.Use(IPRateLimiter(60, 2))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	codes := make([]int, 3)
	for i := range codes {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil)
		req.RemoteAddr = "10.0.0.9:1234"
		e.ServeHTTP(rec, req)
		codes[i] = rec.Code
	}
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}, codes)
}

func TestIPRateLimiter_IPsAreIndependent(t *testing.T) {
	e := echo.New()
	e.Use(IPRateLimiter(60, 1))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	do := func(ip string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil)
		req.RemoteAddr = ip + ":1234"
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do("10.0.0.1"))
	assert.Equal(t, http.StatusTooManyRequests, do("10.0.0.1"))
	assert.Equal(t, http.StatusOK, do("10.0.0.2"))
}

func TestIPRateLimiter_UsesXForwardedFor(t *testing.T) {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Use(IPRateLimiter(60, 1))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	do := func(xff string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil)
		// A private (trusted) direct peer, as in the Caddy deployment.
		req.RemoteAddr = "172.18.0.4:1234"
		req.Header.Set(echo.HeaderXForwardedFor, xff)
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do("203.0.113.10"))
	assert.Equal(t, http.StatusTooManyRequests, do("203.0.113.10"))
	assert.Equal(t, http.StatusOK, do("203.0.113.11"))
}

func TestIPRateLimiter_Disabled(t *testing.T) {
	e := echo.New()
	e.Use(IPRateLimiter(0, 0))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestGraphQLRateLimiter_Disabled(t *testing.T) {
	e := echo.New()
	e.Use(GraphQLRateLimiter(0, 0))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}
