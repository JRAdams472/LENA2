package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupAndHTTPMetrics(t *testing.T) {
	tel, err := Setup(context.Background(), "lena2-test", "", true, nil)
	require.NoError(t, err)
	t.Cleanup(func() { tel.Shutdown(context.Background()) })

	e := echo.New()
	mw, err := HTTPMetrics()
	require.NoError(t, err)
	e.Use(mw)
	e.GET("/ping", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil))
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = httptest.NewRecorder()
	tel.MetricsHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "http_server_requests_total")
	assert.Contains(t, rec.Body.String(), `route="/ping"`)
	// The duration histogram must actually emit buckets, not just exist.
	assert.Contains(t, rec.Body.String(), "http_server_request_duration_seconds_bucket")
}

func TestSetupWithOTLPEndpoint(t *testing.T) {
	// The gRPC trace exporter dials lazily, so an unreachable endpoint is
	// fine here — this exercises the exporter/tracer-provider branch.
	tel, err := Setup(context.Background(), "lena2-test", "localhost:1", true, nil)
	require.NoError(t, err)
	tel.Shutdown(context.Background())
}

func TestSetupWithPool(t *testing.T) {
	// pgx pools connect lazily, so a pool pointing at nothing still works for
	// registering the observable-gauge callback.
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@localhost:1/db")
	require.NoError(t, err)
	defer pool.Close()

	tel, err := Setup(context.Background(), "lena2-test-pool", "", true, pool)
	require.NoError(t, err)
	t.Cleanup(func() { tel.Shutdown(context.Background()) })

	rec := httptest.NewRecorder()
	tel.MetricsHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "db_pool_connections")
}
