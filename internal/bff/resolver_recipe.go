package bff

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/graph-gophers/graphql-go"
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
	// Single-recipe reads preload the same child graph as list pages so
	// nested field resolvers never fall back to a query per row.
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.IdentityService, r.InventoryService, u.UserID, u.HouseholdID, []int64{id}, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, []int64{id}, rc); err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec, rc: rc, as: asOfRecipeChildren(rc)}, nil
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
		return nil, badInputf("servings must be positive")
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
		ingredients:  make(map[int64]inventory.Ingredient),
		units:        make(map[int64]inventory.Unit),
		recipeCounts: make(map[int64]countPair),
		myRatings:    make(map[int64]int16),
		summaries:    make(map[int64]recipe.RatingSummary),
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, []int64{scaled.Recipe.RecipeID}, rc); err != nil {
		return nil, err
	}
	if err := loadRecipeRatings(ctx, r.RecipeService, u.UserID, []int64{scaled.Recipe.RecipeID}, rc); err != nil {
		return nil, err
	}
	rc.itemsBy[scaled.Recipe.RecipeID] = scaled.Items
	rc.stepsBy[scaled.Recipe.RecipeID] = scaled.Steps
	fav, err := r.UserPrefsService.GetRecipeFavorite(ctx, u.UserID, scaled.Recipe.RecipeID)
	if err != nil && !errors.Is(err, domainerr.ErrNotFound) {
		return nil, err
	}
	rc.favorites[scaled.Recipe.RecipeID] = fav.IsFavorite

	// Reuse the shared inventory-child loader so scaledRecipe.items.item
	// and friends never degrade to per-row queries.
	if err := loadRecipeInventoryChildren(ctx, r.InventoryService, r.IdentityService, r.UserPrefsService, rc, u.HouseholdID, nil, nil); err != nil {
		return nil, err
	}

	return &recipeResolver{
		inv:    r.InventoryService,
		rec:    r.RecipeService,
		up:     r.UserPrefsService,
		user:   u,
		recipe: scaled.Recipe,
		rc:     rc,
		as:     asOfRecipeChildren(rc),
	}, nil
}

// Recipes resolves a paginated list of active recipes. Optional filters
// (search text, faceted category IDs, favorites) narrow the set; results are
// engagement-ranked — favorites, then household-used, then personally
// viewed, then searched — with in-tier order following signal strength.
// searchMode=semantic instead embeds the search text and ranks embedded
// recipes by cosine distance blended with a small engagement bump.
func (r *Resolver) Recipes(ctx context.Context, args struct {
	Page        int32
	PageSize    int32
	Search      *string
	CategoryIDs *[]graphql.ID
	IsFavorite  *bool
	MealType    *string
	SearchMode  string
}) (*recipePageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)

	search := recipe.RecipeSearch{
		Active: true,
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}
	if args.Search != nil {
		search.Search = strings.TrimSpace(*args.Search)
	}
	if args.CategoryIDs != nil {
		ids, err := parseIDs(*args.CategoryIDs)
		if err != nil {
			return nil, err
		}
		search.CategoryIDs = ids
	}

	// Engagement ranking inputs — analytics IDs arrive pre-sorted by signal
	// strength so array_position doubles as the in-tier tiebreaker. Failures
	// degrade to name order rather than failing the listing.
	if r.AnalyticsService != nil {
		if eng, err := r.AnalyticsService.RecipeEngagementSets(ctx, u.UserID, u.HouseholdID); err == nil {
			search.UsedIDs = eng.UsedIDs
			search.ViewedIDs = eng.ViewedIDs
			search.SearchTerms = eng.SearchTerms
		}
	}
	var favoriteIDs []int64
	if r.UserPrefsService != nil {
		if ids, err := r.UserPrefsService.ListFavoriteRecipeIDs(ctx, u.UserID); err == nil {
			favoriteIDs = ids
		}
	}
	search.FavoriteIDs = favoriteIDs
	if args.MealType != nil {
		if id, err := r.courseCategoryID(ctx, *args.MealType); err == nil && id != nil {
			search.CourseBoostID = id
		}
	}
	if args.IsFavorite != nil {
		if *args.IsFavorite {
			search.IncludeIDs = favoriteIDs
		} else {
			search.ExcludeIDs = favoriteIDs
		}
	}

	var recipes []recipe.Recipe
	var total int64
	if recipeSearchMode(args.SearchMode) == recipeSearchModeSemantic && search.Search != "" {
		semantic := recipe.SemanticSearch{
			Active:      search.Active,
			CategoryIDs: search.CategoryIDs,
			IncludeIDs:  search.IncludeIDs,
			ExcludeIDs:  search.ExcludeIDs,
			FavoriteIDs: search.FavoriteIDs,
			UsedIDs:     search.UsedIDs,
			ViewedIDs:   search.ViewedIDs,
			Limit:       search.Limit,
			Offset:      search.Offset,
		}
		if r.RecipeEmbedder == nil {
			return nil, errUnavailablef("semantic search isn't available on this deployment")
		}
		vec, err := r.RecipeEmbedder.EmbedQuery(ctx, search.Search)
		if err != nil {
			return nil, errUnavailablef("semantic search isn't available on this deployment")
		}
		semantic.QueryVector = vec
		results, err := r.RecipeService.SearchRecipesSemantic(ctx, semantic)
		if err != nil {
			return nil, err
		}
		recipes = make([]recipe.Recipe, len(results))
		for i, res := range results {
			recipes[i] = res.Recipe
		}
		total, err = r.RecipeService.CountSearchRecipesSemantic(ctx, semantic)
		if err != nil {
			return nil, err
		}
	} else {
		recipes, err = r.RecipeService.SearchRecipes(ctx, search)
		if err != nil {
			return nil, err
		}
		total, err = r.RecipeService.CountSearchRecipes(ctx, search)
		if err != nil {
			return nil, err
		}
	}
	recipeIDs := distinctIDs(recipes, func(rp recipe.Recipe) *int64 { return &rp.RecipeID })
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.IdentityService, r.InventoryService, u.UserID, u.HouseholdID, recipeIDs, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, recipeIDs, rc); err != nil {
		return nil, err
	}
	return &recipePageResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipes: recipes, rc: rc, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// recipeSearchModeSemantic is the schema enum value that switches the
