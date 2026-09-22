package bff

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/async"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/wine"
)

//go:embed schema.graphqls
var schema string

// OCRClient extracts text from an image for the nutrition-label OCR flow.
type OCRClient interface {
	ExtractText(ctx context.Context, image []byte) (string, error)
}

// Resolver is the root GraphQL resolver. It is the only package that is
// allowed to orchestrate across domain modules.
type Resolver struct {
	UOW                    dbtx.UnitOfWork
	AnalyticsService       AnalyticsService
	GroceryService         GroceryService
	InventoryService       InventoryService
	MealPlanService        MealPlanService
	RecipeService          RecipeService
	UserPrefsService       UserPrefsService
	WineService            WineService
	IdentityService        IdentityService
	RecipeImportService    RecipeImportService
	OCRClient              OCRClient
	NutritionPhotoMaxBytes int
	RecipeScanMaxBytes     int

	// bg carries detached analytics/recommendation work on a bounded
	// runner that Shutdown drains. Lazily initialized so tests can keep
	// constructing Resolver literals.
	bgOnce   sync.Once
	bgRunner *async.Runner

	// ocrInFlight tracks in-flight nutrition-OCR jobs per user so one
	// member cannot hold more than one at a time; uploads throttles
	// upload mutations per user.
	ocrMu       sync.Mutex
	ocrInFlight map[int64]int
	uploads     *userRateLimiter
}

// asyncWorkerCap bounds the number of in-flight background tasks.
const asyncWorkerCap = 16

// maxUserOCRJobs bounds in-flight nutrition-OCR jobs per user.
const maxUserOCRJobs = 1

// Services is the set of domain interfaces the BFF orchestrates across.
// Field types are role interfaces so consumers state the minimum surface
// they need; the concrete domain services satisfy them all.
type Services struct {
	Analytics    AnalyticsService
	Grocery      GroceryService
	Inventory    InventoryService
	MealPlan     MealPlanService
	Recipe       RecipeService
	UserPrefs    UserPrefsService
	Wine         WineService
	Identity     IdentityService
	RecipeImport RecipeImportService
	OCR          OCRClient
}

// Options carries resolver limits and tunables out of the constructor so
// adding a knob does not grow the parameter list again.
type Options struct {
	NutritionPhotoMaxBytes int
	RecipeScanMaxBytes     int
	// UploadRatePerMinute bounds upload mutations per user; <=0 uses the
	// built-in default.
	UploadRatePerMinute int
}

// NewResolver returns a new BFF resolver wired to the domain services.
func NewResolver(pool dbtx.Pool, svc Services, opts Options) *Resolver {
	uploadRate := opts.UploadRatePerMinute
	if uploadRate <= 0 {
		uploadRate = 12
	}
	return &Resolver{
		UOW:                    dbtx.NewUnitOfWork(pool),
		AnalyticsService:       svc.Analytics,
		GroceryService:         svc.Grocery,
		InventoryService:       svc.Inventory,
		MealPlanService:        svc.MealPlan,
		RecipeService:          svc.Recipe,
		UserPrefsService:       svc.UserPrefs,
		WineService:            svc.Wine,
		IdentityService:        svc.Identity,
		RecipeImportService:    svc.RecipeImport,
		OCRClient:              svc.OCR,
		NutritionPhotoMaxBytes: opts.NutritionPhotoMaxBytes,
		RecipeScanMaxBytes:     opts.RecipeScanMaxBytes,
		uploads:                newUserRateLimiter(uploadRate),
	}
}

// unitOfWork returns the configured UnitOfWork. Resolvers built as literals
// in tests leave UOW nil and get an inline implementation, so every mutation
// still follows a single InTx code path.
func (r *Resolver) unitOfWork() dbtx.UnitOfWork {
	if r.UOW != nil {
		return r.UOW
	}
	return dbtx.Inline()
}

func (r *Resolver) ensureBG() {
	r.bgOnce.Do(func() {
		r.bgRunner = async.New(asyncWorkerCap)
	})
}

