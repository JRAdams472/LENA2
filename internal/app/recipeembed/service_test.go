package recipeembed

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

func TestText(t *testing.T) {
	r := recipe.Recipe{Name: "Chicken Soup", Description: "A cozy classic."}
	got := Text(r, []string{"chicken", "carrot", "noodles"}, []string{"Soup", "Comfort Food"})
	assert.Equal(t,
		"Chicken Soup\nA cozy classic.\nIngredients: chicken, carrot, noodles\nCategories: Soup, Comfort Food",
		got)
}

func TestTextMinimal(t *testing.T) {
	r := recipe.Recipe{Name: "Toast"}
	assert.Equal(t, "Toast", Text(r, nil, nil))

	withSpace := recipe.Recipe{Name: "Toast", Description: "   "}
	assert.Equal(t, "Toast", Text(withSpace, nil, nil))
}

func TestTextIngredientCap(t *testing.T) {
	names := make([]string, maxTextIngredients+10)
	for i := range names {
		names[i] = "ing"
	}
	got := Text(recipe.Recipe{Name: "Big"}, names, nil)
	// Truncated to the cap — the extra names never reach the model.
	assert.Equal(t, maxTextIngredients, 1+countSubstr(got, ","))
}

func countSubstr(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

// fakeStore is a minimal in-memory Store for Refresh/Sweep tests.
type fakeStore struct {
	recipes   map[int64]recipe.Recipe
	items     map[int64][]recipe.RecipeItem
	cats      map[int64][]recipe.Category
	saved     map[int64]string
	models    map[int64]string
	failIDs   map[int64]bool
	calls     int
	listCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		recipes: map[int64]recipe.Recipe{},
		items:   map[int64][]recipe.RecipeItem{},
		cats:    map[int64][]recipe.Category{},
		saved:   map[int64]string{},
		models:  map[int64]string{},
		failIDs: map[int64]bool{},
	}
}

func (f *fakeStore) GetRecipeByID(_ context.Context, id int64) (recipe.Recipe, error) {
	r, ok := f.recipes[id]
	if !ok {
		return recipe.Recipe{}, errors.New("not found")
	}
	return r, nil
}

func (f *fakeStore) ListRecipeItemsByRecipes(_ context.Context, ids []int64) ([]recipe.RecipeItem, error) {
	var out []recipe.RecipeItem
	for _, id := range ids {
		out = append(out, f.items[id]...)
	}
	return out, nil
}

func (f *fakeStore) ListCategoriesForRecipes(_ context.Context, ids []int64) (map[int64][]recipe.Category, error) {
	out := map[int64][]recipe.Category{}
	for _, id := range ids {
		out[id] = f.cats[id]
	}
	return out, nil
}

func (f *fakeStore) SetRecipeEmbedding(_ context.Context, id int64, embedding, model string) error {
	f.calls++
	if f.failIDs[id] {
		return errors.New("store failed")
	}
	f.saved[id] = embedding
	f.models[id] = model
	return nil
}

func (f *fakeStore) ListEmbeddingCandidates(_ context.Context, model string, limit int32) ([]int64, error) {
	f.listCalls++
	out := []int64{}
	for id := range f.recipes {
		if m, ok := f.models[id]; ok && m == model {
			continue
		}
		out = append(out, id)
		if int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

type fakeNamer struct {
	items       map[int64]string
	ingredients map[int64]string
}

func (n *fakeNamer) GetIngredientsByIDs(_ context.Context, ids []int64) ([]inventory.Ingredient, error) {
	var out []inventory.Ingredient
	for _, id := range ids {
		if name, ok := n.ingredients[id]; ok {
			out = append(out, inventory.Ingredient{IngredientID: id, Name: name})
		}
	}
	return out, nil
}

func (n *fakeNamer) GetItemsByIDs(_ context.Context, ids []int64) ([]inventory.Item, error) {
	var out []inventory.Item
	for _, id := range ids {
		if name, ok := n.items[id]; ok {
			out = append(out, inventory.Item{ItemID: id, Name: name})
		}
	}
	return out, nil
}

func i64(v int64) *int64 { return &v }

func TestRefreshStoresEmbedding(t *testing.T) {
	store := newFakeStore()
	store.recipes[7] = recipe.Recipe{RecipeID: 7, Name: "Chicken Soup", Description: "cozy"}
	store.items[7] = []recipe.RecipeItem{
		{RecipeID: 7, ItemID: 100, IngredientID: i64(50)},
		{RecipeID: 7, ItemID: 101},
	}
	store.cats[7] = []recipe.Category{{Name: "Soup"}}
	namer := &fakeNamer{
		items:       map[int64]string{101: "egg noodles"},
		ingredients: map[int64]string{50: "chicken"},
	}

	s := NewService(llm.NewMockEmbedder(), "nomic-embed-text", store, namer, WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, s.Refresh(context.Background(), 7))

	lit, ok := store.saved[7]
	require.True(t, ok, "embedding stored")
	vec, err := llm.ParseVectorLiteral(lit)
	require.NoError(t, err)
	assert.Len(t, vec, llm.EmbedDims)
	assert.Equal(t, "nomic-embed-text", store.models[7])
}

func TestSweepDrainsAndStopsOnFailure(t *testing.T) {
	store := newFakeStore()
	for i := int64(1); i <= 5; i++ {
		store.recipes[i] = recipe.Recipe{RecipeID: i, Name: "R"}
	}
	s := NewService(llm.NewMockEmbedder(), "m", store, &fakeNamer{},
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithBatchGap(0), WithBatchSize(2))

	n, err := s.Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Len(t, store.saved, 5)
}

func TestSweepStopsWhenNoProgress(t *testing.T) {
	store := newFakeStore()
	store.recipes[1] = recipe.Recipe{RecipeID: 1, Name: "R"}
	store.failIDs[1] = true
	s := NewService(llm.NewMockEmbedder(), "m", store, &fakeNamer{},
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithBatchGap(0), WithBatchSize(10))

	n, err := s.Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	// Terminated instead of spinning on the permanently-failing row.
	assert.LessOrEqual(t, store.listCalls, 2)
}

func TestRunStopsOnCancel(t *testing.T) {
	store := newFakeStore()
	store.recipes[1] = recipe.Recipe{RecipeID: 1, Name: "R"}
	s := NewService(llm.NewMockEmbedder(), "m", store, &fakeNamer{},
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithBatchGap(0), WithInterval(time.Hour))
	s.Start()
	s.Stop()
	// No deadlock: Stop cancels the sweep loop.
}
