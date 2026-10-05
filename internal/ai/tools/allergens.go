package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// AllergenSource is the allergen surface the suggestion tools need.
// *inventory.Service satisfies it.
type AllergenSource interface {
	ListAllergens(ctx context.Context) ([]inventory.Allergen, error)
	ListIngredientAllergensByIngredients(ctx context.Context, ingredientIDs []int64) (map[int64][]inventory.EntityAllergen, error)
	ListItemAllergensByItems(ctx context.Context, itemIDs []int64) (map[int64][]inventory.EntityAllergen, error)
}

// AllergenRegistryRow is one registry entry as the model sees it.
type AllergenRegistryRow struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// AllergenFlagRow is an already-curated flag on a recipe line's target.
type AllergenFlagRow struct {
	AllergenID int64  `json:"allergenId"`
	Kind       string `json:"kind"`
}

// AllergenCandidateItem is one recipe line with every id the model can
// flag plus the flags already curated on it.
type AllergenCandidateItem struct {
	ItemID       *int64            `json:"itemId,omitempty"`
	IngredientID *int64            `json:"ingredientId,omitempty"`
	Name         string            `json:"name"`
	Quantity     float64           `json:"quantity,omitempty"`
	Optional     bool              `json:"optional,omitempty"`
	Flags        []AllergenFlagRow `json:"flags,omitempty"`
}

// AllergenCandidateRecipe is one recipe's line-level flag context.
type AllergenCandidateRecipe struct {
	ID    int64                   `json:"id"`
	Name  string                  `json:"name"`
	Items []AllergenCandidateItem `json:"items"`
}

// AllergenCandidatesOut is the full suggestion context — registry plus
// per-line targets and existing flags.
type AllergenCandidatesOut struct {
	Allergens []AllergenRegistryRow     `json:"allergens"`
	Recipes   []AllergenCandidateRecipe `json:"recipes"`
}

