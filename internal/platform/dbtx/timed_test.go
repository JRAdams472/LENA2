package dbtx

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type fakeExecer struct {
	err     error
	rows    *fakeRows
	queries []string
}

func (f *fakeExecer) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.queries = append(f.queries, sql)
	return pgconn.CommandTag{}, f.err
}

func (f *fakeExecer) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	f.queries = append(f.queries, sql)
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func (f *fakeExecer) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	f.queries = append(f.queries, sql)
	return &fakeRow{err: f.err}
}

type fakeRows struct{ closed bool }

func (r *fakeRows) Close()                                       { r.closed = true }
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Next() bool                                   { return false }
func (r *fakeRows) Scan(_ ...any) error                          { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

type fakeRow struct{ err error }

func (r *fakeRow) Scan(_ ...any) error { return r.err }

func setupMeter(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(prev) })
	return reader
}

func histogramPoints(t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			h, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "metric %s has unexpected type %T", name, m.Data)
			return h.DataPoints
		}
	}
	t.Fatalf("metric %s not exported", name)
	return nil
}

func TestTimedExecer_RecordsQueryRowDuration(t *testing.T) {
	reader := setupMeter(t)
	execer := NewTimedExecer(&fakeExecer{}, "grocery")

	err := execer.QueryRow(context.Background(),
		"-- name: GetGroceryList :one\nSELECT id FROM lists").Scan()
	require.NoError(t, err)

	points := histogramPoints(t, reader, "domain_query_duration_ms")
	require.Len(t, points, 1)
	attrs := points[0].Attributes
	svc, _ := attrs.Value("service")
	assert.Equal(t, "grocery", svc.AsString())
	q, _ := attrs.Value("query")
	assert.Equal(t, "GetGroceryList", q.AsString())
	assert.Equal(t, 2, attrs.Len())
	assert.GreaterOrEqual(t, points[0].Count, uint64(1))
}

func TestTimedExecer_RecordsQueryOnClose(t *testing.T) {
	reader := setupMeter(t)
	rows := &fakeRows{}
	execer := NewTimedExecer(&fakeExecer{rows: rows}, "inventory")

	got, err := execer.Query(context.Background(), "-- name: ListItems :many\nSELECT id FROM items")
	require.NoError(t, err)

	// The sample is recorded only after the rows are closed.
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	got.Close()
	assert.True(t, rows.closed)

	points := histogramPoints(t, reader, "domain_query_duration_ms")
	require.Len(t, points, 1)
	q, _ := points[0].Attributes.Value("query")
	assert.Equal(t, "ListItems", q.AsString())
}

func TestTimedExecer_PropagatesErrors(t *testing.T) {
	reader := setupMeter(t)
	boom := errors.New("boom")
	execer := NewTimedExecer(&fakeExecer{err: boom}, "recipe")

	_, execErr := execer.Exec(context.Background(), "-- name: DeleteRecipe :exec\nDELETE FROM recipes")
	assert.ErrorIs(t, execErr, boom)

	qRows, queryErr := execer.Query(context.Background(), "-- name: ListRecipes :many\nSELECT 1")
	assert.ErrorIs(t, queryErr, boom)
	if qRows != nil {
		qRows.Close()
	}

	scanErr := execer.QueryRow(context.Background(), "SELECT 1").Scan()
	assert.ErrorIs(t, scanErr, boom)

	// Failures still record samples; the unnamed statement lands on
	// the bounded "unknown" label rather than the raw SQL text.
	points := histogramPoints(t, reader, "domain_query_duration_ms")
	queries := map[string]bool{}
	for _, p := range points {
		q, _ := p.Attributes.Value("query")
		queries[q.AsString()] = true
	}
	assert.Equal(t, map[string]bool{"DeleteRecipe": true, "ListRecipes": true, "unknown": true}, queries)
}
