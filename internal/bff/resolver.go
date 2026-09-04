package bff

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

//go:embed schema.graphqls
var schema string

// Resolver is the root GraphQL resolver. It is the only package that is
// allowed to orchestrate across domain modules.
type Resolver struct {
	GroceryService   *grocery.Service
	InventoryService *inventory.Service
	MealPlanService  *mealplan.Service
	RecipeService    *recipe.Service
	UserPrefsService *userprefs.Service
	WineService      *wine.Service
}

// NewResolver returns a new BFF resolver with the domain services.
func NewResolver(gr *grocery.Service, inv *inventory.Service, mp *mealplan.Service, rec *recipe.Service, up *userprefs.Service, wineSvc *wine.Service) *Resolver {
	return &Resolver{GroceryService: gr, InventoryService: inv, MealPlanService: mp, RecipeService: rec, UserPrefsService: up, WineService: wineSvc}
}

func userFromContext(ctx context.Context) (currentuser.User, error) {
	u, ok := currentuser.FromContext(ctx)
	if !ok {
		return currentuser.User{}, errors.New("unauthorized")
	}
	return u, nil
}

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func optionalID(id *graphql.ID) (*int64, error) {
	if id == nil {
		return nil, nil
	}
	v, err := parseID(string(*id))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func int32Ptr(v int32) *int32 {
	if v == 0 {
		return nil
	}
	return &v
}

func clamp(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func timeToGraphQL(t *time.Time) *graphql.Time {
	if t == nil {
		return nil
	}
	return &graphql.Time{Time: *t}
}

func float64OrNil(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func int16OrNil(v int16) *int32 {
	if v == 0 {
		return nil
	}
	i := int32(v)
	return &i
}

// Me resolves the current authenticated user.
func (r *Resolver) Me(ctx context.Context) (*userResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &userResolver{u: u}, nil
}

// Brand resolves a single brand by ID.
func (r *Resolver) Brand(ctx context.Context, args struct{ ID graphql.ID }) (*brandResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	b, err := r.InventoryService.GetBrandByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}

// Brands resolves all catalog brands.
func (r *Resolver) Brands(ctx context.Context) ([]*brandResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	brands, err := r.InventoryService.ListBrands(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*brandResolver, len(brands))
	for i := range brands {
		out[i] = &brandResolver{b: brands[i]}
	}
	return out, nil
}

// Category resolves a single category by ID.
func (r *Resolver) Category(ctx context.Context, args struct{ ID graphql.ID }) (*categoryResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	c, err := r.InventoryService.GetCategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

// Categories resolves all catalog categories.
func (r *Resolver) Categories(ctx context.Context) ([]*categoryResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	cats, err := r.InventoryService.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*categoryResolver, len(cats))
	for i := range cats {
		out[i] = &categoryResolver{c: cats[i]}
	}
	return out, nil
}

// FlavorProfiles resolves all flavor profiles.
func (r *Resolver) FlavorProfiles(ctx context.Context) ([]*flavorProfileResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	profiles, err := r.InventoryService.ListFlavorProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*flavorProfileResolver, len(profiles))
	for i := range profiles {
		out[i] = &flavorProfileResolver{f: profiles[i]}
	}
	return out, nil
}

// NutrientTypes resolves all nutrient types.
func (r *Resolver) NutrientTypes(ctx context.Context) ([]*nutrientTypeResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	types, err := r.InventoryService.ListNutrientTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*nutrientTypeResolver, len(types))
	for i := range types {
		out[i] = &nutrientTypeResolver{n: types[i]}
	}
	return out, nil
}

// Item resolves a single catalog item by ID.
func (r *Resolver) Item(ctx context.Context, args struct{ ID graphql.ID }) (*itemResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	it, err := r.InventoryService.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.InventoryService, it: it}, nil
}

// Items resolves a paginated list of catalog items.
func (r *Resolver) Items(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*itemPageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	items, err := r.InventoryService.ListItems(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &itemPageResolver{inv: r.InventoryService, items: items, page: page, pageSize: pageSize, total: int32(len(items))}, nil
}

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
	return &recipePageResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipes: recipes, page: page, pageSize: pageSize, total: int32(len(recipes))}, nil
}

// Bottle resolves a single wine bottle by ID.
func (r *Resolver) Bottle(ctx context.Context, args struct{ ID graphql.ID }) (*bottleResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	b, err := r.WineService.GetBottleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{b: b}, nil
}

// Bottles resolves a paginated list of wine bottles.
func (r *Resolver) Bottles(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*bottlePageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	bottles, err := r.WineService.ListBottles(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &bottlePageResolver{bottles: bottles, page: page, pageSize: pageSize, total: int32(len(bottles))}, nil
}

// MealPlan resolves a single meal plan by ID.
func (r *Resolver) MealPlan(ctx context.Context, args struct{ ID graphql.ID }) (*mealPlanResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	mp, err := r.MealPlanService.GetMealPlanByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	return &mealPlanResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, plan: mp}, nil
}

// MealPlans resolves the current user's meal plans.
func (r *Resolver) MealPlans(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*mealPlanPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	plans, err := r.MealPlanService.ListMealPlans(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &mealPlanPageResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, plans: plans, page: page, pageSize: pageSize, total: int32(len(plans))}, nil
}

// Nutrition returns a nutrition summary for a meal plan.
func (r *Resolver) Nutrition(ctx context.Context, args struct{ MealPlanID graphql.ID }) ([]*nutritionResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.MealPlanID))
	if err != nil {
		return nil, err
	}
	if _, err := r.MealPlanService.GetMealPlanByID(ctx, mealPlanID, u.UserID); err != nil {
		return nil, err
	}
	slots, err := r.MealPlanService.ListMealSlotsForPlan(ctx, mealPlanID)
	if err != nil {
		return nil, err
	}

	type total struct {
		name   string
		unit   string
		amount float64
	}
	totals := make(map[int64]total)

	addNutrients := func(itemID int64, quantity float64) error {
		nutrients, err := r.InventoryService.ListFoodNutrientsByItem(ctx, itemID)
		if err != nil {
			return err
		}
		for _, n := range nutrients {
			t := totals[n.NutrientID]
			t.name = n.Name
			t.unit = n.Unit
			t.amount += n.Amount * quantity
			totals[n.NutrientID] = t
		}
		return nil
	}

	for _, slot := range slots {
		overridden := make(map[int64]bool)
		slotItems, err := r.MealPlanService.ListMealSlotItems(ctx, slot.SlotID)
		if err != nil {
			return nil, err
		}
		for _, si := range slotItems {
			if si.ItemID == nil {
				continue
			}
			if err := addNutrients(*si.ItemID, si.Quantity); err != nil {
				return nil, err
			}
			if si.IsFromRecipe {
				overridden[*si.ItemID] = true
			}
		}

		if slot.RecipeID == nil {
			continue
		}
		recipe, err := r.RecipeService.GetRecipeByID(ctx, *slot.RecipeID)
		if err != nil {
			return nil, err
		}
		if recipe.Servings <= 0 {
			continue
		}
		scale := 1.0
		if slot.Servings != nil {
			scale = float64(*slot.Servings) / float64(recipe.Servings)
		}
		recipeItems, err := r.RecipeService.ListRecipeItems(ctx, recipe.RecipeID)
		if err != nil {
			return nil, err
		}
		for _, ri := range recipeItems {
			if overridden[ri.ItemID] {
				continue
			}
			if err := addNutrients(ri.ItemID, ri.Quantity*scale); err != nil {
				return nil, err
			}
		}
	}

	out := make([]*nutritionResolver, 0, len(totals))
	for _, t := range totals {
		out = append(out, &nutritionResolver{nutrition: nutrition{Name: t.name, Unit: t.unit, Amount: t.amount}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].nutrition.Name < out[j].nutrition.Name })
	return out, nil
}

