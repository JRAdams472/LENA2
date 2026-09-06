package telemetry

import (
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics returns an Echo middleware that records request counts and
// durations tagged by method, route, and status. The route template
// (c.Path()) is used rather than the raw URI to keep label cardinality
// bounded.
func HTTPMetrics() echo.MiddlewareFunc {
	meter := otel.Meter("lena2/http")
	requests, reqErr := meter.Int64Counter("http.server.requests",
		metric.WithDescription("HTTP requests by method, route, and status"),
		metric.WithUnit("{request}"))
	duration, durErr := meter.Float64Histogram("http.server.request.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"))
	if reqErr != nil || durErr != nil {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			attrs := metric.WithAttributes(
				attribute.String("method", c.Request().Method),
				attribute.String("route", c.Path()),
				attribute.Int("status", c.Response().Status),
			)
			requests.Add(c.Request().Context(), 1, attrs)
			duration.Record(c.Request().Context(), time.Since(start).Seconds(), attrs)
			return err
		}
	}
}
