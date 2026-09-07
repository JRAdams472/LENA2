package dbtx

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// queryNameRe extracts the sqlc query name from the leading
// `-- name: <Query> :<mode>` comment present in every generated statement.
var queryNameRe = regexp.MustCompile(`-- name:\s*([A-Za-z0-9_]+)`)

// NewTimedExecer wraps an Execer so every call records its wall-clock
// duration into the domain_query_duration_ms histogram labeled by
// {service, query}. The query label is the sqlc query name parsed from the
// statement's `-- name:` comment; statements without one are recorded as
// "unknown". If the histogram cannot be created the error is surfaced via
// the global OTel error handler and the unwrapped Execer is returned so
// query execution is never degraded by a metrics failure.
func NewTimedExecer(inner Execer, service string) Execer {
	duration, err := otel.Meter("lena2/db").Float64Histogram(
		"domain_query_duration_ms",
		metric.WithDescription("SQL query duration by domain service and sqlc query name"),
		metric.WithUnit("ms"))
	if err != nil {
		otel.Handle(fmt.Errorf("db duration histogram: %w", err))
		return inner
	}
	return &timedExecer{inner: inner, service: service, duration: duration}
}

type timedExecer struct {
	inner    Execer
	service  string
	duration metric.Float64Histogram
}

func (t *timedExecer) recorder(ctx context.Context, sql string, start time.Time) func() {
	return sync.OnceFunc(func() {
		query := "unknown"
		if m := queryNameRe.FindStringSubmatch(sql); m != nil {
			query = m[1]
		}
		t.duration.Record(ctx, float64(time.Since(start).Microseconds())/1000,
			metric.WithAttributes(
				attribute.String("service", t.service),
				attribute.String("query", query)))
	})
}

func (t *timedExecer) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	record := t.recorder(ctx, sql, time.Now())
	defer record()
	return t.inner.Exec(ctx, sql, args...)
}

func (t *timedExecer) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	record := t.recorder(ctx, sql, time.Now())
	rows, err := t.inner.Query(ctx, sql, args...)
	if err != nil {
		record()
		return nil, err
	}
	// Duration covers row streaming: the histogram sample is recorded when
	// the caller closes the rows (sqlc always defers Close).
	return &timedRows{Rows: rows, record: record}, nil
}

func (t *timedExecer) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &timedRow{Row: t.inner.QueryRow(ctx, sql, args...), record: t.recorder(ctx, sql, time.Now())}
}

type timedRows struct {
	pgx.Rows
	record func()
}

func (r *timedRows) Close() {
	r.Rows.Close()
	r.record()
}

type timedRow struct {
	pgx.Row
	record func()
}

func (r *timedRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.record()
	return err
}
