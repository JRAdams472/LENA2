package bff

import (
	"context"
	"errors"
	"strconv"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/graph-gophers/graphql-go"
	"github.com/jackc/pgx/v5"
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
	rc := &recipeChildren{recipeCounts: make(map[int64]countPair)}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, []int64{id}, rc); err != nil {
		return nil, err
	}
	var globalCount, personalCount int64
	if p, ok := rc.recipeCounts[id]; ok {
		globalCount, personalCount = p.global, p.personal
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec, globalCount: globalCount, personalCount: personalCount}, nil
}

// ScaledRecipe resolves a recipe with its ingredient quantities scaled to the
// requested number of servings. The recipe itself is not modified.
func (r *Resolver) ScaledRecipe(ctx context.Context, args struct {
	ID       graphql.ID
	Servings int32
}) (*recipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	if args.Servings <= 0 {
		return nil, errors.New("servings must be positive")
	}

	scaled, err := r.RecipeService.ScaleRecipe(ctx, id, args.Servings)
	if err != nil {
		return nil, err
	}

	rc := &recipeChildren{
		itemsBy:      make(map[int64][]recipe.RecipeItem),
		stepsBy:      make(map[int64][]recipe.RecipeStep),
		favorites:    make(map[int64]bool),
		items:        make(map[int64]inventory.Item),
		units:        make(map[int64]inventory.Unit),
		recipeCounts: make(map[int64]countPair),
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, []int64{scaled.Recipe.RecipeID}, rc); err != nil {
		return nil, err
	}
	rc.itemsBy[scaled.Recipe.RecipeID] = scaled.Items
	rc.stepsBy[scaled.Recipe.RecipeID] = scaled.Steps
	fav, err := r.UserPrefsService.GetRecipeFavorite(ctx, u.UserID, scaled.Recipe.RecipeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	rc.favorites[scaled.Recipe.RecipeID] = fav.IsFavorite

	if len(scaled.Items) > 0 {
		itemIDSet := make(map[int64]bool)
		unitIDSet := make(map[int64]bool)
		for _, ri := range scaled.Items {
			itemIDSet[ri.ItemID] = true
			if ri.UnitID != 0 {
				unitIDSet[ri.UnitID] = true
			}
		}
		itemIDs := make([]int64, 0, len(itemIDSet))
		for itemID := range itemIDSet {
			itemIDs = append(itemIDs, itemID)
		}
		items, err := r.InventoryService.GetItemsByIDs(ctx, itemIDs)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			rc.items[it.ItemID] = it
		}
		unitIDs := make([]int64, 0, len(unitIDSet))
		for unitID := range unitIDSet {
			unitIDs = append(unitIDs, unitID)
		}
		units, err := r.InventoryService.GetUnitsByIDs(ctx, unitIDs)
		if err != nil {
			return nil, err
		}
		for _, unit := range units {
			rc.units[unit.UnitID] = unit
		}
	}

	return &recipeResolver{
		inv:    r.InventoryService,
		rec:    r.RecipeService,
		up:     r.UserPrefsService,
		user:   u,
		recipe: scaled.Recipe,
		rc:     rc,
	}, nil
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
	recipeIDs := distinctIDs(recipes, func(rp recipe.Recipe) *int64 { return &rp.RecipeID })
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.InventoryService, u.UserID, recipeIDs, nil)
	if err != nil {
		return nil, err
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, recipeIDs, rc); err != nil {
		return nil, err
	}
	return &recipePageResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipes: recipes, rc: rc, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// parseRecipeChildren converts GraphQL recipe items/steps into service
