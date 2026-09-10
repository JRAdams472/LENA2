package ocrimport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReviewRecipe holds the mapping decisions for one draft recipe. It is the
// interface between the LLM draft and the admin review step.
type ReviewRecipe struct {
	PageID      string        `json:"pageId"`
	Name        string        `json:"name"`
	Description *string       `json:"description,omitempty"`
	Servings    *int          `json:"servings,omitempty"`
	PrepTime    *int          `json:"prepTimeMinutes,omitempty"`
	CookTime    *int          `json:"cookTimeMinutes,omitempty"`
	SourceHint  *string       `json:"sourceHint,omitempty"`
	Items       []MatchResult `json:"items"`
	Steps       []DraftStep   `json:"steps"`
	Approved    bool          `json:"approved"`
}

// LoadReview reads an existing review.json from the page work directory.
func LoadReview(pageDir string) (*ReviewRecipe, error) {
	// #nosec G304 -- pageDir is constructed by the importer from a work-dir and a queue-generated page id.
	data, err := os.ReadFile(filepath.Join(pageDir, "review.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var r ReviewRecipe
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse review.json: %w", err)
	}
	return &r, nil
}

// SaveReview writes the review decisions to review.json.
func SaveReview(pageDir string, r *ReviewRecipe) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pageDir, "review.json"), data, 0o600)
}

// AllResolved reports whether every item has a catalog itemId and unit.
func (r *ReviewRecipe) AllResolved() bool {
	for _, it := range r.Items {
		if it.ItemID == "" || it.Unit == "" {
			return false
		}
	}
	return true
}

// MergeExistingReview carries forward admin decisions from an existing review
// file for the same draft. This keeps manual itemId/unit edits after a re-run.
func MergeExistingReview(existing *ReviewRecipe, mapped *ReviewRecipe) *ReviewRecipe {
	if existing == nil || existing.PageID != mapped.PageID {
		return mapped
	}

	decisions := make(map[int]MatchResult)
	for i, it := range existing.Items {
		if it.ItemID != "" || it.Unit != "" || it.Notes != "" {
			decisions[i] = it
		}
	}

	for i := range mapped.Items {
		if d, ok := decisions[i]; ok {
			if d.ItemID != "" {
				mapped.Items[i].ItemID = d.ItemID
				mapped.Items[i].ItemName = d.ItemName
			}
			if d.Unit != "" {
				mapped.Items[i].Unit = d.Unit
				mapped.Items[i].UnitID = d.UnitID
			}
			if d.Notes != "" {
				mapped.Items[i].Notes = d.Notes
			}
			mapped.Items[i].Approved = d.Approved
		}
	}
	mapped.Approved = existing.Approved
	return mapped
}

// RenderReviewMarkdown returns a human-readable Markdown review report.
func RenderReviewMarkdown(r *ReviewRecipe, ocrText string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Review: %s\n\n", r.Name)
	if r.Description != nil && *r.Description != "" {
		fmt.Fprintf(&b, "**Description:** %s\n\n", *r.Description)
	}
	if r.Servings != nil {
		fmt.Fprintf(&b, "**Servings:** %d\n\n", *r.Servings)
	}
	if r.PrepTime != nil || r.CookTime != nil {
		fmt.Fprintf(&b, "**Prep:** %d min | **Cook:** %d min\n\n",
			derefInt(r.PrepTime), derefInt(r.CookTime))
	}
	if r.SourceHint != nil && *r.SourceHint != "" {
		fmt.Fprintf(&b, "**Source:** %s\n\n", *r.SourceHint)
	}

	b.WriteString("## OCR text\n\n```\n")
	b.WriteString(ocrText)
	b.WriteString("\n```\n\n")

	b.WriteString("## Ingredients\n\n")
	if len(r.Items) == 0 {
		b.WriteString("*No ingredients found.*\n\n")
	}
	for i, it := range r.Items {
		fmt.Fprintf(&b, "%d. ", i+1)
		qty := "-"
		if it.DraftItem.Quantity != nil {
			qty = fmt.Sprintf("%g", *it.DraftItem.Quantity)
		}
		unit := derefString(it.DraftItem.Unit)
		fmt.Fprintf(&b, "**%s %s %s**\n", qty, unit, it.DraftItem.Ingredient)
		if it.DraftItem.Section != nil {
			fmt.Fprintf(&b, "   - Section: %s\n", *it.DraftItem.Section)
		}
		if it.DraftItem.Notes != nil {
			fmt.Fprintf(&b, "   - Notes: %s\n", *it.DraftItem.Notes)
		}
		if it.DraftItem.IsOptional {
			b.WriteString("   - Optional\n")
		}

		fmt.Fprintf(&b, "   - Status: **%s** (confidence: %.2f)\n", it.Status, it.Confidence)
		if it.ItemID != "" {
			fmt.Fprintf(&b, "   - Matched item: %s (%s)\n", it.ItemName, it.ItemID)
		}
		if it.Unit != "" {
			fmt.Fprintf(&b, "   - Canonical unit: %s (%s)\n", it.Unit, it.UnitID)
		}
		if it.Notes != "" {
			fmt.Fprintf(&b, "   - Review note: %s\n", it.Notes)
		}
		if len(it.Suggestions) > 0 {
			b.WriteString("   - Suggestions:\n")
			for _, s := range it.Suggestions {
				fmt.Fprintf(&b, "     - [%s] %s (%.2f)\n", s.Kind, s.Name, s.Score)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Steps\n\n")
	if len(r.Steps) == 0 {
		b.WriteString("*No steps found.*\n\n")
	}
	for _, s := range r.Steps {
		fmt.Fprintf(&b, "%d. %s\n", s.StepNumber, s.Instruction)
	}

	b.WriteString("\n## Admin actions\n\n")
	if r.Approved {
		b.WriteString("- [x] Approved for persistence\n")
	} else {
		b.WriteString("- [ ] Approved for persistence\n")
	}
	b.WriteString("- Edit `review.json` to set `itemId`/`unit` for each item and set `approved: true`.\n")
	b.WriteString("- Run `ocrimport persist` once everything is resolved.\n")

	return b.String()
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// BestSuggestion picks the highest-scoring item suggestion, or nil if none.
func (r MatchResult) BestSuggestion() *Suggestion {
	if len(r.Suggestions) == 0 {
		return nil
	}
	sorted := make([]Suggestion, len(r.Suggestions))
	copy(sorted, r.Suggestions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	if sorted[0].Kind == "item" {
		return &sorted[0]
	}
	for _, s := range sorted {
		if s.Kind == "item" {
			return &s
		}
	}
	return &sorted[0]
}
