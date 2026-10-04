package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// sugAllergenSrc is the allergen registry + existing flags. Ingredient 50
// already carries a milk flag so the validator must drop re-proposals.
type sugAllergenSrc struct{}

func (sugAllergenSrc) ListAllergens(context.Context) ([]inventory.Allergen, error) {
	return []inventory.Allergen{
		{AllergenID: 1, Name: "milk", IsActive: true},
		{AllergenID: 2, Name: "wheat", IsActive: true},
		{AllergenID: 3, Name: "peanuts", IsActive: false}, // inactive — not offered
	}, nil
}

func (sugAllergenSrc) ListIngredientAllergensByIngredients(context.Context, []int64) (map[int64][]inventory.EntityAllergen, error) {
	return map[int64][]inventory.EntityAllergen{
		50: {{AllergenID: 1, Kind: "contains"}},
	}, nil
}

func (sugAllergenSrc) ListItemAllergensByItems(context.Context, []int64) (map[int64][]inventory.EntityAllergen, error) {
	return map[int64][]inventory.EntityAllergen{}, nil
}

// sugAllergenRecipes overrides the shared stub with real recipe lines:
// item 100 (resolves to ingredient 50) and direct ingredient 51.
type sugAllergenRecipes struct{ sugRecipes }

func (sugAllergenRecipes) GetRecipesByIDs(_ context.Context, _ []int64) ([]recipe.Recipe, error) {
	return []recipe.Recipe{{RecipeID: 10, Name: "Enchilada Casserole"}}, nil
}

func (sugAllergenRecipes) ListRecipeItemsByRecipes(context.Context, []int64) ([]recipe.RecipeItem, error) {
	item, ing := int64(100), int64(51)
	return []recipe.RecipeItem{
		{RecipeID: 10, ItemID: &item, Quantity: 1},
		{RecipeID: 10, IngredientID: &ing, Quantity: 2},
	}, nil
}

// sugAllergenNamer resolves item 100 to ingredient 50 (household override
// path) and names things like the shared stub.
type sugAllergenNamer struct{ sugNamer }

func (sugAllergenNamer) ResolveItemIngredients(_ context.Context, _ int64, ids []int64) (map[int64]*int64, error) {
	g := int64(50)
	out := make(map[int64]*int64, len(ids))
	for _, id := range ids {
		out[id] = nil
	}
	out[100] = &g
	return out, nil
}

func allergenService(t *testing.T, p llm.Provider) *Service {
	t.Helper()
	reg := tools.New()
	tools.RegisterAllergenTools(reg, sugAllergenRecipes{}, sugAllergenNamer{}, sugAllergenSrc{})
	return NewService(p, reg, Config{})
}

func enqueueFlagProposals(p *llm.MockProvider, flags ...AllergenFlagProposal) {
	b, _ := json.Marshal(allergenFlagResponse{Flags: flags})
	p.EnqueueText(string(b))
}

func TestSuggestAllergens_Happy(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueFlagProposals(p, AllergenFlagProposal{
		TargetKind: "ingredient", TargetID: 51, AllergenID: 2, Kind: "contains", Reason: "flour contains wheat",
	})
	svc := allergenService(t, p)
	got, err := svc.SuggestAllergens(context.Background(), 7, 9, 10, 8)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ingredient", got[0].TargetKind)
	assert.Equal(t, int64(51), got[0].TargetID)
	assert.Equal(t, int64(2), got[0].AllergenID)
	assert.Equal(t, "contains", got[0].Kind)
	assert.True(t, p.Requests[0].JSONMode)
	assert.Contains(t, p.Requests[0].Messages[1].Content, "Enchilada Casserole")
}

func TestSuggestAllergens_DropsAlreadyFlagged(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueFlagProposals(p,
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 50, AllergenID: 1, Kind: "contains", Reason: "dup"},
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 50, AllergenID: 2, Kind: "may_contain", Reason: "new flag"},
	)
	svc := allergenService(t, p)
	got, err := svc.SuggestAllergens(context.Background(), 7, 9, 10, 8)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].AllergenID)
	assert.Equal(t, "may_contain", got[0].Kind)
}

func TestSuggestAllergens_FiltersInvalid(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueFlagProposals(p,
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 51, AllergenID: 99, Kind: "contains", Reason: "unknown allergen"},
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 51, AllergenID: 3, Kind: "contains", Reason: "inactive allergen"},
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 999, AllergenID: 2, Kind: "contains", Reason: "unknown target"},
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 51, AllergenID: 2, Kind: "definitely", Reason: "bad kind"},
		AllergenFlagProposal{TargetKind: "bogus", TargetID: 51, AllergenID: 2, Kind: "contains", Reason: "bad target kind"},
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 51, AllergenID: 2, Kind: "contains", Reason: "good"},
		AllergenFlagProposal{TargetKind: "ingredient", TargetID: 51, AllergenID: 2, Kind: "may_contain", Reason: "dup pair"},
	)
	svc := allergenService(t, p)
	got, err := svc.SuggestAllergens(context.Background(), 7, 9, 10, 8)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "good", got[0].Reason)
}

func TestSuggestAllergens_NoItemsSkipsModel(t *testing.T) {
	p := llm.NewMockProvider()
	reg := tools.New()
	// emptyRecipes returns a recipe with zero flaggable lines.
	tools.RegisterAllergenTools(reg, emptyRecipes{}, sugAllergenNamer{}, sugAllergenSrc{})
	svc := NewService(p, reg, Config{})
	got, err := svc.SuggestAllergens(context.Background(), 7, 9, 10, 8)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, 0, p.CallCount())
}

// emptyRecipes returns a recipe with no items.
type emptyRecipes struct{ sugRecipes }

func (emptyRecipes) GetRecipesByIDs(_ context.Context, _ []int64) ([]recipe.Recipe, error) {
	return []recipe.Recipe{{RecipeID: 10, Name: "Empty"}}, nil
}