// runAsync submits fn to the bounded background runner. It reports whether
// the task was accepted: when the pool is saturated the task is dropped and
// logged, and callers that promised the user queued work (e.g. nutrition
// OCR) must surface a BUSY error instead of returning true. Best-effort
// analytics callers may ignore the result.
func (r *Resolver) runAsync(name string, timeout time.Duration, fn func(ctx context.Context) error) bool {
	r.ensureBG()
	return r.bgRunner.Submit(name, timeout, fn)
}

// acquireOCRJob reserves one of the per-user OCR slots. It returns nil if
// the user already holds the maximum number of in-flight jobs; otherwise
// it returns a release function the caller must invoke when the job ends.
func (r *Resolver) acquireOCRJob(userID int64) func() {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.ocrInFlight == nil {
		r.ocrInFlight = map[int64]int{}
	}
	if r.ocrInFlight[userID] >= maxUserOCRJobs {
		return nil
	}
	r.ocrInFlight[userID]++
	return func() {
		r.ocrMu.Lock()
		defer r.ocrMu.Unlock()
		if r.ocrInFlight[userID]--; r.ocrInFlight[userID] <= 0 {
			delete(r.ocrInFlight, userID)
		}
	}
}

// uploadLimiter returns the per-user upload rate limiter, lazily built so
// Resolver literals in tests still work.
func (r *Resolver) uploadLimiter() *userRateLimiter {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.uploads == nil {
		r.uploads = newUserRateLimiter(12)
	}
	return r.uploads
}