func keysOf(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// RegisterAllergenTools wires get_recipe_allergen_candidates — the
// read-only context the allergen-flag suggester assembles before calling
// the model.
func RegisterAllergenTools(reg *Registry, recipes RecipeCatalog, items ItemNamer, allergens AllergenSource) {
	reg.Register(llm.ToolSpec{
		Name:        "get_recipe_allergen_candidates",
		Description: "Get the allergen registry plus each recipe's ingredient lines with flaggable target ids and already-curated flags — the candidate set for allergen flag suggestions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"recipeIds": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer"},
					"description": "Recipe IDs to load (max 10)",
				},
			},
			"required": []string{"recipeIds"},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		var a struct {
			RecipeIDs []int64 `json:"recipeIds"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_recipe_allergen_candidates args: %w", err)
			}
		}
		if len(a.RecipeIDs) == 0 {
			return nil, fmt.Errorf("get_recipe_allergen_candidates: recipeIds is required")
		}
		if len(a.RecipeIDs) > 10 {
			a.RecipeIDs = a.RecipeIDs[:10]
		}
		return allergenCandidates(ctx, scope, recipes, items, allergens, a.RecipeIDs)
	})
}

func allergenCandidates(ctx context.Context, scope Scope, recipes RecipeCatalog, items ItemNamer, allergens AllergenSource, recipeIDs []int64) (*AllergenCandidatesOut, error) {
	reg, err := allergens.ListAllergens(ctx)
	if err != nil {
		return nil, fmt.Errorf("allergen registry: %w", err)
	}
	registry := make([]AllergenRegistryRow, 0, len(reg))
	for _, a := range reg {
		if !a.IsActive {
			continue
		}
		registry = append(registry, AllergenRegistryRow{ID: a.AllergenID, Name: a.Name, Description: a.Description})
	}

	list, err := recipes.GetRecipesByIDs(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("get recipes: %w", err)
	}
	rItems, err := recipes.ListRecipeItemsByRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("recipe items: %w", err)
	}

	lk, err := allergenLookups(ctx, scope.HouseholdID, items, allergens, rItems)
	if err != nil {
		return nil, err
	}

	byRecipe := map[int64][]AllergenCandidateItem{}
	for _, ri := range rItems {
		c := candidateItem(ri, lk.names, lk.ingNames, lk.resolved, lk.ingFlags, lk.itemFlags)
		if c.Name == "" {
			continue
		}
		byRecipe[ri.RecipeID] = append(byRecipe[ri.RecipeID], c)
	}

	out := &AllergenCandidatesOut{Allergens: registry}
	for _, r := range list {
		items := byRecipe[r.RecipeID]
		if items == nil {
			items = []AllergenCandidateItem{}
		}
		out.Recipes = append(out.Recipes, AllergenCandidateRecipe{ID: r.RecipeID, Name: r.Name, Items: items})
	}
	return out, nil
}

// candidateLookups bundles every index candidateItem consults.
type candidateLookups struct {
	names     map[int64]string
	ingNames  map[int64]string
	resolved  map[int64]*int64
	ingFlags  map[int64][]inventory.EntityAllergen
	itemFlags map[int64][]inventory.EntityAllergen
}

// allergenLookups loads item names, resolved household ingredient
// overrides, ingredient names, and stored flags for the recipe lines.
func allergenLookups(ctx context.Context, householdID int64, items ItemNamer, allergens AllergenSource, rItems []recipe.RecipeItem) (*candidateLookups, error) {
	itemIDs, ingIDs := map[int64]bool{}, map[int64]bool{}
	for _, ri := range rItems {
		if ri.ItemID != nil {
			itemIDs[*ri.ItemID] = true
		}
		if ri.IngredientID != nil {
			ingIDs[*ri.IngredientID] = true
		}
	}
	iids := keysOf(itemIDs)

	itList, err := items.GetItemsByIDs(ctx, iids)
	if err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	lk := &candidateLookups{
		names:    map[int64]string{},
		ingNames: map[int64]string{},
	}
	for _, it := range itList {
		lk.names[it.ItemID] = it.Name
	}
	// Household override wins over the catalog link — the resolved
	// ingredient is what flags actually attach to at warning time.
	lk.resolved, err = items.ResolveItemIngredients(ctx, householdID, iids)
	if err != nil {
		return nil, fmt.Errorf("resolve item ingredients: %w", err)
	}
	for _, g := range lk.resolved {
		if g != nil {
			ingIDs[*g] = true
		}
	}
	gids := keysOf(ingIDs)

	ingList, err := items.GetIngredientsByIDs(ctx, gids)
	if err != nil {
		return nil, fmt.Errorf("ingredients: %w", err)
	}
	for _, g := range ingList {
		lk.ingNames[g.IngredientID] = g.Name
	}

	lk.ingFlags, err = allergens.ListIngredientAllergensByIngredients(ctx, gids)
	if err != nil {
		return nil, fmt.Errorf("ingredient flags: %w", err)
	}
	lk.itemFlags, err = allergens.ListItemAllergensByItems(ctx, iids)
	if err != nil {
		return nil, fmt.Errorf("item flags: %w", err)
	}
	return lk, nil
}

// allergenFlagRows converts stored entity flags into output rows.
func allergenFlagRows(m map[int64][]inventory.EntityAllergen, id int64) []AllergenFlagRow {
	fs := m[id]
	out := make([]AllergenFlagRow, len(fs))
	for i, f := range fs {
		out[i] = AllergenFlagRow{AllergenID: f.AllergenID, Kind: f.Kind}
	}
	return out
}

// candidateItem converts one recipe line into a flaggable candidate,
// preferring the generic-ingredient identity and layering resolved-item
// flags onto item rows.
func candidateItem(ri recipe.RecipeItem, names, ingNames map[int64]string, resolved map[int64]*int64, ingFlags, itemFlags map[int64][]inventory.EntityAllergen) AllergenCandidateItem {
	c := AllergenCandidateItem{Quantity: ri.Quantity, Optional: ri.IsOptional}
	if ri.IngredientID != nil {
		g := *ri.IngredientID
		c.IngredientID = &g
		c.Name = ingNames[g]
		c.Flags = allergenFlagRows(ingFlags, g)
		return c
	}
	if ri.ItemID == nil {
		return c
	}
	it := *ri.ItemID
	c.ItemID = &it
	c.Name = names[it]
	c.Flags = allergenFlagRows(itemFlags, it)
	if g := resolved[it]; g != nil {
		c.IngredientID = g
		if c.Name == "" {
			c.Name = names[it]
		}
		c.Flags = append(c.Flags, allergenFlagRows(ingFlags, *g)...)
	}
	return c
}
