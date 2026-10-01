package llm

import (
	"context"

	"github.com/JRAdams472/LENA2/internal/platform/vector"
)

// EmbedDims is the vector dimensionality the recipe schema stores. It is
// tied to the default embedding model (nomic-embed-text, 768 dims); a
// different model must produce the same width or writes fail and the row
// stays stale for the backfill sweep.
const EmbedDims = 768

// Embedder produces text embedding vectors for semantic recipe search.
// Documents and queries go through separate entry points so providers with
// asymmetric models (e.g. nomic's search_document:/search_query: prefixes)
// can encode each side correctly.
type Embedder interface {
	// Embed embeds document texts (stored recipe text).
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// EmbedQuery embeds a single search query.
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	// Name identifies the provider implementation.
	Name() string
}

// VectorLiteral formats an embedding as the Postgres vector text literal
// "[0.12,-0.34,...]". Queries pass this through ::vector casts so generated
// sqlc code needs no pgvector parameter types. It delegates to the vector
// column type's formatter so the two never drift.
func VectorLiteral(v []float32) string { return vector.Literal(v) }

// ParseVectorLiteral parses a vector text literal back into floats —
// the inverse of VectorLiteral, used by tests and the mock embedder.
func ParseVectorLiteral(s string) ([]float32, error) { return vector.Parse(s) }
