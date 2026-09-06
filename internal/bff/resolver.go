package bff

import (
	"context"
	_ "embed"
	"errors"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/wine"
)

//go:embed schema.graphqls
var schema string

// Resolver is the root GraphQL resolver. It is the only package that is
// allowed to orchestrate across domain modules.
type Resolver struct {
	GroceryService   GroceryService
	InventoryService InventoryService
	MealPlanService  MealPlanService
	RecipeService    RecipeService
	UserPrefsService UserPrefsService
	WineService      WineService
}

// NewResolver returns a new BFF resolver with the domain services.
func NewResolver(gr GroceryService, inv InventoryService, mp MealPlanService, rec RecipeService, up UserPrefsService, wineSvc WineService) *Resolver {
	return &Resolver{GroceryService: gr, InventoryService: inv, MealPlanService: mp, RecipeService: rec, UserPrefsService: up, WineService: wineSvc}
}

func userFromContext(ctx context.Context) (currentuser.User, error) {
	u, ok := currentuser.FromContext(ctx)
	if !ok {
		return currentuser.User{}, errors.New("unauthorized")
	}
	return u, nil
}

// requireAdmin guards shared-catalog mutations: only users whose
// persisted identity role is 'admin' may modify global data.
func requireAdmin(ctx context.Context) (currentuser.User, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return currentuser.User{}, err
	}
	if !u.IsAdmin {
		return currentuser.User{}, errors.New("forbidden: admin role required")
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

// int32Ptr returns a copy of v so a *int32 field can be populated from a
// GraphQL int32 input without aliasing the args struct.
func int32Ptr(v int32) *int32 {
	return &v
}

// int16Ptr converts a nullable GraphQL int32 input to *int16 for the
// service layer's pointer-or-null convention.
func int16Ptr(v *int32) *int16 {
	if v == nil {
		return nil
	}
	i := int16(*v)
	return &i
}

// int16ToInt32Ptr renders a nullable int16 service field as *int32.
func int16ToInt32Ptr(v *int16) *int32 {
	if v == nil {
		return nil
	}
	i := int32(*v)
	return &i
}

// int64ToInt32 saturates a row count at MaxInt32 for the GraphQL Int field.
func int64ToInt32(n int64) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
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

func (r *userResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.u.UserID, 10)) }

func (r *userResolver) Email() string { return r.u.Email }

func (r *userResolver) DisplayName() *string { return nilIfEmpty(r.u.DisplayName) }

type pageInfoResolver struct {
	page     int32
	pageSize int32
	total    int32
}

func (r *pageInfoResolver) PageNumber() int32 { return r.page }

func (r *pageInfoResolver) PageSize() int32 { return r.pageSize }

func (r *pageInfoResolver) TotalCount() int32 { return r.total }

