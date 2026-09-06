package bff

import (
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
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/graphql", nil))
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
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/graphql", nil))
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusTooManyRequests, do())
	userID = 8
	assert.Equal(t, http.StatusOK, do())
}

func TestGraphQLRateLimiter_Disabled(t *testing.T) {
	e := echo.New()
	e.Use(GraphQLRateLimiter(0, 0))
	e.POST("/graphql", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/graphql", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}
