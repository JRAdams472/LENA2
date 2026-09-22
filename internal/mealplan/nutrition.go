package mealplan

import (
	"fmt"
	"sort"
)

// Nutrition aggregation types are deliberately neutral (no imports of
// sibling domains): the caller adapts inventory/recipe data into these
// shapes, and the dimensional math lives here in the mealplan domain.

// NutritionLine is one consumed quantity of a catalog item.
type NutritionLine struct {
	ItemID   int64
	Quantity float64
	UnitID   int64
}

// NutrientBasis is one nutrient value for an item: Amount units of the
// nutrient per BasisQuantity of BasisUnitID of the food.
type NutrientBasis struct {
	NutrientID    int64
	Name          string
	Unit          string
	Amount        float64
	BasisQuantity float64
	BasisUnitID   int64
}

// NutritionUnit is the unit metadata needed to convert quantities.
type NutritionUnit struct {
	Name         string
	Kind         string // "volume" | "weight" | "count"
	ToBaseFactor *float64
}

// NutritionItem is the item metadata needed to bridge count units to
// mass: NetWeight is the weight of one count unit expressed in UnitID.
type NutritionItem struct {
	UnitID    int64
	NetWeight *float64
}

// NutrientTotal is one aggregated nutrient row.
type NutrientTotal struct {
	NutrientID int64
	Name       string
	Unit       string
	Amount     float64
}

// AggregateNutrition totals nutrient amounts across consumed lines,
// converting each line's quantity into the nutrient's declared basis
// unit. Lines that cannot be converted (incompatible unit kinds, missing
// conversion factors, or count units without an item net weight) are
// skipped and reported in the returned warnings instead of producing a
// dimensionally unsound number.
func AggregateNutrition(
	lines []NutritionLine,
	nutrientsByItem map[int64][]NutrientBasis,
	items map[int64]NutritionItem,
	units map[int64]NutritionUnit,
) ([]NutrientTotal, []string) {
	totals := make(map[int64]*NutrientTotal)
	warningSet := make(map[string]bool)

	for _, line := range lines {
		for _, n := range nutrientsByItem[line.ItemID] {
			basisQty, warn := convertToBasis(line, n.BasisUnitID, items, units)
			if warn != "" {
				warningSet[warn] = true
				continue
			}
			if n.BasisQuantity <= 0 {
				warningSet[fmt.Sprintf("nutrient %s has a non-positive basis quantity", n.Name)] = true
				continue
			}
			t, ok := totals[n.NutrientID]
			if !ok {
				t = &NutrientTotal{NutrientID: n.NutrientID, Name: n.Name, Unit: n.Unit}
				totals[n.NutrientID] = t
			}
			t.Amount += n.Amount * (basisQty / n.BasisQuantity)
		}
	}

	out := make([]NutrientTotal, 0, len(totals))
	for _, t := range totals {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	warnings := make([]string, 0, len(warningSet))
	for w := range warningSet {
		warnings = append(warnings, w)
	}
	sort.Strings(warnings)
	return out, warnings
}

// convertToBasis expresses line.Quantity in units of basisUnitID. It
// handles same-unit, same-kind factor conversion, and count units via
// the item's net weight.
func convertToBasis(line NutritionLine, basisUnitID int64, items map[int64]NutritionItem, units map[int64]NutritionUnit) (float64, string) {
	if line.UnitID == basisUnitID {
		return line.Quantity, ""
	}
	src, srcOK := units[line.UnitID]
	dst, dstOK := units[basisUnitID]
	if !srcOK || !dstOK {
		return 0, fmt.Sprintf("unit metadata missing for unit %d or %d; item %d skipped", line.UnitID, basisUnitID, line.ItemID)
	}
	if q, ok := convertByFactor(line.Quantity, src, dst); ok {
		return q, ""
	}
	// Count units bridge through the item's net weight: N each × netWeight
	// (in the item's canonical unit) → convert that weight to the basis.
	if src.Kind == "count" {
		item, ok := items[line.ItemID]
		if !ok || item.NetWeight == nil || *item.NetWeight <= 0 {
			return 0, fmt.Sprintf("item %d uses count unit %q but has no net weight; nutrient contribution skipped", line.ItemID, src.Name)
		}
		itemUnit, ok := units[item.UnitID]
		if !ok {
			return 0, fmt.Sprintf("item %d canonical unit %d has no metadata; nutrient contribution skipped", line.ItemID, item.UnitID)
		}
		if q, ok := convertByFactor(line.Quantity**item.NetWeight, itemUnit, dst); ok {
			return q, ""
		}
	}
	return 0, fmt.Sprintf("cannot convert %q (%s) to %q (%s) for item %d; nutrient contribution skipped", src.Name, src.Kind, dst.Name, dst.Kind, line.ItemID)
}

func convertByFactor(qty float64, src, dst NutritionUnit) (float64, bool) {
	if src.Kind != dst.Kind || src.ToBaseFactor == nil || dst.ToBaseFactor == nil || *dst.ToBaseFactor == 0 {
		return 0, false
	}
	return qty * *src.ToBaseFactor / *dst.ToBaseFactor, true
}