// recipes query from name-LIKE to embedding-distance ranking.
const recipeSearchModeSemantic = "semantic"

// recipeSearchMode normalizes the enum arg; unknown/empty values fall back
// to keyword behavior.
func recipeSearchMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

// SemanticSearchAvailable reports whether recipe embeddings are configured.
// Clients gate the semantic search toggle on this instead of erroring on
// first use.
func (r *Resolver) SemanticSearchAvailable(ctx context.Context) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	return r.RecipeEmbedder != nil, nil
}

// parseRecipeChildren converts GraphQL recipe items/steps into service
// types so the service can persist them inside one transaction. Unit names
// are resolved to unit IDs via the shared unit catalog; unknown units are
// rejected.
func parseRecipeChildren(ctx context.Context, inv ItemReader, items []recipeItemInput, steps []recipeStepInput) ([]recipe.RecipeItem, []recipe.RecipeStep, error) {
	itemIDs := make([]*int64, len(items))
	ingredientIDs := make([]*int64, len(items))
	var brandOnly []int64
	for i, ri := range items {
		var err error
		itemIDs[i], err = optionalID(ri.ItemID)
		if err != nil {
			return nil, nil, err
		}
		ingredientIDs[i], err = optionalID(ri.IngredientID)
		if err != nil {
			return nil, nil, err
		}
		if itemIDs[i] == nil && ingredientIDs[i] == nil {
			return nil, nil, badInputf("recipe item requires itemId or ingredientId")
		}
		if ingredientIDs[i] == nil {
			brandOnly = append(brandOnly, *itemIDs[i])
		}
	}
	// Brand-only inputs resolve to their linked ingredient in one batch so
	// grocery aggregation keys them correctly. Unlinked items stay
	// brand-only — the link may not exist yet.
	var resolved map[int64]*int64
	if len(brandOnly) > 0 && inv != nil {
		householdID := int64(0)
		if u, ok := currentuser.FromContext(ctx); ok {
			householdID = u.HouseholdID
		}
		var err error
		resolved, err = inv.ResolveItemIngredients(ctx, householdID, brandOnly)
		if err != nil {
			return nil, nil, err
		}
	}

	outItems := make([]recipe.RecipeItem, 0, len(items))
	for i, ri := range items {
		itemID := itemIDs[i]
		ingredientID := ingredientIDs[i]
		if ingredientID == nil {
			ingredientID = resolved[*itemID]
		}
		unitID, err := resolveUnitID(ctx, inv, ri.Unit)
		if err != nil {
			return nil, nil, err
		}
		if ri.Quantity <= 0 {
			return nil, nil, badInputf("item quantity must be positive")
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
		if rs.DurationMinutes != nil && *rs.DurationMinutes < 0 {
			return nil, nil, badInputf("step %d durationMinutes must not be negative", rs.StepNumber)
		}
		outSteps = append(outSteps, recipe.RecipeStep{
			StepNumber:          rs.StepNumber,
			Instruction:         rs.Instruction,
			DurationMinutes:     rs.DurationMinutes,
			StepType:            derefString(rs.StepType),
			IsPassive:           boolValue(rs.IsPassive),
			DependsOnStepNumber: rs.DependsOnStepNumber,
			Appliance:           derefString(rs.Appliance),
		})
	}
	return outItems, outSteps, nil
}

// parseCategoryIDs converts the optional input ID list; nil in → nil out.
func parseCategoryIDs(ids *[]graphql.ID) ([]int64, error) {
	if ids == nil {
		return nil, nil
	}
	return parseIDs(*ids)
}

// CreateRecipe creates a new recipe and its items/steps atomically.
func (r *Resolver) CreateRecipe(ctx context.Context, args struct{ Input createRecipeInput }) (*recipeResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if s := args.Input.Servings; s != nil && *s <= 0 {
		return nil, badInputf("servings must be positive")
	}
	items, steps, err := parseRecipeChildren(ctx, r.InventoryService, args.Input.Items, args.Input.Steps)
	if err != nil {
		return nil, err
	}
	categoryIDs, err := parseCategoryIDs(args.Input.CategoryIDs)
	if err != nil {
		return nil, err
	}
	var rec recipe.Recipe
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		rec, err = r.RecipeService.CreateRecipeWithChildren(ctx, recipe.Recipe{
			Name:            args.Input.Name,
			Description:     derefString(args.Input.Description),
			Servings:        args.Input.Servings,
			PrepTimeMinutes: args.Input.PrepTimeMinutes,
			CookTimeMinutes: args.Input.CookTimeMinutes,
			IsActive:        true,
		}, items, steps, u.Email)
		if err != nil {
			return err
		}
		if len(categoryIDs) > 0 {
			return r.RecipeService.SetRecipeCategories(ctx, rec.RecipeID, categoryIDs, u.Email)
		}
		return nil
	})
	if err != nil {
		return nil, recipeWriteError(err)
	}
	r.recordEventAsync(u.UserID, u.Email, analytics.Event{
		EventType:  analytics.EventRecipeCreated,
		EntityType: analytics.EntityRecipe,
		EntityID:   rec.RecipeID,
	})
	r.computeOverlapAsync(rec.RecipeID)
	r.refreshEmbeddingAsync(rec.RecipeID)
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec, as: r.allergySrc(u)}, nil
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
	patch, err := mergeRecipePatch(existing, args.Input)
	if err != nil {
		return nil, err
	}
	items, steps, err := parseRecipeChildren(ctx, r.InventoryService, args.Input.Items, args.Input.Steps)
	if err != nil {
		return nil, err
	}
	categoryIDs, err := parseCategoryIDs(args.Input.CategoryIDs)
	if err != nil {
		return nil, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.RecipeService.UpdateRecipeWithChildren(ctx, id, patch, items, steps, u.Email); err != nil {
			return err
		}
		if categoryIDs != nil {
			return r.RecipeService.SetRecipeCategories(ctx, id, categoryIDs, u.Email)
		}
		return nil
	})
	if err != nil {
		return nil, recipeWriteError(err)
	}
	updated, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	r.refreshEmbeddingAsync(id)
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: updated, as: r.allergySrc(u)}, nil
}

