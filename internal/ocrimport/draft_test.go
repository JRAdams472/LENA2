package ocrimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDraft(t *testing.T) {
	t.Run("valid minimal draft", func(t *testing.T) {
		d := &RecipeDraft{
			Name:  "Pancakes",
			Items: []DraftItem{{Ingredient: "flour"}},
			Steps: []DraftStep{{StepNumber: 1, Instruction: "Mix"}},
		}
		assert.NoError(t, ValidateDraft(d))
	})

	t.Run("missing name", func(t *testing.T) {
		d := &RecipeDraft{
			Items: []DraftItem{{Ingredient: "flour"}},
			Steps: []DraftStep{{StepNumber: 1, Instruction: "Mix"}},
		}
		assert.EqualError(t, ValidateDraft(d), "draft is missing a recipe name")
	})

	t.Run("no items", func(t *testing.T) {
		d := &RecipeDraft{
			Name:  "Pancakes",
			Steps: []DraftStep{{StepNumber: 1, Instruction: "Mix"}},
		}
		assert.EqualError(t, ValidateDraft(d), "draft has no ingredients")
	})

	t.Run("empty ingredient", func(t *testing.T) {
		d := &RecipeDraft{
			Name:  "Pancakes",
			Items: []DraftItem{{Ingredient: ""}},
			Steps: []DraftStep{{StepNumber: 1, Instruction: "Mix"}},
		}
		assert.EqualError(t, ValidateDraft(d), "item 0 is missing an ingredient name")
	})

	t.Run("invalid step number", func(t *testing.T) {
		d := &RecipeDraft{
			Name:  "Pancakes",
			Items: []DraftItem{{Ingredient: "flour"}},
			Steps: []DraftStep{{StepNumber: 0, Instruction: "Mix"}},
		}
		assert.EqualError(t, ValidateDraft(d), "step 0 has an invalid step number")
	})

	t.Run("empty step instruction", func(t *testing.T) {
		d := &RecipeDraft{
			Name:  "Pancakes",
			Items: []DraftItem{{Ingredient: "flour"}},
			Steps: []DraftStep{{StepNumber: 1, Instruction: ""}},
		}
		assert.EqualError(t, ValidateDraft(d), "step 0 has an empty instruction")
	})

	t.Run("empty steps are allowed", func(t *testing.T) {
		d := &RecipeDraft{
			Name:  "Ingredient list",
			Items: []DraftItem{{Ingredient: "flour"}},
			Steps: []DraftStep{},
		}
		assert.NoError(t, ValidateDraft(d))
	})
}

func TestJSONSchema(t *testing.T) {
	schema := JSONSchema()
	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "items")
	assert.Contains(t, props, "steps")
}
