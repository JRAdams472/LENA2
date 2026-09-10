package ocrimport

import (
	"testing"

	"github.com/JRAdams472/LENA2/internal/platform/bffclient"
	"github.com/stretchr/testify/assert"
)

func sampleCatalog() *CatalogSnapshot {
	return NewCatalogSnapshot(&bffclient.Catalog{
		Units: []bffclient.Unit{
			{ID: "1", Name: "tablespoon", Abbreviation: "tbsp"},
			{ID: "2", Name: "teaspoon", Abbreviation: "tsp"},
			{ID: "3", Name: "cup", Abbreviation: "c"},
			{ID: "4", Name: "ounce", Abbreviation: "oz"},
			{ID: "5", Name: "each", Abbreviation: "ea"},
		},
		Items: []bffclient.Item{
			{ID: "10", Name: "All-Purpose Flour", Unit: "cup", Category: bffclient.Category{ID: "1", Name: "Baking"}},
			{ID: "11", Name: "Unsalted Butter", Unit: "tablespoon", Category: bffclient.Category{ID: "2", Name: "Dairy"}},
		},
		Ingredients: []bffclient.Ingredient{
			{ID: "20", Name: "Sugar", DefaultUnit: "cup"},
		},
	})
}

func TestResolveUnit(t *testing.T) {
	s := sampleCatalog()

	t.Run("tablespoons plural", func(t *testing.T) {
		u, ok := s.ResolveUnit("tablespoons")
		assert.True(t, ok)
		assert.Equal(t, "tablespoon", u.Name)
	})

	t.Run("abbreviation tbsp", func(t *testing.T) {
		u, ok := s.ResolveUnit("tbsp")
		assert.True(t, ok)
		assert.Equal(t, "tablespoon", u.Name)
	})

	t.Run("empty unit defaults to each", func(t *testing.T) {
		u, ok := s.ResolveUnit("")
		assert.True(t, ok)
		assert.Equal(t, "each", u.Name)
	})

	t.Run("unknown unit", func(t *testing.T) {
		_, ok := s.ResolveUnit("gallons")
		assert.False(t, ok)
	})
}

func TestMatchItem(t *testing.T) {
	s := sampleCatalog()

	t.Run("exact item match", func(t *testing.T) {
		m := s.MatchItem("all-purpose flour", 0.92, 0.75)
		assert.Equal(t, "accepted", m.Status)
		assert.Equal(t, "10", m.ItemID)
		assert.InDelta(t, 1.0, m.Confidence, 0.01)
	})

	t.Run("exact ingredient match", func(t *testing.T) {
		m := s.MatchItem("sugar", 0.92, 0.75)
		assert.Equal(t, "suggested", m.Status)
		assert.Contains(t, m.Notes, "generic ingredient")
	})

	t.Run("fuzzy accept", func(t *testing.T) {
		m := s.MatchItem("unsalted butterr", 0.92, 0.75)
		assert.Equal(t, "accepted", m.Status)
		assert.Equal(t, "11", m.ItemID)
	})

	t.Run("fuzzy suggest", func(t *testing.T) {
		m := s.MatchItem("butter", 0.92, 0.75)
		assert.Equal(t, "suggested", m.Status)
		assert.GreaterOrEqual(t, m.Confidence, 0.75)
		assert.Len(t, m.Suggestions, 1)
	})

	t.Run("unmatched", func(t *testing.T) {
		m := s.MatchItem("xylophone", 0.92, 0.75)
		assert.Equal(t, "unmatched", m.Status)
	})
}

func TestMapDraftItem(t *testing.T) {
	s := sampleCatalog()
	unit := "tbsp"
	q := 2.0
	m := s.MapDraftItem(DraftItem{
		Quantity:   &q,
		Unit:       &unit,
		Ingredient: "unsalted butterr",
	}, 0.92, 0.75)

	assert.Equal(t, "accepted", m.Status)
	assert.Equal(t, "11", m.ItemID)
	assert.Equal(t, "tablespoon", m.Unit)
}
