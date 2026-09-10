package ocrimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeName(t *testing.T) {
	assert.Equal(t, "all purpose flour", NormalizeName("  All-Purpose Flour!  "))
	assert.Equal(t, "finely chopped onion", NormalizeName("Finely Chopped Onions..."))
}

func TestSortedWords(t *testing.T) {
	assert.Equal(t, "all flour purpose", SortedWords("All Purpose Flour"))
}

func TestSimilarity(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		assert.Equal(t, 1.0, Similarity("all purpose flour", "all purpose flour"))
	})

	t.Run("reordered words match", func(t *testing.T) {
		score := Similarity("flour all purpose", "all purpose flour")
		assert.InDelta(t, 1.0, score, 0.01)
	})

	t.Run("typo is similar", func(t *testing.T) {
		score := Similarity("all purpouse flour", "all purpose flour")
		assert.Greater(t, score, 0.9)
	})

	t.Run("different strings are low", func(t *testing.T) {
		score := Similarity("chicken broth", "all purpose flour")
		assert.Less(t, score, 0.6)
	})
}