// GroceryList resolves a single grocery list by ID.
func (r *Resolver) GroceryList(ctx context.Context, args struct{ ID graphql.ID }) (*groceryListResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	list, err := r.GroceryService.GetGroceryListByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, list: list}, nil
}

// GroceryLists resolves the current user's grocery lists.
func (r *Resolver) GroceryLists(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*groceryListPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	lists, err := r.GroceryService.ListGroceryLists(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &groceryListPageResolver{g: r.GroceryService, inv: r.InventoryService, lists: lists, page: page, pageSize: pageSize, total: int32(len(lists))}, nil
}

// UserBottles resolves the current user's wine cellar.
func (r *Resolver) UserBottles(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userBottlePageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	bottles, err := r.UserPrefsService.ListUserBottles(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &userBottlePageResolver{wine: r.WineService, bottles: bottles, page: page, pageSize: pageSize, total: int32(len(bottles))}, nil
}

// UserItems resolves the current user's pantry items.
func (r *Resolver) UserItems(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userItemPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	items, err := r.UserPrefsService.ListUserItems(ctx, u.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &userItemPageResolver{inv: r.InventoryService, items: items, page: page, pageSize: pageSize, total: int32(len(items))}, nil
}

// CreateBrand adds a new brand.
func (r *Resolver) CreateBrand(ctx context.Context, args struct{ Input createBrandInput }) (*brandResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	b, err := r.InventoryService.CreateBrand(ctx, args.Input.Name)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}

// CreateCategory adds a new category.
func (r *Resolver) CreateCategory(ctx context.Context, args struct{ Input createCategoryInput }) (*categoryResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	c, err := r.InventoryService.CreateCategory(ctx, args.Input.Name, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

// CreateFlavorProfile adds a new flavor profile.
func (r *Resolver) CreateFlavorProfile(ctx context.Context, args struct{ Input createFlavorProfileInput }) (*flavorProfileResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	f, err := r.InventoryService.CreateFlavorProfile(ctx, args.Input.Name, u.Email)
	if err != nil {
		return nil, err
	}
	return &flavorProfileResolver{f: f}, nil
}

// CreateNutrientType adds a new nutrient type.
func (r *Resolver) CreateNutrientType(ctx context.Context, args struct{ Input createNutrientTypeInput }) (*nutrientTypeResolver, error) {
	_, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	n, err := r.InventoryService.CreateNutrientType(ctx, args.Input.Name, args.Input.Unit)
	if err != nil {
		return nil, err
	}
	return &nutrientTypeResolver{n: n}, nil
}

// CreateItem creates a new catalog item.
func (r *Resolver) CreateItem(ctx context.Context, args struct{ Input createItemInput }) (*itemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	brandID, err := optionalID(args.Input.BrandID)
	if err != nil {
		return nil, err
	}
	catID, err := parseID(string(args.Input.CategoryID))
	if err != nil {
		return nil, err
	}
	it, err := r.InventoryService.CreateItem(ctx, inventory.Item{
		Name:       args.Input.Name,
		BrandID:    brandID,
		Upc12:      derefString(args.Input.Upc12),
		Upc14:      derefString(args.Input.Upc14),
		CategoryID: catID,
		Unit:       args.Input.Unit,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.InventoryService, it: it}, nil
}

// CreateRecipe creates a new recipe and its items/steps.
func (r *Resolver) CreateRecipe(ctx context.Context, args struct{ Input createRecipeInput }) (*recipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := r.RecipeService.CreateRecipe(ctx, recipe.Recipe{
		Name:            args.Input.Name,
		Description:     derefString(args.Input.Description),
		Servings:        int32Value(args.Input.Servings),
		PrepTimeMinutes: int32Value(args.Input.PrepTimeMinutes),
		CookTimeMinutes: int32Value(args.Input.CookTimeMinutes),
		IsActive:        true,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	for _, ri := range args.Input.Items {
		itemID, err := parseID(string(ri.ItemID))
		if err != nil {
			return nil, err
		}
		if err := r.RecipeService.AddRecipeItem(ctx, recipe.RecipeItem{
			RecipeID:   rec.RecipeID,
			ItemID:     itemID,
			Quantity:   ri.Quantity,
			Unit:       ri.Unit,
			Notes:      derefString(ri.Notes),
			IsOptional: boolValue(ri.IsOptional),
		}); err != nil {
			return nil, err
		}
	}
	for _, rs := range args.Input.Steps {
		if _, err := r.RecipeService.AddRecipeStep(ctx, rec.RecipeID, rs.StepNumber, rs.Instruction, u.Email); err != nil {
			return nil, err
		}
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, recipe: rec}, nil
}

// UpdateItem modifies an existing catalog item.
func (r *Resolver) UpdateItem(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateItemInput
}) (*itemResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	brandID := existing.BrandID
	if args.Input.BrandID != nil {
		b, err := optionalID(args.Input.BrandID)
		if err != nil {
			return nil, err
		}
		brandID = b
	}
	categoryID := existing.CategoryID
	if args.Input.CategoryID != nil {
		c, err := parseID(string(*args.Input.CategoryID))
		if err != nil {
			return nil, err
		}
		categoryID = c
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	unit := existing.Unit
	if args.Input.Unit != nil {
		unit = *args.Input.Unit
	}
	upc12 := existing.Upc12
	if args.Input.Upc12 != nil {
		upc12 = *args.Input.Upc12
	}
	upc14 := existing.Upc14
	if args.Input.Upc14 != nil {
		upc14 = *args.Input.Upc14
	}
	u, _ := currentuser.FromContext(ctx)
	if err := r.InventoryService.UpdateItem(ctx, id, inventory.Item{
		Name:       name,
		BrandID:    brandID,
		Upc12:      upc12,
		Upc14:      upc14,
		CategoryID: categoryID,
		Unit:       unit,
	}, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.InventoryService.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.InventoryService, it: updated}, nil
}

// DeleteItem removes a catalog item.
func (r *Resolver) DeleteItem(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteItem(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateRecipe modifies an existing recipe and replaces its items/steps.
func (r *Resolver) UpdateRecipe(ctx context.Context, args struct {
	ID    graphql.ID
	Input createRecipeInput
}) (*recipeResolver, error) {
	u, err := userFromContext(ctx)
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
		servings = *args.Input.Servings
	}
	prep := existing.PrepTimeMinutes
	if args.Input.PrepTimeMinutes != nil {
		prep = *args.Input.PrepTimeMinutes
	}
	cook := existing.CookTimeMinutes
	if args.Input.CookTimeMinutes != nil {
		cook = *args.Input.CookTimeMinutes
	}
	if err := r.RecipeService.UpdateRecipe(ctx, id, recipe.Recipe{
		Name:            name,
		Description:     description,
		Servings:        servings,
		PrepTimeMinutes: prep,
		CookTimeMinutes: cook,
		IsActive:        existing.IsActive,
	}, u.Email); err != nil {
		return nil, err
	}
	// Replace items.
	existingItems, err := r.RecipeService.ListRecipeItems(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, ri := range existingItems {
		if err := r.RecipeService.RemoveRecipeItem(ctx, id, ri.ItemID); err != nil {
			return nil, err
		}
	}
	for _, ri := range args.Input.Items {
		itemID, err := parseID(string(ri.ItemID))
		if err != nil {
			return nil, err
		}
		if err := r.RecipeService.AddRecipeItem(ctx, recipe.RecipeItem{
			RecipeID:   id,
			ItemID:     itemID,
			Quantity:   ri.Quantity,
			Unit:       ri.Unit,
			Notes:      derefString(ri.Notes),
			IsOptional: boolValue(ri.IsOptional),
		}); err != nil {
			return nil, err
		}
	}
	// Replace steps.
	existingSteps, err := r.RecipeService.ListRecipeSteps(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, rs := range existingSteps {
		if err := r.RecipeService.DeleteRecipeStep(ctx, rs.StepID); err != nil {
			return nil, err
		}
	}
	for _, rs := range args.Input.Steps {
		if _, err := r.RecipeService.AddRecipeStep(ctx, id, rs.StepNumber, rs.Instruction, u.Email); err != nil {
			return nil, err
		}
	}
	updated, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, recipe: updated}, nil
}

// DeleteRecipe removes a recipe.
func (r *Resolver) DeleteRecipe(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
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

// AdjustUserItem updates the quantity and purchase date of a user's pantry item.
func (r *Resolver) AdjustUserItem(ctx context.Context, args struct {
	ItemID     graphql.ID
	Quantity   float64
	PurchaseAt *graphql.Time
}) (*userItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserItem(ctx, r.UserPrefsService, u.UserID, itemID)
	if err != nil {
		return nil, err
	}
	isFav := false
	var minQty *float64
	var expiresAt *time.Time
	notes := ""
	if existing != nil {
		isFav = existing.IsFavorite
		minQty = existing.MinQty
		expiresAt = existing.ExpiresAt
		notes = existing.Notes
	}
	var purchaseAt *time.Time
	if args.PurchaseAt != nil {
		purchaseAt = &args.PurchaseAt.Time
	}
	updated, err := r.UserPrefsService.UpsertUserItem(ctx, userprefs.UserItem{
		UserID:     u.UserID,
		ItemID:     itemID,
		CurrentQty: args.Quantity,
		MinQty:     minQty,
		PurchaseAt: purchaseAt,
		ExpiresAt:  expiresAt,
		Notes:      notes,
		IsFavorite: isFav,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: updated}, nil
}

// SetItemFavorite toggles the favorite flag for a user's pantry item.
func (r *Resolver) SetItemFavorite(ctx context.Context, args struct {
	ItemID     graphql.ID
	IsFavorite bool
}) (*userItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserItem(ctx, r.UserPrefsService, u.UserID, itemID)
	if err != nil {
		return nil, err
	}
	currentQty := 0.0
	var minQty *float64
	var purchaseAt *time.Time
	var expiresAt *time.Time
	notes := ""
	if existing != nil {
		currentQty = existing.CurrentQty
		minQty = existing.MinQty
		purchaseAt = existing.PurchaseAt
		expiresAt = existing.ExpiresAt
		notes = existing.Notes
	}
	updated, err := r.UserPrefsService.UpsertUserItem(ctx, userprefs.UserItem{
		UserID:     u.UserID,
		ItemID:     itemID,
		CurrentQty: currentQty,
		MinQty:     minQty,
		PurchaseAt: purchaseAt,
		ExpiresAt:  expiresAt,
		Notes:      notes,
		IsFavorite: args.IsFavorite,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userItemResolver{inv: r.InventoryService, item: updated}, nil
}

// DeleteUserItem removes a user's pantry item by catalog item ID.
func (r *Resolver) DeleteUserItem(ctx context.Context, args struct{ ItemID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	existing, err := findUserItem(ctx, r.UserPrefsService, u.UserID, itemID)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, nil
	}
	if err := r.UserPrefsService.DeleteUserItem(ctx, existing.UserItemID, u.UserID); err != nil {
		return false, err
	}
	return true, nil
}

// AdjustUserBottle updates the quantity of a user's wine cellar holding.
func (r *Resolver) AdjustUserBottle(ctx context.Context, args struct {
	BottleID graphql.ID
	Quantity int32
}) (*userBottleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	bottleID, err := parseID(string(args.BottleID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserBottle(ctx, r.UserPrefsService, u.UserID, bottleID)
	if err != nil {
		return nil, err
	}
	var bottleNum *int32
	var purchaseAt *time.Time
	var purchasePrice *float64
	var storageTemp *float64
	location := ""
	notes := ""
	isFav := false
	if existing != nil {
		bottleNum = existing.BottleNumber
		purchaseAt = existing.PurchaseAt
		purchasePrice = existing.PurchasePrice
		storageTemp = existing.StorageTemp
		location = existing.Location
		notes = existing.Notes
		isFav = existing.IsFavorite
	}
	updated, err := r.UserPrefsService.UpsertUserBottle(ctx, userprefs.UserBottle{
		UserID:        u.UserID,
		BottleID:      bottleID,
		BottleNumber:  bottleNum,
		Quantity:      args.Quantity,
		PurchaseAt:    purchaseAt,
		PurchasePrice: purchasePrice,
		StorageTemp:   storageTemp,
		Location:      location,
		Notes:         notes,
		IsFavorite:    isFav,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userBottleResolver{wine: r.WineService, bottle: updated}, nil
}

// SetBottleFavorite toggles the favorite flag for a user's wine holding.
func (r *Resolver) SetBottleFavorite(ctx context.Context, args struct {
	BottleID   graphql.ID
	IsFavorite bool
}) (*userBottleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	bottleID, err := parseID(string(args.BottleID))
	if err != nil {
		return nil, err
	}
	existing, err := findUserBottle(ctx, r.UserPrefsService, u.UserID, bottleID)
	if err != nil {
		return nil, err
	}
	quantity := int32(0)
	var bottleNum *int32
	var purchaseAt *time.Time
	var purchasePrice *float64
	var storageTemp *float64
	location := ""
	notes := ""
	if existing != nil {
		quantity = existing.Quantity
		bottleNum = existing.BottleNumber
		purchaseAt = existing.PurchaseAt
		purchasePrice = existing.PurchasePrice
		storageTemp = existing.StorageTemp
		location = existing.Location
		notes = existing.Notes
	}
	updated, err := r.UserPrefsService.UpsertUserBottle(ctx, userprefs.UserBottle{
		UserID:        u.UserID,
		BottleID:      bottleID,
		BottleNumber:  bottleNum,
		Quantity:      quantity,
		PurchaseAt:    purchaseAt,
		PurchasePrice: purchasePrice,
		StorageTemp:   storageTemp,
		Location:      location,
		Notes:         notes,
		IsFavorite:    args.IsFavorite,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &userBottleResolver{wine: r.WineService, bottle: updated}, nil
}

// CreateMealPlan creates a new meal plan for the current user.
func (r *Resolver) CreateMealPlan(ctx context.Context, args struct{ Input createMealPlanInput }) (*mealPlanResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	d, err := time.Parse("2006-01-02", args.Input.WeekStartDate)
	if err != nil {
		return nil, err
	}
	var dayOfWeek int32 = 1
	if args.Input.WeekStartDayOfWeek != nil {
		dayOfWeek = *args.Input.WeekStartDayOfWeek
	}
	mp, err := r.MealPlanService.CreateMealPlan(ctx, mealplan.MealPlan{
		UserID:             u.UserID,
		Name:               args.Input.Name,
		WeekStartDate:      d,
		WeekStartDayOfWeek: int16(dayOfWeek),
		IsActive:           true,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &mealPlanResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, plan: mp}, nil
}

// UpdateMealPlan modifies an existing meal plan.
func (r *Resolver) UpdateMealPlan(ctx context.Context, args struct {
	ID    graphql.ID
	Input createMealPlanInput
}) (*mealPlanResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.MealPlanService.GetMealPlanByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != "" {
		name = args.Input.Name
	}
	weekStart := existing.WeekStartDate
	if args.Input.WeekStartDate != "" {
		if d, err := time.Parse("2006-01-02", args.Input.WeekStartDate); err == nil {
			weekStart = d
		} else {
			return nil, err
		}
	}
	dayOfWeek := existing.WeekStartDayOfWeek
	if args.Input.WeekStartDayOfWeek != nil {
		dayOfWeek = int16(*args.Input.WeekStartDayOfWeek)
	}
	if err := r.MealPlanService.UpdateMealPlan(ctx, id, u.UserID, mealplan.MealPlan{
		Name:               name,
		WeekStartDate:      weekStart,
		WeekStartDayOfWeek: dayOfWeek,
		IsActive:           existing.IsActive,
	}, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.MealPlanService.GetMealPlanByID(ctx, id, u.UserID)
	if err != nil {
		return nil, err
	}
	return &mealPlanResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, plan: updated}, nil
}

// DeleteMealPlan removes a meal plan owned by the current user.
func (r *Resolver) DeleteMealPlan(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.MealPlanService.DeleteMealPlan(ctx, id, u.UserID); err != nil {
		return false, err
	}
	return true, nil
}

// AddMealSlot adds a slot to a meal plan.
func (r *Resolver) AddMealSlot(ctx context.Context, args struct{ Input addMealSlotInput }) (*mealSlotResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.Input.MealPlanID))
	if err != nil {
		return nil, err
	}
	var recipeID *int64
	if args.Input.RecipeID != nil {
		rid, err := parseID(string(*args.Input.RecipeID))
		if err != nil {
			return nil, err
		}
		recipeID = &rid
	}
	slot, err := r.MealPlanService.AddMealSlot(ctx, mealplan.MealSlot{
		MealPlanID:      mealPlanID,
		DayOfWeek:       int16(args.Input.DayOfWeek),
		MealType:        args.Input.MealType,
		RecipeID:        recipeID,
		Servings:        args.Input.Servings,
		ReplacementNote: derefString(args.Input.ReplacementNote),
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &mealSlotResolver{mp: r.MealPlanService, inv: r.InventoryService, rec: r.RecipeService, slot: slot}, nil
}

// RemoveMealSlot removes a slot from a meal plan.
func (r *Resolver) RemoveMealSlot(ctx context.Context, args struct{ SlotID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	slotID, err := parseID(string(args.SlotID))
	if err != nil {
		return false, err
	}
	if err := r.MealPlanService.DeleteMealSlot(ctx, slotID); err != nil {
		return false, err
	}
	return true, nil
}

// AddMealSlotItem adds an ad-hoc item to a slot.
func (r *Resolver) AddMealSlotItem(ctx context.Context, args struct{ Input addMealSlotItemInput }) (*mealSlotItemResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	slotID, err := parseID(string(args.Input.SlotID))
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.Input.ItemID))
	if err != nil {
		return nil, err
	}
	isFromRecipe := false
	if args.Input.IsFromRecipe != nil {
		isFromRecipe = *args.Input.IsFromRecipe
	}
	item, err := r.MealPlanService.AddMealSlotItem(ctx, mealplan.MealSlotItem{
		SlotID:       slotID,
		ItemID:       &itemID,
		Quantity:     args.Input.Quantity,
		Unit:         args.Input.Unit,
		IsFromRecipe: isFromRecipe,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &mealSlotItemResolver{inv: r.InventoryService, item: item}, nil
}

// RemoveMealSlotItem removes an item from a slot.
func (r *Resolver) RemoveMealSlotItem(ctx context.Context, args struct{ SlotItemID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	slotItemID, err := parseID(string(args.SlotItemID))
	if err != nil {
		return false, err
	}
	if err := r.MealPlanService.DeleteMealSlotItem(ctx, slotItemID); err != nil {
		return false, err
	}
	return true, nil
}

// GenerateGroceryList generates a grocery list from a meal plan.
func (r *Resolver) GenerateGroceryList(ctx context.Context, args struct{ MealPlanID graphql.ID }) (*groceryListResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mealPlanID, err := parseID(string(args.MealPlanID))
	if err != nil {
		return nil, err
	}
	list, err := r.GroceryService.Generate(ctx, u.UserID, mealPlanID, u.Email)
	if err != nil {
		return nil, err
	}
	return &groceryListResolver{g: r.GroceryService, inv: r.InventoryService, list: list}, nil
}

// ToggleGroceryItemChecked flips the checked state of a grocery list item.
func (r *Resolver) ToggleGroceryItemChecked(ctx context.Context, args struct{ GroceryListItemID graphql.ID }) (*groceryListItemResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return nil, err
	}
	it, err := r.GroceryService.GetGroceryListItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	it.IsChecked = !it.IsChecked
	u, _ := userFromContext(ctx)
	if err := r.GroceryService.UpdateGroceryListItem(ctx, id, it, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.GroceryService.GetGroceryListItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &groceryListItemResolver{inv: r.InventoryService, item: updated}, nil
}

// DeleteGroceryItem removes an item from a grocery list.
func (r *Resolver) DeleteGroceryItem(ctx context.Context, args struct{ GroceryListItemID graphql.ID }) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.GroceryListItemID))
	if err != nil {
		return false, err
	}
	if err := r.GroceryService.DeleteGroceryListItem(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// Types resolves all wine types.
func (r *Resolver) Types(ctx context.Context) ([]*wineTypeResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	types, err := r.WineService.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*wineTypeResolver, len(types))
	for i := range types {
		out[i] = &wineTypeResolver{t: types[i]}
	}
	return out, nil
}

// Countries resolves all wine countries.
func (r *Resolver) Countries(ctx context.Context) ([]*countryResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	countries, err := r.WineService.ListCountries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*countryResolver, len(countries))
	for i := range countries {
		out[i] = &countryResolver{c: countries[i]}
	}
	return out, nil
}

// Regions resolves wine regions for a country.
func (r *Resolver) Regions(ctx context.Context, args struct{ CountryID graphql.ID }) ([]*regionResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	countryID, err := parseID(string(args.CountryID))
	if err != nil {
		return nil, err
	}
	regions, err := r.WineService.ListRegions(ctx, countryID)
	if err != nil {
		return nil, err
	}
	out := make([]*regionResolver, len(regions))
	for i := range regions {
		out[i] = &regionResolver{wine: r.WineService, r: regions[i]}
	}
	return out, nil
}

// Vintages resolves all wine vintages.
func (r *Resolver) Vintages(ctx context.Context) ([]*vintageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	vintages, err := r.WineService.ListVintages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*vintageResolver, len(vintages))
	for i := range vintages {
		out[i] = &vintageResolver{v: vintages[i]}
	}
	return out, nil
}

// GrapeVarieties resolves all grape varieties.
func (r *Resolver) GrapeVarieties(ctx context.Context) ([]*grapeVarietyResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	varieties, err := r.WineService.ListGrapeVarieties(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*grapeVarietyResolver, len(varieties))
	for i := range varieties {
		out[i] = &grapeVarietyResolver{g: varieties[i]}
	}
	return out, nil
}

// CreateVintage adds a new vintage.
func (r *Resolver) CreateVintage(ctx context.Context, args struct{ Input createVintageInput }) (*vintageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	v, err := r.WineService.CreateVintage(ctx, args.Input.Year, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &vintageResolver{v: v}, nil
}

// CreateGrapeVariety adds a new grape variety.
func (r *Resolver) CreateGrapeVariety(ctx context.Context, args struct{ Input createGrapeVarietyInput }) (*grapeVarietyResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	g, err := r.WineService.CreateGrapeVariety(ctx, args.Input.Name, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &grapeVarietyResolver{g: g}, nil
}

// CreateBottle adds a new bottle definition.
func (r *Resolver) CreateBottle(ctx context.Context, args struct{ Input createBottleInput }) (*bottleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	typeID, err := parseID(string(args.Input.TypeID))
	if err != nil {
		return nil, err
	}
	countryID, err := parseID(string(args.Input.CountryID))
	if err != nil {
		return nil, err
	}
	regionID, err := parseID(string(args.Input.RegionID))
	if err != nil {
		return nil, err
	}
	var vineyard string
	if args.Input.Vineyard != nil {
		vineyard = *args.Input.Vineyard
	}
	var abv float64
	if args.Input.Abv != nil {
		abv = *args.Input.Abv
	}
	var acidity int16
	if args.Input.Acidity != nil {
		acidity = int16(*args.Input.Acidity)
	}
	var tanninLevel int16
	if args.Input.TanninLevel != nil {
		tanninLevel = int16(*args.Input.TanninLevel)
	}
	var body int16
	if args.Input.Body != nil {
		body = int16(*args.Input.Body)
	}
	var sweetness int16
	if args.Input.Sweetness != nil {
		sweetness = int16(*args.Input.Sweetness)
	}
	b, err := r.WineService.CreateBottle(ctx, wine.Bottle{
		TypeID:         typeID,
		CountryID:      countryID,
		RegionID:       regionID,
		VintageYear:    args.Input.VintageYear,
		Vineyard:       vineyard,
		Abv:            abv,
		Acidity:        acidity,
		TanninLevel:    tanninLevel,
		Body:           body,
		Sweetness:      sweetness,
		OakIntegration: args.Input.OakIntegration,
		BottleSize:     args.Input.BottleSize,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{b: b}, nil
}

// UpdateBottle modifies an existing bottle definition.
func (r *Resolver) UpdateBottle(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateBottleInput
}) (*bottleResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetBottleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	typeID := existing.TypeID
	if args.Input.TypeID != nil {
		tid, err := parseID(string(*args.Input.TypeID))
		if err != nil {
			return nil, err
		}
		typeID = tid
	}
	countryID := existing.CountryID
	if args.Input.CountryID != nil {
		cid, err := parseID(string(*args.Input.CountryID))
		if err != nil {
			return nil, err
		}
		countryID = cid
	}
	regionID := existing.RegionID
	if args.Input.RegionID != nil {
		rid, err := parseID(string(*args.Input.RegionID))
		if err != nil {
			return nil, err
		}
		regionID = rid
	}
	vintage := existing.VintageYear
	if args.Input.VintageYear != nil {
		vintage = *args.Input.VintageYear
	}
	vineyard := existing.Vineyard
	if args.Input.Vineyard != nil {
		vineyard = *args.Input.Vineyard
	}
	abv := existing.Abv
	if args.Input.Abv != nil {
		abv = *args.Input.Abv
	}
	acidity := existing.Acidity
	if args.Input.Acidity != nil {
		acidity = int16(*args.Input.Acidity)
	}
	tanninLevel := existing.TanninLevel
	if args.Input.TanninLevel != nil {
		tanninLevel = int16(*args.Input.TanninLevel)
	}
	body := existing.Body
	if args.Input.Body != nil {
		body = int16(*args.Input.Body)
	}
	sweetness := existing.Sweetness
	if args.Input.Sweetness != nil {
		sweetness = int16(*args.Input.Sweetness)
	}
	oakIntegration := existing.OakIntegration
	if args.Input.OakIntegration != nil {
		oakIntegration = *args.Input.OakIntegration
	}
	bottleSize := existing.BottleSize
	if args.Input.BottleSize != nil {
		bottleSize = *args.Input.BottleSize
	}
	if err := r.WineService.UpdateBottle(ctx, id, wine.Bottle{
		TypeID:         typeID,
		CountryID:      countryID,
		RegionID:       regionID,
		VintageYear:    vintage,
		Vineyard:       vineyard,
		Abv:            abv,
		Acidity:        acidity,
		TanninLevel:    tanninLevel,
		Body:           body,
		Sweetness:      sweetness,
		OakIntegration: oakIntegration,
		BottleSize:     bottleSize,
	}, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.WineService.GetBottleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{b: updated}, nil
}

func findUserItem(ctx context.Context, svc *userprefs.Service, userID, itemID int64) (*userprefs.UserItem, error) {
	items, err := svc.ListUserItems(ctx, userID, 100_000, 0)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ItemID == itemID {
			return &items[i], nil
		}
	}
	return nil, nil
}

func findUserBottle(ctx context.Context, svc *userprefs.Service, userID, bottleID int64) (*userprefs.UserBottle, error) {
	bottles, err := svc.ListUserBottles(ctx, userID, 100_000, 0)
	if err != nil {
		return nil, err
	}
	for i := range bottles {
		if bottles[i].BottleID == bottleID {
			return &bottles[i], nil
		}
	}
	return nil, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

// userResolver resolves User fields.
type userResolver struct {
	u currentuser.User
}

func (r *userResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.u.UserID, 10)) }
func (r *userResolver) Email() string        { return r.u.Email }
func (r *userResolver) DisplayName() *string { return nilIfEmpty(r.u.DisplayName) }

// itemResolver resolves Item fields.
type itemResolver struct {
	inv *inventory.Service
	it  inventory.Item
}

func (r *itemResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.it.ItemID, 10)) }
func (r *itemResolver) Name() string   { return r.it.Name }
func (r *itemResolver) Upc12() *string { return nilIfEmpty(r.it.Upc12) }
func (r *itemResolver) Upc14() *string { return nilIfEmpty(r.it.Upc14) }
func (r *itemResolver) Unit() string   { return r.it.Unit }
func (r *itemResolver) Brand(ctx context.Context) (*brandResolver, error) {
	if r.it.BrandID == nil {
		return nil, nil
	}
	b, err := r.inv.GetBrandByID(ctx, *r.it.BrandID)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}
func (r *itemResolver) Category(ctx context.Context) (*categoryResolver, error) {
	c, err := r.inv.GetCategoryByID(ctx, r.it.CategoryID)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

type brandResolver struct{ b inventory.Brand }

func (r *brandResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.b.BrandID, 10)) }
func (r *brandResolver) Name() string   { return r.b.Name }

type categoryResolver struct{ c inventory.Category }

func (r *categoryResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.c.CategoryID, 10)) }
func (r *categoryResolver) Name() string         { return r.c.Name }
func (r *categoryResolver) Description() *string { return nilIfEmpty(r.c.Description) }

// flavorProfileResolver resolves an inventory flavor profile.
type flavorProfileResolver struct{ f inventory.FlavorProfile }

func (r *flavorProfileResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.f.FlavorID, 10))
}
func (r *flavorProfileResolver) Name() string   { return r.f.Name }
func (r *flavorProfileResolver) IsActive() bool { return r.f.IsActive }

// nutrientTypeResolver resolves an inventory nutrient type.
type nutrientTypeResolver struct{ n inventory.NutrientType }

func (r *nutrientTypeResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.n.NutrientID, 10))
}
func (r *nutrientTypeResolver) Name() string { return r.n.Name }
func (r *nutrientTypeResolver) Unit() string { return r.n.Unit }

type itemPageResolver struct {
	inv      *inventory.Service
	items    []inventory.Item
	page     int32
	pageSize int32
	total    int32
}

func (r *itemPageResolver) Items() []*itemResolver {
	out := make([]*itemResolver, len(r.items))
	for i := range r.items {
		out[i] = &itemResolver{inv: r.inv, it: r.items[i]}
	}
	return out
}

func (r *itemPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// userItemResolver resolves UserItem fields.
type userItemResolver struct {
	inv  *inventory.Service
	item userprefs.UserItem
}

func (r *userItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.UserItemID, 10))
}
func (r *userItemResolver) CurrentQty() float64       { return r.item.CurrentQty }
func (r *userItemResolver) MinQty() *float64          { return r.item.MinQty }
func (r *userItemResolver) Notes() *string            { return nilIfEmpty(r.item.Notes) }
func (r *userItemResolver) IsFavorite() bool          { return r.item.IsFavorite }
func (r *userItemResolver) PurchaseAt() *graphql.Time { return timeToGraphQL(r.item.PurchaseAt) }
func (r *userItemResolver) ExpiresAt() *graphql.Time  { return timeToGraphQL(r.item.ExpiresAt) }
func (r *userItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	it, err := r.inv.GetItemByID(ctx, r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type userItemPageResolver struct {
	inv      *inventory.Service
	items    []userprefs.UserItem
	page     int32
	pageSize int32
	total    int32
}

func (r *userItemPageResolver) Items() []*userItemResolver {
	out := make([]*userItemResolver, len(r.items))
	for i := range r.items {
		out[i] = &userItemResolver{inv: r.inv, item: r.items[i]}
	}
	return out
}

func (r *userItemPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// userBottleResolver resolves UserBottle fields.
type userBottleResolver struct {
	wine   *wine.Service
	bottle userprefs.UserBottle
}

func (r *userBottleResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.bottle.UserBottleID, 10))
}
func (r *userBottleResolver) BottleNumber() *int32      { return r.bottle.BottleNumber }
func (r *userBottleResolver) Quantity() int32           { return r.bottle.Quantity }
func (r *userBottleResolver) PurchaseAt() *graphql.Time { return timeToGraphQL(r.bottle.PurchaseAt) }
func (r *userBottleResolver) PurchasePrice() *float64   { return r.bottle.PurchasePrice }
func (r *userBottleResolver) StorageTemp() *float64     { return r.bottle.StorageTemp }
func (r *userBottleResolver) Location() *string         { return nilIfEmpty(r.bottle.Location) }
func (r *userBottleResolver) Notes() *string            { return nilIfEmpty(r.bottle.Notes) }
func (r *userBottleResolver) IsFavorite() bool          { return r.bottle.IsFavorite }
func (r *userBottleResolver) Bottle(ctx context.Context) (*bottleResolver, error) {
	b, err := r.wine.GetBottleByID(ctx, r.bottle.BottleID)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{b: b}, nil
}

type userBottlePageResolver struct {
	wine     *wine.Service
	bottles  []userprefs.UserBottle
	page     int32
	pageSize int32
	total    int32
}

func (r *userBottlePageResolver) Items() []*userBottleResolver {
	out := make([]*userBottleResolver, len(r.bottles))
	for i := range r.bottles {
		out[i] = &userBottleResolver{wine: r.wine, bottle: r.bottles[i]}
	}
	return out
}

func (r *userBottlePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type pageInfoResolver struct {
	page     int32
	pageSize int32
	total    int32
}

func (r *pageInfoResolver) PageNumber() int32 { return r.page }
func (r *pageInfoResolver) PageSize() int32   { return r.pageSize }
func (r *pageInfoResolver) TotalCount() int32 { return r.total }

// recipeResolver resolves Recipe fields.
type recipeResolver struct {
	inv    *inventory.Service
	rec    *recipe.Service
	up     *userprefs.Service
	user   currentuser.User
	recipe recipe.Recipe
}

func (r *recipeResolver) ID() graphql.ID          { return graphql.ID(strconv.FormatInt(r.recipe.RecipeID, 10)) }
func (r *recipeResolver) Name() string            { return r.recipe.Name }
func (r *recipeResolver) Description() *string    { return nilIfEmpty(r.recipe.Description) }
func (r *recipeResolver) Servings() *int32        { return int32Ptr(r.recipe.Servings) }
func (r *recipeResolver) PrepTimeMinutes() *int32 { return int32Ptr(r.recipe.PrepTimeMinutes) }
func (r *recipeResolver) CookTimeMinutes() *int32 { return int32Ptr(r.recipe.CookTimeMinutes) }
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
		return false, err
	}
	return fav.IsFavorite, nil
}

type recipeItemResolver struct {
	inv  *inventory.Service
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
func (r *recipeItemResolver) Unit() string      { return r.item.Unit }
func (r *recipeItemResolver) Notes() *string    { return nilIfEmpty(r.item.Notes) }
func (r *recipeItemResolver) IsOptional() bool  { return r.item.IsOptional }

type recipeStepResolver struct{ step recipe.RecipeStep }

func (r *recipeStepResolver) StepNumber() int32   { return r.step.StepNumber }
func (r *recipeStepResolver) Instruction() string { return r.step.Instruction }

type recipePageResolver struct {
	inv      *inventory.Service
	rec      *recipe.Service
	up       *userprefs.Service
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

// mealPlanResolver resolves MealPlan fields.
type mealPlanResolver struct {
	mp   *mealplan.Service
	inv  *inventory.Service
	rec  *recipe.Service
	plan mealplan.MealPlan
}

func (r *mealPlanResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.plan.MealPlanID, 10))
}
func (r *mealPlanResolver) Name() string          { return r.plan.Name }
func (r *mealPlanResolver) WeekStartDate() string { return r.plan.WeekStartDate.Format("2006-01-02") }
func (r *mealPlanResolver) IsActive() bool        { return r.plan.IsActive }
func (r *mealPlanResolver) Slots(ctx context.Context) ([]*mealSlotResolver, error) {
	slots, err := r.mp.ListMealSlotsForPlan(ctx, r.plan.MealPlanID)
	if err != nil {
		return nil, err
	}
	out := make([]*mealSlotResolver, len(slots))
	for i := range slots {
		out[i] = &mealSlotResolver{mp: r.mp, inv: r.inv, rec: r.rec, slot: slots[i]}
	}
	return out, nil
}

// mealSlotResolver resolves MealSlot fields.
type mealSlotResolver struct {
	mp   *mealplan.Service
	inv  *inventory.Service
	rec  *recipe.Service
	slot mealplan.MealSlot
}

func (r *mealSlotResolver) ID() graphql.ID           { return graphql.ID(strconv.FormatInt(r.slot.SlotID, 10)) }
func (r *mealSlotResolver) DayOfWeek() int32         { return int32(r.slot.DayOfWeek) }
func (r *mealSlotResolver) MealType() string         { return r.slot.MealType }
func (r *mealSlotResolver) Servings() *int32         { return r.slot.Servings }
func (r *mealSlotResolver) ReplacementNote() *string { return nilIfEmpty(r.slot.ReplacementNote) }
func (r *mealSlotResolver) Recipe(ctx context.Context) (*recipeResolver, error) {
	if r.slot.RecipeID == nil {
		return nil, nil
	}
	rec, err := r.rec.GetRecipeByID(ctx, *r.slot.RecipeID)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.inv, rec: r.rec, recipe: rec}, nil
}
func (r *mealSlotResolver) Items(ctx context.Context) ([]*mealSlotItemResolver, error) {
	items, err := r.mp.ListMealSlotItems(ctx, r.slot.SlotID)
	if err != nil {
		return nil, err
	}
	out := make([]*mealSlotItemResolver, len(items))
	for i := range items {
		out[i] = &mealSlotItemResolver{inv: r.inv, item: items[i]}
	}
	return out, nil
}

// mealSlotItemResolver resolves MealSlotItem fields.
type mealSlotItemResolver struct {
	inv  *inventory.Service
	item mealplan.MealSlotItem
}

func (r *mealSlotItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.SlotItemID, 10))
}
func (r *mealSlotItemResolver) Quantity() float64  { return r.item.Quantity }
func (r *mealSlotItemResolver) Unit() string       { return r.item.Unit }
func (r *mealSlotItemResolver) IsFromRecipe() bool { return r.item.IsFromRecipe }
func (r *mealSlotItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.item.ItemID == nil {
		return nil, nil
	}
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type mealPlanPageResolver struct {
	mp       *mealplan.Service
	inv      *inventory.Service
	rec      *recipe.Service
	plans    []mealplan.MealPlan
	page     int32
	pageSize int32
	total    int32
}

func (r *mealPlanPageResolver) Items() []*mealPlanResolver {
	out := make([]*mealPlanResolver, len(r.plans))
	for i := range r.plans {
		out[i] = &mealPlanResolver{mp: r.mp, inv: r.inv, rec: r.rec, plan: r.plans[i]}
	}
	return out
}

func (r *mealPlanPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// groceryListResolver resolves GroceryList fields.
type groceryListResolver struct {
	g    *grocery.Service
	inv  *inventory.Service
	list grocery.GroceryList
}

func (r *groceryListResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.list.GroceryListID, 10))
}
func (r *groceryListResolver) GeneratedAt() graphql.Time {
	return graphql.Time{Time: r.list.GeneratedAt}
}
func (r *groceryListResolver) Items(ctx context.Context) ([]*groceryListItemResolver, error) {
	items, err := r.g.ListGroceryListItems(ctx, r.list.GroceryListID)
	if err != nil {
		return nil, err
	}
	out := make([]*groceryListItemResolver, len(items))
	for i := range items {
		out[i] = &groceryListItemResolver{inv: r.inv, item: items[i]}
	}
	return out, nil
}

// groceryListItemResolver resolves GroceryListItem fields.
type groceryListItemResolver struct {
	inv  *inventory.Service
	item grocery.GroceryListItem
}

func (r *groceryListItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.GroceryListItemID, 10))
}
func (r *groceryListItemResolver) ManualItemName() *string { return nilIfEmpty(r.item.ManualItemName) }
func (r *groceryListItemResolver) QuantityNeeded() float64 { return r.item.QuantityNeeded }
func (r *groceryListItemResolver) UnitOfMeasure() *string  { return nilIfEmpty(r.item.UnitOfMeasure) }
func (r *groceryListItemResolver) Source() string          { return r.item.Source }
func (r *groceryListItemResolver) IsChecked() bool         { return r.item.IsChecked }
func (r *groceryListItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.item.ItemID == nil {
		return nil, nil
	}
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it}, nil
}

type groceryListPageResolver struct {
	g        *grocery.Service
	inv      *inventory.Service
	lists    []grocery.GroceryList
	page     int32
	pageSize int32
	total    int32
}

func (r *groceryListPageResolver) Items() []*groceryListResolver {
	out := make([]*groceryListResolver, len(r.lists))
	for i := range r.lists {
		out[i] = &groceryListResolver{g: r.g, inv: r.inv, list: r.lists[i]}
	}
	return out
}

func (r *groceryListPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// bottleResolver resolves Bottle fields.
type bottleResolver struct{ b wine.Bottle }

func (r *bottleResolver) ID() graphql.ID     { return graphql.ID(strconv.FormatInt(r.b.BottleID, 10)) }
func (r *bottleResolver) TypeID() graphql.ID { return graphql.ID(strconv.FormatInt(r.b.TypeID, 10)) }
func (r *bottleResolver) CountryID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.b.CountryID, 10))
}
func (r *bottleResolver) RegionID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.b.RegionID, 10))
}
func (r *bottleResolver) Vineyard() *string     { return nilIfEmpty(r.b.Vineyard) }
func (r *bottleResolver) VintageYear() int32    { return r.b.VintageYear }
func (r *bottleResolver) Abv() *float64         { return float64OrNil(r.b.Abv) }
func (r *bottleResolver) Acidity() *int32       { return int16OrNil(r.b.Acidity) }
func (r *bottleResolver) TanninLevel() *int32   { return int16OrNil(r.b.TanninLevel) }
func (r *bottleResolver) Body() *int32          { return int16OrNil(r.b.Body) }
func (r *bottleResolver) Sweetness() *int32     { return int16OrNil(r.b.Sweetness) }
func (r *bottleResolver) OakIntegration() *bool { return &r.b.OakIntegration }
func (r *bottleResolver) BottleSize() string    { return r.b.BottleSize }

