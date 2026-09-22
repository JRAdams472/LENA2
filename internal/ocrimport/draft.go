package ocrimport

import (
	"encoding/json"
	"errors"
	"fmt"
)

// RecipeDraft is the LLM-structured recipe before catalog mapping. It mirrors
// CreateRecipeInput but uses free-text ingredient names instead of item IDs.
type RecipeDraft struct {
	Name              string      `json:"name"`
	Description       *string     `json:"description,omitempty"`
	Servings          *int        `json:"servings,omitempty"`
	PrepTimeMinutes   *int        `json:"prepTimeMinutes,omitempty"`
	CookTimeMinutes   *int        `json:"cookTimeMinutes,omitempty"`
	Items             []DraftItem `json:"items"`
	Steps             []DraftStep `json:"steps"`
	SourceHint        *string     `json:"sourceHint,omitempty"`
	ProfanityDetected bool        `json:"profanityDetected,omitempty"`
	ProfanityReason   *string     `json:"profanityReason,omitempty"`
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

// Bounds applied to the LLM's JSON before it can reach review rows. They are
// deliberately generous so real recipes pass while injected or degenerate
// output fails validation.
const (
	maxDraftNameLen        = 200
	maxDraftDescriptionLen = 2000
	maxDraftSourceHintLen  = 200
	maxDraftItems          = 100
	maxDraftSteps          = 100
	maxDraftIngredientLen  = 200
	maxDraftTextLen        = 500
	maxDraftInstructionLen = 2000
	maxDraftQuantity       = 100_000.0
	maxDraftServings       = 1_000
	maxDraftMinutes        = 10_080 // one week
	maxDraftStepNumber     = 500
)

// ValidateDraft checks that a RecipeDraft is well-formed enough to continue
// to catalog mapping. Unparseable, oversized, or empty fields become a
// detailed error. If profanity was detected the LLM is trusted and
// validation is skipped so the job can be quarantined.
func ValidateDraft(d *RecipeDraft) error {
	if d == nil {
		return errors.New("draft is nil")
	}
	if d.ProfanityDetected {
		return nil
	}
	if d.Name == "" {
		return errors.New("draft is missing a recipe name")
	}
	if len(d.Name) > maxDraftNameLen {
		return fmt.Errorf("recipe name exceeds %d characters", maxDraftNameLen)
	}
	if d.Description != nil && len(*d.Description) > maxDraftDescriptionLen {
		return fmt.Errorf("description exceeds %d characters", maxDraftDescriptionLen)
	}
	if d.SourceHint != nil && len(*d.SourceHint) > maxDraftSourceHintLen {
		return fmt.Errorf("source hint exceeds %d characters", maxDraftSourceHintLen)
	}
	if d.Servings != nil && (*d.Servings < 1 || *d.Servings > maxDraftServings) {
		return fmt.Errorf("servings must be between 1 and %d", maxDraftServings)
	}
	if d.PrepTimeMinutes != nil && (*d.PrepTimeMinutes < 0 || *d.PrepTimeMinutes > maxDraftMinutes) {
		return fmt.Errorf("prep time must be between 0 and %d minutes", maxDraftMinutes)
	}
	if d.CookTimeMinutes != nil && (*d.CookTimeMinutes < 0 || *d.CookTimeMinutes > maxDraftMinutes) {
		return fmt.Errorf("cook time must be between 0 and %d minutes", maxDraftMinutes)
	}
	if len(d.Items) == 0 {
		return errors.New("draft has no ingredients")
	}
	if len(d.Items) > maxDraftItems {
		return fmt.Errorf("draft has %d items; maximum is %d", len(d.Items), maxDraftItems)
	}
	for i, it := range d.Items {
		if it.Ingredient == "" {
			return fmt.Errorf("item %d is missing an ingredient name", i)
		}
		if len(it.Ingredient) > maxDraftIngredientLen {
			return fmt.Errorf("item %d ingredient exceeds %d characters", i, maxDraftIngredientLen)
		}
		if it.Quantity != nil && (*it.Quantity <= 0 || *it.Quantity > maxDraftQuantity) {
			return fmt.Errorf("item %d quantity must be between 0 and %g", i, maxDraftQuantity)
		}
		if it.Unit != nil && len(*it.Unit) > maxDraftTextLen {
			return fmt.Errorf("item %d unit exceeds %d characters", i, maxDraftTextLen)
		}
		if it.Section != nil && len(*it.Section) > maxDraftTextLen {
			return fmt.Errorf("item %d section exceeds %d characters", i, maxDraftTextLen)
		}
		if it.Notes != nil && len(*it.Notes) > maxDraftTextLen {
			return fmt.Errorf("item %d notes exceeds %d characters", i, maxDraftTextLen)
		}
	}
	if len(d.Steps) > maxDraftSteps {
		return fmt.Errorf("draft has %d steps; maximum is %d", len(d.Steps), maxDraftSteps)
	}
	for i, s := range d.Steps {
		if s.StepNumber <= 0 || s.StepNumber > maxDraftStepNumber {
			return fmt.Errorf("step %d has an invalid step number", i)
		}
		if s.Instruction == "" {
			return fmt.Errorf("step %d has an empty instruction", i)
		}
		if len(s.Instruction) > maxDraftInstructionLen {
			return fmt.Errorf("step %d instruction exceeds %d characters", i, maxDraftInstructionLen)
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
			"profanityDetected": map[string]interface{}{
				"type": "boolean",
			},
			"profanityReason": map[string]interface{}{
				"type":        "string",
				"description": "Explanation of any profanity detected in the source.",
			},
		},
	}
}

// MarshalDraftJSON returns an indented JSON representation of the draft.
func MarshalDraftJSON(d *RecipeDraft) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}
