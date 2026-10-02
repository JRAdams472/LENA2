package ocrimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFillMissingQuantity(t *testing.T) {
	tests := []struct {
		name        string
		ingredient  string
		wantQty     *float64
		wantUnit    string
		wantIng     string
		wantNotes   string
		wantNoSplit bool
	}{
		{
			name:       "range takes lower value and notes the range",
			ingredient: "2-3 dried ancho chiles",
			wantQty:    fp(2),
			wantIng:    "dried ancho chiles",
			wantNotes:  "range 2-3",
		},
		{
			name:       "simple quantity and unit",
			ingredient: "1 cup boiling water",
			wantQty:    fp(1),
			wantUnit:   "cup",
			wantIng:    "boiling water",
		},
		{
			name:       "package size moves to notes",
			ingredient: "1 28-oz can whole tomatoes",
			wantQty:    fp(1),
			wantUnit:   "can",
			wantIng:    "whole tomatoes",
			wantNotes:  "28-oz",
		},
		{
			name:       "mixed fraction",
			ingredient: "3 1/2 tsp kosher salt",
			wantQty:    fp(3.5),
			wantUnit:   "tsp",
			wantIng:    "kosher salt",
		},
		{
			name:       "bare fraction",
			ingredient: "1/2 cup sugar",
			wantQty:    fp(0.5),
			wantUnit:   "cup",
			wantIng:    "sugar",
		},
		{
			name:       "unicode fraction",
			ingredient: "½ cup sugar",
			wantQty:    fp(0.5),
			wantUnit:   "cup",
			wantIng:    "sugar",
		},
		{
			name:       "unicode mixed fraction",
			ingredient: "1½ cups flour",
			wantQty:    fp(1.5),
			wantUnit:   "cups",
			wantIng:    "flour",
		},
		{
			name:       "decimal quantity",
			ingredient: "2.5 lbs ground beef",
			wantQty:    fp(2.5),
			wantUnit:   "lbs",
			wantIng:    "ground beef",
		},
		{
			name:       "en dash range",
			ingredient: "4 – 6 sprigs thyme",
			wantQty:    fp(4),
			wantUnit:   "sprigs",
			wantIng:    "thyme",
			wantNotes:  "range 4-6",
		},
		{
			name:       "of connector dropped",
			ingredient: "1 can of tomatoes",
			wantQty:    fp(1),
			wantUnit:   "can",
			wantIng:    "tomatoes",
		},
		{
			name:       "word quantity before unit",
			ingredient: "one onion, diced",
			wantQty:    fp(1),
			wantIng:    "onion, diced",
		},
		{
			name:        "word quantity without unit is not split",
			ingredient:  "a few grinds of pepper",
			wantNoSplit: true,
		},
		{
			name:       "quantity without unit",
			ingredient: "2 ripe avocados",
			wantQty:    fp(2),
			wantIng:    "ripe avocados",
		},
		{
			name:        "no leading quantity untouched",
			ingredient:  "kosher salt to taste",
			wantNoSplit: true,
		},
		{
			name:        "mid-string quantity untouched",
			ingredient:  "peanut butter, 2 generous scoops",
			wantNoSplit: true,
		},
		{
			name:       "fluid ounce two-word unit",
			ingredient: "8 fl oz milk",
			wantQty:    fp(8),
			wantUnit:   "fl oz",
			wantIng:    "milk",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := DraftItem{Ingredient: tc.ingredient}
			FillMissingQuantity(&d)
			if tc.wantNoSplit {
				assert.Nil(t, d.Quantity)
				assert.Equal(t, tc.ingredient, d.Ingredient)
				return
			}
			require.NotNil(t, d.Quantity)
			assert.InDelta(t, *tc.wantQty, *d.Quantity, 0.001)
			assert.Equal(t, tc.wantIng, d.Ingredient)
			if tc.wantUnit == "" {
				assert.Nil(t, d.Unit)
			} else {
				require.NotNil(t, d.Unit)
				assert.Equal(t, tc.wantUnit, *d.Unit)
			}
			if tc.wantNotes != "" {
				require.NotNil(t, d.Notes)
				assert.Contains(t, *d.Notes, tc.wantNotes)
			}
		})
	}
}

func TestFillMissingQuantity_ExistingModelValues(t *testing.T) {
	t.Run("keeps model quantity", func(t *testing.T) {
		d := DraftItem{Ingredient: "2 cups flour", Quantity: fp(3)}
		FillMissingQuantity(&d)
		assert.InDelta(t, 3, *d.Quantity, 0.001)
		assert.Equal(t, "2 cups flour", d.Ingredient)
	})

	t.Run("keeps model unit, fills quantity", func(t *testing.T) {
		d := DraftItem{Ingredient: "2 ancho chiles", Unit: strPtr("each")}
		FillMissingQuantity(&d)
		assert.InDelta(t, 2, *d.Quantity, 0.001)
		assert.Equal(t, "each", *d.Unit)
		assert.Equal(t, "ancho chiles", d.Ingredient)
	})

	t.Run("appends extras to existing notes", func(t *testing.T) {
		d := DraftItem{Ingredient: "2-3 chiles", Notes: strPtr("stemmed")}
		FillMissingQuantity(&d)
		assert.InDelta(t, 2, *d.Quantity, 0.001)
		assert.Equal(t, "stemmed; range 2-3", *d.Notes)
	})
}

func fp(f float64) *float64 { return &f }
