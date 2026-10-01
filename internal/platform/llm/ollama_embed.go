package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/JRAdams472/LENA2/internal/platform/ollamaclient"
)

// OllamaEmbedder embeds texts via a local Ollama embedding model. Nomic-style
// models expect asymmetric prefixes: "search_document:" on stored text and
// "search_query:" on queries, so Embed and EmbedQuery are separate paths.
type OllamaEmbedder struct {
	client   *ollamaclient.Client
	model    string
	docPre   string
	queryPre string
}

// NewOllamaEmbedder builds an Embedder against baseURL with the named
// embedding model. docPrefix/queryPrefix default to the nomic convention
// when empty; pass "-" for a bare model with no prefix.
func NewOllamaEmbedder(baseURL, model string, timeout time.Duration, docPrefix, queryPrefix string) *OllamaEmbedder {
	if docPrefix == "" {
		docPrefix = "search_document: "
	} else if docPrefix == "-" {
		docPrefix = ""
	}
	if queryPrefix == "" {
		queryPrefix = "search_query: "
	} else if queryPrefix == "-" {
		queryPrefix = ""
	}
	c := ollamaclient.NewWithTimeout(baseURL, model, 0, 0, timeout)
	return &OllamaEmbedder{client: c, model: model, docPre: docPrefix, queryPre: queryPrefix}
}

// Name identifies the provider implementation.
func (e *OllamaEmbedder) Name() string { return "ollama" }

// Embed embeds document texts with the document prefix.
func (e *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	inputs := make([]string, len(texts))
	for i, t := range texts {
		inputs[i] = e.docPre + t
	}
	raw, err := e.client.Embed(ctx, e.model, inputs)
	if err != nil {
		return nil, fmt.Errorf("embed documents: %w", err)
	}
	return toFloat32(raw), nil
}

// EmbedQuery embeds one search query with the query prefix.
func (e *OllamaEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	raw, err := e.client.Embed(ctx, e.model, []string{e.queryPre + query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("embed query: empty response")
	}
	return toFloat32(raw)[0], nil
}

func toFloat32(in [][]float64) [][]float32 {
	out := make([][]float32, len(in))
	for i, v := range in {
		f := make([]float32, len(v))
		for j, x := range v {
			f[j] = float32(x)
		}
		out[i] = f
	}
	return out
}
