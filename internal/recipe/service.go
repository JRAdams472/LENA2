// Package recipe owns the catalog of recipes: recipe definitions, their
// item lists and preparation steps. Per-user favorites live in userprefs.
package recipe

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JRAdams472/LENA2/internal/recipe/sqlc"
)

// Service provides catalog operations for the recipe domain.
type Service struct {
	q sqlc.Querier
}

// NewService creates a recipe Service using the given connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

// Recipe is a catalog recipe definition.
type Recipe struct {
	RecipeID        int64
	Name            string
	Description     string
	Servings        int32
	PrepTimeMinutes int32
	CookTimeMinutes int32
	IsActive        bool
}

// CreateRecipe adds a new recipe.
func (s *Service) CreateRecipe(ctx context.Context, arg Recipe, by string) (Recipe, error) {
	row, err := s.q.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Name:            arg.Name,
		Description:     textOrNull(arg.Description),
		Servings:        int4OrNull(arg.Servings),
		PrepTimeMinutes: int4OrNull(arg.PrepTimeMinutes),
		CookTimeMinutes: int4OrNull(arg.CookTimeMinutes),
		IsActive:        arg.IsActive,
		CreatedBy:       by,
		UpdatedBy:       textOrNull(by),
	})
	if err != nil {
		return Recipe{}, fmt.Errorf("create recipe: %w", err)
	}
	return toRecipe(row), nil
}

// GetRecipeByID returns a recipe by its primary key.
func (s *Service) GetRecipeByID(ctx context.Context, recipeID int64) (Recipe, error) {
	row, err := s.q.GetRecipeByID(ctx, recipeID)
	if err != nil {
		return Recipe{}, fmt.Errorf("get recipe by id: %w", err)
	}
	return toRecipe(row), nil
}