// recipeWriteError maps a service-layer write failure to a client-safe
// error: unique name violations become CONFLICT.
func recipeWriteError(err error) error {
	if errors.Is(err, domainerr.ErrConflict) {
		return &clientError{msg: "a recipe with that name already exists", code: codeConflict}
	}
	return err
}

// mergeRecipePatch applies a PATCH-style input over the existing recipe:
// empty/unset input fields keep their current values.
func mergeRecipePatch(existing recipe.Recipe, in createRecipeInput) (recipe.Recipe, error) {
	patch := existing
	patch.RecipeID = 0 // identity is passed separately to UpdateRecipeWithChildren
	if in.Name != "" {
		patch.Name = in.Name
	}
	patch.Description = coalesce(existing.Description, in.Description)
	patch.PrepTimeMinutes = coalescePtr(existing.PrepTimeMinutes, in.PrepTimeMinutes)
	patch.CookTimeMinutes = coalescePtr(existing.CookTimeMinutes, in.CookTimeMinutes)
	if in.Servings != nil {
		if *in.Servings <= 0 {
			return patch, badInputf("servings must be positive")
		}
		patch.Servings = in.Servings
	}
	return patch, nil
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

// RateRecipe upserts the current user's 1-5 star rating for a recipe and
// returns the recipe with fresh rating fields.
func (r *Resolver) RateRecipe(ctx context.Context, args struct {
	RecipeID graphql.ID
	Rating   int32
}) (*recipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	rating, err := checkedInt16(args.Rating, "rating", 1, 5)
	if err != nil {
		return nil, err
	}
	if _, err := r.RecipeService.SetRating(ctx, u.UserID, id, rating, u.Email); err != nil {
		return nil, err
	}
	r.recordEventAsync(u.UserID, u.Email, analytics.Event{
		EventType:  analytics.EventRatingGiven,
		EntityType: analytics.EntityRecipe,
		EntityID:   id,
	})
	rec, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rc := &recipeChildren{
		recipeCounts: make(map[int64]countPair),
		myRatings:    make(map[int64]int16),
		summaries:    make(map[int64]recipe.RatingSummary),
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, []int64{id}, rc); err != nil {
		return nil, err
	}
	if err := loadRecipeRatings(ctx, r.RecipeService, u.UserID, []int64{id}, rc); err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec, rc: rc, as: asOfRecipeChildren(rc)}, nil
}

// courseCategoryName is the category group whose members double as meal
// types (Breakfast, Lunch, Dinner, ...).
const courseCategoryName = "Course"

// courseCategoryID resolves a meal_type string to its Course category id
// (case-insensitive name match). Unknown meal types yield nil, nil — the
// caller then simply skips the boost.
func (r *Resolver) courseCategoryID(ctx context.Context, mealType string) (*int64, error) {
	want := strings.ToLower(strings.TrimSpace(mealType))
	if want == "" {
		return nil, nil
	}
	groups, err := r.RecipeService.ListCategoryGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if !strings.EqualFold(g.Name, courseCategoryName) {
			continue
		}
		cats, err := r.RecipeService.ListCategoriesByGroup(ctx, g.CategoryGroupID)
		if err != nil {
			return nil, err
		}
		for _, c := range cats {
			if strings.EqualFold(c.Name, want) {
				id := c.CategoryID
				return &id, nil
			}
		}
	}
	return nil, nil
}

// ratingRecencyMinRating is the minimum star rating for a recipe to be
// suggested under the rating_recency reason.
const ratingRecencyMinRating = 4

