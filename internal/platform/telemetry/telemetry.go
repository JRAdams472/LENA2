// Package telemetry wires OpenTelemetry tracing, metrics, and Postgres
// pool instrumentation for the LENA2 service.
package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Telemetry owns the OTel providers for the process lifetime.
type Telemetry struct {
	metricsHandler http.Handler
	tp             *sdktrace.TracerProvider
	mp             *sdkmetric.MeterProvider
}

// Setup configures the global tracer and meter providers. Traces are
// exported to otlpEndpoint when non-empty; when empty the global no-op
// provider remains so instrumentation stays cheap and local development
// needs no collector. Metrics are always exported in Prometheus format via
// MetricsHandler.
func Setup(ctx context.Context, serviceName, otlpEndpoint string, pool *pgxpool.Pool) (*Telemetry, error) {
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	t := &Telemetry{}
	if otlpEndpoint != "" {
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("otel trace exporter: %w", err)
		}
		t.tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(exp),
		)
		otel.SetTracerProvider(t.tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}))
	}

	exporter, err := promexporter.New()
	if err != nil {
		return nil, fmt.Errorf("otel prometheus exporter: %w", err)
	}
	t.mp = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(t.mp)
	t.metricsHandler = promhttp.Handler()

	if pool != nil {
		if err := t.observePool(pool); err != nil {
			return nil, fmt.Errorf("otel pool metrics: %w", err)
		}
	}
	return t, nil
}

// MetricsHandler serves the /metrics endpoint in Prometheus format.
func (t *Telemetry) MetricsHandler() http.Handler { return t.metricsHandler }

// Shutdown flushes and stops the providers.
func (t *Telemetry) Shutdown(ctx context.Context) {
	if t.tp != nil {
		_ = t.tp.Shutdown(ctx)
	}
	if t.mp != nil {
		_ = t.mp.Shutdown(ctx)
	}
}

// observePool publishes pgx pool saturation metrics: connection counts by
// state plus the cumulative acquire counters.
func (t *Telemetry) observePool(pool *pgxpool.Pool) error {
	meter := t.mp.Meter("lena2/postgres")
	connections, err := meter.Int64ObservableGauge("db.pool.connections",
		metric.WithDescription("pgx pool connections by state"),
		metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	acquires, err := meter.Int64ObservableCounter("db.pool.acquires",
		metric.WithDescription("pgx pool acquire attempts by result"))
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		s := pool.Stat()
		o.ObserveInt64(connections, int64(s.TotalConns()), metric.WithAttributes(attribute.String("state", "total")))
		o.ObserveInt64(connections, int64(s.IdleConns()), metric.WithAttributes(attribute.String("state", "idle")))
		o.ObserveInt64(connections, int64(s.AcquiredConns()), metric.WithAttributes(attribute.String("state", "acquired")))
		o.ObserveInt64(connections, int64(s.ConstructingConns()), metric.WithAttributes(attribute.String("state", "constructing")))
		o.ObserveInt64(acquires, s.AcquireCount(), metric.WithAttributes(attribute.String("result", "acquired")))
		o.ObserveInt64(acquires, s.EmptyAcquireCount(), metric.WithAttributes(attribute.String("result", "waited")))
		o.ObserveInt64(acquires, s.CanceledAcquireCount(), metric.WithAttributes(attribute.String("result", "canceled")))
		return nil
	}, connections, acquires)
	return err
}
