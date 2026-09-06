package bff

import (
	"context"

	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/introspection"
	"github.com/graph-gophers/graphql-go/trace/tracer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// graphQLTracer adapts graphql-go's tracer.Tracer to OpenTelemetry so every
// query, validation, and non-trivial field resolution becomes a child span
// of the HTTP request span. Trivial fields (plain scalar accesses) are
// skipped to keep traces focused on resolver work.
type graphQLTracer struct {
	tr oteltrace.Tracer
}

func newGraphQLTracer() *graphQLTracer {
	return &graphQLTracer{tr: otel.Tracer("lena2/graphql")}
}

func (t *graphQLTracer) TraceQuery(ctx context.Context, queryString, operationName string, _ map[string]any, _ map[string]*introspection.Type) (context.Context, tracer.QueryFinishFunc) {
	name := operationName
	if name == "" {
		name = "anonymous"
	}
	ctx, span := t.tr.Start(ctx, "graphql "+name,
		oteltrace.WithAttributes(
			attribute.String("graphql.operation.name", operationName),
			attribute.String("graphql.operation.query", queryString),
		))
	return ctx, func(errs []*gqlerrors.QueryError) {
		finishGraphQLSpan(span, errs)
	}
}

func (t *graphQLTracer) TraceField(ctx context.Context, _, typeName, fieldName string, trivial bool, _ map[string]any) (context.Context, tracer.FieldFinishFunc) {
	if trivial {
		return ctx, func(*gqlerrors.QueryError) {}
	}
	ctx, span := t.tr.Start(ctx, "graphql.field "+typeName+"."+fieldName,
		oteltrace.WithAttributes(
			attribute.String("graphql.type", typeName),
			attribute.String("graphql.field", fieldName),
		))
	return ctx, func(err *gqlerrors.QueryError) {
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
