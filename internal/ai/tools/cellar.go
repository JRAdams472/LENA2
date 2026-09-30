package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/platform/llm"
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
	var cocktailIDs []int64
	for _, r := range list {
		for _, c := range catsByRecipe[r.RecipeID] {
			if c.GroupName == "Dish Type" && c.Name == "Cocktail" {
				cocktailIDs = append(cocktailIDs, r.RecipeID)
				break
			}
		}
	}
	if len(cocktailIDs) == 0 {
		return []CocktailRow{}, nil
	}
	if len(cocktailIDs) > int(limit) {
		cocktailIDs = cocktailIDs[:limit]
	}
	nameByID := map[int64]string{}
	for _, r := range list {
		nameByID[r.RecipeID] = r.Name
	}
	recipeItems, err := recipes.ListRecipeItemsByRecipes(ctx, cocktailIDs)
	if err != nil {
		return nil, fmt.Errorf("list cocktail ingredients: %w", err)
	}
	itemIDs := map[int64]bool{}
	for _, ri := range recipeItems {
		itemIDs[ri.ItemID] = true
	}
	itemNames := map[int64]string{}
	if items != nil && len(itemIDs) > 0 {
		idList := make([]int64, 0, len(itemIDs))
		for id := range itemIDs {
			idList = append(idList, id)
		}
		if list, err := items.GetItemsByIDs(ctx, idList); err == nil {
			for _, it := range list {
				itemNames[it.ItemID] = it.Name
			}
		}
	}
	ingByRecipe := map[int64][]string{}
	for _, ri := range recipeItems {
		if n := itemNames[ri.ItemID]; n != "" {
			ingByRecipe[ri.RecipeID] = append(ingByRecipe[ri.RecipeID], n)
		}
	}
	out := make([]CocktailRow, 0, len(cocktailIDs))
	for _, id := range cocktailIDs {
		out = append(out, CocktailRow{ID: id, Name: nameByID[id], Ingredients: ingByRecipe[id]})
	}
	return out, nil
}