// types so the service can persist them inside one transaction. Unit names
// are resolved to unit IDs via the shared unit catalog; unknown units are
// rejected.
func parseRecipeChildren(ctx context.Context, inv InventoryService, items []recipeItemInput, steps []recipeStepInput) ([]recipe.RecipeItem, []recipe.RecipeStep, error) {
	outItems := make([]recipe.RecipeItem, 0, len(items))
	for _, ri := range items {
		itemID, err := parseID(string(ri.ItemID))
		if err != nil {
			return nil, nil, err
		}
		ingredientID, err := optionalID(ri.IngredientID)
		if err != nil {
			return nil, nil, err
		}
		unitID, err := resolveUnitID(ctx, inv, ri.Unit)
		if err != nil {
			return nil, nil, err
		}
		outItems = append(outItems, recipe.RecipeItem{
			ItemID:       itemID,
			IngredientID: ingredientID,
			Quantity:     ri.Quantity,
			UnitID:       unitID,
			SectionName:  derefString(ri.Section),
			DisplayOrder: int32Value(ri.DisplayOrder),
			Notes:        derefString(ri.Notes),
			IsOptional:   boolValue(ri.IsOptional),
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
	items, steps, err := parseRecipeChildren(ctx, r.InventoryService, args.Input.Items, args.Input.Steps)
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
	recordEventAsync(r.AnalyticsService, u.UserID, u.Email, analytics.Event{
		EventType:  analytics.EventRecipeCreated,
		EntityType: analytics.EntityRecipe,
		EntityID:   rec.RecipeID,
	})
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
	items, steps, err := parseRecipeChildren(ctx, r.InventoryService, args.Input.Items, args.Input.Steps)
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

// recipeResolver resolves Recipe fields. When rc is non-nil its
// batch-loaded maps are used instead of per-recipe service calls.
type recipeResolver struct {
	inv           InventoryService
	rec           RecipeService
	up            UserPrefsService
	user          currentuser.User
	recipe        recipe.Recipe
	rc            *recipeChildren
	globalCount   int64
	personalCount int64
}

func (r *recipeResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.recipe.RecipeID, 10)) }

func (r *recipeResolver) Name() string { return r.recipe.Name }

func (r *recipeResolver) Description() *string { return nilIfEmpty(r.recipe.Description) }

func (r *recipeResolver) Servings() *int32 { return r.recipe.Servings }

func (r *recipeResolver) PrepTimeMinutes() *int32 { return r.recipe.PrepTimeMinutes }

func (r *recipeResolver) CookTimeMinutes() *int32 { return r.recipe.CookTimeMinutes }

func (r *recipeResolver) Items(ctx context.Context) ([]*recipeItemResolver, error) {
	var items []recipe.RecipeItem
	var itemsByID map[int64]inventory.Item
	var ch *itemChildren
	if r.rc != nil {
		items = r.rc.itemsBy[r.recipe.RecipeID]
		itemsByID = r.rc.items
		ch = r.rc.itemChildren
	} else {
		var err error
		items, err = r.rec.ListRecipeItems(ctx, r.recipe.RecipeID)
		if err != nil {
			return nil, err
		}
	}
	var units map[int64]inventory.Unit
	if r.rc != nil {
		units = r.rc.units
	}
	out := make([]*recipeItemResolver, len(items))
	for i := range items {
		out[i] = &recipeItemResolver{inv: r.inv, item: items[i], items: itemsByID, ch: ch, units: units}
	}
	return out, nil
}

// ItemSections groups the recipe's items by section name in display order.
// Items without a section land in a group with a null name.
func (r *recipeResolver) ItemSections(ctx context.Context) ([]*recipeItemSectionResolver, error) {
	var items []recipe.RecipeItem
	var itemsByID map[int64]inventory.Item
	var ch *itemChildren
	var units map[int64]inventory.Unit
	if r.rc != nil {
		items = r.rc.itemsBy[r.recipe.RecipeID]
		itemsByID = r.rc.items
		ch = r.rc.itemChildren
		units = r.rc.units
	} else {
		var err error
		items, err = r.rec.ListRecipeItems(ctx, r.recipe.RecipeID)
		if err != nil {
			return nil, err
		}
	}
	// Items arrive ordered by display_order; group by first-seen section.
	var sections []*recipeItemSectionResolver
	byName := make(map[string]*recipeItemSectionResolver)
	for _, ri := range items {
		sec, ok := byName[ri.SectionName]
		if !ok {
			sec = &recipeItemSectionResolver{name: ri.SectionName}
			byName[ri.SectionName] = sec
			sections = append(sections, sec)
		}
		sec.items = append(sec.items, &recipeItemResolver{inv: r.inv, item: ri, items: itemsByID, ch: ch, units: units})
	}
	return sections, nil
}

func (r *recipeResolver) Steps(ctx context.Context) ([]*recipeStepResolver, error) {
	var steps []recipe.RecipeStep
	if r.rc != nil {
		steps = r.rc.stepsBy[r.recipe.RecipeID]
	} else {
		var err error
		steps, err = r.rec.ListRecipeSteps(ctx, r.recipe.RecipeID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*recipeStepResolver, len(steps))
	for i := range steps {
		out[i] = &recipeStepResolver{step: steps[i]}
	}
	return out, nil
}

func (r *recipeResolver) IsFavorite(ctx context.Context) (bool, error) {
	if r.rc != nil {
		return r.rc.favorites[r.recipe.RecipeID], nil
	}
	fav, err := r.up.GetRecipeFavorite(ctx, r.user.UserID, r.recipe.RecipeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return fav.IsFavorite, nil
}

func (r *recipeResolver) SelectionCount() int32 {
	if r.rc != nil {
		if p, ok := r.rc.recipeCounts[r.recipe.RecipeID]; ok {
			return int64ToInt32(p.global)
		}
	}
	return int64ToInt32(r.globalCount)
}

func (r *recipeResolver) PersonalSelectionCount() int32 {
	if r.rc != nil {
		if p, ok := r.rc.recipeCounts[r.recipe.RecipeID]; ok {
			return int64ToInt32(p.personal)
		}
	}
	return int64ToInt32(r.personalCount)
}

type recipeItemResolver struct {
	inv   InventoryService
	item  recipe.RecipeItem
	items map[int64]inventory.Item
	ch    *itemChildren
	units map[int64]inventory.Unit
}

func (r *recipeItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.RecipeItemID, 10))
}

func (r *recipeItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.items != nil {
		it, ok := r.items[r.item.ItemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch}, nil
	}
	it, err := r.inv.GetItemByID(ctx, r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

// Ingredient resolves the brand-agnostic ingredient linked to this recipe
// item, when set. Scaffolding only — nothing populates ingredient_id yet.
func (r *recipeItemResolver) Ingredient(ctx context.Context) (*ingredientResolver, error) {
	if r.item.IngredientID == nil {
		return nil, nil
	}
	in, err := r.inv.GetIngredientByID(ctx, *r.item.IngredientID)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.inv, in: in}, nil
}

func (r *recipeItemResolver) Quantity() float64 { return r.item.Quantity }

func (r *recipeItemResolver) Unit(ctx context.Context) (string, error) {
	return unitName(ctx, r.inv, r.units, r.item.UnitID)
}

func (r *recipeItemResolver) Section() *string { return nilIfEmpty(r.item.SectionName) }

func (r *recipeItemResolver) DisplayOrder() int32 { return r.item.DisplayOrder }

func (r *recipeItemResolver) Notes() *string { return nilIfEmpty(r.item.Notes) }

func (r *recipeItemResolver) IsOptional() bool { return r.item.IsOptional }

// recipeItemSectionResolver resolves a named group of recipe items.
type recipeItemSectionResolver struct {
	name  string
	items []*recipeItemResolver
}

func (r *recipeItemSectionResolver) Name() *string { return nilIfEmpty(r.name) }

func (r *recipeItemSectionResolver) Items() []*recipeItemResolver { return r.items }

type recipeStepResolver struct{ step recipe.RecipeStep }

func (r *recipeStepResolver) StepNumber() int32 { return r.step.StepNumber }

func (r *recipeStepResolver) Instruction() string { return r.step.Instruction }

type recipePageResolver struct {
	inv      InventoryService
	rec      RecipeService
	up       UserPrefsService
	user     currentuser.User
	recipes  []recipe.Recipe
	rc       *recipeChildren
	page     int32
	pageSize int32
	total    int32
}

func (r *recipePageResolver) Items() []*recipeResolver {
	out := make([]*recipeResolver, len(r.recipes))
	for i := range r.recipes {
		out[i] = &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: r.recipes[i], rc: r.rc}
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
	ItemID       graphql.ID
	IngredientID *graphql.ID
	Quantity     float64
	Unit         string
	Section      *string
	DisplayOrder *int32
	Notes        *string
	IsOptional   *bool
}

type recipeStepInput struct {
	StepNumber  int32
	Instruction string
}
