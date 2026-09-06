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
