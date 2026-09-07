package bff

import (
	"errors"
	"fmt"
	"log/slog"

	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/jackc/pgx/v5"
)

// GraphQL extensions.code values emitted with resolver errors. They match
// the codes the web client already handles in clients/web/lib/api.ts.
const (
	codeUnauthenticated = "UNAUTHENTICATED"
	codeForbidden       = "FORBIDDEN"
	codeBadUserInput    = "BAD_USER_INPUT"
	codeNotFound        = "NOT_FOUND"
	codeInternal        = "INTERNAL"
)

// clientError is a resolver error whose message is safe to return to the
// client verbatim. Any other resolver error is replaced with a generic
// "internal server error" before serialization so pgx/Postgres details
// (constraint names, column names, SQLSTATE) never reach the wire.
type clientError struct {
	msg  string
	code string
}

func (e *clientError) Error() string { return e.msg }

func errUnauthenticated() error { return &clientError{msg: "unauthorized", code: codeUnauthenticated} }
func errForbidden() error {
	return &clientError{msg: "forbidden: admin role required", code: codeForbidden}
}

// badInputf returns a client-safe validation error (BAD_USER_INPUT).
func badInputf(format string, a ...any) error {
	return &clientError{msg: fmt.Sprintf(format, a...), code: codeBadUserInput}
}

// sanitizeQueryErrors rewrites resolver errors in a GraphQL response so
// only client-safe messages are returned. Syntax/validation errors (no
// ResolverError) already carry safe messages and pass through unchanged.
// The original error is logged with the request id for correlation.
func sanitizeQueryErrors(errs []*gqlerrors.QueryError, requestID string) {
	for _, qe := range errs {
		if qe == nil || qe.ResolverError == nil {
			continue
		}
		var ce *clientError
		switch {
		case errors.As(qe.ResolverError, &ce):
			qe.Message = ce.msg
			qe.Extensions = map[string]any{"code": ce.code}
		case errors.Is(qe.ResolverError, pgx.ErrNoRows):
			qe.Message = "not found"
			qe.Extensions = map[string]any{"code": codeNotFound}
		default:
			slog.Default().Error("graphql resolver error",
				"request_id", requestID,
				"error", qe.ResolverError,
			)
			qe.Message = "internal server error"
			qe.Extensions = map[string]any{"code": codeInternal}
		}
	}
}