// RecommendedRecipes merges cached ingredient-overlap recommendations with
// live rating-recency, category-affinity, and household-trending
// suggestions. Each source is normalized to its own max so scores compare
// across reasons; dedupe keeps the best score per recipe and the result is
// sorted by score capped at limit.
func (r *Resolver) RecommendedRecipes(ctx context.Context, args struct{ Limit int32 }) ([]*recipeRecommendationResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	limit := clamp(args.Limit, 1, 50)

	var sources []recommendationSource
	overlap, err := r.AnalyticsService.ListRecipeRecommendations(ctx, u.UserID, analytics.ReasonIngredientOverlap, limit)
	if err != nil {
		return nil, err
	}
	overlapScores := make(map[int64]float64, len(overlap))
	for _, o := range overlap {
		overlapScores[o.RecipeID] = o.Score
	}
	sources = append(sources, recommendationSource{reason: analytics.ReasonIngredientOverlap, scores: overlapScores})
	recency, err := r.ratingRecencyScores(ctx, u.UserID, u.HouseholdID, limit)
	if err != nil {
		return nil, err
	}
	sources = append(sources, recommendationSource{reason: analytics.ReasonRatingRecency, scores: recency})
	if r.AnalyticsService != nil {
		affinity, err := r.categoryAffinityScores(ctx, u.HouseholdID, limit)
		if err != nil {
			return nil, err
		}
		sources = append(sources, recommendationSource{reason: analytics.ReasonCategoryAffinity, scores: affinity})
		trending, err := r.householdTrendingScores(ctx, u.HouseholdID, limit)
		if err != nil {
			return nil, err
		}
		sources = append(sources, recommendationSource{reason: analytics.ReasonHouseholdTrending, scores: trending})
	}
	recipeIDs, best := mergeRecommendations(sources, limit)

	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.IdentityService, r.InventoryService, u.UserID, u.HouseholdID, recipeIDs, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, recipeIDs, rc); err != nil {
		return nil, err
	}
	out := make([]*recipeRecommendationResolver, 0, len(recipeIDs))
	for _, id := range recipeIDs {
		rec, ok := rc.recipes[id]
		if !ok {
			continue
		}
		out = append(out, &recipeRecommendationResolver{
			inv:    r.InventoryService,
			rec:    r.RecipeService,
			up:     r.UserPrefsService,
			user:   u,
			recipe: rec,
			reason: best[id].reason,
			score:  best[id].score,
			rc:     rc,
		})
	}
	return out, nil
}

// ratingRecencyScores computes rating-recency candidates: recipe ratings
// and meal-plan last-used dates are read from their own domains and
// combined here (A1-01 — SQL never crosses schemas). Recipes never
// planned score 1; the score decays linearly to 0 over 180 days since the
// last plan week. Returns the best `limit` scores keyed by recipe.
func (r *Resolver) ratingRecencyScores(ctx context.Context, userID, householdID int64, limit int32) (map[int64]float64, error) {
	rated, err := r.RecipeService.ListRatedAtLeast(ctx, userID, ratingRecencyMinRating)
	if err != nil {
		return nil, err
	}
	ratedIDs := distinctIDs(rated, func(rr recipe.RecipeRating) *int64 { return &rr.RecipeID })
	lastPlanned, err := r.MealPlanService.LastPlannedDates(ctx, householdID, ratedIDs)
	if err != nil {
		return nil, err
	}
	const recencyWindowDays = 180.0
	today := time.Now()
	recency := make(map[int64]float64, len(rated))
	for _, rr := range rated {
		days := recencyWindowDays
		if last, ok := lastPlanned[rr.RecipeID]; ok {
			days = today.Sub(last).Hours() / 24
		}
		recency[rr.RecipeID] = clampFloat(days/recencyWindowDays, 0, 1)
	}
	// Keep the best `limit` recency candidates — the SQL no longer limits.
	recencyIDs := make([]int64, 0, len(recency))
	for id := range recency {
		recencyIDs = append(recencyIDs, id)
	}
	slices.SortFunc(recencyIDs, func(a, b int64) int {
		if recency[a] != recency[b] {
			return cmp.Compare(recency[b], recency[a])
		}
		return cmp.Compare(a, b)
	})
	if len(recencyIDs) > int(limit) {
		recencyIDs = recencyIDs[:int(limit)]
	}
	kept := make(map[int64]float64, len(recencyIDs))
	for _, id := range recencyIDs {
		kept[id] = recency[id]
	}
	return kept, nil
}

// householdTrendingDays is the recent window for velocity comparisons, and
// householdTrendingMinRecent is the minimum events inside that window
// before a recipe counts as trending — one recent hit isn't a trend.
const (
	householdTrendingDays      = 30
	householdTrendingMinRecent = 2
)