type bottlePageResolver struct {
	bottles  []wine.Bottle
	page     int32
	pageSize int32
	total    int32
}

func (r *bottlePageResolver) Items() []*bottleResolver {
	out := make([]*bottleResolver, len(r.bottles))
	for i := range r.bottles {
		out[i] = &bottleResolver{b: r.bottles[i]}
	}
	return out
}

func (r *bottlePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// wineTypeResolver resolves a wine type.
type wineTypeResolver struct{ t wine.Type }

func (r *wineTypeResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.t.TypeID, 10)) }
func (r *wineTypeResolver) Name() string         { return r.t.Name }
func (r *wineTypeResolver) Description() *string { return nilIfEmpty(r.t.Description) }

// countryResolver resolves a wine country.
type countryResolver struct{ c wine.Country }

func (r *countryResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.c.CountryID, 10)) }
func (r *countryResolver) Name() string         { return r.c.Name }
func (r *countryResolver) IsoCode() string      { return r.c.IsoCode }
func (r *countryResolver) Description() *string { return nilIfEmpty(r.c.Description) }

// regionResolver resolves a wine region.
type regionResolver struct {
	wine *wine.Service
	r    wine.Region
}

func (r *regionResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.r.RegionID, 10)) }
func (r *regionResolver) Name() string         { return r.r.Name }
func (r *regionResolver) Description() *string { return nilIfEmpty(r.r.Description) }
func (r *regionResolver) Country(ctx context.Context) (*countryResolver, error) {
	c, err := r.wine.GetCountryByID(ctx, r.r.CountryID)
	if err != nil {
		return nil, err
	}
	return &countryResolver{c: c}, nil
}

