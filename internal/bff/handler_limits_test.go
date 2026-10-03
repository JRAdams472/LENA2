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

	"github.com/JRAdams472/LENA2/internal/ai"
	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/testutil"
	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func postGraphQL(t *testing.T, h echo.HandlerFunc, query string) *graphql.Response {
	t.Helper()
	return postGraphQLCtx(t, context.Background(), h, query)
}

func postGraphQLCtx(t *testing.T, ctx context.Context, h echo.HandlerFunc, query string) *graphql.Response {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/graphql", strings.NewReader(`{"query":`+strconv.Quote(query)+`}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, rec)
	require.NoError(t, h(c))

	var resp graphql.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return &resp
}

func TestGraphQLHandler_MaxDepth(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 0, graphql.MaxDepth(2))
	require.NoError(t, err)
	// Query depth 3 exceeds the limit and must be rejected at validation.
	resp := postGraphQL(t, h, `{ items { items { id name } } }`)
	require.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0].Message, "depth")
}

func TestGraphQLHandler_MaxDepthAllowed(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 0, graphql.MaxDepth(10))
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
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 0, graphql.MaxQueryLength(8))
	require.NoError(t, err)
	resp := postGraphQL(t, h, `{ me { email } }`)
	require.NotEmpty(t, resp.Errors)
}

func TestGraphQLHandler_CostBudgetExceeded(t *testing.T) {
	// Budget of 1: the second top-level field resolution trips the limiter
	// and cancels the execution context.
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 1)
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

func TestGraphQLHandler_IntrospectionAdminOnly(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 0,
		graphql.RestrictIntrospection(AllowIntrospectionForAdmins(false)))
	require.NoError(t, err)
	const introspect = `{ __schema { mutationType { fields { name } } } }`

	// graphql-go drops __schema/__type selections when introspection is
	// disallowed rather than erroring — members get empty data.
	member := postGraphQLCtx(t, testutil.WithUser(context.Background(), 7, "m@example.com"), h, introspect)
	assert.NotContains(t, string(member.Data), "mutationType")

	admin := postGraphQLCtx(t, testutil.WithAdmin(context.Background(), 1, "a@example.com"), h, introspect)
	require.Empty(t, admin.Errors)
	assert.Contains(t, string(admin.Data), "mutationType")
}

func TestGraphQLHandler_IntrospectionDisableAll(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 0,
		graphql.RestrictIntrospection(AllowIntrospectionForAdmins(true)))
	require.NoError(t, err)

	resp := postGraphQLCtx(t, testutil.WithAdmin(context.Background(), 1, "a@example.com"), h,
		`{ __schema { queryType { name } } }`)
	assert.NotContains(t, string(resp.Data), "queryType")
}

func TestGraphQLHandler_EnumRejectsInvalidValue(t *testing.T) {
	h, err := NewGraphQLHandler(&Resolver{}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	// "superadmin" is not a declared Role value; validation must reject it
	// before any resolver runs.
	resp := postGraphQL(t, h, `mutation { setUserRole(userId: "2", role: superadmin) { id } }`)
	require.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0].Message, "superadmin")
}

func TestGraphQLHandler_AIQueryUsesExtendedTimeout(t *testing.T) {
	// The assistant's provider call outlives the interactive budget; the AI
	// timeout must cover it while normal queries keep the short leash.
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Available().Return(true).AnyTimes()
	svc.EXPECT().Ask(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _, _ int64, _ string) (ai.Answer, error) {
			select {
			case <-time.After(300 * time.Millisecond):
				return ai.Answer{Text: "done"}, nil
			case <-ctx.Done():
				return ai.Answer{}, ctx.Err()
			}
		}).AnyTimes()

	h, err := NewGraphQLHandler(&Resolver{AIService: svc}, 50*time.Millisecond, 2*time.Second, 0)
	require.NoError(t, err)

	ctx := testutil.WithUser(context.Background(), 7, "ai@example.com")
	resp := postGraphQLCtx(t, ctx, h, `{ askAssistant(question: "what's for dinner?") { answer } }`)
	assert.Empty(t, resp.Errors, "AI query should finish within the AI budget")
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.JSONEq(t, `{"answer":"done"}`, string(data["askAssistant"]))

	// Same query without the AI timeout configured times out at 50ms.
	h2, err := NewGraphQLHandler(&Resolver{AIService: svc}, 50*time.Millisecond, 0, 0)
	require.NoError(t, err)
	resp2 := postGraphQLCtx(t, testutil.WithUser(context.Background(), 8, "ai2@example.com"), h2, `{ askAssistant(question: "hi") { answer } }`)
	var foundTimeout bool
	for _, e := range resp2.Errors {
		if e.Extensions["code"] == codeTimeout {
			foundTimeout = true
		}
	}
	assert.True(t, foundTimeout, "without GRAPHQL_AI_TIMEOUT the same query should hit the normal deadline")
}

func TestUsesAIProvider(t *testing.T) {
	for q, want := range map[string]bool{
		`{ askAssistant(question: "hi") { answer } }`:                          true,
		`query { suggestMeals(mealPlanId: "1") { recipe { id } } }`:            true,
		`{ suggestPairings(recipeId: "3") { bottle { id } } }`:                 true,
		`{ suggestCocktails { recipe { id } } }`:                               true,
		`{ suggestEventFixes(foodEventId: "2") { action } }`:                   true,
		`{ me { id email } }`:                                                  false,
		`{ aiAvailable assistantTools { name } }`:                              false,
		`{ prepareAssistantRequest(name: "suggest-meals") { prompt } }`:        false,
		`{ callAssistantTool(name: "get_pantry_inventory", arguments: "{}") }`: false,
	} {
		assert.Equal(t, want, usesAIProvider(q), "query %q", q)
	}
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
