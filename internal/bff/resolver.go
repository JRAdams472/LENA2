package bff

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

//go:embed schema.graphqls
var schema string

// Resolver is the root GraphQL resolver. It is the only package that is
// allowed to orchestrate across domain modules.
type Resolver struct {
	InventoryService *inventory.Service
	RecipeService    *recipe.Service
	UserPrefsService *userprefs.Service
}

// NewResolver returns a new BFF resolver with the domain services.
func NewResolver(inv *inventory.Service, rec *recipe.Service, up *userprefs.Service) *Resolver {
	return &Resolver{InventoryService: inv, RecipeService: rec, UserPrefsService: up}
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
	if _, err := userFromContext(ctx); err != nil {
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
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, recipe: rec}, nil
}

// Recipes resolves a paginated list of active recipes.
func (r *Resolver) Recipes(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*recipePageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	recipes, err := r.RecipeService.ListRecipes(ctx, true, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	return &recipePageResolver{inv: r.InventoryService, rec: r.RecipeService, recipes: recipes, page: page, pageSize: pageSize, total: int32(len(recipes))}, nil
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
	recipes  []recipe.Recipe
	page     int32
	pageSize int32
	total    int32
}

func (r *recipePageResolver) Items() []*recipeResolver {
	out := make([]*recipeResolver, len(r.recipes))
	for i := range r.recipes {
		out[i] = &recipeResolver{inv: r.inv, rec: r.rec, recipe: r.recipes[i]}
	}
	return out
}

func (r *recipePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// Inputs map directly to the GraphQL input types.
type createBrandInput struct {
	Name string
}

type createCategoryInput struct {
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
