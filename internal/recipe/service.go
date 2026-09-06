// Package recipe owns the catalog of recipes: recipe definitions, their
// item lists and preparation steps. Per-user favorites live in userprefs.
package recipe

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/recipe/sqlc"
)

// Service provides catalog operations for the recipe domain.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
}

// NewService creates a recipe Service using the given connection pool.
func NewService(pool dbtx.Pool) *Service {
	return &Service{q: sqlc.New(pool), pool: pool}
}

// withTx runs fn against a sqlc.Querier bound to a single transaction on
// the domain's pool, committing on success and rolling back on error.
func (s *Service) withTx(ctx context.Context, fn func(q sqlc.Querier) error) error {
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(sqlc.New(tx))
	})
}

// Recipe is a catalog recipe definition. Nullable numeric fields are
// pointers so a real 0 is distinguishable from "not set".
type Recipe struct {
	RecipeID        int64
	Name            string
	Description     string
	Servings        *int32
	PrepTimeMinutes *int32
	CookTimeMinutes *int32
	IsActive        bool
}

// CreateRecipe adds a new recipe.
func (s *Service) CreateRecipe(ctx context.Context, arg Recipe, by string) (Recipe, error) {
	return createRecipe(ctx, s.q, arg, by)
}

func createRecipe(ctx context.Context, q sqlc.Querier, arg Recipe, by string) (Recipe, error) {
	row, err := q.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Name:            arg.Name,
		Description:     textOrNull(arg.Description),
		Servings:        optInt4(arg.Servings),
		PrepTimeMinutes: optInt4(arg.PrepTimeMinutes),
		CookTimeMinutes: optInt4(arg.CookTimeMinutes),
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

// CountRecipes returns the total number of recipes with the given active flag.
func (s *Service) CountRecipes(ctx context.Context, active bool) (int64, error) {
	n, err := s.q.CountRecipes(ctx, active)
	if err != nil {
		return 0, fmt.Errorf("count recipes: %w", err)
	}
	return n, nil
}

// GetRecipesByIDs returns a set of recipes in a single query.
func (s *Service) GetRecipesByIDs(ctx context.Context, recipeIDs []int64) ([]Recipe, error) {
	rows, err := s.q.GetRecipesByIDs(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("get recipes by ids: %w", err)
	}
	out := make([]Recipe, len(rows))
	for i := range rows {
		out[i] = toRecipe(rows[i])
	}
	return out, nil
}

// UpdateRecipe modifies an existing recipe.
func (s *Service) UpdateRecipe(ctx context.Context, recipeID int64, arg Recipe, by string) error {
	return updateRecipeRow(ctx, s.q, recipeID, arg, by)
}

func updateRecipeRow(ctx context.Context, q sqlc.Querier, recipeID int64, arg Recipe, by string) error {
	return q.UpdateRecipe(ctx, sqlc.UpdateRecipeParams{
		RecipeID:        recipeID,
		Name:            arg.Name,
		Description:     textOrNull(arg.Description),
		Servings:        optInt4(arg.Servings),
		PrepTimeMinutes: optInt4(arg.PrepTimeMinutes),
		CookTimeMinutes: optInt4(arg.CookTimeMinutes),
		IsActive:        arg.IsActive,
		UpdatedBy:       textOrNull(by),
	})
}

// DeleteRecipe removes a recipe and its related items/steps.
func (s *Service) DeleteRecipe(ctx context.Context, recipeID int64) error {
	return s.q.DeleteRecipe(ctx, recipeID)
}

// CreateRecipeWithChildren creates a recipe plus its items and steps in a
// single transaction; any failure rolls back the whole write.
func (s *Service) CreateRecipeWithChildren(ctx context.Context, arg Recipe, items []RecipeItem, steps []RecipeStep, by string) (Recipe, error) {
	var rec Recipe
	err := s.withTx(ctx, func(q sqlc.Querier) error {
		var err error
		rec, err = createRecipe(ctx, q, arg, by)
		if err != nil {
			return err
		}
		for _, item := range items {
			item.RecipeID = rec.RecipeID
			if err := addRecipeItem(ctx, q, item); err != nil {
				return err
			}
		}
		for _, step := range steps {
			if _, err := addRecipeStep(ctx, q, rec.RecipeID, step.StepNumber, step.Instruction, by); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Recipe{}, err
	}
	return rec, nil
}

// UpdateRecipeWithChildren updates a recipe row and replaces its items and
// steps in a single transaction; any failure leaves the prior state intact.
func (s *Service) UpdateRecipeWithChildren(ctx context.Context, recipeID int64, arg Recipe, items []RecipeItem, steps []RecipeStep, by string) error {
	return s.withTx(ctx, func(q sqlc.Querier) error {
		if err := updateRecipeRow(ctx, q, recipeID, arg, by); err != nil {
			return fmt.Errorf("update recipe: %w", err)
		}
		if err := q.DeleteRecipeItems(ctx, recipeID); err != nil {
			return fmt.Errorf("replace recipe items: %w", err)
		}
		if err := q.DeleteRecipeSteps(ctx, recipeID); err != nil {
			return fmt.Errorf("replace recipe steps: %w", err)
		}
		for _, item := range items {
			item.RecipeID = recipeID
			if err := addRecipeItem(ctx, q, item); err != nil {
				return err
			}
		}
		for _, step := range steps {
			if _, err := addRecipeStep(ctx, q, recipeID, step.StepNumber, step.Instruction, by); err != nil {
				return err
			}
		}
		return nil
	})
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
	return addRecipeItem(ctx, s.q, arg)
}

func addRecipeItem(ctx context.Context, q sqlc.Querier, arg RecipeItem) error {
	qty, err := numericFromFloat64(arg.Quantity)
	if err != nil {
		return fmt.Errorf("add recipe item: %w", err)
	}
	return q.AddRecipeItem(ctx, sqlc.AddRecipeItemParams{
		RecipeID:   arg.RecipeID,
		ItemID:     arg.ItemID,
		Quantity:   qty,
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
	return toRecipeItems(rows)
}

// ListRecipeItemsByRecipes returns all items for a set of recipes in one query.
func (s *Service) ListRecipeItemsByRecipes(ctx context.Context, recipeIDs []int64) ([]RecipeItem, error) {
	rows, err := s.q.ListRecipeItemsByRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("list recipe items by recipes: %w", err)
	}
	return toRecipeItems(rows)
}

func toRecipeItems(rows []sqlc.RecipeRecipeItem) ([]RecipeItem, error) {
	out := make([]RecipeItem, len(rows))
	for i := range rows {
		ri, err := toRecipeItem(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = ri
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
	return addRecipeStep(ctx, s.q, recipeID, stepNumber, instruction, by)
}

func addRecipeStep(ctx context.Context, q sqlc.Querier, recipeID int64, stepNumber int32, instruction, by string) (RecipeStep, error) {
	row, err := q.AddRecipeStep(ctx, sqlc.AddRecipeStepParams{
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

// ListRecipeStepsByRecipes returns all steps for a set of recipes in one query.
func (s *Service) ListRecipeStepsByRecipes(ctx context.Context, recipeIDs []int64) ([]RecipeStep, error) {
	rows, err := s.q.ListRecipeStepsByRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("list recipe steps by recipes: %w", err)
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
		v := row.Servings.Int32
		r.Servings = &v
	}
	if row.PrepTimeMinutes.Valid {
		v := row.PrepTimeMinutes.Int32
		r.PrepTimeMinutes = &v
	}
	if row.CookTimeMinutes.Valid {
		v := row.CookTimeMinutes.Int32
		r.CookTimeMinutes = &v
	}
	return r
}

func toRecipeItem(row sqlc.RecipeRecipeItem) (RecipeItem, error) {
	ri := RecipeItem{
		RecipeID:   row.RecipeID,
		ItemID:     row.ItemID,
		Unit:       row.Unit,
		Notes:      row.Notes.String,
		IsOptional: row.IsOptional,
	}
	if row.Quantity.Valid {
		v, err := row.Quantity.Float64Value()
		if err != nil {
			return RecipeItem{}, fmt.Errorf("recipe %d item %d quantity: %w", row.RecipeID, row.ItemID, err)
		}
		ri.Quantity = v.Float64
	}
	return ri, nil
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

func optInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func numericFromFloat64(f float64) (pgtype.Numeric, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: value is not finite", f)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: %w", f, err)
	}
	n.Valid = true
	return n, nil
}
