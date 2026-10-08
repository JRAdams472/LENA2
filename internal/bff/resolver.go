package bff

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
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
	EventService           EventService
	RecipeService          RecipeService
	UserPrefsService       UserPrefsService
	WineService            WineService
	IdentityService        IdentityService
	RecipeImportService    RecipeImportService
	HouseholdService       HouseholdService
	NotifierService        NotifierService
	AuthInvalidator        AuthInvalidator
	AIService              AIService
	OCRClient              OCRClient
	ShoppingClient         ShoppingLinkClient
	RecipeEmbedder         RecipeEmbedder
	IdemStore              IdempotencyStore
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
	ocrMu         sync.Mutex
	ocrInFlight   map[int64]int
	uploads       *userRateLimiter
	aiCalls       *userRateLimiter
	aiToolCalls   *userRateLimiter
	invites       *userRateLimiter
	shoppingLinks *userRateLimiter
}

// asyncWorkerCap bounds the number of in-flight background tasks.
const asyncWorkerCap = 16

// maxUserOCRJobs bounds in-flight nutrition-OCR jobs per user.
const maxUserOCRJobs = 1

// Services is the set of domain interfaces the BFF orchestrates across.
// Field types are role interfaces so consumers state the minimum surface
// they need; the concrete domain services satisfy them all.
type Services struct {
	Analytics      AnalyticsService
	Grocery        GroceryService
	Inventory      InventoryService
	MealPlan       MealPlanService
	Event          EventService
	Recipe         RecipeService
	UserPrefs      UserPrefsService
	Wine           WineService
	Identity       IdentityService
	RecipeImport   RecipeImportService
	Household      HouseholdService
	Notifier       NotifierService
	Auth           AuthInvalidator
	AI             AIService
	OCR            OCRClient
	Shopping       ShoppingLinkClient
	RecipeEmbedder RecipeEmbedder
}

