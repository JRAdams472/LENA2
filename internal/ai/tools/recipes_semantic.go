package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// SemanticRecipeCatalog is the recipe surface the semantic search tool
// needs: embedding search plus the standard row decorators.
type SemanticRecipeCatalog interface {
	RecipeCatalog
	SearchRecipesSemantic(ctx context.Context, arg recipe.SemanticSearch) ([]recipe.SemanticResult, error)
}

// QueryEmbedder embeds one free-text query into a pgvector literal.
// recipeembed.Service implements it.
type QueryEmbedder interface {
	EmbedQuery(ctx context.Context, query string) (string, error)
}

// semanticRecipeRow is one hit as the model sees it — the usual recipe row
// plus a similarity score (1 - cosine distance, so higher is better).
type semanticRecipeRow struct {
	RecipeRow
	Description string  `json:"description,omitempty"`
	Score       float64 `json:"score"`
}

// RegisterSemanticSearchTool wires search_recipes_semantic. main only calls
// this when an embedder is configured, so the model never sees a tool that
// can't run.
func RegisterSemanticSearchTool(reg *Registry, recipes SemanticRecipeCatalog, embedder QueryEmbedder) {
	reg.Register(llm.ToolSpec{
		Name: "search_recipes_semantic",
		Description: "Search the recipe catalog by meaning — vibe, cuisine, or situation queries " +
			"like 'something cozy for a rainy night' or 'light summer pasta'. Matches concepts, not " +
			"exact words. Embeddings can't express exclusions: for requests like 'no eggs', verify " +
			"each result's ingredients with get_recipe_details instead of trusting the match.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "What the user is in the mood for (1-500 chars)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum rows to return (default 8, max 20)",
				},
			},
			"required": []string{"query"},
		},
	}, func(ctx context.Context, _ Scope, args json.RawMessage) (any, error) {
		var a struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("search_recipes_semantic args: %w", err)
		}
		a.Query = strings.TrimSpace(a.Query)
		if a.Query == "" || len(a.Query) > 500 {
			return nil, fmt.Errorf("search_recipes_semantic: query must be 1-500 characters")
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 8
		}
		if limit > 20 {
			limit = 20
		}
		vec, err := embedder.EmbedQuery(ctx, a.Query)
		if err != nil {
			return nil, fmt.Errorf("semantic embed: %w", err)
		}
		hits, err := recipes.SearchRecipesSemantic(ctx, recipe.SemanticSearch{
			Active:      true,
			QueryVector: vec,
			Limit:       int32(limit),
		})
		if err != nil {
			return nil, fmt.Errorf("semantic search: %w", err)
		}
		recs := make([]recipe.Recipe, len(hits))
		for i, h := range hits {
			recs[i] = h.Recipe
		}
		rows, err := recipeRows(ctx, recipes, recs)
		if err != nil {
			return nil, err
		}
		out := make([]semanticRecipeRow, len(rows))
		for i, row := range rows {
			desc := hits[i].Recipe.Description
			if r := []rune(desc); len(r) > 200 {
				desc = string(r[:200]) + "…"
			}
			out[i] = semanticRecipeRow{RecipeRow: row, Description: desc, Score: 1 - hits[i].Distance}
		}
		return out, nil
	})
}
