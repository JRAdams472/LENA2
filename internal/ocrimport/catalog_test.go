package ocrimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testUnit struct {
	id           string
	name         string
	abbreviation string
}

func (u testUnit) ID() string           { return u.id }
func (u testUnit) Name() string         { return u.name }
func (u testUnit) Abbreviation() string { return u.abbreviation }

type testItem struct {
	id   string
	name string
}

func (i testItem) ID() string   { return i.id }
func (i testItem) Name() string { return i.name }

type testIngredient struct {
	id   string
	name string
}

func (i testIngredient) ID() string   { return i.id }
func (i testIngredient) Name() string { return i.name }

func sampleCatalog() *CatalogSnapshot {
	return NewCatalogSnapshot(&StaticCatalog{
		UnitsField: []CatalogUnit{
			testUnit{id: "1", name: "tablespoon", abbreviation: "tbsp"},
			testUnit{id: "2", name: "teaspoon", abbreviation: "tsp"},
			testUnit{id: "3", name: "cup", abbreviation: "c"},
			testUnit{id: "4", name: "ounce", abbreviation: "oz"},
			testUnit{id: "5", name: "each", abbreviation: "ea"},
		},
		ItemsField: []CatalogItem{
			testItem{id: "10", name: "All-Purpose Flour"},
			testItem{id: "11", name: "Unsalted Butter"},
		},
		IngredientsField: []CatalogIngredient{
			testIngredient{id: "20", name: "Sugar"},
		},
	})
}

func TestResolveUnit(t *testing.T) {
	s := sampleCatalog()

	t.Run("tablespoons plural", func(t *testing.T) {
		u, ok := s.ResolveUnit("tablespoons")
		assert.True(t, ok)
		assert.Equal(t, "tablespoon", u.Name())
	})

	t.Run("abbreviation tbsp", func(t *testing.T) {
		u, ok := s.ResolveUnit("tbsp")
		assert.True(t, ok)
		assert.Equal(t, "tablespoon", u.Name())
	})

	t.Run("empty unit defaults to each", func(t *testing.T) {
		u, ok := s.ResolveUnit("")
		assert.True(t, ok)
		assert.Equal(t, "each", u.Name())
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
