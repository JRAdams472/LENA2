package bff

import (
	"context"
	"errors"
	"testing"

	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestGraphQLTracer_ProducesSpans(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	tr := newGraphQLTracer()
	ctx, queryDone := tr.TraceQuery(context.Background(), "{ items { items { name } } }", "ListItems", nil, nil)
	fctx, fieldDone := tr.TraceField(ctx, "", "Query", "items", false, nil)
	// Field spans must be children of the query span.
	assert.True(t, oteltrace.SpanContextFromContext(fctx).IsValid())

	fieldDone(nil)
	queryDone(nil)

	spans := rec.Ended()
	require.Len(t, spans, 2)
	assert.Equal(t, "graphql.field Query.items", spans[0].Name())
	assert.Equal(t, "graphql ListItems", spans[1].Name())
	assert.Equal(t, spans[1].SpanContext().SpanID(), spans[0].Parent().SpanID())
	assert.Equal(t, codes.Unset, spans[0].Status().Code)
	assert.Equal(t, codes.Unset, spans[1].Status().Code)
}

func TestGraphQLTracer_RecordsErrors(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	tr := newGraphQLTracer()
	ctx, fieldDone := tr.TraceField(context.Background(), "", "Query", "items", false, nil)
	fieldDone(&gqlerrors.QueryError{ResolverError: errors.New("boom")})

	_, queryDone := tr.TraceQuery(context.Background(), "x", "", nil, nil)
	queryDone([]*gqlerrors.QueryError{{ResolverError: errors.New("fail")}})

	_ = ctx
	spans := rec.Ended()
	require.Len(t, spans, 2)
	assert.Equal(t, codes.Error, spans[0].Status().Code)
	assert.Equal(t, codes.Error, spans[1].Status().Code)
}

func TestGraphQLTracer_TrivialFieldSkipsSpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	tr := newGraphQLTracer()
	ctx, done := tr.TraceField(context.Background(), "", "Item", "name", true, nil)
	done(nil)
	assert.Empty(t, rec.Ended())
	assert.False(t, oteltrace.SpanContextFromContext(ctx).IsValid())
}

// collectHistogram gathers one data point for the named histogram from the
// given reader, failing the test if it is absent.
func collectHistogram(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.Histogram[float64] {
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
			return h
		}
	}
	t.Fatalf("metric %s not exported", name)
	return metricdata.Histogram[float64]{}
}

func TestGraphQLTracer_RecordsResolverDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	defer otel.SetMeterProvider(prev)

	tr := newGraphQLTracer()
	// Argument values must never appear in labels — only the schema-derived
	// operation type and field name (LENA-022 / LENA-070 cardinality guard).
	ctx, done := tr.TraceField(context.Background(), "", "Mutation", "rateRecipe", false,
		map[string]any{"input": map[string]any{"recipeId": "secret-arg-value", "rating": 5}})
	done(nil)
	_ = ctx

	h := collectHistogram(t, reader, "graphql_resolver_duration_ms")
	require.Len(t, h.DataPoints, 1)
	attrs := h.DataPoints[0].Attributes
	op, ok := attrs.Value("operation_type")
	require.True(t, ok)
	assert.Equal(t, "mutation", op.AsString())
	field, ok := attrs.Value("field")
	require.True(t, ok)
	assert.Equal(t, "rateRecipe", field.AsString())
	assert.Equal(t, 2, attrs.Len(), "labels must contain only operation_type and field")
	for _, kv := range attrs.ToSlice() {
		assert.NotEqual(t, "secret-arg-value", kv.Value.AsString())
	}
}

func TestGraphQLTracer_NestedFieldInheritsOperationType(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	defer otel.SetMeterProvider(prev)

	tr := newGraphQLTracer()
	_, rootDone := tr.TraceField(context.Background(), "", "Query", "recipe", false, nil)
	rootDone(nil)
	// Rebuild context the way graphql-go does: the ctx returned for the root
	// field flows into nested field resolution.
	ctx, rootDone2 := tr.TraceField(context.Background(), "", "Query", "recipe", false, nil)
	_, nestedDone := tr.TraceField(ctx, "", "Recipe", "ingredients", false, nil)
	nestedDone(nil)
	rootDone2(nil)

	h := collectHistogram(t, reader, "graphql_resolver_duration_ms")
	var nested metricdata.HistogramDataPoint[float64]
	found := false
	for _, dp := range h.DataPoints {
		f, _ := dp.Attributes.Value("field")
		if f.AsString() == "ingredients" {
			nested, found = dp, true
		}
	}
	require.True(t, found, "nested field sample missing")
	op, _ := nested.Attributes.Value("operation_type")
	assert.Equal(t, "query", op.AsString())
}
