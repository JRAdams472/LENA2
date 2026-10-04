package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

func TestProposeBatch(t *testing.T) {
	ingredients := []ingredientRow{
		{ID: 1, Name: "peanut butter"},
		{ID: 2, Name: "worcestershire sauce"},
		{ID: 3, Name: "carrots"},
	}
	known := []string{"peanuts", "fish", "gluten"}

	t.Run("flags allergens, marks new ones, and collects unresolved", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(req llm.Request) (llm.Response, error) {
			assert.True(t, req.JSONMode, "propose must request JSON output")
			assert.Contains(t, req.Messages[0].Content, "worcestershire sauce")
			assert.Contains(t, req.Messages[0].Content, "peanuts")
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"ingredient_id":1,"allergens":[{"name":"Peanuts","kind":"contains"}],"confidence":"high","notes":"peanut spread"},
					{"ingredient_id":2,"allergens":[{"name":"fish","kind":"contains"},{"name":"Histamine","kind":"CONTAINS"}],"confidence":"medium","notes":"anchovies; histamine not in registry"}
				],"unresolved":[
					{"ingredient_id":3,"reason":"no allergens"}
				]}`},
			}, nil
		}

		ms, us, err := proposeBatch(context.Background(), prov, known, ingredients)
		require.NoError(t, err)
		require.Len(t, ms, 2)
		// Known allergen (case-insensitive normalization) — not new.
		assert.Equal(t, "peanuts", ms[0].Allergens[0].Name)
		assert.Equal(t, "contains", ms[0].Allergens[0].Kind)
		assert.False(t, ms[0].Allergens[0].NewAllergen)
		// Hidden source: fish in worcestershire; novel allergen flagged for review.
		assert.Equal(t, "fish", ms[1].Allergens[0].Name)
		assert.Equal(t, "histamine", ms[1].Allergens[1].Name)
		assert.True(t, ms[1].Allergens[1].NewAllergen)
		require.Len(t, us, 1)
		assert.Equal(t, int64(3), us[0].IngredientID)
		assert.Equal(t, "carrots", us[0].IngredientName)
	})

	t.Run("drops empty and duplicate flags and normalizes kind", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"ingredient_id":1,"allergens":[
						{"name":"peanuts","kind":"contains"},
						{"name":"Peanuts","kind":"may_contain"},
						{"name":"  ","kind":"contains"},
						{"name":"dust","kind":"weird"}
					],"confidence":"high"}
				]}`},
			}, nil
		}

		ms, _, err := proposeBatch(context.Background(), prov, known, ingredients)
		require.NoError(t, err)
		require.Len(t, ms, 1)
		require.Len(t, ms[0].Allergens, 2, "duplicate normalized name and empty name are dropped")
		assert.Equal(t, "peanuts", ms[0].Allergens[0].Name)
		// Invalid kind coerces to contains — safest classification wins.
		assert.Equal(t, "contains", ms[0].Allergens[1].Kind)
	})

	t.Run("skips mappings with no usable flags and propagates errors", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[{"ingredient_id":1,"allergens":[{"name":"","kind":"contains"}]}]}`},
			}, nil
		}
		ms, _, err := proposeBatch(context.Background(), prov, known, ingredients)
		require.NoError(t, err)
		assert.Empty(t, ms)

		boom := errors.New("ollama down")
		prov.Handler = func(llm.Request) (llm.Response, error) { return llm.Response{}, boom }
		_, _, err = proposeBatch(context.Background(), prov, known, ingredients)
		assert.ErrorIs(t, err, boom)

		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{Content: "not json"}}, nil
		}
		_, _, err = proposeBatch(context.Background(), prov, known, ingredients)
		assert.ErrorContains(t, err, "parse model reply")
	})
}

func TestArtifactRoundTrip(t *testing.T) {
	// The artifact schema is the review contract between propose and
	// apply; a rename breaks reviewed files silently, so pin the shape.
	raw := `{
		"generated_at": "2026-10-04T00:00:00Z",
		"model": "qwen2.5:7b-instruct",
		"mappings": [{"ingredient_id": 42, "ingredient_name": "soy sauce", "allergens": [{"name": "soy", "kind": "contains"}, {"name": "wheat", "kind": "contains"}], "confidence": "high", "notes": "fermented soy and wheat"}],
		"unresolved": [{"ingredient_id": 7, "ingredient_name": "salt", "reason": "no allergens"}]
	}`
	var art artifact
	require.NoError(t, json.Unmarshal([]byte(raw), &art))
	require.Len(t, art.Mappings, 1)
	assert.Equal(t, int64(42), art.Mappings[0].IngredientID)
	require.Len(t, art.Mappings[0].Allergens, 2)
	assert.Equal(t, "wheat", art.Mappings[0].Allergens[1].Name)
	assert.Equal(t, "contains", art.Mappings[0].Allergens[1].Kind)
	require.Len(t, art.Unresolved, 1)
	assert.Equal(t, "no allergens", art.Unresolved[0].Reason)
}