// Shutdown drains in-flight background work before returning: it waits
// for the analytics/recommendation workers and the recipe-import pool to
// finish, and only cancels the shared background context when the caller's
// deadline expires — in-flight tasks are never aborted just because
// shutdown was requested. Call it after HTTP drain and before pool close.
func (r *Resolver) Shutdown(ctx context.Context) error {
	r.ensureBG()
	done := make(chan struct{})
	go func() {
		err := r.bgRunner.Shutdown(ctx)
		if err == nil && r.RecipeImportService != nil {
			err = r.RecipeImportService.Shutdown(ctx)
		}
		if err != nil {
			slog.Default().Error("background shutdown incomplete", "error", err)
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func userFromContext(ctx context.Context) (currentuser.User, error) {
	u, ok := currentuser.FromContext(ctx)
	if !ok {
		return currentuser.User{}, errUnauthenticated()
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
		return currentuser.User{}, errForbidden()
	}
	return u, nil
}

func parseID(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, badInputf("invalid id %q", s)
	}
	return v, nil
}

// recordEventAsync emits an analytics event on the bounded background
// worker so that tracking never blocks or breaks the caller.
func (r *Resolver) recordEventAsync(userID int64, by string, e analytics.Event) {
	e.UserID = userID
	r.runAsync("record analytics event", 5*time.Second, func(ctx context.Context) error {
		return r.AnalyticsService.RecordEvent(ctx, e, by)
	})
}

// computeOverlapAsync recomputes ingredient-overlap recommendations for a
// newly created recipe on the bounded background worker so recipe
// creation stays fast. If recommendation volume outgrows this in-process
// approach, it should move to a proper job queue.
func (r *Resolver) computeOverlapAsync(newRecipeID int64) {
	if r.AnalyticsService == nil {
		return
	}
	r.runAsync("compute ingredient overlap suggestions", 30*time.Second, func(ctx context.Context) error {
		_, err := r.AnalyticsService.ComputeIngredientOverlapSuggestions(ctx, newRecipeID)
		return err
	})
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

// coalesce returns next when set, otherwise cur — for PATCH-style inputs
// where a nil field means "leave unchanged".
func coalesce[T any](cur T, next *T) T {
	if next != nil {
		return *next
	}
	return cur
}

// coalescePtr is coalesce for nullable (pointer) fields.
func coalescePtr[T any](cur *T, next *T) *T {
	if next != nil {
		return next
	}
	return cur
}

// coalesceID is coalesce for GraphQL ID input fields: nil input keeps the
// existing id, non-nil is parsed.
func coalesceID(cur int64, next *graphql.ID) (int64, error) {
	if next == nil {
		return cur, nil
	}
	return parseID(string(*next))
}

// coalesceOptionalID is coalesceID for nullable int64 fields.
func coalesceOptionalID(cur *int64, next *graphql.ID) (*int64, error) {
	if next == nil {
		return cur, nil
	}
	return optionalID(next)
}

// coalesceCheckedInt16 is coalesce for range-checked *int16 fields.
func coalesceCheckedInt16(cur *int16, next *int32, field string, lo, hi int16) (*int16, error) {
	if next == nil {
		return cur, nil
	}
	return checkedInt16Ptr(next, field, lo, hi)
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
// checkedInt16 converts a GraphQL int32 to int16, rejecting values outside
// [lo, hi] so out-of-range input can never wrap around into a "valid"
// int16 at the cast.
func checkedInt16(v int32, field string, lo, hi int16) (int16, error) {
	if v < int32(lo) || v > int32(hi) {
		return 0, badInputf("%s must be between %d and %d", field, lo, hi)
	}
	//nolint:gosec // v is range-checked against [lo, hi] immediately above.
	return int16(v), nil
}

// checkedInt16Ptr is checkedInt16 for nullable GraphQL int32 input,
// preserving nil.
func checkedInt16Ptr(v *int32, field string, lo, hi int16) (*int16, error) {
	if v == nil {
		return nil, nil
	}
	i, err := checkedInt16(*v, field, lo, hi)
	if err != nil {
		return nil, err
	}
	return &i, nil
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
	//nolint:gosec // n is saturated at MaxInt32 immediately above.
	return int32(n)
}

func clamp(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func timeToGraphQL(t *time.Time) *graphql.Time {
	if t == nil {
		return nil
	}
	return &graphql.Time{Time: *t}
}

// Me resolves the current authenticated user. When the identity service
// is wired the profile fields are read fresh from identity.users so role,
// name, and backup-email changes are reflected immediately.
func (r *Resolver) Me(ctx context.Context) (*userResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if r.IdentityService != nil {
		return r.userByID(ctx, u.UserID)
	}
	role := identity.RoleMember
	if u.IsAdmin {
		role = identity.RoleAdmin
	}
	return &userResolver{u: identity.User{
		UserID:      u.UserID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        role,
		IsActive:    true,
	}}, nil
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
	u         identity.User
	protected bool
}

func (r *userResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.u.UserID, 10)) }

func (r *userResolver) Email() string { return r.u.Email }

func (r *userResolver) DisplayName() *string { return nilIfEmpty(r.u.DisplayName) }

func (r *userResolver) FirstName() *string { return nilIfEmpty(r.u.FirstName) }

func (r *userResolver) LastName() *string { return nilIfEmpty(r.u.LastName) }

func (r *userResolver) BackupEmail() *string { return nilIfEmpty(r.u.BackupEmail) }

func (r *userResolver) Role() string { return r.u.Role }

func (r *userResolver) IsActive() bool { return r.u.IsActive }

func (r *userResolver) IsProtected() bool { return r.protected }

func (r *userResolver) LastLoginAt() *graphql.Time {
	if r.u.LastLoginAt == nil {
		return nil
	}
	return &graphql.Time{Time: *r.u.LastLoginAt}
}

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
	brands      map[int64]inventory.Brand
	categories  map[int64]inventory.Category
	nutrients   map[int64][]inventory.FoodNutrient
	flavors     map[int64][]inventory.FoodFlavor
	units       map[int64]inventory.Unit
	itemCounts  map[int64]countPair
	brandCounts map[int64]countPair
}

// loadUnits fetches a set of units in one query, keyed by ID.
func loadUnits(ctx context.Context, inv ItemReader, unitIDs []int64) (map[int64]inventory.Unit, error) {
	units := make(map[int64]inventory.Unit)
	if len(unitIDs) == 0 {
		return units, nil
	}
	rows, err := inv.GetUnitsByIDs(ctx, unitIDs)
	if err != nil {
		return nil, err
	}
	for _, u := range rows {
		units[u.UnitID] = u
	}
	return units, nil
}

// resolveUnitID maps a unit name or abbreviation (e.g. "cup", "c") to its
// canonical unit_id. Unknown units are rejected rather than stored.
func resolveUnitID(ctx context.Context, inv ItemReader, name string) (int64, error) {
	u, err := inv.GetUnitByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return 0, badInputf("unknown unit %q", name)
	}
	return u.UnitID, nil
}

// unitName renders a unit's display name from a preloaded map, falling back
// to a lazy service call when units is nil or misses the id.
func unitName(ctx context.Context, inv ItemReader, units map[int64]inventory.Unit, unitID int64) (string, error) {
	if units != nil {
		if u, ok := units[unitID]; ok {
			return u.Name, nil
		}
		slog.Default().Warn("unit missing from preloaded set; lazy-loading", "unit_id", unitID)
	}
	u, err := inv.GetUnitByID(ctx, unitID)
	if err != nil {
		return "", err
	}
	return u.Name, nil
}

// loadIngredients fetches a set of brand-agnostic ingredients in one query,
// keyed by ID.
func loadIngredients(ctx context.Context, inv ItemReader, ingredientIDs []int64) (map[int64]inventory.Ingredient, error) {
	ingredients := make(map[int64]inventory.Ingredient)
	if len(ingredientIDs) == 0 {
		return ingredients, nil
	}
	rows, err := inv.GetIngredientsByIDs(ctx, ingredientIDs)
	if err != nil {
		return nil, err
	}
	for _, in := range rows {
		ingredients[in.IngredientID] = in
	}
	return ingredients, nil
}

// unitNamePtr is unitName for nullable unit references (e.g. grocery list
// items where the unit may be unset).
func unitNamePtr(ctx context.Context, inv ItemReader, units map[int64]inventory.Unit, unitID *int64) (*string, error) {
	if unitID == nil {
		return nil, nil
	}
	name, err := unitName(ctx, inv, units, *unitID)
	if err != nil {
		return nil, err
	}
	return &name, nil
}

// loadItems fetches a set of catalog items in one query, keyed by ID.
func loadItems(ctx context.Context, inv ItemReader, itemIDs []int64) (map[int64]inventory.Item, error) {
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
func loadItemChildren(ctx context.Context, inv ItemReader, items []inventory.Item) (*itemChildren, error) {
	ch := &itemChildren{
		brands:      make(map[int64]inventory.Brand),
		categories:  make(map[int64]inventory.Category),
		nutrients:   make(map[int64][]inventory.FoodNutrient),
		flavors:     make(map[int64][]inventory.FoodFlavor),
		units:       make(map[int64]inventory.Unit),
		itemCounts:  make(map[int64]countPair),
		brandCounts: make(map[int64]countPair),
	}
	if len(items) == 0 {
		return ch, nil
	}
	itemIDs := make([]int64, len(items))
	unitIDSet := make(map[int64]bool)
	brandIDSet := make(map[int64]bool)
	categoryIDSet := make(map[int64]bool)
	for i, it := range items {
		itemIDs[i] = it.ItemID
		if it.BrandID != nil {
			brandIDSet[*it.BrandID] = true
		}
		categoryIDSet[it.CategoryID] = true
		if it.UnitID != 0 {
			unitIDSet[it.UnitID] = true
		}
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
	unitIDs := make([]int64, 0, len(unitIDSet))
	for id := range unitIDSet {
		unitIDs = append(unitIDs, id)
	}
	slices.Sort(unitIDs)
	ch.units, err = loadUnits(ctx, inv, unitIDs)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// countPair holds the global and per-user selection counts for a catalog
// entity. The zero value means no count data was loaded.
type countPair struct {
	global   int64
	personal int64
}

// recipeChildren holds recipe rows batch-loaded for a list response so
// nested recipe field resolvers do not issue a query per row.
type recipeChildren struct {
	recipes      map[int64]recipe.Recipe
	itemsBy      map[int64][]recipe.RecipeItem
	stepsBy      map[int64][]recipe.RecipeStep
	favorites    map[int64]bool
	items        map[int64]inventory.Item
	ingredients  map[int64]inventory.Ingredient
	itemChildren *itemChildren
	units        map[int64]inventory.Unit
	recipeCounts map[int64]countPair
	myRatings    map[int64]int16
	summaries    map[int64]recipe.RatingSummary
}

// loadRecipeChildren batch-loads the recipes, their items and steps, the
// current user's favorite flags, and the catalog rows for every item those
// recipes reference. The returned itemID set is merged into extraItemIDs so
// callers can also resolve items referenced from elsewhere (e.g. meal slot
// overrides) with the same maps.
func loadRecipeChildren(ctx context.Context, rec RecipeService, up UserPrefsService, inv ItemReader, userID int64, recipeIDs, extraItemIDs []int64) (*recipeChildren, error) {
	rc := &recipeChildren{
		recipes:      make(map[int64]recipe.Recipe),
		itemsBy:      make(map[int64][]recipe.RecipeItem),
		stepsBy:      make(map[int64][]recipe.RecipeStep),
		favorites:    make(map[int64]bool),
		units:        make(map[int64]inventory.Unit),
		recipeCounts: make(map[int64]countPair),
		myRatings:    make(map[int64]int16),
		summaries:    make(map[int64]recipe.RatingSummary),
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
		if up != nil {
			favs, err := up.ListRecipeFavorites(ctx, userID, recipeIDs)
			if err != nil {
				return nil, err
			}
			for _, f := range favs {
				rc.favorites[f.RecipeID] = f.IsFavorite
			}
		}
		if err := loadRecipeRatings(ctx, rec, userID, recipeIDs, rc); err != nil {
			return nil, err
		}
	}
	if err := loadRecipeInventoryChildren(ctx, inv, rc, extraItemIDs); err != nil {
		return nil, err
	}
	return rc, nil
}

// loadRecipeInventoryChildren batch-loads the inventory-side children
// referenced by rc.itemsBy (plus any extra item IDs): catalog items,
// their children, recipe-item units, and brand-agnostic ingredients.
func loadRecipeInventoryChildren(ctx context.Context, inv ItemReader, rc *recipeChildren, extraItemIDs []int64) error {
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
	if inv == nil {
		return nil
	}
	items, err := loadItems(ctx, inv, itemIDs)
	if err != nil {
		return err
	}
	rc.items = items
	list := make([]inventory.Item, 0, len(items))
	for _, it := range items {
		list = append(list, it)
	}
	ch, err := loadItemChildren(ctx, inv, list)
	if err != nil {
		return err
	}
	rc.itemChildren = ch
	// Recipe items carry their own unit_id; preload them alongside the
	// catalog-item units so unit display never issues a query per row.
	unitIDSet := make(map[int64]bool)
	for _, items := range rc.itemsBy {
		for _, ri := range items {
			if ri.UnitID != 0 {
				unitIDSet[ri.UnitID] = true
			}
		}
	}
	unitIDs := make([]int64, 0, len(unitIDSet))
	for id := range unitIDSet {
		unitIDs = append(unitIDs, id)
	}
	slices.Sort(unitIDs)
	rc.units, err = loadUnits(ctx, inv, unitIDs)
	if err != nil {
		return err
	}
	// Recipe items may reference brand-agnostic ingredients; batch-load
	// them so nested ingredient resolvers never issue a query per row.
	ingredientIDSet := make(map[int64]bool)
	for _, items := range rc.itemsBy {
		for _, ri := range items {
			if ri.IngredientID != nil {
				ingredientIDSet[*ri.IngredientID] = true
			}
		}
	}
	ingredientIDs := make([]int64, 0, len(ingredientIDSet))
	for id := range ingredientIDSet {
		ingredientIDs = append(ingredientIDs, id)
	}
	slices.Sort(ingredientIDs)
	rc.ingredients, err = loadIngredients(ctx, inv, ingredientIDs)
	if err != nil {
		return err
	}
	return nil
}

// loadItemSelectionCounts populates the per-user and global selection count
// maps on itemChildren for the given item IDs.
func loadItemSelectionCounts(ctx context.Context, an AnalyticsService, userID int64, itemIDs []int64, ch *itemChildren) error {
	if an == nil || len(itemIDs) == 0 {
		return nil
	}
	userCounts, err := an.GetUserSelectionCounts(ctx, userID, analytics.EntityItem, itemIDs)
	if err != nil {
		return fmt.Errorf("load item selection counts: %w", err)
	}
	globalCounts, err := an.GetGlobalSelectionCounts(ctx, analytics.EntityItem, itemIDs)
	if err != nil {
		return fmt.Errorf("load item selection counts: %w", err)
	}
	for _, c := range userCounts {
		p := ch.itemCounts[c.EntityID]
		p.personal = c.SelectCount
		ch.itemCounts[c.EntityID] = p
	}
	for _, c := range globalCounts {
		p := ch.itemCounts[c.EntityID]
		p.global = c.SelectCount
		ch.itemCounts[c.EntityID] = p
	}
	return nil
}

// loadBrandSelectionCounts populates the per-user and global selection count
// maps on itemChildren for the given brand IDs.
func loadBrandSelectionCounts(ctx context.Context, an AnalyticsService, userID int64, brandIDs []int64, ch *itemChildren) error {
	if an == nil || len(brandIDs) == 0 {
		return nil
	}
	userCounts, err := an.GetUserSelectionCounts(ctx, userID, analytics.EntityBrand, brandIDs)
	if err != nil {
		return fmt.Errorf("load brand selection counts: %w", err)
	}
	globalCounts, err := an.GetGlobalSelectionCounts(ctx, analytics.EntityBrand, brandIDs)
	if err != nil {
		return fmt.Errorf("load brand selection counts: %w", err)
	}
	for _, c := range userCounts {
		p := ch.brandCounts[c.EntityID]
		p.personal = c.SelectCount
		ch.brandCounts[c.EntityID] = p
	}
	for _, c := range globalCounts {
		p := ch.brandCounts[c.EntityID]
		p.global = c.SelectCount
		ch.brandCounts[c.EntityID] = p
	}
	return nil
}

// loadRecipeSelectionCounts populates the per-user and global selection count
// map on recipeChildren for the given recipe IDs.
func loadRecipeSelectionCounts(ctx context.Context, an AnalyticsService, userID int64, recipeIDs []int64, rc *recipeChildren) error {
	if an == nil || len(recipeIDs) == 0 {
		return nil
	}
	userCounts, err := an.GetUserSelectionCounts(ctx, userID, analytics.EntityRecipe, recipeIDs)
	if err != nil {
		return fmt.Errorf("load recipe selection counts: %w", err)
	}
	globalCounts, err := an.GetGlobalSelectionCounts(ctx, analytics.EntityRecipe, recipeIDs)
	if err != nil {
		return fmt.Errorf("load recipe selection counts: %w", err)
	}
	for _, c := range userCounts {
		p := rc.recipeCounts[c.EntityID]
		p.personal = c.SelectCount
		rc.recipeCounts[c.EntityID] = p
	}
	for _, c := range globalCounts {
		p := rc.recipeCounts[c.EntityID]
		p.global = c.SelectCount
		rc.recipeCounts[c.EntityID] = p
	}
	return nil
}

// loadRecipeRatings populates the per-user rating and aggregate rating
// summary maps on recipeChildren for the given recipe IDs.
func loadRecipeRatings(ctx context.Context, rec RecipeService, userID int64, recipeIDs []int64, rc *recipeChildren) error {
	if rec == nil || len(recipeIDs) == 0 {
		return nil
	}
	mine, err := rec.ListRecipeRatings(ctx, userID, recipeIDs)
	if err != nil {
		return fmt.Errorf("load recipe ratings: %w", err)
	}
	for _, m := range mine {
		rc.myRatings[m.RecipeID] = m.Rating
	}
	summaries, err := rec.ListRatingSummaries(ctx, recipeIDs)
	if err != nil {
		return fmt.Errorf("load recipe ratings: %w", err)
	}
	for _, s := range summaries {
		rc.summaries[s.RecipeID] = s
	}
	return nil
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
func loadBottleChildren(ctx context.Context, wineSvc BottleReader, bottleIDs []int64, includeBottles bool) (*bottleChildren, error) {
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

// errQueryTimeout marks requests cancelled by the handler's deadline; the
// handler maps it onto the TIMEOUT GraphQL error code after Exec returns.
var errQueryTimeout = errors.New("graphql request timed out")

// costLimiter counts field resolutions for one request via TraceField and
// cancels the execution context once the budget is exceeded.
type costLimiter struct {
	max      int
	count    atomic.Int64
	exceeded atomic.Bool
	fire     context.CancelFunc
}

func (l *costLimiter) add() {
	if l.max <= 0 || l.fire == nil {
		return
	}
	if l.count.Add(1) > int64(l.max) {
		l.exceeded.Store(true)
		l.fire()
	}
}

type costLimiterContextKey struct{}

// NewGraphQLHandler returns an Echo handler that executes GraphQL requests.
// timeout bounds each execution (empty means use the default), maxCost caps
// the number of field resolutions per request (<= 0 disables the budget),
// and extra schema options (e.g. graphql.MaxDepth, graphql.MaxQueryLength)
// are applied on top of the built-in tracer. A schema parse failure is
// returned as an error rather than panicking.
func NewGraphQLHandler(r *Resolver, timeout time.Duration, maxCost int, schemaOpts ...graphql.SchemaOpt) (echo.HandlerFunc, error) {
	opts := append([]graphql.SchemaOpt{graphql.Tracer(newGraphQLTracer())}, schemaOpts...)
	parsed, err := graphql.ParseSchema(schema, r, opts...)
	if err != nil {
		return nil, fmt.Errorf("parse graphql schema: %w", err)
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return func(c echo.Context) error {
		var req struct {
			Query         string                 `json:"query"`
			Variables     map[string]interface{} `json:"variables"`
			OperationName string                 `json:"operationName"`
		}
		if err := c.Bind(&req); err != nil {
			// Return bind failures in GraphQL error shape so clients get a
			// consistent contract instead of an Echo HTML error page.
			return c.JSON(http.StatusOK, map[string]any{
				"errors": []map[string]any{{
					"message":    "invalid request body",
					"extensions": map[string]any{"code": codeBadUserInput},
				}},
			})
		}
		ctx, cancel := context.WithTimeoutCause(c.Request().Context(), timeout, errQueryTimeout)
		limiter := &costLimiter{max: maxCost, fire: cancel}
		ctx = context.WithValue(ctx, costLimiterContextKey{}, limiter)

		resp := parsed.Exec(ctx, req.Query, req.OperationName, req.Variables)
		cancel()
		sanitizeQueryErrors(resp.Errors, c.Response().Header().Get(echo.HeaderXRequestID))
		applyLimitErrors(ctx, resp, limiter)
		return c.JSON(http.StatusOK, resp)
	}, nil
}

// applyLimitErrors appends the deadline/cost GraphQL error when the
// request exceeded either bound.
func applyLimitErrors(ctx context.Context, resp *graphql.Response, limiter *costLimiter) {
	switch {
	case errors.Is(context.Cause(ctx), errQueryTimeout):
		resp.Errors = append(resp.Errors, &gqlerrors.QueryError{
			Message:    "request timed out",
			Extensions: map[string]any{"code": codeTimeout},
		})
	case limiter.exceeded.Load():
		resp.Errors = append(resp.Errors, &gqlerrors.QueryError{
			Message:    "query exceeds cost budget",
			Extensions: map[string]any{"code": codeCostExceeded},
		})
	}
}

// pageArgs clamps pagination input to sane bounds; every list resolver
// funnels through it so no unbounded LIMIT/OFFSET reaches the database.
func pageArgs(page, pageSize int32) (int32, int32) {
	return clamp(page, 1, 1_000_000), clamp(pageSize, 1, 100)
}
