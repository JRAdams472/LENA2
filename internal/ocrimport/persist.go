package ocrimport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JRAdams472/LENA2/internal/platform/bffclient"
)

// PersistedRecord is written when a recipe has been successfully imported.
type PersistedRecord struct {
	RecipeID   string    `json:"recipeId"`
	Name       string    `json:"name"`
	SourcePath string    `json:"sourcePath"`
	SourceHash string    `json:"sourceHash"`
	CreatedAt  time.Time `json:"createdAt"`
}

// LoadPersisted reads a persisted.json record, if any.
func LoadPersisted(pageDir string) (*PersistedRecord, error) {
	// #nosec G304 -- pageDir is constructed by the importer from a work-dir and a queue-generated page id.
	data, err := os.ReadFile(filepath.Join(pageDir, "persisted.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var r PersistedRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse persisted.json: %w", err)
	}
	return &r, nil
}

// SavePersisted writes a persisted.json record.
func SavePersisted(pageDir string, r *PersistedRecord) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pageDir, "persisted.json"), data, 0o600)
}

// ErrorRecord captures why a recipe could not be persisted.
type ErrorRecord struct {
	PageID  string    `json:"pageId"`
	Error   string    `json:"error"`
	Created time.Time `json:"createdAt"`
}

// SaveError writes an error.json record for a failed persist attempt.
func SaveError(pageDir string, rec ErrorRecord) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pageDir, "error.json"), data, 0o600)
}

// ToCreateRecipeInput converts an approved ReviewRecipe into the GraphQL
// CreateRecipeInput. It returns an error if any item is missing a catalog
// itemId or resolvable unit.
func (r *ReviewRecipe) ToCreateRecipeInput() (*bffclient.CreateRecipeInput, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("recipe name is empty")
	}
	if !r.AllResolved() {
		return nil, fmt.Errorf("recipe has unresolved items")
	}

	items := make([]bffclient.RecipeItemInput, 0, len(r.Items))
	for i, it := range r.Items {
		if it.ItemID == "" {
			return nil, fmt.Errorf("item %d (%s) has no catalog itemId", i, it.DraftItem.Ingredient)
		}
		if it.Unit == "" {
			return nil, fmt.Errorf("item %d (%s) has no resolvable unit", i, it.DraftItem.Ingredient)
		}

		qty := 1.0
		if it.DraftItem.Quantity != nil {
			qty = *it.DraftItem.Quantity
		}

		displayOrder := i + 1
		input := bffclient.RecipeItemInput{
			ItemID:       it.ItemID,
			Quantity:     qty,
			Unit:         it.Unit,
			Section:      it.DraftItem.Section,
			DisplayOrder: &displayOrder,
			Notes:        it.DraftItem.Notes,
		}
		if it.DraftItem.IsOptional {
			input.IsOptional = &it.DraftItem.IsOptional
		}
		items = append(items, input)
	}

	steps := make([]bffclient.RecipeStepInput, 0, len(r.Steps))
	for _, s := range r.Steps {
		steps = append(steps, bffclient.RecipeStepInput{
			StepNumber:  s.StepNumber,
			Instruction: s.Instruction,
		})
	}

	return &bffclient.CreateRecipeInput{
		Name:            r.Name,
		Description:     r.Description,
		Servings:        r.Servings,
		PrepTimeMinutes: r.PrepTime,
		CookTimeMinutes: r.CookTime,
		Items:           items,
		Steps:           steps,
	}, nil
}