// distinctIDs collects the unique non-nil IDs produced by f, sorted so
// generated queries are deterministic.
func distinctIDs[T any](xs []T, f func(T) *int64) []int64 {
	set := make(map[int64]bool)
	for _, x := range xs {
		if id := f(x); id != nil {
			set[*id] = true
		}
	}
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// itemChildren holds inventory rows batch-loaded for a list response so
// nested item field resolvers do not issue a query per row. When a child
// resolver's ch field is nil it falls back to lazy service calls.
type itemChildren struct {
	brands     map[int64]inventory.Brand
	categories map[int64]inventory.Category
	nutrients  map[int64][]inventory.FoodNutrient
	flavors    map[int64][]inventory.FoodFlavor
}

// loadItems fetches a set of catalog items in one query, keyed by ID.
func loadItems(ctx context.Context, inv InventoryService, itemIDs []int64) (map[int64]inventory.Item, error) {
	items := make(map[int64]inventory.Item)
	if len(itemIDs) == 0 {
		return items, nil
	}
	rows, err := inv.GetItemsByIDs(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	for _, it := range rows {
		items[it.ItemID] = it
	}
	return items, nil
}

// loadItemChildren batch-loads the brand, category, nutrient and flavor
// rows referenced by items.
func loadItemChildren(ctx context.Context, inv InventoryService, items []inventory.Item) (*itemChildren, error) {
	ch := &itemChildren{
		brands:     make(map[int64]inventory.Brand),
		categories: make(map[int64]inventory.Category),
		nutrients:  make(map[int64][]inventory.FoodNutrient),
		flavors:    make(map[int64][]inventory.FoodFlavor),
	}
	if len(items) == 0 {
		return ch, nil
	}
	itemIDs := make([]int64, len(items))
	brandIDSet := make(map[int64]bool)
	categoryIDSet := make(map[int64]bool)
	for i, it := range items {
		itemIDs[i] = it.ItemID
		if it.BrandID != nil {
			brandIDSet[*it.BrandID] = true
		}
		categoryIDSet[it.CategoryID] = true
	}
	brandIDs := make([]int64, 0, len(brandIDSet))
	for id := range brandIDSet {
		brandIDs = append(brandIDs, id)
	}
	slices.Sort(brandIDs)
	categoryIDs := make([]int64, 0, len(categoryIDSet))
	for id := range categoryIDSet {
		categoryIDs = append(categoryIDs, id)
	}
	slices.Sort(categoryIDs)
	slices.Sort(itemIDs)
	if len(brandIDs) > 0 {
		brands, err := inv.GetBrandsByIDs(ctx, brandIDs)
		if err != nil {
			return nil, err
		}
		for _, b := range brands {
			ch.brands[b.BrandID] = b
		}
	}
	categories, err := inv.GetCategoriesByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}
	for _, c := range categories {
		ch.categories[c.CategoryID] = c
	}
	nutrients, err := inv.ListFoodNutrientsByItems(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	for _, n := range nutrients {
		ch.nutrients[n.ItemID] = append(ch.nutrients[n.ItemID], n)
	}
	flavors, err := inv.ListFoodFlavorsByItems(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	for _, f := range flavors {
		ch.flavors[f.ItemID] = append(ch.flavors[f.ItemID], f)
	}
	return ch, nil
}

// recipeChildren holds recipe rows batch-loaded for a list response so
// nested recipe field resolvers do not issue a query per row.
type recipeChildren struct {
	recipes      map[int64]recipe.Recipe
	itemsBy      map[int64][]recipe.RecipeItem
	stepsBy      map[int64][]recipe.RecipeStep
	favorites    map[int64]bool
	items        map[int64]inventory.Item
	itemChildren *itemChildren
}

// loadRecipeChildren batch-loads the recipes, their items and steps, the
// current user's favorite flags, and the catalog rows for every item those
// recipes reference. The returned itemID set is merged into extraItemIDs so
// callers can also resolve items referenced from elsewhere (e.g. meal slot
// overrides) with the same maps.
func loadRecipeChildren(ctx context.Context, rec RecipeService, up UserPrefsService, inv InventoryService, userID int64, recipeIDs, extraItemIDs []int64) (*recipeChildren, error) {
	rc := &recipeChildren{
		recipes:   make(map[int64]recipe.Recipe),
		itemsBy:   make(map[int64][]recipe.RecipeItem),
		stepsBy:   make(map[int64][]recipe.RecipeStep),
		favorites: make(map[int64]bool),
	}
	if len(recipeIDs) > 0 {
		recipes, err := rec.GetRecipesByIDs(ctx, recipeIDs)
		if err != nil {
			return nil, err
		}
		for _, rcp := range recipes {
			rc.recipes[rcp.RecipeID] = rcp
		}
		items, err := rec.ListRecipeItemsByRecipes(ctx, recipeIDs)
		if err != nil {
			return nil, err
		}
		for _, ri := range items {
			rc.itemsBy[ri.RecipeID] = append(rc.itemsBy[ri.RecipeID], ri)
		}
		steps, err := rec.ListRecipeStepsByRecipes(ctx, recipeIDs)
		if err != nil {
			return nil, err
		}
		for _, s := range steps {
			rc.stepsBy[s.RecipeID] = append(rc.stepsBy[s.RecipeID], s)
		}
		favs, err := up.ListRecipeFavorites(ctx, userID, recipeIDs)
		if err != nil {
			return nil, err
		}
		for _, f := range favs {
			rc.favorites[f.RecipeID] = f.IsFavorite
		}
	}
	itemIDSet := make(map[int64]bool)
	for _, id := range extraItemIDs {
		itemIDSet[id] = true
	}
	for _, items := range rc.itemsBy {
		for _, ri := range items {
			itemIDSet[ri.ItemID] = true
		}
	}
	itemIDs := make([]int64, 0, len(itemIDSet))
	for id := range itemIDSet {
		itemIDs = append(itemIDs, id)
	}
	slices.Sort(itemIDs)
	items, err := loadItems(ctx, inv, itemIDs)
	if err != nil {
		return nil, err
	}
	rc.items = items
	list := make([]inventory.Item, 0, len(items))
	for _, it := range items {
		list = append(list, it)
	}
	ch, err := loadItemChildren(ctx, inv, list)
	if err != nil {
		return nil, err
	}
	rc.itemChildren = ch
	return rc, nil
}

// bottleChildren holds wine rows batch-loaded for a list response so
// nested bottle field resolvers do not issue a query per row.
type bottleChildren struct {
	bottles  map[int64]wine.Bottle
	grapesBy map[int64][]wine.BottleGrapeVariety
	favorsBy map[int64][]wine.BottleFlavorProfile
}

// loadBottleChildren batch-loads the bottles (when includeBottles is set),
// grape varieties and flavor profiles for a set of bottle IDs.
func loadBottleChildren(ctx context.Context, wineSvc WineService, bottleIDs []int64, includeBottles bool) (*bottleChildren, error) {
	bc := &bottleChildren{
		bottles:  make(map[int64]wine.Bottle),
		grapesBy: make(map[int64][]wine.BottleGrapeVariety),
		favorsBy: make(map[int64][]wine.BottleFlavorProfile),
	}
	if len(bottleIDs) == 0 {
		return bc, nil
	}
	if includeBottles {
		bottles, err := wineSvc.GetBottlesByIDs(ctx, bottleIDs)
		if err != nil {
			return nil, err
		}
		for _, b := range bottles {
			bc.bottles[b.BottleID] = b
		}
	}
	grapes, err := wineSvc.ListBottleGrapeVarietiesByBottles(ctx, bottleIDs)
	if err != nil {
		return nil, err
	}
	for _, g := range grapes {
		bc.grapesBy[g.BottleID] = append(bc.grapesBy[g.BottleID], g)
	}
	favors, err := wineSvc.ListBottleFlavorProfilesByBottles(ctx, bottleIDs)
	if err != nil {
		return nil, err
	}
	for _, f := range favors {
		bc.favorsBy[f.BottleID] = append(bc.favorsBy[f.BottleID], f)
	}
	return bc, nil
}

// NewGraphQLHandler returns an Echo handler that executes GraphQL requests.
func NewGraphQLHandler(r *Resolver) echo.HandlerFunc {
	parsed, err := graphql.ParseSchema(schema, r, graphql.Tracer(newGraphQLTracer()))
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
		return c.JSON(http.StatusOK, resp)
	}
}
