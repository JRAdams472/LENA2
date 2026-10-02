package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/idempotency"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func TestIsMutationOperation(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{`mutation { toggleGroceryItemChecked(id: "1") { id } }`, true},
		{`mutation Toggle { toggleGroceryItemChecked(id: "1") { id } }`, true},
		{`  mutation { a } `, true},
		{"# comment\nmutation { a }", true},
		{"mutation{}", true},
		{`{ me { id } }`, false},
		{`query { me { id } }`, false},
		{`query Me { me { id } }`, false},
		{`subscription { updates }`, false},
		{`mutationWasHere { a }`, false},
		{`# only a comment`, false},
		{``, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, isMutationOperation(tc.query), "query: %q", tc.query)
	}
}

// fakeIdemStore records calls and scripts Begin responses.
type fakeIdemStore struct {
	beginFn   func(key string, hash []byte) (*idempotency.Stored, error)
	begins    []string
	completed [][]byte
	abandoned []string
	cfg       idempotency.Config
}

func (f *fakeIdemStore) Begin(_ context.Context, _ int64, key string, hash []byte) (*idempotency.Stored, error) {
	f.begins = append(f.begins, key)
	if f.beginFn != nil {
		return f.beginFn(key, hash)
	}
	return nil, nil
}

func (f *fakeIdemStore) Complete(_ context.Context, _ int64, _ string, _ int32, body []byte) error {
	f.completed = append(f.completed, body)
	return nil
}

func (f *fakeIdemStore) Abandon(_ context.Context, _ int64, key string) error {
	f.abandoned = append(f.abandoned, key)
	return nil
}

func (f *fakeIdemStore) Config() idempotency.Config {
	return f.cfg
}

// postDedup posts body through the handler as user 7 and returns recorder.
func postDedup(t *testing.T, h echo.HandlerFunc, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := testutil.WithUser(context.Background(), 7, "idem@example.com")
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := e.NewContext(req, rec)
	require.NoError(t, h(c))
	return rec
}

func TestIdemHandler_QuerySkipsDedup(t *testing.T) {
	store := &fakeIdemStore{}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	postDedup(t, h, []byte(`{"query":"{ me { id } }"}`), nil)
	assert.Empty(t, store.begins, "queries must not claim idempotency keys")
}

func TestIdemHandler_MutationCompletesClaim(t *testing.T) {
	store := &fakeIdemStore{}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	// An unknown mutation still produces a GraphQL response body — enough
	// to exercise the claim/complete cycle without resolver services.
	rec := postDedup(t, h,
		[]byte(`{"query":"mutation { noSuchMutation }"}`),
		map[string]string{"Idempotency-Key": "key-1"})
	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, store.begins, 1)
	assert.Equal(t, "key-1", store.begins[0])
	require.Len(t, store.completed, 1, "response should be cached for replay")
	assert.JSONEq(t, rec.Body.String(), string(store.completed[0]))
}

func TestIdemHandler_AutoKeyFallback(t *testing.T) {
	store := &fakeIdemStore{}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	postDedup(t, h, []byte(`{"query":"mutation { noSuchMutation }"}`), nil)
	require.Len(t, store.begins, 1)
	assert.Contains(t, store.begins[0], idempotency.AutoKeyPrefix)
}

func TestIdemHandler_ReplaysStoredResponse(t *testing.T) {
	stored := &idempotency.Stored{Status: 200, Body: []byte(`{"data":{"cached":true}}`)}
	store := &fakeIdemStore{beginFn: func(string, []byte) (*idempotency.Stored, error) {
		return stored, nil
	}}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	rec := postDedup(t, h, []byte(`{"query":"mutation { noSuchMutation }"}`), nil)
	assert.JSONEq(t, `{"data":{"cached":true}}`, rec.Body.String())
	assert.Equal(t, "true", rec.Header().Get("Idempotency-Replayed"))
	assert.Empty(t, store.completed, "replays must not execute or re-cache")
}

// assertIdemRejection parses the body as a strict GraphQL error response —
// rejections must be the only thing written (a rejected request must not
// also execute and append a second body).
func assertIdemRejection(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body struct {
		Errors []struct {
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body),
		"rejection must be a clean JSON body, got %q", rec.Body.String())
	require.NotEmpty(t, body.Errors)
	assert.Equal(t, code, body.Errors[0].Extensions["code"])
}

func TestIdemHandler_KeyReusedConflict(t *testing.T) {
	store := &fakeIdemStore{beginFn: func(string, []byte) (*idempotency.Stored, error) {
		return nil, idempotency.ErrKeyReused
	}}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	rec := postDedup(t, h, []byte(`{"query":"mutation { noSuchMutation }"}`), nil)
	assertIdemRejection(t, rec, codeIdemKeyReused)
	assert.Empty(t, store.completed)
}

func TestIdemHandler_InFlight(t *testing.T) {
	store := &fakeIdemStore{beginFn: func(string, []byte) (*idempotency.Stored, error) {
		return nil, idempotency.ErrInFlight
	}}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	rec := postDedup(t, h, []byte(`{"query":"mutation { noSuchMutation }"}`), nil)
	assertIdemRejection(t, rec, codeIdemInFlight)
	assert.Empty(t, store.completed)
}

func TestIdemHandler_KeyTooLong(t *testing.T) {
	store := &fakeIdemStore{cfg: idempotency.Config{MaxKeyLen: 8}}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	rec := postDedup(t, h, []byte(`{"query":"mutation { noSuchMutation }"}`),
		map[string]string{"Idempotency-Key": "way-too-long-key"})
	assertIdemRejection(t, rec, codeIdemKeyInvalid)
	assert.Empty(t, store.begins, "invalid keys must be rejected before claim")
}

func TestIdemHandler_StoreErrorFailsOpen(t *testing.T) {
	store := &fakeIdemStore{beginFn: func(string, []byte) (*idempotency.Stored, error) {
		return nil, errors.New("db down")
	}}
	h, err := NewGraphQLHandler(&Resolver{IdemStore: store}, 5*time.Second, 0, 0)
	require.NoError(t, err)
	rec := postDedup(t, h, []byte(`{"query":"mutation { noSuchMutation }"}`), nil)
	// Dedup failure must not break the API — the mutation still executes.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "noSuchMutation")
	assert.Empty(t, store.completed)
}

func TestIdemClaim_OversizedResponseAbandons(t *testing.T) {
	store := &fakeIdemStore{cfg: idempotency.Config{MaxResponseSz: 4}}
	cl := &idemClaim{store: store, userID: 7, key: "k"}
	cl.complete([]byte(`{"data":{"too":"big"}}`))
	assert.Empty(t, store.completed)
	assert.Equal(t, []string{"k"}, store.abandoned)
	assert.True(t, cl.done)
}

func TestIdemClaim_AbandonIfPending(t *testing.T) {
	store := &fakeIdemStore{}
	cl := &idemClaim{store: store, userID: 7, key: "k"}
	cl.abandonIfPending()
	assert.Equal(t, []string{"k"}, store.abandoned)
	// A second unwind must not abandon again.
	cl.abandonIfPending()
	assert.Len(t, store.abandoned, 1)
}
