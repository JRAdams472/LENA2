package ocrimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMatchItem_ContentWordGate(t *testing.T) {
	s := NewCatalogSnapshot(&StaticCatalog{
		UnitsField: []CatalogUnit{
			testUnit{id: "1", name: "cup", abbreviation: "c"},
			testUnit{id: "2", name: "each"},
		},
		ItemsField: []CatalogItem{
			testItem{id: "50", name: "Strudel Apple Mini 3.2 Oz"},
			testItem{id: "51", name: "Chile Ancho"},
			testItem{id: "52", name: "Rid Step 1 Lice Killing Shampoo"},
		},
		IngredientsField: []CatalogIngredient{
			testIngredient{id: "60", name: "water"},
		},
	})

	t.Run("digit-heavy line does not surface unrelated SKUs", func(t *testing.T) {
		m := s.MatchItem("2-3 dried ancho chiles", 0.92, 0.75)
		// the real chile shares words and auto-accepts; the junk SKUs share
		// nothing and never become candidates.
		assert.Equal(t, "accepted", m.Status)
		assert.Equal(t, "51", m.ItemID)
		for _, sug := range m.Suggestions {
			assert.NotEqual(t, "50", sug.ID, "strudel shares no content word")
			assert.NotEqual(t, "52", sug.ID)
		}
	})

	t.Run("no shared content words -> unmatched", func(t *testing.T) {
		m := s.MatchItem("1 cup boiling water", 0.92, 0.75)
		for _, sug := range m.Suggestions {
			assert.NotEqual(t, "52", sug.ID)
			assert.NotEqual(t, "50", sug.ID)
		}
	})
}

func TestMatchItem_IngredientPreference(t *testing.T) {
	s := NewCatalogSnapshot(&StaticCatalog{
		UnitsField: []CatalogUnit{
			testUnit{id: "1", name: "clove"},
			testUnit{id: "2", name: "each"},
		},
		ItemsField: []CatalogItem{
			testItem{id: "70", name: "Kraft Garlic Powder"},
		},
		IngredientsField: []CatalogIngredient{
			testIngredient{id: "71", name: "garlic"},
		},
	})
	m := s.MatchItem("garlic clove", 0.92, 0.75)
	require.NotEmpty(t, m.Suggestions)
	assert.Equal(t, "ingredient", m.Suggestions[0].Kind)
	assert.Equal(t, "71", m.Suggestions[0].ID)
}
