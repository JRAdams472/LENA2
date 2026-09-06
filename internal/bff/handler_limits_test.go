package bff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postGraphQL(t *testing.T, h echo.HandlerFunc, query string) *graphql.Response {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":`+strconv.Quote(query)+`}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, rec)
	require.NoError(t, h(c))

	var resp graphql.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return &resp
}

func TestGraphQLHandler_MaxDepth(t *testing.T) {
	h := NewGraphQLHandler(&Resolver{}, graphql.MaxDepth(2))
	// Query depth 3 exceeds the limit and must be rejected at validation.
	resp := postGraphQL(t, h, `{ items { items { id name } } }`)
	require.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0].Message, "depth")
}

func TestGraphQLHandler_MaxDepthAllowed(t *testing.T) {
	h := NewGraphQLHandler(&Resolver{}, graphql.MaxDepth(10))
	// A shallow query is not rejected by the depth rule; it may still fail
	// downstream (no services configured) but must not be a validation
	// depth error.
	resp := postGraphQL(t, h, `{ items { items { id } } }`)
	for _, e := range resp.Errors {
		assert.NotContains(t, e.Message, "depth")
	}
}

func TestGraphQLHandler_MaxQueryLength(t *testing.T) {
	h := NewGraphQLHandler(&Resolver{}, graphql.MaxQueryLength(8))
	resp := postGraphQL(t, h, `{ me { email } }`)
	require.NotEmpty(t, resp.Errors)
}
