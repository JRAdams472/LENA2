package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorLiteralRoundTrip(t *testing.T) {
	v := []float32{0.5, -1.25, 3.1415927, 0}
	lit := VectorLiteral(v)
	assert.Equal(t, "[0.5,-1.25,3.1415927,0]", lit)

	back, err := ParseVectorLiteral(lit)
	require.NoError(t, err)
	assert.Equal(t, v, back)
}

func TestParseVectorLiteralErrors(t *testing.T) {
	for _, s := range []string{"", "abc", "[1,2", "1,2]", "[a,b]", "[]"} {
		_, err := ParseVectorLiteral(s)
		if s == "[]" {
			assert.NoError(t, err)
			continue
		}
		assert.Error(t, err, s)
	}
}

func TestMockEmbedderDeterministic(t *testing.T) {
	e := NewMockEmbedder()
	ctx := context.Background()

	a, err := e.EmbedQuery(ctx, "cozy chicken soup")
	require.NoError(t, err)
	b, err := e.EmbedQuery(ctx, "cozy chicken soup")
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.Len(t, a, EmbedDims)

	// Unit-normalized.
	var norm float64
	for _, x := range a {
		norm += float64(x) * float64(x)
	}
	assert.InDelta(t, 1.0, norm, 1e-5)
}

func TestMockEmbedderSimilarity(t *testing.T) {
	e := NewMockEmbedder()
	ctx := context.Background()

	cos := func(a, b []float32) float64 {
		var dot float64
		for i := range a {
			dot += float64(a[i]) * float64(b[i])
		}
		return dot // vectors are L2-normalized, so dot == cosine
	}

	soupQ, _ := e.EmbedQuery(ctx, "warm soup")
	soupDoc, _ := e.Embed(ctx, []string{"Chicken Soup\nwarm broth noodles"})
	cakeDoc, _ := e.Embed(ctx, []string{"Chocolate Cake\ncocoa frosting dessert"})
	assert.Greater(t, cos(soupQ, soupDoc[0]), cos(soupQ, cakeDoc[0]))

	empty, err := e.EmbedQuery(ctx, "")
	require.NoError(t, err)
	var norm float64
	for _, x := range empty {
		norm += float64(x) * float64(x)
	}
	assert.Equal(t, 0.0, norm) // empty input → zero vector
}

func TestOllamaEmbedderPrefixesAndDecode(t *testing.T) {
	var gotInputs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "nomic-embed-text", req.Model)
		gotInputs = req.Input

		embs := make([][]float64, len(req.Input))
		for i := range embs {
			embs[i] = []float64{0.1, -0.2, 0.3}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": req.Model, "embeddings": embs})
	}))
	defer server.Close()

	e := NewOllamaEmbedder(server.URL, "nomic-embed-text", 5*time.Second, "", "")
	docs, err := e.Embed(context.Background(), []string{"Chicken Soup"})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, []float32{0.1, -0.2, 0.3}, docs[0])
	assert.Equal(t, "search_document: Chicken Soup", gotInputs[0])

	q, err := e.EmbedQuery(context.Background(), "cozy dinner")
	require.NoError(t, err)
	assert.Equal(t, "search_query: cozy dinner", gotInputs[0])
	assert.Len(t, q, 3)
}

func TestOllamaEmbedderNoPrefix(t *testing.T) {
	var gotInputs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.Unmarshal(body, &req)
		gotInputs = req.Input
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "m", "embeddings": [][]float64{{0.1}}})
	}))
	defer server.Close()

	// "-" opts out of the nomic prefix convention.
	e := NewOllamaEmbedder(server.URL, "m", 5*time.Second, "-", "-")
	_, err := e.EmbedQuery(context.Background(), "plain")
	require.NoError(t, err)
	assert.Equal(t, "plain", gotInputs[0])
}

func TestOllamaEmbedderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()

	e := NewOllamaEmbedder(server.URL, "missing", 5*time.Second, "", "")
	_, err := e.EmbedQuery(context.Background(), "x")
	assert.ErrorContains(t, err, "status 404")
}
