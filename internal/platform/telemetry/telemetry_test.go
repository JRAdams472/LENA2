package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupAndHTTPMetrics(t *testing.T) {
	tel, err := Setup(context.Background(), "lena2-test", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { tel.Shutdown(context.Background()) })

	e := echo.New()
	e.Use(HTTPMetrics())
	e.GET("/ping", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = httptest.NewRecorder()
	tel.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "http_server_requests_total")
	assert.Contains(t, rec.Body.String(), `route="/ping"`)
}
