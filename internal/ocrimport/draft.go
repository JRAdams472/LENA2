package ocrimport

import (
	"encoding/json"
	"errors"
	"fmt"
)

// RecipeDraft is the LLM-structured recipe before catalog mapping. It mirrors
// CreateRecipeInput but uses free-text ingredient names instead of item IDs.
type RecipeDraft struct {
	Name            string      `json:"name"`
	Description     *string     `json:"description,omitempty"`
	Servings        *int        `json:"servings,omitempty"`
	PrepTimeMinutes *int        `json:"prepTimeMinutes,omitempty"`
	CookTimeMinutes *int        `json:"cookTimeMinutes,omitempty"`
	Items           []DraftItem `json:"items"`
	Steps           []DraftStep `json:"steps"`
	SourceHint      *string     `json:"sourceHint,omitempty"`
}

// DraftItem is one ingredient line extracted from a recipe.
type DraftItem struct {
	Quantity   *float64 `json:"quantity"`
	Unit       *string  `json:"unit"`
	Ingredient string   `json:"ingredient"`
	Section    *string  `json:"section,omitempty"`
	Notes      *string  `json:"notes,omitempty"`
	IsOptional bool     `json:"isOptional"`
}

// DraftStep is one instruction step.
type DraftStep struct {
	StepNumber  int    `json:"stepNumber"`
	Instruction string `json:"instruction"`
}

// ValidateDraft checks that a RecipeDraft is well-formed enough to continue
// to catalog mapping. Unparseable or empty fields become a detailed error.
func ValidateDraft(d *RecipeDraft) error {
	if d == nil {
		return errors.New("draft is nil")
	}
	if d.Name == "" {
		return errors.New("draft is missing a recipe name")
	}
	if len(d.Items) == 0 {
		return errors.New("draft has no ingredients")
	}
	for i, it := range d.Items {
		if it.Ingredient == "" {
			return fmt.Errorf("item %d is missing an ingredient name", i)
		}
	}
	for i, s := range d.Steps {
		if s.StepNumber <= 0 {
			return fmt.Errorf("step %d has an invalid step number", i)
		}
		if s.Instruction == "" {
			return fmt.Errorf("step %d has an empty instruction", i)
		}
	}
	return nil
}

// JSONSchema returns the JSON schema used to constrain the LLM output. It is
// embedded in prompts and can be sent to Ollama's "format" field.
func JSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The recipe title exactly as written.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Brief description or headnote, if any.",
			},
			"servings": map[string]interface{}{
				"type": "integer",
			},
			"prepTimeMinutes": map[string]interface{}{
				"type": "integer",
			},
			"cookTimeMinutes": map[string]interface{}{
				"type": "integer",
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"quantity": map[string]interface{}{
							"type": "number",
						},
						"unit": map[string]interface{}{
							"type": "string",
						},
						"ingredient": map[string]interface{}{
							"type":        "string",
							"description": "The bare ingredient noun phrase (e.g. 'all-purpose flour').",
						},
						"section": map[string]interface{}{
							"type": "string",
						},
						"notes": map[string]interface{}{
							"type": "string",
						},
						"isOptional": map[string]interface{}{
							"type":    "boolean",
							"default": false,
						},
					},
					"required": []string{"ingredient"},
				},
			},
			"steps": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"stepNumber": map[string]interface{}{
							"type":        "integer",
							"description": "One-based step index.",
						},
						"instruction": map[string]interface{}{
							"type":        "string",
							"description": "The full instruction text for this step.",
						},
					},
					"required": []string{"stepNumber", "instruction"},
				},
			},
			"sourceHint": map[string]interface{}{
				"type":        "string",
				"description": "Page, book, or source reference, if any.",
			},
		},
		"required": []string{"name", "items", "steps"},
	}
}

// MarshalDraftJSON returns an indented JSON representation of the draft.
func MarshalDraftJSON(d *RecipeDraft) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}
