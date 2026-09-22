package inventory

// ConvertQuantity converts a quantity between two units of the same kind
// using their base factors (milliliters for volume, grams for weight). It
// reports false when the units differ in kind or either side lacks a
// conversion — summing would be dimensionally meaningless. Identical units
// always convert.
func ConvertQuantity(quantity float64, from, to Unit) (float64, bool) {
	if from.UnitID == to.UnitID {
		return quantity, true
	}
	if from.Kind != to.Kind {
		return 0, false
	}
	if from.ToBaseFactor == nil || to.ToBaseFactor == nil || *to.ToBaseFactor == 0 {
		return 0, false
	}
	return quantity * *from.ToBaseFactor / *to.ToBaseFactor, true
}
