package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// RecipeCatalog is the recipe surface the recipe tools need.
type RecipeCatalog interface {
	RecipeLookup
	ListRecipes(ctx context.Context, active bool, limit, offset int32) ([]recipe.Recipe, error)
	ListCategoriesForRecipes(ctx context.Context, recipeIDs []int64) (map[int64][]recipe.Category, error)
	ListRecipeItemsByRecipes(ctx context.Context, recipeIDs []int64) ([]recipe.RecipeItem, error)
}

// RecipeRow is one catalog recipe as the model sees it — the candidate
// list for meal suggestions.
type RecipeRow struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Categories []string `json:"categories,omitempty"`
	Servings   *int32   `json:"servings,omitempty"`
	PrepMin    *int32   `json:"prepMinutes,omitempty"`
	CookMin    *int32   `json:"cookMinutes,omitempty"`
}

// RecipeDetailRow is a recipe with its ingredient list.
type RecipeDetailRow struct {
	RecipeRow
	Ingredients []recipeIngredientRow `json:"ingredients"`
}

type recipeIngredientRow struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit,omitempty"`
	Optional bool    `json:"optional,omitempty"`
}

// RegisterRecipeTools wires list_recipes and get_recipe_details. Recipes
// are a global catalog (not household-scoped), so these need no household
// check — but they still only run under an authenticated user's scope.
func RegisterRecipeTools(reg *Registry, recipes RecipeCatalog, items ItemNamer) {
	reg.Register(llm.ToolSpec{
		Name:        "list_recipes",
		Description: "List active recipes in the catalog with categories and times — the candidate set for meal suggestions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum rows to return (default 80, max 200)",
				},
			},
		},
	}, func(ctx context.Context, _ Scope, args json.RawMessage) (any, error) {
		var a struct {
			Limit int `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("list_recipes args: %w", err)
			}
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 80
		}
		if limit > 200 {
			limit = 200
		}
		list, err := recipes.ListRecipes(ctx, true, int32(limit), 0)
		if err != nil {
			return nil, fmt.Errorf("list recipes: %w", err)
		}
		return recipeRows(ctx, recipes, list)
	})

	reg.Register(llm.ToolSpec{
		Name:        "get_recipe_details",
		Description: "Get recipes with their ingredient lists — use to check which pantry items a recipe consumes.",
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
	}, func(ctx context.Context, _ Scope, args json.RawMessage) (any, error) {
		var a struct {
			RecipeIDs []int64 `json:"recipeIds"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_recipe_details args: %w", err)
			}
		}
		if len(a.RecipeIDs) == 0 {
			return nil, fmt.Errorf("get_recipe_details: recipeIds is required")
		}
		if len(a.RecipeIDs) > 10 {
			a.RecipeIDs = a.RecipeIDs[:10]
		}
		list, err := recipes.GetRecipesByIDs(ctx, a.RecipeIDs)
		if err != nil {
			return nil, fmt.Errorf("get recipes: %w", err)
		}
		rows, err := recipeRows(ctx, recipes, list)
		if err != nil {
			return nil, err
		}
		return attachIngredients(ctx, recipes, items, rows)
	})
}

func recipeRows(ctx context.Context, recipes RecipeCatalog, list []recipe.Recipe) ([]RecipeRow, error) {
	if len(list) == 0 {
		return []RecipeRow{}, nil
	}
	ids := make([]int64, len(list))
	for i, r := range list {
		ids[i] = r.RecipeID
	}
	cats, err := recipes.ListCategoriesForRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("recipe categories: %w", err)
	}
	out := make([]RecipeRow, len(list))
	for i, r := range list {
		row := RecipeRow{
			ID:       r.RecipeID,
			Name:     r.Name,
			Servings: r.Servings,
			PrepMin:  r.PrepTimeMinutes,
			CookMin:  r.CookTimeMinutes,
		}
		for _, c := range cats[r.RecipeID] {
			row.Categories = append(row.Categories, c.Name)
		}
		out[i] = row
	}
	return out, nil
}

// attachIngredients expands rows into RecipeDetailRow with item/unit names.
func attachIngredients(ctx context.Context, recipes RecipeCatalog, items ItemNamer, rows []RecipeRow) ([]RecipeDetailRow, error) {
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	rItems, err := recipes.ListRecipeItemsByRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("recipe items: %w", err)
	}
	if len(rItems) == 0 {
		out := make([]RecipeDetailRow, len(rows))
		for i, r := range rows {
			out[i] = RecipeDetailRow{RecipeRow: r, Ingredients: []recipeIngredientRow{}}
		}
		return out, nil
	}

	itemIDs := map[int64]bool{}
	ingIDs := map[int64]bool{}
	unitIDs := map[int64]bool{}
	for _, ri := range rItems {
		if ri.ItemID != nil {
			itemIDs[*ri.ItemID] = true
		}
		if ri.IngredientID != nil {
			ingIDs[*ri.IngredientID] = true
		}
		unitIDs[ri.UnitID] = true
	}
	iids := make([]int64, 0, len(itemIDs))
	for id := range itemIDs {
		iids = append(iids, id)
	}
	names := map[int64]string{}
	ingNames := map[int64]string{}
	if items != nil {
		list, err := items.GetItemsByIDs(ctx, iids)
		if err != nil {
			return nil, fmt.Errorf("item names: %w", err)
		}
		for _, it := range list {
			names[it.ItemID] = it.Name
		}
		gids := make([]int64, 0, len(ingIDs))
		for id := range ingIDs {
			gids = append(gids, id)
		}
		if len(gids) > 0 {
			gs, err := items.GetIngredientsByIDs(ctx, gids)
			if err != nil {
				return nil, fmt.Errorf("ingredient names: %w", err)
			}
			for _, g := range gs {
				ingNames[g.IngredientID] = g.Name
			}
		}
	}
	units := map[int64]string{}
	if len(unitIDs) > 0 {
		uids := make([]int64, 0, len(unitIDs))
		for id := range unitIDs {
			uids = append(uids, id)
		}
		list, err := items.GetUnitsByIDs(ctx, uids)
		if err != nil {
			return nil, fmt.Errorf("unit names: %w", err)
		}
		for _, u := range list {
			units[u.UnitID] = u.Abbreviation
		}
	}

	byRecipe := map[int64][]recipeIngredientRow{}
	for _, ri := range rItems {
		var name string
		if ri.IngredientID != nil {
			name = ingNames[*ri.IngredientID]
		}
		if name == "" && ri.ItemID != nil {
			var ok bool
			if name, ok = names[*ri.ItemID]; !ok {
				name = fmt.Sprintf("item #%d", *ri.ItemID)
			}
		}
		if name == "" {
			continue
		}
		byRecipe[ri.RecipeID] = append(byRecipe[ri.RecipeID], recipeIngredientRow{
			Name:     name,
			Quantity: ri.Quantity,
			Unit:     units[ri.UnitID],
			Optional: ri.IsOptional,
		})
	}
	out := make([]RecipeDetailRow, len(rows))
	for i, r := range rows {
		ing := byRecipe[r.ID]
		if ing == nil {
			ing = []recipeIngredientRow{}
		}
		out[i] = RecipeDetailRow{RecipeRow: r, Ingredients: ing}
	}
	return out, nil
}