// ListRecipes returns a paginated list of active/inactive recipes.
func (s *Service) ListRecipes(ctx context.Context, active bool, limit, offset int32) ([]Recipe, error) {
	rows, err := s.q.ListRecipes(ctx, sqlc.ListRecipesParams{IsActive: active, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	out := make([]Recipe, len(rows))
	for i := range rows {
		out[i] = toRecipe(rows[i])
	}
	return out, nil
}

// UpdateRecipe modifies an existing recipe.
func (s *Service) UpdateRecipe(ctx context.Context, recipeID int64, arg Recipe, by string) error {
	return s.q.UpdateRecipe(ctx, sqlc.UpdateRecipeParams{
		RecipeID:        recipeID,
		Name:            arg.Name,
		Description:     textOrNull(arg.Description),
		Servings:        int4OrNull(arg.Servings),
		PrepTimeMinutes: int4OrNull(arg.PrepTimeMinutes),
		CookTimeMinutes: int4OrNull(arg.CookTimeMinutes),
		IsActive:        arg.IsActive,
		UpdatedBy:       textOrNull(by),
	})
}

// DeleteRecipe removes a recipe and its related items/steps.
func (s *Service) DeleteRecipe(ctx context.Context, recipeID int64) error {
	return s.q.DeleteRecipe(ctx, recipeID)
}

// RecipeItem is one ingredient in a recipe.
type RecipeItem struct {
	RecipeID   int64
	ItemID     int64
	Quantity   float64
	Unit       string
	Notes      string
	IsOptional bool
}

// AddRecipeItem adds an item to a recipe.
func (s *Service) AddRecipeItem(ctx context.Context, arg RecipeItem) error {
	return s.q.AddRecipeItem(ctx, sqlc.AddRecipeItemParams{
		RecipeID:   arg.RecipeID,
		ItemID:     arg.ItemID,
		Quantity:   numericFromFloat64(arg.Quantity),
		Unit:       arg.Unit,
		Notes:      textOrNull(arg.Notes),
		IsOptional: arg.IsOptional,
	})
}

// ListRecipeItems returns all items for a recipe.
func (s *Service) ListRecipeItems(ctx context.Context, recipeID int64) ([]RecipeItem, error) {
	rows, err := s.q.ListRecipeItems(ctx, recipeID)
	if err != nil {
		return nil, fmt.Errorf("list recipe items: %w", err)
	}
	out := make([]RecipeItem, len(rows))
	for i := range rows {
		out[i] = toRecipeItem(rows[i])
	}
	return out, nil
}

// RemoveRecipeItem removes an item from a recipe.
func (s *Service) RemoveRecipeItem(ctx context.Context, recipeID, itemID int64) error {
	return s.q.RemoveRecipeItem(ctx, sqlc.RemoveRecipeItemParams{RecipeID: recipeID, ItemID: itemID})
}

// RecipeStep is one step in a recipe.
type RecipeStep struct {
	StepID      int64
	RecipeID    int64
	StepNumber  int32
	Instruction string
}

// AddRecipeStep adds a step to a recipe.
func (s *Service) AddRecipeStep(ctx context.Context, recipeID int64, stepNumber int32, instruction, by string) (RecipeStep, error) {
	row, err := s.q.AddRecipeStep(ctx, sqlc.AddRecipeStepParams{
		RecipeID:    recipeID,
		StepNumber:  stepNumber,
		Instruction: instruction,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return RecipeStep{}, fmt.Errorf("add recipe step: %w", err)
	}
	return toRecipeStep(row), nil
}

// ListRecipeSteps returns all steps for a recipe.
func (s *Service) ListRecipeSteps(ctx context.Context, recipeID int64) ([]RecipeStep, error) {
	rows, err := s.q.ListRecipeSteps(ctx, recipeID)
	if err != nil {
		return nil, fmt.Errorf("list recipe steps: %w", err)
	}
	out := make([]RecipeStep, len(rows))
	for i := range rows {
		out[i] = toRecipeStep(rows[i])
	}
	return out, nil
}

// UpdateRecipeStep modifies a step.
func (s *Service) UpdateRecipeStep(ctx context.Context, stepID int64, stepNumber int32, instruction, by string) error {
	return s.q.UpdateRecipeStep(ctx, sqlc.UpdateRecipeStepParams{
		StepID:      stepID,
		StepNumber:  stepNumber,
		Instruction: instruction,
		UpdatedBy:   textOrNull(by),
	})
}

// DeleteRecipeStep removes a step.
func (s *Service) DeleteRecipeStep(ctx context.Context, stepID int64) error {
	return s.q.DeleteRecipeStep(ctx, stepID)
}

func toRecipe(row sqlc.RecipeRecipe) Recipe {
	r := Recipe{
		RecipeID: row.RecipeID,
		Name:     row.Name,
		IsActive: row.IsActive,
	}
	r.Description = row.Description.String
	if row.Servings.Valid {
		r.Servings = row.Servings.Int32
	}
	if row.PrepTimeMinutes.Valid {
		r.PrepTimeMinutes = row.PrepTimeMinutes.Int32
	}
	if row.CookTimeMinutes.Valid {
		r.CookTimeMinutes = row.CookTimeMinutes.Int32
	}
	return r
}

func toRecipeItem(row sqlc.RecipeRecipeItem) RecipeItem {
	ri := RecipeItem{
		RecipeID:   row.RecipeID,
		ItemID:     row.ItemID,
		Unit:       row.Unit,
		Notes:      row.Notes.String,
		IsOptional: row.IsOptional,
	}
	if row.Quantity.Valid {
		f8, _ := row.Quantity.Float64Value()
		ri.Quantity = f8.Float64
	}
	return ri
}

func toRecipeStep(row sqlc.RecipeRecipeStep) RecipeStep {
	return RecipeStep{
		StepID:      row.StepID,
		RecipeID:    row.RecipeID,
		StepNumber:  row.StepNumber,
		Instruction: row.Instruction,
	}
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func int4OrNull(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}

func numericFromFloat64(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	n.Valid = true
	return n
}
