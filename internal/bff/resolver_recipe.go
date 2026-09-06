package bff

import (
	"context"
	"errors"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/graph-gophers/graphql-go"
	"github.com/jackc/pgx/v5"
	"strconv"
)

// Recipe resolves a single recipe by ID.
func (r *Resolver) Recipe(ctx context.Context, args struct{ ID graphql.ID }) (*recipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	rec, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec}, nil
}

// Recipes resolves a paginated list of active recipes.
func (r *Resolver) Recipes(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*recipePageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	recipes, err := r.RecipeService.ListRecipes(ctx, true, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.RecipeService.CountRecipes(ctx, true)
	if err != nil {
		return nil, err
	}
	return &recipePageResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipes: recipes, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// parseRecipeChildren converts GraphQL recipe items/steps into service
// types so the service can persist them inside one transaction.
func parseRecipeChildren(items []recipeItemInput, steps []recipeStepInput) ([]recipe.RecipeItem, []recipe.RecipeStep, error) {
	outItems := make([]recipe.RecipeItem, 0, len(items))
	for _, ri := range items {
		itemID, err := parseID(string(ri.ItemID))
		if err != nil {
			return nil, nil, err
		}
		outItems = append(outItems, recipe.RecipeItem{
			ItemID:     itemID,
			Quantity:   ri.Quantity,
			Unit:       ri.Unit,
			Notes:      derefString(ri.Notes),
			IsOptional: boolValue(ri.IsOptional),
		})
	}
	outSteps := make([]recipe.RecipeStep, 0, len(steps))
	for _, rs := range steps {
		outSteps = append(outSteps, recipe.RecipeStep{
			StepNumber:  rs.StepNumber,
			Instruction: rs.Instruction,
		})
	}
	return outItems, outSteps, nil
}

// CreateRecipe creates a new recipe and its items/steps atomically.
func (r *Resolver) CreateRecipe(ctx context.Context, args struct{ Input createRecipeInput }) (*recipeResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	items, steps, err := parseRecipeChildren(args.Input.Items, args.Input.Steps)
	if err != nil {
		return nil, err
	}
	rec, err := r.RecipeService.CreateRecipeWithChildren(ctx, recipe.Recipe{
		Name:            args.Input.Name,
		Description:     derefString(args.Input.Description),
		Servings:        args.Input.Servings,
		PrepTimeMinutes: args.Input.PrepTimeMinutes,
		CookTimeMinutes: args.Input.CookTimeMinutes,
		IsActive:        true,
	}, items, steps, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec}, nil
}

// UpdateRecipe modifies an existing recipe and replaces its items/steps
// atomically. Omitted scalar fields keep their current values.
func (r *Resolver) UpdateRecipe(ctx context.Context, args struct {
	ID    graphql.ID
	Input createRecipeInput
}) (*recipeResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != "" {
		name = args.Input.Name
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	servings := existing.Servings
	if args.Input.Servings != nil {
		servings = args.Input.Servings
	}
	prep := existing.PrepTimeMinutes
	if args.Input.PrepTimeMinutes != nil {
		prep = args.Input.PrepTimeMinutes
	}
	cook := existing.CookTimeMinutes
	if args.Input.CookTimeMinutes != nil {
		cook = args.Input.CookTimeMinutes
	}
	items, steps, err := parseRecipeChildren(args.Input.Items, args.Input.Steps)
	if err != nil {
		return nil, err
	}
	if err := r.RecipeService.UpdateRecipeWithChildren(ctx, id, recipe.Recipe{
		Name:            name,
		Description:     description,
		Servings:        servings,
		PrepTimeMinutes: prep,
		CookTimeMinutes: cook,
		IsActive:        existing.IsActive,
	}, items, steps, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: updated}, nil
}

// DeleteRecipe removes a recipe.
func (r *Resolver) DeleteRecipe(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.RecipeService.DeleteRecipe(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// SetRecipeFavorite toggles the current user's favorite flag for a recipe.
func (r *Resolver) SetRecipeFavorite(ctx context.Context, args struct {
	RecipeID   graphql.ID
	IsFavorite bool
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.RecipeID))
	if err != nil {
		return false, err
	}
	if _, err := r.UserPrefsService.SetRecipeFavorite(ctx, u.UserID, id, args.IsFavorite, u.Email); err != nil {
		return false, err
	}
	return args.IsFavorite, nil
}

// recipeResolver resolves Recipe fields.
type recipeResolver struct {
	inv    InventoryService
	rec    RecipeService
	up     UserPrefsService
	user   currentuser.User
	recipe recipe.Recipe
}

func (r *recipeResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.recipe.RecipeID, 10)) }

func (r *recipeResolver) Name() string { return r.recipe.Name }

func (r *recipeResolver) Description() *string { return nilIfEmpty(r.recipe.Description) }

func (r *recipeResolver) Servings() *int32 { return r.recipe.Servings }

func (r *recipeResolver) PrepTimeMinutes() *int32 { return r.recipe.PrepTimeMinutes }

func (r *recipeResolver) CookTimeMinutes() *int32 { return r.recipe.CookTimeMinutes }

func (r *recipeResolver) Items(ctx context.Context) ([]*recipeItemResolver, error) {
	items, err := r.rec.ListRecipeItems(ctx, r.recipe.RecipeID)
	if err != nil {
		return nil, err
	}
	out := make([]*recipeItemResolver, len(items))
	for i := range items {
		out[i] = &recipeItemResolver{inv: r.inv, item: items[i]}
	}
	return out, nil
}

func (r *recipeResolver) Steps(ctx context.Context) ([]*recipeStepResolver, error) {
	steps, err := r.rec.ListRecipeSteps(ctx, r.recipe.RecipeID)
	if err != nil {
		return nil, err
	}
	out := make([]*recipeStepResolver, len(steps))
	for i := range steps {
		out[i] = &recipeStepResolver{step: steps[i]}
	}
	return out, nil
}

func (r *recipeResolver) IsFavorite(ctx context.Context) (bool, error) {
	fav, err := r.up.GetRecipeFavorite(ctx, r.user.UserID, r.recipe.RecipeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return fav.IsFavorite, nil
}

type recipeItemResolver struct {
	inv  InventoryService
	item recipe.RecipeItem
}

func (r *recipeItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	it, err := r.inv.GetItemByID(ctx, r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

func (r *recipeItemResolver) Quantity() float64 { return r.item.Quantity }

func (r *recipeItemResolver) Unit() string { return r.item.Unit }

func (r *recipeItemResolver) Notes() *string { return nilIfEmpty(r.item.Notes) }

func (r *recipeItemResolver) IsOptional() bool { return r.item.IsOptional }

type recipeStepResolver struct{ step recipe.RecipeStep }

func (r *recipeStepResolver) StepNumber() int32 { return r.step.StepNumber }

func (r *recipeStepResolver) Instruction() string { return r.step.Instruction }

type recipePageResolver struct {
	inv      InventoryService
	rec      RecipeService
	up       UserPrefsService
	user     currentuser.User
	recipes  []recipe.Recipe
	page     int32
	pageSize int32
	total    int32
}

func (r *recipePageResolver) Items() []*recipeResolver {
	out := make([]*recipeResolver, len(r.recipes))
	for i := range r.recipes {
		out[i] = &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: r.recipes[i]}
	}
	return out
}

func (r *recipePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type createRecipeInput struct {
	Name            string
	Description     *string
	Servings        *int32
	PrepTimeMinutes *int32
	CookTimeMinutes *int32
	Items           []recipeItemInput
	Steps           []recipeStepInput
}

type recipeItemInput struct {
	ItemID     graphql.ID
	Quantity   float64
	Unit       string
	Notes      *string
	IsOptional *bool
}

type recipeStepInput struct {
	StepNumber  int32
	Instruction string
}
