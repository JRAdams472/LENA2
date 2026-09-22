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

	// bg carries detached analytics/recommendation work: at most
	// asyncWorkerCap in-flight goroutines, all scoped to a cancelable
	// context that Shutdown drains. Lazily initialized so tests can keep
	// constructing Resolver literals.
	bgOnce   sync.Once
	bgCtx    context.Context
	bgCancel context.CancelFunc
	bgSem    chan struct{}
	bgWG     sync.WaitGroup

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

// NewResolver returns a new BFF resolver with the domain services.
func NewResolver(pool dbtx.Pool, an AnalyticsService, gr GroceryService, inv InventoryService, mp MealPlanService, rec RecipeService, up UserPrefsService, wineSvc WineService, idn IdentityService, recipeImport RecipeImportService, ocr OCRClient, nutritionPhotoMaxBytes, recipeScanMaxBytes int) *Resolver {
	return &Resolver{UOW: dbtx.NewUnitOfWork(pool), AnalyticsService: an, GroceryService: gr, InventoryService: inv, MealPlanService: mp, RecipeService: rec, UserPrefsService: up, WineService: wineSvc, IdentityService: idn, RecipeImportService: recipeImport, OCRClient: ocr, NutritionPhotoMaxBytes: nutritionPhotoMaxBytes, RecipeScanMaxBytes: recipeScanMaxBytes, uploads: newUserRateLimiter(12)}
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
		r.bgCtx, r.bgCancel = context.WithCancel(context.Background())
		r.bgSem = make(chan struct{}, asyncWorkerCap)
	})
}

// runAsync executes fn in a detached, bounded, shutdown-aware goroutine.
// It reports whether the task was accepted: when the worker pool is
// saturated the task is dropped and logged, and callers that promised the
// user queued work (e.g. nutrition OCR) must surface a BUSY error instead
// of returning true. Best-effort analytics callers may ignore the result.
func (r *Resolver) runAsync(name string, timeout time.Duration, fn func(ctx context.Context) error) bool {
	r.ensureBG()
	select {
	case r.bgSem <- struct{}{}:
	default:
		slog.Default().Warn("async worker saturated; dropping task", "task", name)
		return false
	}
	r.bgWG.Add(1)
	go func() {
		defer r.bgWG.Done()
		defer func() { <-r.bgSem }()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Default().Error("async task panic recovered", "task", name, "recover", rec)
			}
		}()
		ctx, cancel := context.WithTimeout(r.bgCtx, timeout)
		defer cancel()
		if err := fn(ctx); err != nil {
			slog.Default().Error("async task failed", "task", name, "error", err)
		}
	}()
	return true
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
		r.bgWG.Wait()
		if r.RecipeImportService != nil {
			if err := r.RecipeImportService.Shutdown(ctx); err != nil {
				slog.Default().Error("recipe import service shutdown", "error", err)
			}
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		r.bgCancel()
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
func loadUnits(ctx context.Context, inv InventoryService, unitIDs []int64) (map[int64]inventory.Unit, error) {
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
func resolveUnitID(ctx context.Context, inv InventoryService, name string) (int64, error) {
	u, err := inv.GetUnitByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return 0, badInputf("unknown unit %q", name)
	}
	return u.UnitID, nil
}

// unitName renders a unit's display name from a preloaded map, falling back
// to a lazy service call when units is nil.
func unitName(ctx context.Context, inv InventoryService, units map[int64]inventory.Unit, unitID int64) (string, error) {
	if units != nil {
		if u, ok := units[unitID]; ok {
			return u.Name, nil
		}
		return "", fmt.Errorf("unit %d missing from preloaded set", unitID)
	}
	u, err := inv.GetUnitByID(ctx, unitID)
	if err != nil {
		return "", err
	}
	return u.Name, nil
}

// unitNamePtr is unitName for nullable unit references (e.g. grocery list
// items where the unit may be unset).
func unitNamePtr(ctx context.Context, inv InventoryService, units map[int64]inventory.Unit, unitID *int64) (*string, error) {
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
func loadRecipeChildren(ctx context.Context, rec RecipeService, up UserPrefsService, inv InventoryService, userID int64, recipeIDs, extraItemIDs []int64) (*recipeChildren, error) {
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
		favs, err := up.ListRecipeFavorites(ctx, userID, recipeIDs)
		if err != nil {
			return nil, err
		}
		for _, f := range favs {
			rc.favorites[f.RecipeID] = f.IsFavorite
		}
		if err := loadRecipeRatings(ctx, rec, userID, recipeIDs, rc); err != nil {
			return nil, err
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
		return nil, err
	}
	return rc, nil
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

// errQueryTimeout and errQueryCostExceeded are context causes set by the
// handler's deadline and the tracer's cost counter; the handler maps them
// onto GraphQL error codes after Exec returns.
var (
	errQueryTimeout      = errors.New("graphql request timed out")
	errQueryCostExceeded = errors.New("graphql query exceeds cost budget")
)

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
		return c.JSON(http.StatusOK, resp)
	}, nil
}

// pageArgs clamps pagination input to sane bounds; every list resolver
// funnels through it so no unbounded LIMIT/OFFSET reaches the database.
func pageArgs(page, pageSize int32) (int32, int32) {
	return clamp(page, 1, 1_000_000), clamp(pageSize, 1, 100)
}
