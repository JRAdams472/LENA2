package bff

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
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
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.InventoryService, u.UserID, []int64{id}, nil)
	if err != nil {
		return nil, err
	}
	if err := loadRecipeSelectionCounts(ctx, r.AnalyticsService, u.UserID, []int64{id}, rc); err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec, rc: rc}, nil
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
	if err := loadRecipeInventoryChildren(ctx, r.InventoryService, rc, nil); err != nil {
		return nil, err
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
	page, pageSize := pageArgs(args.Page, args.PageSize)
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
func parseRecipeChildren(ctx context.Context, inv ItemReader, items []recipeItemInput, steps []recipeStepInput) ([]recipe.RecipeItem, []recipe.RecipeStep, error) {
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
	r.recordEventAsync(u.UserID, u.Email, analytics.Event{
		EventType:  analytics.EventRecipeCreated,
		EntityType: analytics.EntityRecipe,
		EntityID:   rec.RecipeID,
	})
	r.computeOverlapAsync(rec.RecipeID)
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
	patch, err := mergeRecipePatch(existing, args.Input)
	if err != nil {
		return nil, err
	}
	items, steps, err := parseRecipeChildren(ctx, r.InventoryService, args.Input.Items, args.Input.Steps)
	if err != nil {
		return nil, err
	}
	if err := r.RecipeService.UpdateRecipeWithChildren(ctx, id, patch, items, steps, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.RecipeService.GetRecipeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: updated}, nil
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
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: rec, rc: rc}, nil
}

// ratingRecencyMinRating is the minimum star rating for a recipe to be
// suggested under the rating_recency reason.
const ratingRecencyMinRating = 4

// RecommendedRecipes merges cached ingredient-overlap recommendations with
// live rating-recency suggestions, deduplicates by recipe keeping the
// highest score, and returns up to limit results sorted by score.
func (r *Resolver) RecommendedRecipes(ctx context.Context, args struct{ Limit int32 }) ([]*recipeRecommendationResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	limit := clamp(args.Limit, 1, 50)

	overlap, err := r.AnalyticsService.ListRecipeRecommendations(ctx, u.UserID, analytics.ReasonIngredientOverlap, limit)
	if err != nil {
		return nil, err
	}
	recency, err := r.ratingRecencyScores(ctx, u.UserID, u.HouseholdID, limit)
	if err != nil {
		return nil, err
	}
	recipeIDs, best := mergeRecommendations(overlap, recency, limit)

	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.InventoryService, u.UserID, recipeIDs, nil)
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

// scoredRec pairs a recommendation reason with its score.
type scoredRec struct {
	reason string
	score  float64
}

// mergeRecommendations deduplicates overlap and rating-recency candidates
// keeping the best score per recipe, and returns recipe IDs sorted by
// score (ties by id) capped at limit.
func mergeRecommendations(overlap []analytics.Recommendation, recency map[int64]float64, limit int32) ([]int64, map[int64]scoredRec) {
	best := make(map[int64]scoredRec, len(overlap)+len(recency))
	for _, o := range overlap {
		best[o.RecipeID] = scoredRec{reason: analytics.ReasonIngredientOverlap, score: o.Score}
	}
	for recipeID, score := range recency {
		if cur, ok := best[recipeID]; !ok || score > cur.score {
			best[recipeID] = scoredRec{reason: analytics.ReasonRatingRecency, score: score}
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
	return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: r.recipe, rc: r.rc}
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
	if r.items != nil {
		it, ok := r.items[r.item.ItemID]
		if !ok {
			return nil, nil
		}
		return &itemResolver{inv: r.inv, it: it, ch: r.ch}, nil
	}
	slog.Default().Warn("recipeItem.item missed preload; lazy-loading", "item_id", r.item.ItemID)
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
	if r.ingredients != nil {
		in, ok := r.ingredients[*r.item.IngredientID]
		if !ok {
			return nil, nil
		}
		return &ingredientResolver{inv: r.inv, in: in}, nil
	}
	slog.Default().Warn("recipeItem.ingredient missed preload; lazy-loading", "ingredient_id", *r.item.IngredientID)
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
	StepNumber          int32
	Instruction         string
	DurationMinutes     *int32
	StepType            *string
	IsPassive           *bool
	DependsOnStepNumber *int32
	Appliance           *string
}
