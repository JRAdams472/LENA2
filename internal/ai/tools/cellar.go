package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

// CellarReader is the household-bottle surface the sommelier tools need.
type CellarReader interface {
	ListHouseholdBottles(ctx context.Context, householdID int64, limit, offset int32) ([]userprefs.HouseholdBottle, error)
}

// BottleLookup resolves bottle catalog rows and type names.
type BottleLookup interface {
	GetBottlesByIDs(ctx context.Context, bottleIDs []int64) ([]wine.Bottle, error)
	ListTypes(ctx context.Context) ([]wine.Type, error)
}

// CellarBottleRow is one bottle in the household cellar.
type CellarBottleRow struct {
	BottleID    int64    `json:"bottleId"`
	Vineyard    string   `json:"vineyard"`
	VintageYear int32    `json:"vintageYear,omitempty"`
	Type        string   `json:"type,omitempty"`
	Abv         *float64 `json:"abv,omitempty"`
	Quantity    int32    `json:"quantity"`
	Location    string   `json:"location,omitempty"`
}

// CocktailRow is one Cocktail-category recipe with its ingredient list so
// the model can judge what the household can make.
type CocktailRow struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Ingredients []string `json:"ingredients,omitempty"`
}

// RegisterCellarTools wires get_wine_cellar and list_cocktail_recipes.
// The cellar is household-scoped; cocktails are catalog recipes filtered
// to the seeded Dish Type "Cocktail" category.
func RegisterCellarTools(reg *Registry, cellar CellarReader, bottles BottleLookup, recipes RecipeCatalog, items ItemNamer) {
	reg.Register(llm.ToolSpec{
		Name:        "get_wine_cellar",
		Description: "List the household's wine cellar bottles with vineyard, vintage, type, and quantity.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum rows to return (default 60, max 200)",
				},
			},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		var a struct {
			Limit int `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_wine_cellar args: %w", err)
			}
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 60
		}
		if limit > 200 {
			limit = 200
		}
		if scope.HouseholdID == 0 {
			return []CellarBottleRow{}, nil
		}
		return wineCellarRows(ctx, cellar, bottles, scope.HouseholdID, int32(limit))
	})

	reg.Register(llm.ToolSpec{
		Name:        "list_cocktail_recipes",
		Description: "List catalog recipes tagged with the Cocktail dish type, including ingredient names.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum rows to return (default 60, max 200)",
				},
			},
		},
	}, func(ctx context.Context, _ Scope, args json.RawMessage) (any, error) {
		var a struct {
			Limit int `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("list_cocktail_recipes args: %w", err)
			}
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 60
		}
		if limit > 200 {
			limit = 200
		}
		return cocktailRows(ctx, recipes, items, int32(limit))
	})
}

func wineCellarRows(ctx context.Context, cellar CellarReader, bottles BottleLookup, householdID int64, limit int32) ([]CellarBottleRow, error) {
	hb, err := cellar.ListHouseholdBottles(ctx, householdID, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("list cellar bottles: %w", err)
	}
	if len(hb) == 0 {
		return []CellarBottleRow{}, nil
	}
	ids := make([]int64, len(hb))
	for i, b := range hb {
		ids[i] = b.BottleID
	}
	bottleByID := map[int64]wine.Bottle{}
	if bottles != nil {
		list, err := bottles.GetBottlesByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("resolve bottles: %w", err)
		}
		for _, b := range list {
			bottleByID[b.BottleID] = b
		}
	}
	typeNames := map[int64]string{}
	if bottles != nil {
		types, err := bottles.ListTypes(ctx)
		if err == nil {
			for _, t := range types {
				typeNames[t.TypeID] = t.Name
			}
		}
	}
	out := make([]CellarBottleRow, 0, len(hb))
	for _, b := range hb {
		row := CellarBottleRow{
			BottleID: b.BottleID,
			Quantity: b.Quantity,
			Location: b.Location,
		}
		if bottle, ok := bottleByID[b.BottleID]; ok {
			row.Vineyard = bottle.Vineyard
			row.VintageYear = bottle.VintageYear
			row.Abv = bottle.Abv
			row.Type = typeNames[bottle.TypeID]
		}
		out = append(out, row)
	}
	return out, nil
}

