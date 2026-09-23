package bff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postGraphQL(t *testing.T, h echo.HandlerFunc, query string) *graphql.Response {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", strings.NewReader(`{"query":`+strconv.Quote(query)+`}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, rec)
	require.NoError(t, h(c))

	var resp graphql.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return &resp
}

func TestGraphQLHandler_MaxDepth(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, graphql.MaxDepth(2))
	require.NoError(t, err)
	// Query depth 3 exceeds the limit and must be rejected at validation.
	resp := postGraphQL(t, h, `{ items { items { id name } } }`)
	require.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0].Message, "depth")
}

func TestGraphQLHandler_MaxDepthAllowed(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, graphql.MaxDepth(10))
	require.NoError(t, err)
	// A shallow query is not rejected by the depth rule; it may still fail
	// downstream (no services configured) but must not be a validation
	// depth error.
	resp := postGraphQL(t, h, `{ items { items { id } } }`)
	for _, e := range resp.Errors {
		assert.NotContains(t, e.Message, "depth")
	}
}

func TestGraphQLHandler_MaxQueryLength(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, graphql.MaxQueryLength(8))
	require.NoError(t, err)
	resp := postGraphQL(t, h, `{ me { email } }`)
	require.NotEmpty(t, resp.Errors)
}

func TestGraphQLHandler_CostBudgetExceeded(t *testing.T) {
	// Budget of 1: the second top-level field resolution trips the limiter
	// and cancels the execution context.
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 1)
	require.NoError(t, err)
	resp := postGraphQL(t, h, `{ me { id } units { id } }`)
	require.NotEmpty(t, resp.Errors)
	var found bool
	for _, e := range resp.Errors {
		if e.Message == "query exceeds cost budget" {
			found = true
			assert.Equal(t, codeCostExceeded, e.Extensions["code"])
		}
	}
	assert.True(t, found, "expected a cost-budget error, got %v", resp.Errors)
}

func TestGraphQLHandler_EnumRejectsInvalidValue(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0)
	require.NoError(t, err)
	// "superadmin" is not a declared Role value; validation must reject it
	// before any resolver runs.
	resp := postGraphQL(t, h, `mutation { setUserRole(userId: "2", role: superadmin) { id } }`)
	require.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0].Message, "superadmin")
}

func TestApplyLimitErrors_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), time.Millisecond, errQueryTimeout)
	defer cancel()
	<-ctx.Done() // deterministic expiry
	resp := &graphql.Response{}
	applyLimitErrors(ctx, resp, &costLimiter{})
	require.Len(t, resp.Errors, 1)
	assert.Equal(t, codeTimeout, resp.Errors[0].Extensions["code"])
}
