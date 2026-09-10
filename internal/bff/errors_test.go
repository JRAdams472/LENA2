package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

func TestSanitizeQueryErrors(t *testing.T) {
	t.Run("syntax errors pass through unchanged", func(t *testing.T) {
		qe := &gqlerrors.QueryError{Message: "graphql syntax error: unexpected"}
		sanitizeQueryErrors([]*gqlerrors.QueryError{qe}, "req-1")
		assert.Equal(t, "graphql syntax error: unexpected", qe.Message)
		assert.Nil(t, qe.Extensions)
	})

	t.Run("client errors keep message and gain code", func(t *testing.T) {
		qe := &gqlerrors.QueryError{Message: "unauthorized", ResolverError: errUnauthenticated()}
		sanitizeQueryErrors([]*gqlerrors.QueryError{qe}, "req-1")
		assert.Equal(t, "unauthorized", qe.Message)
		assert.Equal(t, codeUnauthenticated, qe.Extensions["code"])
	})

	t.Run("not found maps to NOT_FOUND", func(t *testing.T) {
		qe := &gqlerrors.QueryError{
			Message:       "get recipe: no rows in result set",
			ResolverError: fmt.Errorf("get recipe: %w", pgx.ErrNoRows),
		}
		sanitizeQueryErrors([]*gqlerrors.QueryError{qe}, "req-1")
		assert.Equal(t, "not found", qe.Message)
		assert.Equal(t, codeNotFound, qe.Extensions["code"])
	})

	t.Run("internal errors are masked", func(t *testing.T) {
		qe := &gqlerrors.QueryError{
			Message:       `get items: ERROR: duplicate key value violates unique constraint "items_pkey" (SQLSTATE 23505)`,
			ResolverError: errors.New(`get items: ERROR: duplicate key value violates unique constraint "items_pkey" (SQLSTATE 23505)`),
		}
		sanitizeQueryErrors([]*gqlerrors.QueryError{qe}, "req-1")
		assert.Equal(t, "internal server error", qe.Message)
		assert.Equal(t, codeInternal, qe.Extensions["code"])
	})
}

// A service failure must surface as a generic message + INTERNAL code,
// not as the raw pgx error text.
func TestGraphQLHandler_MasksInternalErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockInventoryService(ctrl)
	inv.EXPECT().ListBrandsVisible(gomock.Any(), int64(1)).
		Return(nil, errors.New(`list brands visible: ERROR: relation "inventory.brand" does not exist (SQLSTATE 42P01)`))

	r := &Resolver{InventoryService: inv}
	h, err := NewGraphQLHandler(r)
	require.NoError(t, err)

	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql",
		strings.NewReader(`{"query":"{ brands { id name } }"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req.WithContext(testenv.WithUser(req.Context(), 1, "u@example.com")), rec)
	require.NoError(t, h(c))

	var resp graphql.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Errors)
	assert.Equal(t, "internal server error", resp.Errors[0].Message)
	assert.NotContains(t, rec.Body.String(), "SQLSTATE")
	assert.Equal(t, "INTERNAL", resp.Errors[0].Extensions["code"])
}

// Client-safe errors (auth, bad input) keep their message and carry a code.
func TestGraphQLHandler_PassesClientErrors(t *testing.T) {
	r := &Resolver{}
	h, err := NewGraphQLHandler(r)
	require.NoError(t, err)

	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql",
		strings.NewReader(`{"query":"{ me { id } }"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req.WithContext(context.Background()), rec)
	require.NoError(t, h(c))

	var resp graphql.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Errors)
	assert.Equal(t, "unauthorized", resp.Errors[0].Message)
	assert.Equal(t, "UNAUTHENTICATED", resp.Errors[0].Extensions["code"])
}
