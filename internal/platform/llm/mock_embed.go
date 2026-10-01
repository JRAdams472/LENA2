package llm

import (
	"context"
	"hash/fnv"
	"math"
	"regexp"
	"strings"
)

// MockEmbedder is a deterministic bag-of-words Embedder for unit tests and
// e2e: each lowercase word hashes into one dimension of a 768-dim vector,
// then the vector is L2-normalized. Shared vocabulary produces real cosine
// similarity, so semantic search behaves meaningfully without a model.
type MockEmbedder struct{}

// NewMockEmbedder returns the deterministic mock embedder.
func NewMockEmbedder() *MockEmbedder { return &MockEmbedder{} }

// Name identifies the provider implementation.
func (m *MockEmbedder) Name() string { return "mock" }

var embedWordSplit = regexp.MustCompile(`[^a-z0-9]+`)

// Embed embeds document texts.
func (m *MockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = mockVector(t)
	}
	return out, nil
}

// EmbedQuery embeds one search query.
func (m *MockEmbedder) EmbedQuery(_ context.Context, query string) ([]float32, error) {
	return mockVector(query), nil
}

// mockVector hashes each word into a dimension bucket and L2-normalizes.
// The 768 width matches the schema's vector(768) column.
func mockVector(text string) []float32 {
	v := make([]float32, EmbedDims)
	words := embedWordSplit.Split(strings.ToLower(text), -1)
	for _, w := range words {
		if w == "" {
			continue
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(w))
		v[h.Sum32()%EmbedDims]++
	}
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	if norm == 0 {
		return v
	}
	scale := float32(1.0 / math.Sqrt(norm))
	for i := range v {
		v[i] *= scale
	}
	return v
}