// Options carries resolver limits and tunables out of the constructor so
// adding a knob does not grow the parameter list again.
type Options struct {
	NutritionPhotoMaxBytes int
	RecipeScanMaxBytes     int
	// UploadRatePerMinute bounds upload mutations per user; <=0 uses the
	// built-in default.
	UploadRatePerMinute int
	// Idempotency enables request dedup for mutations when non-nil.
	Idempotency IdempotencyStore
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
		EventService:           svc.Event,
		RecipeService:          svc.Recipe,
		UserPrefsService:       svc.UserPrefs,
		WineService:            svc.Wine,
		IdentityService:        svc.Identity,
		RecipeImportService:    svc.RecipeImport,
		HouseholdService:       svc.Household,
		NotifierService:        svc.Notifier,
		AuthInvalidator:        svc.Auth,
		AIService:              svc.AI,
		OCRClient:              svc.OCR,
		ShoppingClient:         svc.Shopping,
		RecipeEmbedder:         svc.RecipeEmbedder,
		NutritionPhotoMaxBytes: opts.NutritionPhotoMaxBytes,
		RecipeScanMaxBytes:     opts.RecipeScanMaxBytes,
		IdemStore:              opts.Idempotency,
		uploads:                newUserRateLimiter(uploadRate),
		invites:                newUserRateLimiter(10),
		shoppingLinks:          newUserRateLimiter(10),
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

// inviteLimiter returns the per-user invite rate limiter, lazily built so
// Resolver literals in tests still work. Throttles household-invite
// creation so sequential user-ID probing (LEN-29 finding 3) is bounded.
func (r *Resolver) inviteLimiter() *userRateLimiter {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.invites == nil {
		r.invites = newUserRateLimiter(10)
	}
	return r.invites
}

// shoppingLimiter bounds createShoppingLink calls per user so a busy
// client cannot burn through the deployment's IDP quota. Lazily built so
// Resolver literals in tests still work.
func (r *Resolver) shoppingLimiter() *userRateLimiter {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.shoppingLinks == nil {
		r.shoppingLinks = newUserRateLimiter(10)
	}
	return r.shoppingLinks
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
		if r.NotifierService != nil {
			r.NotifierService.Stop()
		}
		if r.AnalyticsService != nil {
			r.AnalyticsService.Stop()
		}
		if err == nil && r.RecipeImportService != nil {
			err = r.RecipeImportService.Shutdown(ctx)
		}
		if s, ok := r.RecipeEmbedder.(interface{ Stop() }); ok {
			s.Stop()
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

// parseIDs converts a GraphQL ID list; any malformed entry fails the call.
func parseIDs(ids []graphql.ID) ([]int64, error) {
	out := make([]int64, len(ids))
	for i, id := range ids {
		v, err := parseID(string(id))
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// recordEventAsync emits an analytics event on the bounded background
// worker so that tracking never blocks or breaks the caller.
func (r *Resolver) recordEventAsync(userID int64, by string, e analytics.Event) {
	if r.AnalyticsService == nil {
		return
	}
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

// refreshEmbeddingAsync re-embeds a recipe after a save on the bounded
// background worker: a slow or down embedding backend never blocks or
// breaks the write. Failures leave embedding NULL and the backfill sweep
// retries on its next pass.
func (r *Resolver) refreshEmbeddingAsync(recipeID int64) {
	if r.RecipeEmbedder == nil {
		return
	}
	r.runAsync("refresh recipe embedding", 45*time.Second, func(ctx context.Context) error {
		return r.RecipeEmbedder.Refresh(ctx, recipeID)
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
	gt := graphqlTime(*t)
	return &gt
}

// graphqlTime normalizes timestamps emitted over GraphQL: pgx returns
// timestamptz in the process-local zone, and normalizing to UTC keeps the
// wire format deterministic ("Z" suffix) regardless of host TZ.
func graphqlTime(t time.Time) graphql.Time {
	return graphql.Time{Time: t.UTC()}
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
	var householdID *int64
	if u.HouseholdID != 0 {
		householdID = &u.HouseholdID
	}
	return &userResolver{u: identity.User{
		UserID:       u.UserID,
		Email:        u.Email,
		DisplayName:  u.DisplayName,
		Role:         role,
		IsActive:     true,
		HouseholdID:  householdID,
		IsSearchable: u.IsSearchable,
	}, root: r}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	return b != nil && *b
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

// userResolver resolves User fields. root gives field resolvers access to
// services for self-gated lookups like household membership.
type userResolver struct {
	u         identity.User
	protected bool
	root      *Resolver
}

func (r *userResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.u.UserID, 10)) }

func (r *userResolver) Email() string { return r.u.Email }

func (r *userResolver) DisplayName() *string { return nilIfEmpty(r.u.DisplayName) }

func (r *userResolver) FirstName() *string { return nilIfEmpty(r.u.FirstName) }

func (r *userResolver) LastName() *string { return nilIfEmpty(r.u.LastName) }

func (r *userResolver) BackupEmail() *string { return nilIfEmpty(r.u.BackupEmail) }

// Birthdate is YYYY-MM-DD — a date-only string, not a timestamp, so the
// user's zone can't shift the day.
func (r *userResolver) Birthdate() *string {
	if r.u.Birthdate == nil {
		return nil
	}
	s := r.u.Birthdate.Format(time.DateOnly)
	return &s
}

func (r *userResolver) Role() string { return r.u.Role }

func (r *userResolver) IsActive() bool { return r.u.IsActive }

func (r *userResolver) IsProtected() bool { return r.protected }

func (r *userResolver) LastLoginAt() *graphql.Time {
	if r.u.LastLoginAt == nil {
		return nil
	}
	return timeToGraphQL(r.u.LastLoginAt)
}

func (r *userResolver) IsSearchable() bool { return r.u.IsSearchable }

// Household resolves the user's household only when the resolved user is
// the caller — admin user lists and other-user views never expose
// household membership.
func (r *userResolver) Household(ctx context.Context) (*householdResolver, error) {
	if r.root == nil || r.root.HouseholdService == nil || r.u.HouseholdID == nil {
		return nil, nil
	}
	caller, err := userFromContext(ctx)
	if err != nil || caller.UserID != r.u.UserID {
		return nil, nil
	}
	return r.root.householdWithMembers(ctx, *r.u.HouseholdID, r.u.UserID, *r.u.HouseholdID)
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

// intersectIDs returns the IDs present in both slices, preserving a's
// order. Used when a listing can be scoped by several ID sets (search
// term, brand, explicit IDs) — the intersection is the only correct
// combination since each set is an AND filter.
func intersectIDs(a, b []int64) []int64 {
	in := make(map[int64]bool, len(b))
	for _, id := range b {
		in[id] = true
	}
	out := make([]int64, 0, len(a))
	for _, id := range a {
		if in[id] {
			out = append(out, id)
		}
	}
	return out
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
	// ingredients covers catalog links and override targets alike;
	// resolved maps item_id -> effective ingredient (override wins).
	ingredients map[int64]inventory.Ingredient
	resolved    map[int64]*int64
	// ac holds the batch-loaded allergen knowledge (flags, registry rows,
	// member records); as is the lazy-load source for resolvers built
	// without this preload.
	ac *allergyContext
	as *allergySource
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

// sortedIDs returns m's keys in ascending order so the batch lookups that
// follow always see a deterministic input list.
func sortedIDs[V any](m map[int64]V) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func newItemChildren() *itemChildren {
	return &itemChildren{
		brands:      make(map[int64]inventory.Brand),
		categories:  make(map[int64]inventory.Category),
		nutrients:   make(map[int64][]inventory.FoodNutrient),
		flavors:     make(map[int64][]inventory.FoodFlavor),
		units:       make(map[int64]inventory.Unit),
		itemCounts:  make(map[int64]countPair),
		brandCounts: make(map[int64]countPair),
		ingredients: make(map[int64]inventory.Ingredient),
		resolved:    make(map[int64]*int64),
	}
}

// itemIDSets holds the sorted ID collections derived from a set of
// catalog items for the child batch loads.
type itemIDSets struct {
	itemIDs     []int64
	brandIDs    []int64
	categoryIDs []int64
	unitIDs     []int64
}

func collectItemIDSets(items []inventory.Item) itemIDSets {
	itemIDs := make([]int64, len(items))
	unitSet := make(map[int64]bool)
	brandSet := make(map[int64]bool)
	categorySet := make(map[int64]bool)
	for i, it := range items {
		itemIDs[i] = it.ItemID
		if it.BrandID != nil {
			brandSet[*it.BrandID] = true
		}
		categorySet[it.CategoryID] = true
		if it.UnitID != 0 {
			unitSet[it.UnitID] = true
		}
	}
	slices.Sort(itemIDs)
	return itemIDSets{
		itemIDs:     itemIDs,
		brandIDs:    sortedIDs(brandSet),
		categoryIDs: sortedIDs(categorySet),
		unitIDs:     sortedIDs(unitSet),
	}
}

// loadCatalogRows batch-loads the item children keyed off the items' own
// foreign keys: brands, categories, nutrients, flavors and units.
func (ch *itemChildren) loadCatalogRows(ctx context.Context, inv ItemReader, sets itemIDSets) error {
	if len(sets.brandIDs) > 0 {
		brands, err := inv.GetBrandsByIDs(ctx, sets.brandIDs)
		if err != nil {
			return err
		}
		for _, b := range brands {
			ch.brands[b.BrandID] = b
		}
	}
	categories, err := inv.GetCategoriesByIDs(ctx, sets.categoryIDs)
	if err != nil {
		return err
	}
	for _, c := range categories {
		ch.categories[c.CategoryID] = c
	}
	nutrients, err := inv.ListFoodNutrientsByItems(ctx, sets.itemIDs)
	if err != nil {
		return err
	}
	for _, n := range nutrients {
		ch.nutrients[n.ItemID] = append(ch.nutrients[n.ItemID], n)
	}
	flavors, err := inv.ListFoodFlavorsByItems(ctx, sets.itemIDs)
	if err != nil {
		return err
	}
	for _, f := range flavors {
		ch.flavors[f.ItemID] = append(ch.flavors[f.ItemID], f)
	}
	ch.units, err = loadUnits(ctx, inv, sets.unitIDs)
	return err
}

// mergedIngredientIDs unions each item's catalog link, the override-aware
// resolved ingredient, and caller-supplied extras into the sorted ID list
// the ingredient and flag batch loads share.
func mergedIngredientIDs(items []inventory.Item, resolved map[int64]*int64, extra []int64) []int64 {
	set := make(map[int64]bool)
	for _, it := range items {
		if it.IngredientID != nil {
			set[*it.IngredientID] = true
		}
	}
	for _, ingID := range resolved {
		if ingID != nil {
			set[*ingID] = true
		}
	}
	for _, ingID := range extra {
		set[ingID] = true
	}
	return sortedIDs(set)
}

// loadItemChildren batch-loads the brand, category, nutrient, flavor and
// ingredient rows referenced by items, plus the allergen knowledge needed
// to render flags and warnings. householdID scopes the override-aware
// item -> ingredient resolution and the member records (pass 0 — with nil
// id/up — for catalog-only loads like admin browse). extraIngredientIDs
// are entity-line ingredients not reachable through the items (recipe,
// slot, grocery and event lines) so their flags batch-load too.
func loadItemChildren(ctx context.Context, inv ItemReader, id IdentityService, up UserPrefsService, items []inventory.Item, householdID int64, extraIngredientIDs []int64) (*itemChildren, error) {
	ch := newItemChildren()
	if len(items) == 0 {
		// Ingredient-only entity sets (e.g. a recipe whose lines are all
		// ingredient-keyed) still need their flags and warnings.
		ch.as = &allergySource{inv: inv, id: id, up: up, householdID: householdID}
		var err error
		ch.ac, err = ch.as.loadAllergyContext(ctx, extraIngredientIDs, nil, nil)
		if err != nil {
			return nil, err
		}
		return ch, nil
	}
	sets := collectItemIDSets(items)
	if err := ch.loadCatalogRows(ctx, inv, sets); err != nil {
		return nil, err
	}
	// Resolved (override-aware) ingredients plus the catalog link itself —
	// the union feeds both the ingredient and householdIngredient fields.
	resolved, err := inv.ResolveItemIngredients(ctx, householdID, sets.itemIDs)
	if err != nil {
		return nil, err
	}
	ch.resolved = resolved
	ingredientIDs := mergedIngredientIDs(items, resolved, extraIngredientIDs)
	ch.ingredients, err = loadIngredients(ctx, inv, ingredientIDs)
	if err != nil {
		return nil, err
	}
	ch.as = &allergySource{inv: inv, id: id, up: up, householdID: householdID}
	ch.ac, err = ch.as.loadAllergyContext(ctx, ingredientIDs, sets.itemIDs, resolved)
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
// nested recipe field resolvers do not issue a query per row. itemsBy and
// stepsBy carry the delta-applied (household) view when a delta exists;
// the canonical rows live in canonItemsBy/canonStepsBy for
// view: canonical requests.
type recipeChildren struct {
	recipes      map[int64]recipe.Recipe
	itemsBy      map[int64][]recipe.RecipeItem
	stepsBy      map[int64][]recipe.RecipeStep
	canonItemsBy map[int64][]recipe.RecipeItem
	canonStepsBy map[int64][]recipe.RecipeStep
	deltas       map[int64]*recipe.RecipeDelta
	categoriesBy map[int64][]recipe.Category
	favorites    map[int64]bool
	items        map[int64]inventory.Item
	ingredients  map[int64]inventory.Ingredient
	itemChildren *itemChildren
	units        map[int64]inventory.Unit
	recipeCounts map[int64]countPair
	myRatings    map[int64]int16
	summaries    map[int64]recipe.RatingSummary
}

// recipeChildLoaders bundles the domain services the recipe child loaders
// read through, keeping the loader signatures short.
type recipeChildLoaders struct {
	rec RecipeService
	up  UserPrefsService
	id  IdentityService
	inv ItemReader
}

func (r *Resolver) childLoaders() recipeChildLoaders {
	return recipeChildLoaders{rec: r.RecipeService, up: r.UserPrefsService, id: r.IdentityService, inv: r.InventoryService}
}

func newRecipeChildren() *recipeChildren {
	return &recipeChildren{
		recipes:      make(map[int64]recipe.Recipe),
		itemsBy:      make(map[int64][]recipe.RecipeItem),
		stepsBy:      make(map[int64][]recipe.RecipeStep),
		canonItemsBy: make(map[int64][]recipe.RecipeItem),
		canonStepsBy: make(map[int64][]recipe.RecipeStep),
		favorites:    make(map[int64]bool),
		units:        make(map[int64]inventory.Unit),
		recipeCounts: make(map[int64]countPair),
		myRatings:    make(map[int64]int16),
		summaries:    make(map[int64]recipe.RatingSummary),
	}
}

// loadRecipeRows fills the recipe-side maps (recipes, items, steps,
// categories, favorites, ratings) for the given recipe IDs.
func (rc *recipeChildren) loadRecipeRows(ctx context.Context, l recipeChildLoaders, userID int64, recipeIDs []int64) error {
	recipes, err := l.rec.GetRecipesByIDs(ctx, recipeIDs)
	if err != nil {
		return err
	}
	for _, rcp := range recipes {
		rc.recipes[rcp.RecipeID] = rcp
	}
	items, err := l.rec.ListRecipeItemsByRecipes(ctx, recipeIDs)
	if err != nil {
		return err
	}
	for _, ri := range items {
		rc.itemsBy[ri.RecipeID] = append(rc.itemsBy[ri.RecipeID], ri)
	}
	steps, err := l.rec.ListRecipeStepsByRecipes(ctx, recipeIDs)
	if err != nil {
		return err
	}
	for _, s := range steps {
		rc.stepsBy[s.RecipeID] = append(rc.stepsBy[s.RecipeID], s)
	}
	cats, err := l.rec.ListCategoriesForRecipes(ctx, recipeIDs)
	if err != nil {
		return err
	}
	rc.categoriesBy = cats
	if l.up != nil {
		favs, err := l.up.ListRecipeFavorites(ctx, userID, recipeIDs)
		if err != nil {
			return err
		}
		for _, f := range favs {
			rc.favorites[f.RecipeID] = f.IsFavorite
		}
	}
	return loadRecipeRatings(ctx, l.rec, userID, recipeIDs, rc)
}

// loadRecipeChildren batch-loads the recipes, their items and steps, the
// current user's favorite flags, and the catalog rows for every item those
// recipes reference. The returned itemID set is merged into extraItemIDs so
// callers can also resolve items referenced from elsewhere (e.g. meal slot
// overrides) with the same maps. householdID scopes override-aware
// item -> ingredient resolution inside the batch-loaded item children.
// extraIngredientIDs are line ingredients outside recipe_item (slot/event
// lines) so their allergen flags batch-load too.
func loadRecipeChildren(ctx context.Context, l recipeChildLoaders, userID, householdID int64, recipeIDs, extraItemIDs, extraIngredientIDs []int64) (*recipeChildren, error) {
	rc := newRecipeChildren()
	if len(recipeIDs) > 0 {
		if err := rc.loadRecipeRows(ctx, l, userID, recipeIDs); err != nil {
			return nil, err
		}
		// Deltas apply before the inventory/allergen loads so every
		// downstream resolution sees the household's effective lines.
		if err := rc.applyDeltas(ctx, l.rec, householdID, recipeIDs); err != nil {
			return nil, err
		}
	}
	if err := loadRecipeInventoryChildren(ctx, l, rc, householdID, extraItemIDs, extraIngredientIDs); err != nil {
		return nil, err
	}
	return rc, nil
}

// applyDeltas loads the household's deltas for these recipes and folds
// them into itemsBy/stepsBy, keeping the canonical copies aside for
// view: canonical requests.
func (rc *recipeChildren) applyDeltas(ctx context.Context, rec RecipeDeltaStore, householdID int64, recipeIDs []int64) error {
	deltas, err := rec.ListRecipeDeltas(ctx, householdID, recipeIDs)
	if err != nil {
		return fmt.Errorf("load recipe deltas: %w", err)
	}
	rc.deltas = deltas
	for rid, d := range deltas {
		rc.canonItemsBy[rid] = rc.itemsBy[rid]
		rc.canonStepsBy[rid] = rc.stepsBy[rid]
		eff := recipe.ApplyDelta(rc.itemsBy[rid], rc.stepsBy[rid], d)
		rc.itemsBy[rid] = eff.Items
		rc.stepsBy[rid] = eff.Steps
	}
	return nil
}

// recipeItemIDs returns the sorted union of extra IDs and every recipe
// item's ItemID.
func recipeItemIDs(itemsBy map[int64][]recipe.RecipeItem, extra []int64) []int64 {
	set := make(map[int64]bool, len(extra))
	for _, id := range extra {
		set[id] = true
	}
	for _, items := range itemsBy {
		for _, ri := range items {
			if ri.ItemID != nil {
				set[*ri.ItemID] = true
			}
		}
	}
	return sortedIDs(set)
}

// recipeItemIngredientIDs returns the sorted union of extra IDs and every
// recipe item's IngredientID — the line ingredients that feed the allergen
// flag load even when no item binds them.
func recipeItemIngredientIDs(itemsBy map[int64][]recipe.RecipeItem, extra []int64) []int64 {
	set := make(map[int64]bool, len(extra))
	for _, id := range extra {
		set[id] = true
	}
	for _, items := range itemsBy {
		for _, ri := range items {
			if ri.IngredientID != nil {
				set[*ri.IngredientID] = true
			}
		}
	}
	return sortedIDs(set)
}

// recipeItemUnitIDs returns the sorted set of unit IDs carried by the
// recipe items themselves so unit display never issues a query per row.
func recipeItemUnitIDs(itemsBy map[int64][]recipe.RecipeItem) []int64 {
	set := make(map[int64]bool)
	for _, items := range itemsBy {
		for _, ri := range items {
			if ri.UnitID != 0 {
				set[ri.UnitID] = true
			}
		}
	}
	return sortedIDs(set)
}

// deltaUnitIDs unions the unit IDs carried on delta rows (adjust/add) so
// the delta editor resolves units without extra loads.
func deltaUnitIDs(deltas map[int64]*recipe.RecipeDelta, base []int64) []int64 {
	if len(deltas) == 0 {
		return base
	}
	set := make(map[int64]bool, len(base))
	for _, id := range base {
		set[id] = true
	}
	for _, d := range deltas {
		for _, di := range d.Items {
			if di.UnitID != nil {
				set[*di.UnitID] = true
			}
		}
	}
	return sortedIDs(set)
}

// mergedItemsBy returns the union of effective and canonical line maps —
// canonical IDs for a recipe append after that recipe's effective lines.
func mergedItemsBy(itemsBy, canonItemsBy map[int64][]recipe.RecipeItem) map[int64][]recipe.RecipeItem {
	if len(canonItemsBy) == 0 {
		return itemsBy
	}
	out := make(map[int64][]recipe.RecipeItem, len(itemsBy)+len(canonItemsBy))
	for rid, items := range itemsBy {
		out[rid] = items
	}
	for rid, canon := range canonItemsBy {
		out[rid] = append(out[rid], canon...)
	}
	return out
}

// deltaRefIDs appends the item/ingredient IDs delta rows reference —
// substitute/add targets and orphaned payloads.
func deltaRefIDs(deltas map[int64]*recipe.RecipeDelta, itemIDs, ingredientIDs []int64) ([]int64, []int64) {
	for _, d := range deltas {
		for _, di := range d.Items {
			if di.ItemID != nil {
				itemIDs = append(itemIDs, *di.ItemID)
			}
			if di.IngredientID != nil {
				ingredientIDs = append(ingredientIDs, *di.IngredientID)
			}
		}
	}
	return itemIDs, ingredientIDs
}

// loadRecipeInventoryChildren batch-loads the inventory-side children
// referenced by rc.itemsBy (plus any extra item/ingredient IDs): catalog
// items, their children, recipe-item units, and brand-agnostic
// ingredients.
func loadRecipeInventoryChildren(ctx context.Context, l recipeChildLoaders, rc *recipeChildren, householdID int64, extraItemIDs, extraIngredientIDs []int64) error {
	if l.inv == nil {
		return nil
	}
	// Canonical lines and delta rows can reference items/ingredients the
	// effective list dropped (substituted or removed lines, orphaned
	// tweaks). Union every referenced ID into the preload so
	// view: canonical and the delta rows resolve without lazy loads.
	allItemsBy := mergedItemsBy(rc.itemsBy, rc.canonItemsBy)
	extraItemIDs, extraIngredientIDs = deltaRefIDs(rc.deltas, extraItemIDs, extraIngredientIDs)
	itemIDs := recipeItemIDs(allItemsBy, extraItemIDs)
	items, err := loadItems(ctx, l.inv, itemIDs)
	if err != nil {
		return err
	}
	rc.items = items
	list := make([]inventory.Item, 0, len(items))
	for _, it := range items {
		list = append(list, it)
	}
	ch, err := loadItemChildren(ctx, l.inv, l.id, l.up, list, householdID, recipeItemIngredientIDs(allItemsBy, extraIngredientIDs))
	if err != nil {
		return err
	}
	rc.itemChildren = ch
	rc.units, err = loadUnits(ctx, l.inv, deltaUnitIDs(rc.deltas, recipeItemUnitIDs(allItemsBy)))
	if err != nil {
		return err
	}
	// Recipe items may reference brand-agnostic ingredients; batch-load
	// them so nested ingredient resolvers never issue a query per row.
	rc.ingredients, err = loadIngredients(ctx, l.inv, recipeItemIngredientIDs(allItemsBy, extraIngredientIDs))
	return err
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

// loadBottleSelectionCounts fetches the per-user and global selection
// counts for the given bottle IDs.
func loadBottleSelectionCounts(ctx context.Context, an AnalyticsService, userID int64, bottleIDs []int64) (map[int64]countPair, error) {
	out := make(map[int64]countPair, len(bottleIDs))
	if an == nil || len(bottleIDs) == 0 {
		return out, nil
	}
	userCounts, err := an.GetUserSelectionCounts(ctx, userID, analytics.EntityBottle, bottleIDs)
	if err != nil {
		return nil, fmt.Errorf("load bottle selection counts: %w", err)
	}
	globalCounts, err := an.GetGlobalSelectionCounts(ctx, analytics.EntityBottle, bottleIDs)
	if err != nil {
		return nil, fmt.Errorf("load bottle selection counts: %w", err)
	}
	for _, c := range userCounts {
		p := out[c.EntityID]
		p.personal = c.SelectCount
		out[c.EntityID] = p
	}
	for _, c := range globalCounts {
		p := out[c.EntityID]
		p.global = c.SelectCount
		out[c.EntityID] = p
	}
	return out, nil
}

// errQueryTimeout marks requests cancelled by the handler's deadline; the
// handler maps it onto the TIMEOUT GraphQL error code after Exec returns.
var errQueryTimeout = errors.New("graphql request timed out")

// aiProviderFieldPattern matches the root fields whose resolvers call the
// LLM provider. aiAvailable/assistantTools/assistantPrompt/callAssistantTool/
// prepareAssistantRequest stay on the normal budget — they never reach the
// model. A stray match inside a string literal only widens a deadline, so
// the cheap text scan is safe.
var aiProviderFieldPattern = regexp.MustCompile(
	`\b(?:askAssistant|suggestMeals|suggestEventFixes|suggestPairings|suggestCocktails|suggestRecipeAllergens)\b`)

// usesAIProvider reports whether the query touches an LLM-backed field and
// therefore deserves the AI budget instead of the interactive one.
func usesAIProvider(query string) bool {
	return aiProviderFieldPattern.MatchString(query)
}

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
// timeout bounds each execution (empty means use the default); aiTimeout
// bounds executions that call the LLM provider (empty or <= timeout keeps
// timeout for everything). maxCost caps the number of field resolutions per
// request (<= 0 disables the budget), and extra schema options (e.g.
// graphql.MaxDepth, graphql.MaxQueryLength) are applied on top of the
// built-in tracer. A schema parse failure is returned as an error rather
// than panicking.
func NewGraphQLHandler(r *Resolver, timeout time.Duration, aiTimeout time.Duration, maxCost int, schemaOpts ...graphql.SchemaOpt) (echo.HandlerFunc, error) {
	opts := append([]graphql.SchemaOpt{graphql.Tracer(newGraphQLTracer())}, schemaOpts...)
	parsed, err := graphql.ParseSchema(schema, r, opts...)
	if err != nil {
		return nil, fmt.Errorf("parse graphql schema: %w", err)
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return func(c echo.Context) error {
		req, body, ok := decodeGraphQLRequest(c)
		if !ok {
			// Return bind failures in GraphQL error shape so clients get a
			// consistent contract instead of an Echo HTML error page.
			return c.JSON(http.StatusOK, badRequestErrorBody())
		}

		// Mutation dedup: a replayed response skips execution entirely, and
		// a held claim caches the response for the next identical retry.
		claim, handled, err := r.dedupMutation(c, req, body)
		if handled || err != nil {
			return err
		}
		if claim != nil {
			defer claim.abandonIfPending()
		}

		resp := execGraphQL(c, parsed, req, timeout, aiTimeout, maxCost)
		out, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if claim != nil {
			claim.complete(out)
		}
		return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, out)
	}, nil
}

// graphQLRequest is the decoded GraphQL request body.
type graphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operationName"`
}

// decodeGraphQLRequest reads the body once up front — the dedup layer
// hashes the exact bytes, so they must be captured before decoding — and
// returns the parsed request. ok is false on read/bind failure.
func decodeGraphQLRequest(c echo.Context) (graphQLRequest, []byte, bool) {
	body, err := io.ReadAll(c.Request().Body)
	var req graphQLRequest
	ok := err == nil && json.Unmarshal(body, &req) == nil
	return req, body, ok
}

// badRequestErrorBody is the GraphQL error payload for a malformed body.
func badRequestErrorBody() map[string]any {
	return map[string]any{
		"errors": []map[string]any{{
			"message":    "invalid request body",
			"extensions": map[string]any{"code": codeBadUserInput},
		}},
	}
}

// dedupMutation runs the mutation dedup gate. handled reports whether the
// response was already written (rejected or replayed), in which case err
// carries the write result.
func (r *Resolver) dedupMutation(c echo.Context, req graphQLRequest, body []byte) (*idemClaim, bool, error) {
	if r.IdemStore == nil || !isMutationOperation(req.Query) {
		return nil, false, nil
	}
	u, uerr := userFromContext(c.Request().Context())
	if uerr != nil {
		return nil, false, nil
	}
	claim, stored, rej := r.beginDedup(c, u.UserID, body)
	if rej != nil {
		return nil, true, c.JSON(http.StatusOK, idemErrorBody(rej.msg, rej.code))
	}
	if stored != nil {
		c.Response().Header().Set(headerIdempotencyReplayed, "true")
		return nil, true, c.Blob(http.StatusOK, echo.MIMEApplicationJSON, stored.Body)
	}
	return claim, false, nil
}

// execGraphQL executes the request under the time/cost budgets and
// sanitizes the response errors.
func execGraphQL(c echo.Context, parsed *graphql.Schema, req graphQLRequest, timeout, aiTimeout time.Duration, maxCost int) *graphql.Response {
	execTimeout := timeout
	if aiTimeout > timeout && usesAIProvider(req.Query) {
		execTimeout = aiTimeout
	}
	ctx, cancel := context.WithTimeoutCause(c.Request().Context(), execTimeout, errQueryTimeout)
	limiter := &costLimiter{max: maxCost, fire: cancel}
	ctx = context.WithValue(ctx, costLimiterContextKey{}, limiter)

	resp := parsed.Exec(ctx, req.Query, req.OperationName, req.Variables)
	cancel()
	sanitizeQueryErrors(resp.Errors, c.Response().Header().Get(echo.HeaderXRequestID))
	applyLimitErrors(ctx, resp, limiter)
	return resp
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