func cocktailRows(ctx context.Context, recipes RecipeCatalog, items ItemNamer, limit int32) ([]CocktailRow, error) {
	list, err := recipes.ListRecipes(ctx, true, 200, 0)
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	ids := make([]int64, len(list))
	for i, r := range list {
		ids[i] = r.RecipeID
	}
	catsByRecipe, err := recipes.ListCategoriesForRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list recipe categories: %w", err)
	}
	cocktailIDs := cocktailIDsFor(list, catsByRecipe, int(limit))
	if len(cocktailIDs) == 0 {
		return []CocktailRow{}, nil
	}
	nameByID := map[int64]string{}
	for _, r := range list {
		nameByID[r.RecipeID] = r.Name
	}
	recipeItems, err := recipes.ListRecipeItemsByRecipes(ctx, cocktailIDs)
	if err != nil {
		return nil, fmt.Errorf("list cocktail ingredients: %w", err)
	}
	itemNames, ingNames := cocktailNameMaps(ctx, items, recipeItems)
	ingByRecipe := cocktailIngredientNames(recipeItems, itemNames, ingNames)
	out := make([]CocktailRow, 0, len(cocktailIDs))
	for _, id := range cocktailIDs {
		out = append(out, CocktailRow{ID: id, Name: nameByID[id], Ingredients: ingByRecipe[id]})
	}
	return out, nil
}

// cocktailIDsFor picks recipes categorized "Dish Type"/"Cocktail",
// capped at limit.
func cocktailIDsFor(list []recipe.Recipe, catsByRecipe map[int64][]recipe.Category, limit int) []int64 {
	var ids []int64
	for _, r := range list {
		for _, c := range catsByRecipe[r.RecipeID] {
			if c.GroupName == "Dish Type" && c.Name == "Cocktail" {
				ids = append(ids, r.RecipeID)
				break
			}
		}
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

// cocktailNameMaps resolves item and generic-ingredient display names for
// cocktail recipe lines. Lookup failures degrade to unnamed rows rather
// than aborting the tool call.
func cocktailNameMaps(ctx context.Context, items ItemNamer, recipeItems []recipe.RecipeItem) (map[int64]string, map[int64]string) {
	itemIDs := map[int64]bool{}
	ingIDs := map[int64]bool{}
	for _, ri := range recipeItems {
		if ri.ItemID != nil {
			itemIDs[*ri.ItemID] = true
		}
		if ri.IngredientID != nil {
			ingIDs[*ri.IngredientID] = true
		}
	}
	itemNames := map[int64]string{}
	ingNames := map[int64]string{}
	if items == nil {
		return itemNames, ingNames
	}
	if len(itemIDs) > 0 {
		if list, err := items.GetItemsByIDs(ctx, keysOf(itemIDs)); err == nil {
			for _, it := range list {
				itemNames[it.ItemID] = it.Name
			}
		}
	}
	if len(ingIDs) > 0 {
		if list, err := items.GetIngredientsByIDs(ctx, keysOf(ingIDs)); err == nil {
			for _, g := range list {
				ingNames[g.IngredientID] = g.Name
			}
		}
	}
	return itemNames, ingNames
}

// cocktailIngredientNames groups resolved ingredient names by recipe,
// preferring the generic-ingredient name over the branded item name.
func cocktailIngredientNames(recipeItems []recipe.RecipeItem, itemNames, ingNames map[int64]string) map[int64][]string {
	byRecipe := map[int64][]string{}
	for _, ri := range recipeItems {
		var n string
		if ri.IngredientID != nil {
			n = ingNames[*ri.IngredientID]
		}
		if n == "" && ri.ItemID != nil {
			n = itemNames[*ri.ItemID]
		}
		if n != "" {
			byRecipe[ri.RecipeID] = append(byRecipe[ri.RecipeID], n)
		}
	}
	return byRecipe
}
