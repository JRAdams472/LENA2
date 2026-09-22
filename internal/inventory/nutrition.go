package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/JRAdams472/LENA2/internal/inventory/nutritionparse"
)

// nutrientLabelMaxLen matches the implicit catalog limit for nutrient type
// names; anything longer is not a real nutrient and is ignored.
const nutrientLabelMaxLen = 100

// ApplyNutritionLabel matches parsed nutrition-label nutrients to catalog
// nutrient types and replaces the item's nutrient rows in one transaction.
// Unknown labels are not auto-created; only existing catalog types are
// applied so non-admin members cannot add global reference data.
func (s *Service) ApplyNutritionLabel(ctx context.Context, itemID int64, parsed []nutritionparse.Nutrient, by string) error {
	entries := make([]NutrientEntry, 0, len(parsed))
	matched := 0
	skipped := 0
	unknown := 0
	for _, n := range parsed {
		label := strings.TrimSpace(n.Label)
		if label == "" || len(label) > nutrientLabelMaxLen {
			skipped++
			continue
		}
		nt, err := s.GetNutrientTypeByName(ctx, label)
		if err != nil {
			// Unknown labels are not created here; admins use createNutrientType.
			unknown++
			continue
		}
		matched++
		entries = append(entries, NutrientEntry{
			NutrientID: nt.NutrientID,
			Amount:     n.Amount,
		})
	}

	if err := s.SetItemNutrients(ctx, itemID, entries, by); err != nil {
		return fmt.Errorf("set item nutrients: %w", err)
	}

	slog.Default().Info("nutrition ocr completed",
		"item_id", itemID,
		"parsed", len(parsed),
		"matched", matched,
		"unknown", unknown,
		"skipped", skipped,
		"by", by,
	)
	return nil
}
