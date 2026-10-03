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
	items := []itemRow{
		{ID: 1, Name: "Green Giant Whole Kernel Corn"},
		{ID: 2, Name: "Kraft Macaroni & Cheese"},
		{ID: 3, Name: "Mystery Blend"},
	}
	known := []string{"corn", "flour", "sugar"}

	t.Run("maps, flags new ingredients, and collects unresolved", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(req llm.Request) (llm.Response, error) {
			assert.True(t, req.JSONMode, "propose must request JSON output")
			assert.Contains(t, req.Messages[0].Content, "Green Giant Whole Kernel Corn")
			assert.Contains(t, req.Messages[0].Content, "corn")
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"item_id":1,"ingredient":"Corn","confidence":"high","notes":"canned corn"},
					{"item_id":2,"ingredient":"macaroni and cheese","confidence":"medium"}
				],"unresolved":[
					{"item_id":3,"reason":"mixed product, no single ingredient"}
				]}`},
			}, nil
		}

		ms, us, err := proposeBatch(context.Background(), prov, known, items)
		require.NoError(t, err)
		require.Len(t, ms, 2)
		// Known ingredient (case-insensitive normalization) — not new.
		assert.Equal(t, "corn", ms[0].Ingredient)
		assert.False(t, ms[0].NewIngredient)
		assert.Equal(t, "Green Giant Whole Kernel Corn", ms[0].ItemName)
		// Novel ingredient gets flagged for review.
		assert.True(t, ms[1].NewIngredient)
		require.Len(t, us, 1)
		assert.Equal(t, int64(3), us[0].ItemID)
		assert.Equal(t, "Mystery Blend", us[0].ItemName)
	})

	t.Run("drops duplicate and empty mappings", func(t *testing.T) {
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{
				Content: `{"mappings":[
					{"item_id":1,"ingredient":"corn","confidence":"high"},
					{"item_id":1,"ingredient":"flour","confidence":"low"},
					{"item_id":2,"ingredient":"  ","confidence":"high"}
				]}`},
			}, nil
		}

		ms, _, err := proposeBatch(context.Background(), prov, known, items)
		require.NoError(t, err)
		require.Len(t, ms, 1, "second mapping for the same item and empty names are dropped")
		assert.Equal(t, "corn", ms[0].Ingredient)
	})

	t.Run("propagates provider and parse errors", func(t *testing.T) {
		boom := errors.New("ollama down")
		prov := llm.NewMockProvider()
		prov.Handler = func(llm.Request) (llm.Response, error) { return llm.Response{}, boom }
		_, _, err := proposeBatch(context.Background(), prov, known, items)
		assert.ErrorIs(t, err, boom)

		prov.Handler = func(llm.Request) (llm.Response, error) {
			return llm.Response{Message: llm.Message{Content: "not json"}}, nil
		}
		_, _, err = proposeBatch(context.Background(), prov, known, items)
		assert.ErrorContains(t, err, "parse model reply")
	})
}

func TestArtifactRoundTrip(t *testing.T) {
	// The artifact schema is the review contract between propose and apply;
	// a rename breaks reviewed files silently, so pin the JSON shape.
	raw := `{
		"generated_at": "2026-10-06T00:00:00Z",
		"model": "qwen3:8b",
		"mappings": [{"item_id": 42, "item_name": "Kellogg's Corn Flakes", "ingredient": "corn flakes", "new_ingredient": true, "confidence": "medium", "notes": "cereal"}],
		"unresolved": [{"item_id": 7, "item_name": "Party Mix", "reason": "mixed product"}]
	}`
	var art artifact
	require.NoError(t, json.Unmarshal([]byte(raw), &art))
	require.Len(t, art.Mappings, 1)
	assert.Equal(t, int64(42), art.Mappings[0].ItemID)
	assert.Equal(t, "corn flakes", art.Mappings[0].Ingredient)
	assert.True(t, art.Mappings[0].NewIngredient)
	require.Len(t, art.Unresolved, 1)
	assert.Equal(t, "mixed product", art.Unresolved[0].Reason)
}