// categoryAffinityScores converts the household's recipe usage into
// per-category affinity, then scores each active recipe by the affinity of
// its categories. Recipes share categories with what the household already
// cooks, so the reason reads as "matches your household's tastes".
func (r *Resolver) categoryAffinityScores(ctx context.Context, householdID int64, limit int32) (map[int64]float64, error) {
	usage, err := r.AnalyticsService.HouseholdRecipeUsage(ctx, householdID)
	if err != nil {
		return nil, err
	}
	if len(usage) == 0 {
		return map[int64]float64{}, nil
	}
	recipes, err := r.RecipeService.ListRecipes(ctx, true, 5000, 0)
	if err != nil {
		return nil, err
	}
	idSet := make(map[int64]bool, len(recipes)+len(usage))
	for _, rp := range recipes {
		idSet[rp.RecipeID] = true
	}
	for id := range usage {
		idSet[id] = true
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	cats, err := r.RecipeService.ListCategoriesForRecipes(ctx, ids)
	if err != nil {
		return nil, err
	}
	catHits := make(map[int64]int64)
	for recipeID, hits := range usage {
		for _, c := range cats[recipeID] {
			catHits[c.CategoryID] += hits
		}
	}
	scores := make(map[int64]float64)
	for _, rp := range recipes {
		var s float64
		for _, c := range cats[rp.RecipeID] {
			s += float64(catHits[c.CategoryID])
		}
		if s > 0 {
			scores[rp.RecipeID] = s
		}
	}
	return normalizeScores(topScoredIDs(scores, limit)), nil
}

// householdTrendingScores finds recipes whose recent event velocity
// outpaces their own lifetime rate for the household — a rising signal,
// not just a popular one.
func (r *Resolver) householdTrendingScores(ctx context.Context, householdID int64, limit int32) (map[int64]float64, error) {
	velocities, err := r.AnalyticsService.HouseholdRecipeVelocities(ctx, householdID, householdTrendingDays)
	if err != nil {
		return nil, err
	}
	scores := make(map[int64]float64)
	for _, v := range velocities {
		if v.RecentCount < householdTrendingMinRecent || v.AgeDays <= householdTrendingDays {
			continue
		}
		recentRate := float64(v.RecentCount) / householdTrendingDays
		lifeRate := float64(v.TotalCount) / v.AgeDays
		if recentRate > lifeRate {
			scores[v.RecipeID] = recentRate / lifeRate
		}
	}
	return normalizeScores(topScoredIDs(scores, limit)), nil
}

// normalizeScores rescales a score map to [0,1] by its max so unbounded
// signals merge comparably against the [0,1] overlap/recency sources.
func normalizeScores(scores map[int64]float64) map[int64]float64 {
	var peak float64
	for _, s := range scores {
		if s > peak {
			peak = s
		}
	}
	if peak <= 0 {
		return scores
	}
	for id, s := range scores {
		scores[id] = s / peak
	}
	return scores
}

// topScoredIDs trims a score map to its top `limit` entries (score desc,
// id asc) so downstream merge work stays bounded.
func topScoredIDs(scores map[int64]float64, limit int32) map[int64]float64 {
	if len(scores) <= int(limit) {
		return scores
	}
	ids := make([]int64, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b int64) int {
		if scores[a] != scores[b] {
			return cmp.Compare(scores[b], scores[a])
		}
		return cmp.Compare(a, b)
	})
	out := make(map[int64]float64, int(limit))
	for _, id := range ids[:int(limit)] {
		out[id] = scores[id]
	}
	return out
}

// scoredRec pairs a recommendation reason with its score.
type scoredRec struct {
	reason string
	score  float64
}

// recommendationSource is one scored candidate set: a reason plus scores
// keyed by recipe. Every source reports on a [0,1] scale — unbounded
// signals (hit sums, velocity ratios) are max-normalized by their scorer
// before they reach this merge.
type recommendationSource struct {
	reason string
	scores map[int64]float64
}

// mergeRecommendations keeps the best score (and its reason) per recipe —
// earlier sources win ties — and returns recipe IDs sorted by score
// (ties by id) capped at limit.
func mergeRecommendations(sources []recommendationSource, limit int32) ([]int64, map[int64]scoredRec) {
	best := make(map[int64]scoredRec)
	for _, src := range sources {
		for recipeID, score := range src.scores {
			if cur, ok := best[recipeID]; !ok || score > cur.score {
				best[recipeID] = scoredRec{reason: src.reason, score: score}
			}
		}
	}
	recipeIDs := make([]int64, 0, len(best))
	for id := range best {
		recipeIDs = append(recipeIDs, id)
	}
	slices.SortFunc(recipeIDs, func(a, b int64) int {
		if best[a].score != best[b].score {
			return cmp.Compare(best[b].score, best[a].score)
		}
		return cmp.Compare(a, b)
	})
	if limit >= 0 && len(recipeIDs) > int(limit) {
		recipeIDs = recipeIDs[:int(limit)]
	}
	return recipeIDs, best
}

type recipeRecommendationResolver struct {
	inv    ItemReader
	rec    RecipeService
	up     UserPrefsService
	user   currentuser.User
	recipe recipe.Recipe
	reason string
	score  float64
	rc     *recipeChildren
}

func (r *recipeRecommendationResolver) Recipe() *recipeResolver {
	return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: r.recipe, rc: r.rc, as: asOfRecipeChildren(r.rc)}
}

func (r *recipeRecommendationResolver) Reason() string { return r.reason }

func (r *recipeRecommendationResolver) Score() float64 { return r.score }

// recipeResolver resolves Recipe fields. When rc is non-nil its
// batch-loaded maps are used instead of per-recipe service calls.
type recipeResolver struct {
	inv           ItemReader
	rec           RecipeService
	up            UserPrefsService
	user          currentuser.User
	recipe        recipe.Recipe
	rc            *recipeChildren
	as            *allergySource
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
	var ingredients map[int64]inventory.Ingredient
	var ch *itemChildren
	if r.rc != nil {
		items = r.rc.itemsBy[r.recipe.RecipeID]
		itemsByID = r.rc.items
		ingredients = r.rc.ingredients
		ch = r.rc.itemChildren
	} else {
		slog.Default().Warn("recipe.items missed preload; lazy-loading", "recipe_id", r.recipe.RecipeID)
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
		out[i] = &recipeItemResolver{inv: r.inv, item: items[i], items: itemsByID, ingredients: ingredients, ch: ch, units: units}
	}
	return out, nil
}