// vintageResolver resolves a wine vintage.
type vintageResolver struct{ v wine.Vintage }

func (r *vintageResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.v.VintageID, 10)) }
func (r *vintageResolver) Year() int32          { return r.v.Year }
func (r *vintageResolver) Description() *string { return nilIfEmpty(r.v.Description) }
func (r *vintageResolver) IsActive() bool       { return r.v.IsActive }

// grapeVarietyResolver resolves a grape variety.
type grapeVarietyResolver struct{ g wine.GrapeVariety }

func (r *grapeVarietyResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.g.GrapeVarietyID, 10))
}
func (r *grapeVarietyResolver) Name() string         { return r.g.Name }
func (r *grapeVarietyResolver) Description() *string { return nilIfEmpty(r.g.Description) }
func (r *grapeVarietyResolver) IsActive() bool       { return r.g.IsActive }

type nutrition struct {
	Name   string
	Unit   string
	Amount float64
}

// nutritionResolver resolves a nutrition summary row.
type nutritionResolver struct{ nutrition nutrition }

func (r *nutritionResolver) Name() string    { return r.nutrition.Name }
func (r *nutritionResolver) Unit() string    { return r.nutrition.Unit }
func (r *nutritionResolver) Amount() float64 { return r.nutrition.Amount }

