package bff

import (
	"context"
	"fmt"
	"strings"
	"time"

	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/introspection"
	"github.com/graph-gophers/graphql-go/trace/tracer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// opTypeContextKey carries the GraphQL operation type (query, mutation,
// subscription) from a root field down to nested resolver fields so their
// duration samples are attributed to the enclosing operation.
type opTypeContextKey struct{}

// graphQLTracer adapts graphql-go's tracer.Tracer to OpenTelemetry so every
// query, validation, and non-trivial field resolution becomes a child span
// of the HTTP request span. Trivial fields (plain scalar accesses) are
// skipped to keep traces focused on resolver work. Non-trivial field
// resolutions are also timed into the graphql_resolver_duration_ms
// histogram labeled by {operation_type, field}; labels carry schema-derived
// names only — never argument values or raw query text.
type graphQLTracer struct {
	tr               oteltrace.Tracer
	resolverDuration metric.Float64Histogram
}

func newGraphQLTracer() *graphQLTracer {
	t := &graphQLTracer{tr: otel.Tracer("lena2/graphql")}
	duration, err := otel.Meter("lena2/graphql").Float64Histogram(
		"graphql_resolver_duration_ms",
		metric.WithDescription("GraphQL field resolver duration by operation type and field"),
		metric.WithUnit("ms"))
	if err != nil {
		otel.Handle(fmt.Errorf("graphql resolver duration histogram: %w", err))
	} else {
		t.resolverDuration = duration
	}
	return t
}

func (t *graphQLTracer) TraceQuery(ctx context.Context, _, operationName string, _ map[string]any, _ map[string]*introspection.Type) (context.Context, tracer.QueryFinishFunc) {
	name := operationName
	if name == "" {
		name = "anonymous"
	}
	ctx, span := t.tr.Start(ctx, "graphql "+name,
		oteltrace.WithAttributes(
			attribute.String("graphql.operation.name", operationName),
		))
	return ctx, func(errs []*gqlerrors.QueryError) {
		finishGraphQLSpan(span, errs)
	}
}

func (t *graphQLTracer) TraceField(ctx context.Context, _, typeName, fieldName string, trivial bool, _ map[string]any) (context.Context, tracer.FieldFinishFunc) {
	if trivial {
		return ctx, func(*gqlerrors.QueryError) {}
	}
	opType, _ := ctx.Value(opTypeContextKey{}).(string)
	switch typeName {
	case "Query", "Mutation", "Subscription":
		opType = strings.ToLower(typeName)
		ctx = context.WithValue(ctx, opTypeContextKey{}, opType)
	}
	start := time.Now()
	ctx, span := t.tr.Start(ctx, "graphql.field "+typeName+"."+fieldName,
		oteltrace.WithAttributes(
			attribute.String("graphql.type", typeName),
			attribute.String("graphql.field", fieldName),
		))
	return ctx, func(err *gqlerrors.QueryError) {
		if t.resolverDuration != nil {
			if opType == "" {
				opType = "unknown"
			}
			t.resolverDuration.Record(ctx, float64(time.Since(start).Microseconds())/1000,
				metric.WithAttributes(
					attribute.String("operation_type", opType),
					attribute.String("field", fieldName)))
		}
		if err != nil {
			finishGraphQLSpan(span, []*gqlerrors.QueryError{err})
			return
		}
		span.End()
	}
}

func (t *graphQLTracer) TraceValidation(ctx context.Context) tracer.ValidationFinishFunc {
	_, span := t.tr.Start(ctx, "graphql.validation")
	return func(errs []*gqlerrors.QueryError) {
		finishGraphQLSpan(span, errs)
	}
}

func finishGraphQLSpan(span oteltrace.Span, errs []*gqlerrors.QueryError) {
	if len(errs) > 0 {
		span.SetStatus(codes.Error, errs[0].Error())
		span.RecordError(errs[0])
	}
	span.End()
}
