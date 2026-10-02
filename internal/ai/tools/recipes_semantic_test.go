package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/recipe"
)

type stubSemanticRecipes struct {
	*stubRecipes
	hits    []recipe.SemanticResult
	lastArg recipe.SemanticSearch
	err     error
}

func (s *stubSemanticRecipes) SearchRecipesSemantic(_ context.Context, arg recipe.SemanticSearch) ([]recipe.SemanticResult, error) {
	s.lastArg = arg
	return s.hits, s.err
}

type stubEmbedder struct {
	lit string
	err error
}

func (s stubEmbedder) EmbedQuery(_ context.Context, q string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.lit + "|" + q, nil
}

func semanticFixture() (*Registry, *stubSemanticRecipes) {
	rc := &stubSemanticRecipes{
		stubRecipes: &stubRecipes{
			recipes: map[int64]recipe.Recipe{
				1: {RecipeID: 1, Name: "Pancakes", Description: "fluffy"},
				2: {RecipeID: 2, Name: "Pasta"},
			},
			cats: map[int64][]recipe.Category{1: {{CategoryID: 5, Name: "Breakfast"}}},
		},
		hits: []recipe.SemanticResult{
			{Recipe: recipe.Recipe{RecipeID: 1, Name: "Pancakes", Description: "fluffy"}, Distance: 0.1},
			{Recipe: recipe.Recipe{RecipeID: 2, Name: "Pasta"}, Distance: 0.55},
		},
	}
	reg := New()
	RegisterSemanticSearchTool(reg, rc, stubEmbedder{lit: "[1,0]"})
	return reg, rc
}

func TestSearchRecipesSemantic_Happy(t *testing.T) {
	reg, rc := semanticFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9},
		"search_recipes_semantic", json.RawMessage(`{"query":"cozy breakfast"}`))
	require.NoError(t, err)
	assert.Equal(t, "[1,0]|cozy breakfast", rc.lastArg.QueryVector)
	assert.Equal(t, int32(8), rc.lastArg.Limit)
	rows, ok := out.([]semanticRecipeRow)
	require.True(t, ok)
	require.Len(t, rows, 2)
	assert.Equal(t, "Pancakes", rows[0].Name)
	assert.Equal(t, "fluffy", rows[0].Description)
	assert.Equal(t, []string{"Breakfast"}, rows[0].Categories)
	assert.InDelta(t, 0.9, rows[0].Score, 0.001)
	assert.InDelta(t, 0.45, rows[1].Score, 0.001)
}

func TestSearchRecipesSemantic_LimitClamp(t *testing.T) {
	reg, rc := semanticFixture()
	_, err := reg.Call(context.Background(), Scope{UserID: 7},
		"search_recipes_semantic", json.RawMessage(`{"query":"soup","limit":99}`))
	require.NoError(t, err)
	assert.Equal(t, int32(20), rc.lastArg.Limit)
}

func TestSearchRecipesSemantic_BadArgs(t *testing.T) {
	reg, _ := semanticFixture()
	for _, raw := range []string{`{}`, `{"query":""}`, `{"query":"  "}`} {
		_, err := reg.Call(context.Background(), Scope{UserID: 7},
			"search_recipes_semantic", json.RawMessage(raw))
		assert.Error(t, err, raw)
	}
	q := `{"query":"` + string(repeatChar('x', 501)) + `"}`
	_, err := reg.Call(context.Background(), Scope{UserID: 7},
		"search_recipes_semantic", json.RawMessage(q))
	assert.Error(t, err)
}

func repeatChar(c byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return b
}

func TestSearchRecipesSemantic_EmbedderError(t *testing.T) {
	rc := &stubSemanticRecipes{stubRecipes: &stubRecipes{recipes: map[int64]recipe.Recipe{}}}
	reg := New()
	RegisterSemanticSearchTool(reg, rc, stubEmbedder{err: errors.New("ollama down")})
	_, err := reg.Call(context.Background(), Scope{UserID: 7},
		"search_recipes_semantic", json.RawMessage(`{"query":"soup"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ollama down")
}

func TestSearchRecipesSemantic_EmptyResults(t *testing.T) {
	rc := &stubSemanticRecipes{stubRecipes: &stubRecipes{recipes: map[int64]recipe.Recipe{}}}
	reg := New()
	RegisterSemanticSearchTool(reg, rc, stubEmbedder{lit: "[0]"})
	out, err := reg.Call(context.Background(), Scope{UserID: 7},
		"search_recipes_semantic", json.RawMessage(`{"query":"nothing"}`))
	require.NoError(t, err)
	rows, ok := out.([]semanticRecipeRow)
	require.True(t, ok)
	assert.Empty(t, rows)
}