// recipeItems returns the recipe's lines from the preload or a lazy fetch.
func (r *recipeResolver) recipeItems(ctx context.Context) ([]recipe.RecipeItem, error) {
	if r.rc != nil {
		return r.rc.itemsBy[r.recipe.RecipeID], nil
	}
	return r.rec.ListRecipeItems(ctx, r.recipe.RecipeID)
}

// allergenSet unions every recipe line's resolved flags.
func (r *recipeResolver) allergenSet(ctx context.Context) (*allergyContext, map[int64]string, error) {
	var ac *allergyContext
	items := []recipe.RecipeItem(nil)
	if r.rc != nil && r.rc.itemChildren != nil && r.rc.itemChildren.ac != nil {
		ac = r.rc.itemChildren.ac
		items = r.rc.itemsBy[r.recipe.RecipeID]
	} else {
		var err error
		items, err = r.rec.ListRecipeItems(ctx, r.recipe.RecipeID)
		if err != nil {
			return nil, nil, err
		}
		var ingredientIDs, itemIDs []int64
		for _, ri := range items {
			if ri.IngredientID != nil {
				ingredientIDs = append(ingredientIDs, *ri.IngredientID)
			}
			if ri.ItemID != nil {
				itemIDs = append(itemIDs, *ri.ItemID)
			}
		}
		ac, err = lazyAllergenCtx(ctx, r.as, ingredientIDs, itemIDs)
		if err != nil {
			return nil, nil, err
		}
	}
	set := make(map[int64]string)
	for _, ri := range items {
		ac.addInto(set, ri.IngredientID, ri.ItemID)
	}
	return ac, set, nil
}

// Allergens resolves the union of every recipe line's allergen flags.
func (r *recipeResolver) Allergens(ctx context.Context) ([]*allergenFlagResolver, error) {
	ac, set, err := r.allergenSet(ctx)
	if err != nil {
		return nil, err
	}
	return ac.flagResolvers(set), nil
}

// AllergyWarnings resolves which household members conflict with this
// recipe.
func (r *recipeResolver) AllergyWarnings(ctx context.Context) ([]*allergyWarningResolver, error) {
	ac, set, err := r.allergenSet(ctx)
	if err != nil {
		return nil, err
	}
	return ac.warningResolvers(set), nil
}