// Inputs map directly to the GraphQL input types.
type createBrandInput struct {
	Name string
}

type createCategoryInput struct {
	Name        string
	Description *string
}

type createFlavorProfileInput struct {
	Name string
}

type createNutrientTypeInput struct {
	Name string
	Unit string
}

type createVintageInput struct {
	Year        int32
	Description *string
}

type createGrapeVarietyInput struct {
	Name        string
	Description *string
}

type createItemInput struct {
	Name       string
	BrandID    *graphql.ID
	Upc12      *string
	Upc14      *string
	CategoryID graphql.ID
	Unit       string
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

type updateItemInput struct {
	Name       *string
	BrandID    *graphql.ID
	Upc12      *string
	Upc14      *string
	CategoryID *graphql.ID
	Unit       *string
}

type createMealPlanInput struct {
	Name               string
	WeekStartDate      string
	WeekStartDayOfWeek *int32
}

type addMealSlotInput struct {
	MealPlanID      graphql.ID
	DayOfWeek       int32
	MealType        string
	RecipeID        *graphql.ID
	Servings        *int32
	ReplacementNote *string
}

type addMealSlotItemInput struct {
	SlotID       graphql.ID
	ItemID       graphql.ID
	Quantity     float64
	Unit         string
	IsFromRecipe *bool
}

type createBottleInput struct {
	TypeID         graphql.ID
	CountryID      graphql.ID
	RegionID       graphql.ID
	VintageYear    int32
	Vineyard       *string
	Abv            *float64
	Acidity        *int32
	TanninLevel    *int32
	Body           *int32
	Sweetness      *int32
	OakIntegration bool
	BottleSize     string
}

type updateBottleInput struct {
	TypeID         *graphql.ID
	CountryID      *graphql.ID
	RegionID       *graphql.ID
	VintageYear    *int32
	Vineyard       *string
	Abv            *float64
	Acidity        *int32
	TanninLevel    *int32
	Body           *int32
	Sweetness      *int32
	OakIntegration *bool
	BottleSize     *string
}

// NewGraphQLHandler returns an Echo handler that executes GraphQL requests.
func NewGraphQLHandler(r *Resolver) echo.HandlerFunc {
	parsed, err := graphql.ParseSchema(schema, r)
	if err != nil {
		panic(err)
	}
	return func(c echo.Context) error {
		var req struct {
			Query         string                 `json:"query"`
			Variables     map[string]interface{} `json:"variables"`
			OperationName string                 `json:"operationName"`
		}
		if err := c.Bind(&req); err != nil {
			return err
		}
		resp := parsed.Exec(c.Request().Context(), req.Query, req.OperationName, req.Variables)
		return c.JSONBlob(http.StatusOK, resp.Data)
	}
}
