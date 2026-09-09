package nutritionparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTypicalLabel(t *testing.T) {
	text := `Nutrition Facts
Serving Size 1 cup (240ml)
Calories 120
Total Fat 4g
    Saturated Fat 1.5g
    Trans Fat 0g
Cholesterol 0mg
Sodium 180mg
Total Carbohydrate 22g
    Dietary Fiber 4g
    Total Sugars 8g
        Includes 4g Added Sugars
Protein 6g
Vitamin D 2mcg
Calcium 260mg
Iron 1mg
Potassium 380mg`

	got := Parse(text)
	want := []Nutrient{
		{Label: "Calories", Amount: 120, Unit: "kcal"},
		{Label: "Total Fat", Amount: 4, Unit: "g"},
		{Label: "Saturated Fat", Amount: 1.5, Unit: "g"},
		{Label: "Trans Fat", Amount: 0, Unit: "g"},
		{Label: "Cholesterol", Amount: 0, Unit: "mg"},
		{Label: "Sodium", Amount: 180, Unit: "mg"},
		{Label: "Total Carbohydrate", Amount: 22, Unit: "g"},
		{Label: "Dietary Fiber", Amount: 4, Unit: "g"},
		{Label: "Total Sugars", Amount: 8, Unit: "g"},
		{Label: "Added Sugars", Amount: 4, Unit: "g"},
		{Label: "Protein", Amount: 6, Unit: "g"},
		{Label: "Vitamin D", Amount: 2, Unit: "mcg"},
		{Label: "Calcium", Amount: 260, Unit: "mg"},
		{Label: "Iron", Amount: 1, Unit: "mg"},
		{Label: "Potassium", Amount: 380, Unit: "mg"},
	}
	assert.Equal(t, want, got)
}

func TestParseNoisyOCR(t *testing.T) {
	text := `*Calor1es* 120 kcal
Total F4t 4 g
Sod1um.. 150mg`

	got := Parse(text)
	assert.Len(t, got, 3)
	// The noisy labels are not recognized, but the numbers and units are
	// still extracted so the data is not silently lost.
	assert.True(t, hasNutrient(got, Nutrient{Amount: 120, Unit: "kcal"}))
	assert.True(t, hasNutrient(got, Nutrient{Amount: 4, Unit: "g"}))
	assert.True(t, hasNutrient(got, Nutrient{Amount: 150, Unit: "mg"}))
}

func TestParseUnknownNutrient(t *testing.T) {
	text := "Biotin 30mcg"
	got := Parse(text)
	assert.Equal(t, []Nutrient{{Label: "Biotin", Amount: 30, Unit: "mcg"}}, got)
}

func TestParsePercentageIgnored(t *testing.T) {
	text := "Vitamin D 2mcg 10%"
	got := Parse(text)
	assert.Equal(t, []Nutrient{{Label: "Vitamin D", Amount: 2, Unit: "mcg"}}, got)
}

func TestParseDropsLinesWithoutNumber(t *testing.T) {
	text := "Nutrition Facts\nNot a nutrient\n"
	assert.Empty(t, Parse(text))
}

func TestParseDropsLinesWithoutRecognizedUnit(t *testing.T) {
	text := "Serving Size 1 cup (240ml)"
	assert.Empty(t, Parse(text))
}

func hasNutrient(nutrients []Nutrient, want Nutrient) bool {
	for _, n := range nutrients {
		if n.Amount == want.Amount && n.Unit == want.Unit {
			return true
		}
	}
	return false
}
