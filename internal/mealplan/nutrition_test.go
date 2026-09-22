package mealplan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f64(v float64) *float64 { return &v }

func TestAggregateNutrition(t *testing.T) {
	units := map[int64]NutritionUnit{
		1: {Name: "gram", Kind: "weight", ToBaseFactor: f64(1)},
		2: {Name: "kilogram", Kind: "weight", ToBaseFactor: f64(1000)},
		3: {Name: "cup", Kind: "volume", ToBaseFactor: f64(236.588)},
		4: {Name: "each", Kind: "count"},
	}
	protein := NutrientBasis{NutrientID: 1, Name: "Protein", Unit: "g", Amount: 10, BasisQuantity: 100, BasisUnitID: 1}

	t.Run("same unit", func(t *testing.T) {
		totals, warnings := AggregateNutrition(
			[]NutritionLine{{ItemID: 1, Quantity: 250, UnitID: 1}},
			map[int64][]NutrientBasis{1: {protein}},
			nil, units,
		)
		require.Empty(t, warnings)
		require.Len(t, totals, 1)
		assert.InDelta(t, 25.0, totals[0].Amount, 0.0001) // 10 per 100 g × 250 g
	})

	t.Run("same-kind conversion", func(t *testing.T) {
		totals, warnings := AggregateNutrition(
			[]NutritionLine{{ItemID: 1, Quantity: 0.5, UnitID: 2}}, // 0.5 kg = 500 g
			map[int64][]NutrientBasis{1: {protein}},
			nil, units,
		)
		require.Empty(t, warnings)
		require.Len(t, totals, 1)
		assert.InDelta(t, 50.0, totals[0].Amount, 0.0001)
	})

	t.Run("count via net weight", func(t *testing.T) {
		totals, warnings := AggregateNutrition(
			[]NutritionLine{{ItemID: 2, Quantity: 2, UnitID: 4}}, // 2 each × 120 g
			map[int64][]NutrientBasis{2: {protein}},
			map[int64]NutritionItem{2: {UnitID: 1, NetWeight: f64(120)}},
			units,
		)
		require.Empty(t, warnings)
		require.Len(t, totals, 1)
		assert.InDelta(t, 24.0, totals[0].Amount, 0.0001)
	})

	t.Run("count without net weight warns", func(t *testing.T) {
		totals, warnings := AggregateNutrition(
			[]NutritionLine{{ItemID: 2, Quantity: 2, UnitID: 4}},
			map[int64][]NutrientBasis{2: {protein}},
			map[int64]NutritionItem{2: {UnitID: 1}},
			units,
		)
		assert.Empty(t, totals)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "net weight")
	})

	t.Run("incompatible kinds warn", func(t *testing.T) {
		totals, warnings := AggregateNutrition(
			[]NutritionLine{{ItemID: 3, Quantity: 1, UnitID: 3}}, // volume vs weight basis
			map[int64][]NutrientBasis{3: {protein}},
			nil, units,
		)
		assert.Empty(t, totals)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "cannot convert")
	})

	t.Run("aggregates across lines and sorts", func(t *testing.T) {
		nutrients := map[int64][]NutrientBasis{
			1: {
				protein,
				{NutrientID: 2, Name: "Carbs", Unit: "g", Amount: 20, BasisQuantity: 100, BasisUnitID: 1},
			},
		}
		totals, warnings := AggregateNutrition(
			[]NutritionLine{
				{ItemID: 1, Quantity: 100, UnitID: 1},
				{ItemID: 1, Quantity: 1, UnitID: 2}, // 1 kg
			},
			nutrients, nil, units,
		)
		require.Empty(t, warnings)
		require.Len(t, totals, 2)
		assert.Equal(t, "Carbs", totals[0].Name)
		assert.InDelta(t, 220.0, totals[0].Amount, 0.0001)
		assert.Equal(t, "Protein", totals[1].Name)
		assert.InDelta(t, 110.0, totals[1].Amount, 0.0001)
	})
}
