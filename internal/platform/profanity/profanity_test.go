package profanity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetector_Check(t *testing.T) {
	d := New("")

	t.Run("clean text", func(t *testing.T) {
		ok, terms := d.Check("This is a clean recipe.")
		assert.False(t, ok)
		assert.Empty(t, terms)
	})

	t.Run("flags profanity", func(t *testing.T) {
		ok, terms := d.Check("This recipe is damn good.")
		assert.True(t, ok)
		assert.Equal(t, []string{"damn"}, terms)
	})

	t.Run("word boundary only", func(t *testing.T) {
		ok, terms := d.Check("I need some asparagus.")
		assert.False(t, ok)
		assert.Empty(t, terms)
	})

	t.Run("extra terms", func(t *testing.T) {
		d2 := New("doofus")
		ok, terms := d2.Check("That doofus burned the sauce.")
		assert.True(t, ok)
		assert.Equal(t, []string{"doofus"}, terms)
	})

	t.Run("comma separated extra terms", func(t *testing.T) {
		d2 := New("doofus, dingus")
		ok, terms := d2.Check("You doofus, you dingus!")
		assert.True(t, ok)
		assert.ElementsMatch(t, []string{"doofus", "dingus"}, terms)
	})
}