// ItemSections groups the recipe's items by section name in display order.
// Items without a section land in a group with a null name.
func (r *recipeResolver) ItemSections(ctx context.Context) ([]*recipeItemSectionResolver, error) {
	var items []recipe.RecipeItem
	var itemsByID map[int64]inventory.Item
	var ingredients map[int64]inventory.Ingredient
	var ch *itemChildren
	var units map[int64]inventory.Unit
	if r.rc != nil {
		items = r.rc.itemsBy[r.recipe.RecipeID]
		itemsByID = r.rc.items
		ingredients = r.rc.ingredients
		ch = r.rc.itemChildren
		units = r.rc.units
	} else {
		slog.Default().Warn("recipe.itemSections missed preload; lazy-loading", "recipe_id", r.recipe.RecipeID)
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
		sec.items = append(sec.items, &recipeItemResolver{inv: r.inv, item: ri, items: itemsByID, ingredients: ingredients, ch: ch, units: units})
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
		if errors.Is(err, domainerr.ErrNotFound) {
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

// MyRating resolves the current user's rating, or null when unrated.
func (r *recipeResolver) MyRating(ctx context.Context) (*int32, error) {
	if r.rc != nil {
		if v, ok := r.rc.myRatings[r.recipe.RecipeID]; ok {
			rating := int32(v)
			return &rating, nil
		}
		return nil, nil
	}
	rating, err := r.rec.GetUserRating(ctx, r.user.UserID, r.recipe.RecipeID)
	if err != nil {
		if errors.Is(err, domainerr.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	v := int32(rating.Rating)
	return &v, nil
}

// AverageRating resolves the mean rating across all users, or null when the
// recipe has no ratings.
func (r *recipeResolver) AverageRating(ctx context.Context) (*float64, error) {
	s, ok, err := r.ratingSummary(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return &s.AverageRating, nil
}

func (r *recipeResolver) RatingCount(ctx context.Context) (int32, error) {
	s, ok, err := r.ratingSummary(ctx)
	if err != nil || !ok {
		return 0, err
	}
	return int64ToInt32(s.RatingCount), nil
}

func (r *recipeResolver) ratingSummary(ctx context.Context) (recipe.RatingSummary, bool, error) {
	if r.rc != nil {
		s, ok := r.rc.summaries[r.recipe.RecipeID]
		return s, ok, nil
	}
	summaries, err := r.rec.ListRatingSummaries(ctx, []int64{r.recipe.RecipeID})
	if err != nil {
		return recipe.RatingSummary{}, false, err
	}
	if len(summaries) == 0 {
		return recipe.RatingSummary{}, false, nil
	}
	return summaries[0], true, nil
}

type recipeItemResolver struct {
	inv         ItemReader
	item        recipe.RecipeItem
	items       map[int64]inventory.Item
	ingredients map[int64]inventory.Ingredient
	ch          *itemChildren
	units       map[int64]inventory.Unit
}

func (r *recipeItemResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.item.RecipeItemID, 10))
}

func (r *recipeItemResolver) Item(ctx context.Context) (*itemResolver, error) {
	if r.item.ItemID == nil {
		return nil, nil
	}
	if r.items != nil {
		it, ok := r.items[*r.item.ItemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch, as: asOfItemChildren(r.ch)}, nil
	}
	slog.Default().Warn("recipeItem.item missed preload; lazy-loading", "item_id", *r.item.ItemID)
	it, err := r.inv.GetItemByID(ctx, *r.item.ItemID)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.inv, it: it, as: asOfItemChildren(r.ch)}, nil
}

// Ingredient resolves the brand-agnostic ingredient linked to this recipe
// item, when set. Scaffolding only — nothing populates ingredient_id yet.
func (r *recipeItemResolver) Ingredient(ctx context.Context) (*ingredientResolver, error) {
	if r.item.IngredientID == nil {
		return nil, nil
	}
	if r.ingredients != nil {
		in, ok := r.ingredients[*r.item.IngredientID]
		if !ok {
			return nil, nil
		}
		return &ingredientResolver{inv: r.inv, in: in, as: asOfItemChildren(r.ch)}, nil
	}
	slog.Default().Warn("recipeItem.ingredient missed preload; lazy-loading", "ingredient_id", *r.item.IngredientID)
	in, err := r.inv.GetIngredientByID(ctx, *r.item.IngredientID)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.inv, in: in, as: asOfItemChildren(r.ch)}, nil
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

func (r *recipeStepResolver) DurationMinutes() *int32 { return r.step.DurationMinutes }

func (r *recipeStepResolver) StepType() *string { return nilIfEmpty(r.step.StepType) }

func (r *recipeStepResolver) IsPassive() bool { return r.step.IsPassive }

func (r *recipeStepResolver) DependsOnStepNumber() *int32 { return r.step.DependsOnStepNumber }

func (r *recipeStepResolver) Appliance() *string { return nilIfEmpty(r.step.Appliance) }

type recipePageResolver struct {
	inv      ItemReader
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
		out[i] = &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: r.recipes[i], rc: r.rc, as: asOfRecipeChildren(r.rc)}
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
	CategoryIDs     *[]graphql.ID
	Items           []recipeItemInput
	Steps           []recipeStepInput
}

type recipeItemInput struct {
	ItemID       *graphql.ID
	IngredientID *graphql.ID
	Quantity     float64
	Unit         string
	Section      *string
	DisplayOrder *int32
	Notes        *string
	IsOptional   *bool
}

type recipeStepInput struct {
	StepNumber          int32
	Instruction         string
	DurationMinutes     *int32
	StepType            *string
	IsPassive           *bool
	DependsOnStepNumber *int32
	Appliance           *string
}

// ---------- recipe categories ----------

type recipeCategoryGroupResolver struct {
	g          recipe.CategoryGroup
	categories []recipe.Category
	rec        RecipeService
}

func (r *recipeCategoryGroupResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.g.CategoryGroupID, 10))
}
func (r *recipeCategoryGroupResolver) Name() string        { return r.g.Name }
func (r *recipeCategoryGroupResolver) Exclusive() bool     { return r.g.Exclusive }
func (r *recipeCategoryGroupResolver) DisplayOrder() int32 { return r.g.DisplayOrder }
func (r *recipeCategoryGroupResolver) Categories() []*recipeCategoryResolver {
	out := make([]*recipeCategoryResolver, len(r.categories))
	for i, c := range r.categories {
		out[i] = &recipeCategoryResolver{c: c}
	}
	return out
}

type recipeCategoryResolver struct {
	c recipe.Category
}

func (r *recipeCategoryResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.c.CategoryID, 10))
}
func (r *recipeCategoryResolver) Name() string { return r.c.Name }
func (r *recipeCategoryResolver) Group() *recipeCategoryGroupResolver {
	return &recipeCategoryGroupResolver{g: recipe.CategoryGroup{
		CategoryGroupID: r.c.CategoryGroupID,
		Name:            r.c.GroupName,
		Exclusive:       r.c.GroupExclusive,
		DisplayOrder:    r.c.GroupDisplayOrder,
	}}
}

// RecipeCategoryGroups returns the full taxonomy — groups in display order,
// each with its categories preloaded.
func (r *Resolver) RecipeCategoryGroups(ctx context.Context) ([]*recipeCategoryGroupResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	groups, err := r.RecipeService.ListCategoryGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*recipeCategoryGroupResolver, len(groups))
	for i, g := range groups {
		cats, err := r.RecipeService.ListCategoriesByGroup(ctx, g.CategoryGroupID)
		if err != nil {
			return nil, err
		}
		out[i] = &recipeCategoryGroupResolver{g: g, categories: cats, rec: r.RecipeService}
	}
	return out, nil
}

// Categories resolves a recipe's assigned categories via the preload map.
func (r *recipeResolver) Categories() []*recipeCategoryResolver {
	if r.rc == nil {
		return nil
	}
	cats := r.rc.categoriesBy[r.recipe.RecipeID]
	out := make([]*recipeCategoryResolver, len(cats))
	for i, c := range cats {
		out[i] = &recipeCategoryResolver{c: c}
	}
	return out
}

// SetRecipeCategories replaces a recipe's category set; any authenticated
// household member may categorize recipes (admins curate the taxonomy).
func (r *Resolver) SetRecipeCategories(ctx context.Context, args struct {
	RecipeID    graphql.ID
	CategoryIDs []graphql.ID
}) (*recipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	ids, err := parseIDs(args.CategoryIDs)
	if err != nil {
		return nil, err
	}
	if err := r.RecipeService.SetRecipeCategories(ctx, recipeID, ids, u.Email); err != nil {
		return nil, err
	}
	rcp, err := r.RecipeService.GetRecipeByID(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.IdentityService, r.InventoryService, u.UserID, u.HouseholdID, []int64{recipeID}, nil, nil)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rcp, rc: rc, as: asOfRecipeChildren(rc)}, nil
}

// CreateRecipeCategoryGroup adds a taxonomy group; admins only.
func (r *Resolver) CreateRecipeCategoryGroup(ctx context.Context, args struct {
	Input struct {
		Name         string
		Exclusive    *bool
		DisplayOrder *int32
	}
}) (*recipeCategoryGroupResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	exclusive := true
	if args.Input.Exclusive != nil {
		exclusive = *args.Input.Exclusive
	}
	g, err := r.RecipeService.CreateCategoryGroup(ctx, recipe.CategoryGroup{
		Name: args.Input.Name, Exclusive: exclusive, DisplayOrder: int32Value(args.Input.DisplayOrder),
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeCategoryGroupResolver{g: g, rec: r.RecipeService}, nil
}

// UpdateRecipeCategoryGroup renames a group or flips its exclusivity;
// omitted fields keep their current values.
func (r *Resolver) UpdateRecipeCategoryGroup(ctx context.Context, args struct {
	ID    graphql.ID
	Input struct {
		Name         *string
		Exclusive    *bool
		DisplayOrder *int32
	}
}) (*recipeCategoryGroupResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	groups, err := r.RecipeService.ListCategoryGroups(ctx)
	if err != nil {
		return nil, err
	}
	var existing *recipe.CategoryGroup
	for i := range groups {
		if groups[i].CategoryGroupID == id {
			existing = &groups[i]
			break
		}
	}
	if existing == nil {
		return nil, domainerr.ErrNotFound
	}
	patch := *existing
	if args.Input.Name != nil {
		patch.Name = *args.Input.Name
	}
	if args.Input.Exclusive != nil {
		patch.Exclusive = *args.Input.Exclusive
	}
	if args.Input.DisplayOrder != nil {
		patch.DisplayOrder = *args.Input.DisplayOrder
	}
	g, err := r.RecipeService.UpdateCategoryGroup(ctx, id, patch, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeCategoryGroupResolver{g: g, rec: r.RecipeService}, nil
}

// DeleteRecipeCategoryGroup removes an empty group; the service rejects
// deletion while the group still holds categories.
func (r *Resolver) DeleteRecipeCategoryGroup(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.RecipeService.DeleteCategoryGroup(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// CreateRecipeCategory adds a category inside a group; admins only.
func (r *Resolver) CreateRecipeCategory(ctx context.Context, args struct {
	Input struct {
		GroupID graphql.ID
		Name    string
	}
}) (*recipeCategoryResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	groupID, err := parseID(string(args.Input.GroupID))
	if err != nil {
		return nil, err
	}
	c, err := r.RecipeService.CreateCategory(ctx, recipe.Category{
		CategoryGroupID: groupID, Name: args.Input.Name,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeCategoryResolver{c: c}, nil
}

// UpdateRecipeCategory renames a category.
func (r *Resolver) UpdateRecipeCategory(ctx context.Context, args struct {
	ID    graphql.ID
	Input struct {
		Name *string
	}
}) (*recipeCategoryResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if args.Input.Name == nil {
		return nil, badInputf("nothing to update")
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	c, err := r.RecipeService.UpdateCategory(ctx, id, recipe.Category{Name: *args.Input.Name}, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeCategoryResolver{c: c}, nil
}

// DeleteRecipeCategory removes a category; assignments cascade away.
func (r *Resolver) DeleteRecipeCategory(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.RecipeService.DeleteCategory(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// RecordView logs a "looked at this entity" interaction — fire-and-forget,
// and it deliberately does not bump selection counts (a view is weaker
// than a pick).
func (r *Resolver) RecordView(ctx context.Context, args struct {
	EntityType string
	EntityID   graphql.ID
}) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	entityID, err := parseID(string(args.EntityID))
	if err != nil {
		return false, err
	}
	r.runAsync("record view", 5*time.Second, func(ctx context.Context) error {
		return r.AnalyticsService.RecordView(ctx, analytics.Event{
			UserID:     u.UserID,
			EventType:  viewEventType(args.EntityType),
			EntityType: strings.TrimSpace(args.EntityType),
			EntityID:   entityID,
		}, u.Email)
	})
	return true, nil
}

// viewEventType derives the '<entity>_viewed' event name; the
// UserViewedEntityIDs ranking query derives it the same way, so any entity
// type works without a lookup table.
func viewEventType(entityType string) string {
	t := strings.ToLower(strings.TrimSpace(entityType))
	if t == "" {
		return "viewed"
	}
	return t + "_viewed"
}
