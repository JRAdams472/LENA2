// Package recipe owns the catalog of recipes: recipe definitions, their
// item lists and preparation steps. Per-user favorites live in userprefs.
package recipe

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe/sqlc"
)

// Service provides catalog operations for the recipe domain.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates a recipe Service using the given connection pool. The
// querier resolves a ctx-carried transaction first (see dbtx.ContextExecer)
// so calls made inside a UnitOfWork join that transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "recipe")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "recipe"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "recipe")) }
	}
	c := *s
	c.q = newQ(tx)
	c.tx = tx
	c.newQ = newQ
	return &c
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise. If the service is already bound to a transaction, or
// ctx already carries a UnitOfWork transaction, fn runs in that transaction
// instead of starting a new one.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	if s.tx != nil || dbtx.HasTx(ctx) {
		return fn(s)
	}
	if s.pool == nil {
		return fmt.Errorf("recipe: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// withTx runs fn against a sqlc.Querier bound to a single transaction on
// the domain's pool, committing on success and rolling back on error.
func (s *Service) withTx(ctx context.Context, fn func(q sqlc.Querier) error) error {
	return s.InTx(ctx, func(tx *Service) error { return fn(tx.q) })
}

// Recipe is a catalog recipe definition. Nullable numeric fields are
// pointers so a real 0 is distinguishable from "not set".
type Recipe struct {
	RecipeID        int64
	Name            string
	Description     string
	Servings        *int32
	PrepTimeMinutes *int32
	CookTimeMinutes *int32
	IsActive        bool
	// UpdatedAt feeds delta drift detection (RecipeDelta.Stale); nil until
	// the recipe's first canonical edit.
	UpdatedAt *time.Time
}

// CreateRecipe adds a new recipe.
func (s *Service) CreateRecipe(ctx context.Context, arg Recipe, by string) (Recipe, error) {
	return createRecipe(ctx, s.q, arg, by)
}

func createRecipe(ctx context.Context, q sqlc.Querier, arg Recipe, by string) (Recipe, error) {
	row, err := q.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Name:            arg.Name,
		Description:     textOrNull(arg.Description),
		Servings:        optInt4(arg.Servings),
		PrepTimeMinutes: optInt4(arg.PrepTimeMinutes),
		CookTimeMinutes: optInt4(arg.CookTimeMinutes),
		IsActive:        arg.IsActive,
		CreatedBy:       by,
		UpdatedBy:       textOrNull(by),
	})
	if err != nil {
		return Recipe{}, fmt.Errorf("create recipe: %w", err)
	}
	return toRecipe(row), nil
}

// GetRecipeByID returns a recipe by its primary key.
func (s *Service) GetRecipeByID(ctx context.Context, recipeID int64) (Recipe, error) {
	row, err := s.q.GetRecipeByID(ctx, recipeID)
	if err != nil {
		return Recipe{}, fmt.Errorf("get recipe by id: %w", domainerr.FromStorage(err))
	}
	return toRecipe(row), nil
}

// ScaledRecipe is a recipe whose ingredient quantities have been scaled to
// a target number of servings.
type ScaledRecipe struct {
	Recipe Recipe
	Items  []RecipeItem
	Steps  []RecipeStep
}

// ScaleRecipe returns the recipe with its ingredient quantities scaled to
// the requested number of servings. The returned Recipe reflects the target
// servings; Items and Steps are copies that do not affect the catalog.
func (s *Service) ScaleRecipe(ctx context.Context, recipeID int64, servings int32) (ScaledRecipe, error) {
	rec, err := s.GetRecipeByID(ctx, recipeID)
	if err != nil {
		return ScaledRecipe{}, fmt.Errorf("scale recipe: %w", err)
	}
	if rec.Servings == nil || *rec.Servings <= 0 {
		return ScaledRecipe{}, fmt.Errorf("scale recipe: recipe has no valid base servings")
	}
	if servings <= 0 {
		return ScaledRecipe{}, fmt.Errorf("scale recipe: target servings must be positive")
	}

	items, err := s.ListRecipeItems(ctx, recipeID)
	if err != nil {
		return ScaledRecipe{}, fmt.Errorf("scale recipe: %w", err)
	}
	steps, err := s.ListRecipeSteps(ctx, recipeID)
	if err != nil {
		return ScaledRecipe{}, fmt.Errorf("scale recipe: %w", err)
	}

	baseServings := *rec.Servings
	factor := float64(servings) / float64(baseServings)
	scaledItems := make([]RecipeItem, len(items))
	for i, it := range items {
		it.Quantity *= factor
		scaledItems[i] = it
	}

	rec.Servings = &servings
	return ScaledRecipe{Recipe: rec, Items: scaledItems, Steps: steps}, nil
}

// ListRecipes returns a paginated list of active/inactive recipes.
func (s *Service) ListRecipes(ctx context.Context, active bool, limit, offset int32) ([]Recipe, error) {
	rows, err := s.q.ListRecipes(ctx, sqlc.ListRecipesParams{IsActive: active, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	out := make([]Recipe, len(rows))
	for i := range rows {
		out[i] = toRecipe(rows[i])
	}
	return out, nil
}

// CountRecipes returns the total number of recipes with the given active flag.
func (s *Service) CountRecipes(ctx context.Context, active bool) (int64, error) {
	n, err := s.q.CountRecipes(ctx, active)
	if err != nil {
		return 0, fmt.Errorf("count recipes: %w", err)
	}
	return n, nil
}

// GetRecipesByIDs returns a set of recipes in a single query.
func (s *Service) GetRecipesByIDs(ctx context.Context, recipeIDs []int64) ([]Recipe, error) {
	rows, err := s.q.GetRecipesByIDs(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("get recipes by ids: %w", err)
	}
	out := make([]Recipe, len(rows))
	for i := range rows {
		out[i] = toRecipe(rows[i])
	}
	return out, nil
}

// UpdateRecipe modifies an existing recipe.
func (s *Service) UpdateRecipe(ctx context.Context, recipeID int64, arg Recipe, by string) error {
	return updateRecipeRow(ctx, s.q, recipeID, arg, by)
}

func updateRecipeRow(ctx context.Context, q sqlc.Querier, recipeID int64, arg Recipe, by string) error {
	n, err := q.UpdateRecipe(ctx, sqlc.UpdateRecipeParams{
		RecipeID:        recipeID,
		Name:            arg.Name,
		Description:     textOrNull(arg.Description),
		Servings:        optInt4(arg.Servings),
		PrepTimeMinutes: optInt4(arg.PrepTimeMinutes),
		CookTimeMinutes: optInt4(arg.CookTimeMinutes),
		IsActive:        arg.IsActive,
		UpdatedBy:       textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("update recipe: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("update recipe: %w", domainerr.ErrNotFound)
	}
	return nil
}

// DeleteRecipe removes a recipe and its related items/steps.
func (s *Service) DeleteRecipe(ctx context.Context, recipeID int64) error {
	return s.q.DeleteRecipe(ctx, recipeID)
}

// CreateRecipeWithChildren creates a recipe plus its items and steps in a
// single transaction; any failure rolls back the whole write.
func (s *Service) CreateRecipeWithChildren(ctx context.Context, arg Recipe, items []RecipeItem, steps []RecipeStep, by string) (Recipe, error) {
	var rec Recipe
	err := s.withTx(ctx, func(q sqlc.Querier) error {
		var err error
		rec, err = createRecipe(ctx, q, arg, by)
		if err != nil {
			return err
		}
		for _, item := range items {
			item.RecipeID = rec.RecipeID
			if err := addRecipeItem(ctx, q, item); err != nil {
				return err
			}
		}
		for _, step := range steps {
			step.RecipeID = rec.RecipeID
			if _, err := addRecipeStep(ctx, q, step, by); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Recipe{}, err
	}
	return rec, nil
}

// UpdateRecipeWithChildren updates a recipe row and replaces its items and
// steps in a single transaction; any failure leaves the prior state intact.
func (s *Service) UpdateRecipeWithChildren(ctx context.Context, recipeID int64, arg Recipe, items []RecipeItem, steps []RecipeStep, by string) error {
	return s.withTx(ctx, func(q sqlc.Querier) error {
		if err := updateRecipeRow(ctx, q, recipeID, arg, by); err != nil {
			return fmt.Errorf("update recipe: %w", err)
		}
		if err := q.DeleteRecipeItems(ctx, recipeID); err != nil {
			return fmt.Errorf("replace recipe items: %w", err)
		}
		if err := q.DeleteRecipeSteps(ctx, recipeID); err != nil {
			return fmt.Errorf("replace recipe steps: %w", err)
		}
		for _, item := range items {
			item.RecipeID = recipeID
			if err := addRecipeItem(ctx, q, item); err != nil {
				return err
			}
		}
		for _, step := range steps {
			step.RecipeID = recipeID
			if _, err := addRecipeStep(ctx, q, step, by); err != nil {
				return err
			}
		}
		return nil
	})
}

// RecipeItem is one ingredient in a recipe. IngredientID is the primary
// reference (the brand-agnostic ingredient); ItemID is an optional
// preferred-brand hint. SectionName groups items (e.g. "crust",
// "filling"); DisplayOrder controls ordering within the recipe.
// DeltaKind is set only when the line came through ApplyDelta — "" means
// untouched canonical, otherwise one of the DeltaItem* kinds.
type RecipeItem struct {
	RecipeItemID int64
	RecipeID     int64
	ItemID       *int64
	IngredientID *int64
	Quantity     float64
	UnitID       int64
	SectionName  string
	DisplayOrder int32
	Notes        string
	IsOptional   bool
	DeltaKind    string
}

// AddRecipeItem adds an item to a recipe.
func (s *Service) AddRecipeItem(ctx context.Context, arg RecipeItem) error {
	return addRecipeItem(ctx, s.q, arg)
}

func addRecipeItem(ctx context.Context, q sqlc.Querier, arg RecipeItem) error {
	if arg.Quantity <= 0 {
		return &domainerr.ValidationError{Msg: "recipe item quantity must be greater than zero"}
	}
	if arg.ItemID == nil && arg.IngredientID == nil {
		return &domainerr.ValidationError{Msg: "recipe item requires an item or ingredient"}
	}
	qty, err := numericFromFloat64(arg.Quantity)
	if err != nil {
		return fmt.Errorf("add recipe item: %w", err)
	}
	return q.AddRecipeItem(ctx, sqlc.AddRecipeItemParams{
		RecipeID:     arg.RecipeID,
		ItemID:       optInt8(arg.ItemID),
		IngredientID: optInt8(arg.IngredientID),
		Quantity:     qty,
		UnitID:       arg.UnitID,
		SectionName:  textOrNull(arg.SectionName),
		DisplayOrder: arg.DisplayOrder,
		Notes:        textOrNull(arg.Notes),
		IsOptional:   arg.IsOptional,
	})
}

// ListRecipeItems returns all items for a recipe.
func (s *Service) ListRecipeItems(ctx context.Context, recipeID int64) ([]RecipeItem, error) {
	rows, err := s.q.ListRecipeItems(ctx, recipeID)
	if err != nil {
		return nil, fmt.Errorf("list recipe items: %w", err)
	}
	return toRecipeItems(rows)
}

// ListRecipeItemsByRecipes returns all items for a set of recipes in one query.
func (s *Service) ListRecipeItemsByRecipes(ctx context.Context, recipeIDs []int64) ([]RecipeItem, error) {
	rows, err := s.q.ListRecipeItemsByRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("list recipe items by recipes: %w", err)
	}
	return toRecipeItems(rows)
}

func toRecipeItems(rows []sqlc.RecipeRecipeItem) ([]RecipeItem, error) {
	out := make([]RecipeItem, len(rows))
	for i := range rows {
		ri, err := toRecipeItem(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = ri
	}
	return out, nil
}

// RemoveRecipeItem removes an item from a recipe by its surrogate key.
func (s *Service) RemoveRecipeItem(ctx context.Context, recipeItemID int64) error {
	return s.q.RemoveRecipeItem(ctx, recipeItemID)
}

// RecipeStep is one step in a recipe. The timing fields feed the future
// event-timeline scheduler: DurationMinutes is how long the step takes,
// IsPassive marks hands-off work (rest, bake, marinade) that frees the
// cook, DependsOnStepNumber overrides the default previous-step edge, and
// Appliance names the resource the step occupies.
// DeltaKind mirrors RecipeItem.DeltaKind — set only on delta-applied
// views, one of the DeltaStep* kinds.
type RecipeStep struct {
	StepID              int64
	RecipeID            int64
	StepNumber          int32
	Instruction         string
	DurationMinutes     *int32
	StepType            string
	IsPassive           bool
	DependsOnStepNumber *int32
	Appliance           string
	DeltaKind           string
}

// AddRecipeStep adds a step to a recipe.
func (s *Service) AddRecipeStep(ctx context.Context, recipeID int64, stepNumber int32, instruction, by string) (RecipeStep, error) {
	return addRecipeStep(ctx, s.q, RecipeStep{RecipeID: recipeID, StepNumber: stepNumber, Instruction: instruction}, by)
}

func addRecipeStep(ctx context.Context, q sqlc.Querier, step RecipeStep, by string) (RecipeStep, error) {
	row, err := q.AddRecipeStep(ctx, sqlc.AddRecipeStepParams{
		RecipeID:            step.RecipeID,
		StepNumber:          step.StepNumber,
		Instruction:         step.Instruction,
		DurationMinutes:     optInt4(step.DurationMinutes),
		StepType:            textOrNull(step.StepType),
		IsPassive:           step.IsPassive,
		DependsOnStepNumber: optInt4(step.DependsOnStepNumber),
		Appliance:           textOrNull(step.Appliance),
		CreatedBy:           by,
		UpdatedBy:           textOrNull(by),
	})
	if err != nil {
		return RecipeStep{}, fmt.Errorf("add recipe step: %w", err)
	}
	return toRecipeStep(row), nil
}

// ListRecipeSteps returns all steps for a recipe.
func (s *Service) ListRecipeSteps(ctx context.Context, recipeID int64) ([]RecipeStep, error) {
	rows, err := s.q.ListRecipeSteps(ctx, recipeID)
	if err != nil {
		return nil, fmt.Errorf("list recipe steps: %w", err)
	}
	out := make([]RecipeStep, len(rows))
	for i := range rows {
		out[i] = toRecipeStep(rows[i])
	}
	return out, nil
}

// ListRecipeStepsByRecipes returns all steps for a set of recipes in one query.
func (s *Service) ListRecipeStepsByRecipes(ctx context.Context, recipeIDs []int64) ([]RecipeStep, error) {
	rows, err := s.q.ListRecipeStepsByRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("list recipe steps by recipes: %w", err)
	}
	out := make([]RecipeStep, len(rows))
	for i := range rows {
		out[i] = toRecipeStep(rows[i])
	}
	return out, nil
}

// UpdateRecipeStep modifies a step.
func (s *Service) UpdateRecipeStep(ctx context.Context, stepID int64, stepNumber int32, instruction, by string) error {
	n, err := s.q.UpdateRecipeStep(ctx, sqlc.UpdateRecipeStepParams{
		StepID:      stepID,
		StepNumber:  stepNumber,
		Instruction: instruction,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("update recipe step: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("update recipe step: %w", domainerr.ErrNotFound)
	}
	return nil
}

// DeleteRecipeStep removes a step.
func (s *Service) DeleteRecipeStep(ctx context.Context, stepID int64) error {
	return s.q.DeleteRecipeStep(ctx, stepID)
}

// RecipeRating is one user's 1-5 star rating of a recipe.
type RecipeRating struct {
	UserID   int64
	RecipeID int64
	Rating   int16
}

// RatingSummary is the aggregate rating for a recipe across all users.
type RatingSummary struct {
	RecipeID      int64
	AverageRating float64
	RatingCount   int64
}

// SetRating creates or updates a user's rating for a recipe. Ratings must
// be in the inclusive range 1-5.
func (s *Service) SetRating(ctx context.Context, userID, recipeID int64, rating int16, by string) (RecipeRating, error) {
	if rating < 1 || rating > 5 {
		return RecipeRating{}, fmt.Errorf("set rating: rating must be between 1 and 5")
	}
	row, err := s.q.UpsertRecipeRating(ctx, sqlc.UpsertRecipeRatingParams{
		UserID:    userID,
		RecipeID:  recipeID,
		Rating:    rating,
		CreatedBy: by,
		UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return RecipeRating{}, fmt.Errorf("set rating: %w", err)
	}
	return RecipeRating{UserID: row.UserID, RecipeID: row.RecipeID, Rating: row.Rating}, nil
}

// GetUserRating returns a user's rating for a recipe, or an error wrapping
// domainerr.ErrNotFound when the user has not rated it.
func (s *Service) GetUserRating(ctx context.Context, userID, recipeID int64) (RecipeRating, error) {
	row, err := s.q.GetRecipeRating(ctx, sqlc.GetRecipeRatingParams{UserID: userID, RecipeID: recipeID})
	if err != nil {
		return RecipeRating{}, fmt.Errorf("get recipe rating: %w", domainerr.FromStorage(err))
	}
	return RecipeRating{UserID: row.UserID, RecipeID: row.RecipeID, Rating: row.Rating}, nil
}

// ListRecipeRatings returns a user's ratings for a set of recipes in a
// single query.
func (s *Service) ListRecipeRatings(ctx context.Context, userID int64, recipeIDs []int64) ([]RecipeRating, error) {
	rows, err := s.q.ListRecipeRatings(ctx, sqlc.ListRecipeRatingsParams{UserID: userID, RecipeIds: recipeIDs})
	if err != nil {
		return nil, fmt.Errorf("list recipe ratings: %w", err)
	}
	out := make([]RecipeRating, len(rows))
	for i := range rows {
		out[i] = RecipeRating{UserID: rows[i].UserID, RecipeID: rows[i].RecipeID, Rating: rows[i].Rating}
	}
	return out, nil
}

// ListRatingSummaries returns the average rating and rating count for a set
// of recipes in a single query. Recipes with no ratings are absent.
func (s *Service) ListRatingSummaries(ctx context.Context, recipeIDs []int64) ([]RatingSummary, error) {
	rows, err := s.q.ListRecipeRatingSummaries(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("list rating summaries: %w", err)
	}
	out := make([]RatingSummary, len(rows))
	for i := range rows {
		out[i] = RatingSummary{
			RecipeID:      rows[i].RecipeID,
			AverageRating: rows[i].AverageRating,
			RatingCount:   rows[i].RatingCount,
		}
	}
	return out, nil
}

// ListRatedAtLeast returns the user's recipe ratings at or above
// minRating. Recency scoring lives in the BFF, which joins this data to
// mealplan.LastPlannedDates; recipe SQL never crosses schemas.
func (s *Service) ListRatedAtLeast(ctx context.Context, userID int64, minRating int16) ([]RecipeRating, error) {
	rows, err := s.q.ListRecipeRatingsAtLeast(ctx, sqlc.ListRecipeRatingsAtLeastParams{UserID: userID, Rating: minRating})
	if err != nil {
		return nil, fmt.Errorf("list rated at least: %w", err)
	}
	out := make([]RecipeRating, len(rows))
	for i, r := range rows {
		out[i] = RecipeRating{UserID: userID, RecipeID: r.RecipeID, Rating: r.Rating}
	}
	return out, nil
}

func toRecipe(row sqlc.RecipeRecipe) Recipe {
	r := Recipe{
		RecipeID: row.RecipeID,
		Name:     row.Name,
		IsActive: row.IsActive,
	}
	r.Description = row.Description.String
	if row.Servings.Valid {
		v := row.Servings.Int32
		r.Servings = &v
	}
	if row.PrepTimeMinutes.Valid {
		v := row.PrepTimeMinutes.Int32
		r.PrepTimeMinutes = &v
	}
	if row.CookTimeMinutes.Valid {
		v := row.CookTimeMinutes.Int32
		r.CookTimeMinutes = &v
	}
	if row.UpdatedAt.Valid {
		v := row.UpdatedAt.Time
		r.UpdatedAt = &v
	}
	return r
}

func toRecipeItem(row sqlc.RecipeRecipeItem) (RecipeItem, error) {
	ri := RecipeItem{
		RecipeItemID: row.RecipeItemID,
		RecipeID:     row.RecipeID,
		UnitID:       row.UnitID,
		SectionName:  row.SectionName.String,
		DisplayOrder: row.DisplayOrder,
		Notes:        row.Notes.String,
		IsOptional:   row.IsOptional,
	}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		ri.ItemID = &v
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		ri.IngredientID = &v
	}
	if row.Quantity.Valid {
		v, err := row.Quantity.Float64Value()
		if err != nil {
			return RecipeItem{}, fmt.Errorf("recipe item %d quantity: %w", row.RecipeItemID, err)
		}
		ri.Quantity = v.Float64
	}
	return ri, nil
}

func toRecipeStep(row sqlc.RecipeRecipeStep) RecipeStep {
	s := RecipeStep{
		StepID:      row.StepID,
		RecipeID:    row.RecipeID,
		StepNumber:  row.StepNumber,
		Instruction: row.Instruction,
		StepType:    row.StepType.String,
		IsPassive:   row.IsPassive,
		Appliance:   row.Appliance.String,
	}
	if row.DurationMinutes.Valid {
		v := row.DurationMinutes.Int32
		s.DurationMinutes = &v
	}
	if row.DependsOnStepNumber.Valid {
		v := row.DependsOnStepNumber.Int32
		s.DependsOnStepNumber = &v
	}
	return s
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func optInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func optInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func numericFromFloat64(f float64) (pgtype.Numeric, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: value is not finite", f)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: %w", f, err)
	}
	n.Valid = true
	return n, nil
}

// ---------- recipe categories (0035) ----------

// CategoryGroup is a typed bucket of recipe categories (Course, Cuisine,
// ...). Exclusive groups allow at most one category per recipe.
type CategoryGroup struct {
	CategoryGroupID int64
	Name            string
	Exclusive       bool
	DisplayOrder    int32
}

// Category is a single label inside a group, with the group's fields
// denormalized for callers that need exclusivity or display order.
type Category struct {
	CategoryID        int64
	CategoryGroupID   int64
	Name              string
	GroupName         string
	GroupExclusive    bool
	GroupDisplayOrder int32
}

func toCategoryGroup(row sqlc.RecipeCategoryGroup) CategoryGroup {
	return CategoryGroup{
		CategoryGroupID: row.CategoryGroupID,
		Name:            row.Name,
		Exclusive:       row.Exclusive,
		DisplayOrder:    row.DisplayOrder,
	}
}

func toCategory(row sqlc.RecipeCategory) Category {
	return Category{CategoryID: row.CategoryID, CategoryGroupID: row.CategoryGroupID, Name: row.Name}
}

// CreateCategoryGroup adds a category group (admin-curated taxonomy).
func (s *Service) CreateCategoryGroup(ctx context.Context, arg CategoryGroup, by string) (CategoryGroup, error) {
	row, err := s.q.CreateCategoryGroup(ctx, sqlc.CreateCategoryGroupParams{
		Name: arg.Name, Exclusive: arg.Exclusive, DisplayOrder: arg.DisplayOrder,
		CreatedBy: by, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return CategoryGroup{}, fmt.Errorf("create category group: %w", domainerr.FromStorage(err))
	}
	return toCategoryGroup(row), nil
}

// UpdateCategoryGroup edits a group's name/exclusivity/order.
func (s *Service) UpdateCategoryGroup(ctx context.Context, groupID int64, arg CategoryGroup, by string) (CategoryGroup, error) {
	row, err := s.q.UpdateCategoryGroup(ctx, sqlc.UpdateCategoryGroupParams{
		CategoryGroupID: groupID, Name: arg.Name, Exclusive: arg.Exclusive,
		DisplayOrder: arg.DisplayOrder, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return CategoryGroup{}, fmt.Errorf("update category group: %w", domainerr.FromStorage(err))
	}
	return toCategoryGroup(row), nil
}

// DeleteCategoryGroup removes a group; a group that still has categories
// cannot be deleted (RESTRICT) — the caller gets a friendly validation error.
func (s *Service) DeleteCategoryGroup(ctx context.Context, groupID int64) error {
	n, err := s.q.CountCategoriesInGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("delete category group: %w", domainerr.FromStorage(err))
	}
	if n > 0 {
		return &domainerr.ValidationError{Field: "id", Msg: "group still has categories — delete or reassign them first"}
	}
	if err := s.q.DeleteCategoryGroup(ctx, groupID); err != nil {
		return fmt.Errorf("delete category group: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ListCategoryGroups returns all groups in display order.
func (s *Service) ListCategoryGroups(ctx context.Context) ([]CategoryGroup, error) {
	rows, err := s.q.ListCategoryGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list category groups: %w", err)
	}
	out := make([]CategoryGroup, len(rows))
	for i, r := range rows {
		out[i] = toCategoryGroup(r)
	}
	return out, nil
}

// CreateCategory adds a category to a group.
func (s *Service) CreateCategory(ctx context.Context, arg Category, by string) (Category, error) {
	row, err := s.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		CategoryGroupID: arg.CategoryGroupID, Name: arg.Name,
		CreatedBy: by, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", domainerr.FromStorage(err))
	}
	return toCategory(row), nil
}

// UpdateCategory renames a category (the group is fixed).
func (s *Service) UpdateCategory(ctx context.Context, categoryID int64, arg Category, by string) (Category, error) {
	row, err := s.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		CategoryID: categoryID, Name: arg.Name, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return Category{}, fmt.Errorf("update category: %w", domainerr.FromStorage(err))
	}
	return toCategory(row), nil
}

// DeleteCategory removes a category; recipe assignments cascade away.
func (s *Service) DeleteCategory(ctx context.Context, categoryID int64) error {
	if err := s.q.DeleteCategory(ctx, categoryID); err != nil {
		return fmt.Errorf("delete category: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ListCategoriesByGroup returns a group's categories alphabetically.
func (s *Service) ListCategoriesByGroup(ctx context.Context, groupID int64) ([]Category, error) {
	rows, err := s.q.ListCategoriesByGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = toCategory(r)
	}
	return out, nil
}

// ListCategoriesByIDs returns the categories for a set of IDs with their
// groups' exclusivity — the input to assignment validation.
func (s *Service) ListCategoriesByIDs(ctx context.Context, categoryIDs []int64) ([]Category, error) {
	rows, err := s.q.ListCategoriesByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, fmt.Errorf("list categories by ids: %w", err)
	}
	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = Category{
			CategoryID: r.CategoryID, CategoryGroupID: r.CategoryGroupID, Name: r.Name,
			GroupName: r.GroupName, GroupExclusive: r.GroupExclusive, GroupDisplayOrder: r.GroupDisplayOrder,
		}
	}
	return out, nil
}

// ListCategoriesForRecipes batch-loads every recipe's categories (with group
// metadata) for resolver preloads — returns a map keyed by recipe ID.
func (s *Service) ListCategoriesForRecipes(ctx context.Context, recipeIDs []int64) (map[int64][]Category, error) {
	rows, err := s.q.ListCategoriesForRecipes(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("list categories for recipes: %w", err)
	}
	out := make(map[int64][]Category, len(recipeIDs))
	for _, r := range rows {
		out[r.RecipeID] = append(out[r.RecipeID], Category{
			CategoryID: r.CategoryID, CategoryGroupID: r.CategoryGroupID, Name: r.Name,
			GroupName: r.GroupName, GroupExclusive: r.GroupExclusive, GroupDisplayOrder: r.GroupDisplayOrder,
		})
	}
	return out, nil
}

// SetRecipeCategories replaces a recipe's category assignments atomically.
// A recipe may hold at most one category per exclusive group — picking two
// from the same exclusive group is a validation error.
func (s *Service) SetRecipeCategories(ctx context.Context, recipeID int64, categoryIDs []int64, by string) error {
	if _, err := s.q.GetRecipeByID(ctx, recipeID); err != nil {
		return fmt.Errorf("set recipe categories: %w", domainerr.FromStorage(err))
	}
	if err := s.validateCategorySet(ctx, categoryIDs); err != nil {
		return err
	}
	return s.withTx(ctx, func(q sqlc.Querier) error {
		if err := q.ClearRecipeCategories(ctx, recipeID); err != nil {
			return fmt.Errorf("set recipe categories: %w", err)
		}
		if len(categoryIDs) == 0 {
			return nil
		}
		if err := q.AddRecipeCategories(ctx, sqlc.AddRecipeCategoriesParams{
			Column2: categoryIDs, RecipeID: recipeID, AssignedBy: by,
		}); err != nil {
			return fmt.Errorf("set recipe categories: %w", domainerr.FromStorage(err))
		}
		return nil
	})
}

// validateCategorySet enforces per-exclusive-group uniqueness.
func (s *Service) validateCategorySet(ctx context.Context, categoryIDs []int64) error {
	if len(categoryIDs) == 0 {
		return nil
	}
	cats, err := s.ListCategoriesByIDs(ctx, categoryIDs)
	if err != nil {
		return err
	}
	if len(cats) != len(categoryIDs) {
		return &domainerr.ValidationError{Field: "categoryIds", Msg: "one or more categories do not exist"}
	}
	seen := make(map[int64]string, len(cats))
	for _, c := range cats {
		if !c.GroupExclusive {
			continue
		}
		if prev, dup := seen[c.CategoryGroupID]; dup {
			return &domainerr.ValidationError{
				Field: "categoryIds",
				Msg:   fmt.Sprintf("a recipe can't be both %q and %q (%s)", prev, c.Name, c.GroupName),
			}
		}
		seen[c.CategoryGroupID] = c.Name
	}
	return nil
}

// ---------- filtered, engagement-ranked search ----------

// RecipeSearch carries the filter and ranking inputs for SearchRecipes. All
// sets are optional; nil/empty disables them. Include/Exclude filter the
// result set (favorites); Favorite/Used/Viewed/SearchTerms drive the tiered
// ordering. Used/Viewed should arrive pre-sorted by signal strength.
type RecipeSearch struct {
	Active      bool
	Search      string
	CategoryIDs []int64
	IncludeIDs  []int64
	ExcludeIDs  []int64
	FavoriteIDs []int64
	UsedIDs     []int64
	ViewedIDs   []int64
	SearchTerms []string
	// CourseBoostID, when set, ranks recipes tagged with that category
	// directly below favorites — the meal-plan slot picker passes the
	// Course category matching the slot's meal_type.
	CourseBoostID *int64
	Limit         int32
	Offset        int32
}

func (rs RecipeSearch) params() sqlc.SearchRecipesParams {
	return sqlc.SearchRecipesParams{
		IsActive: rs.Active, Search: rs.Search,
		CategoryIds: rs.CategoryIDs, IncludeIds: rs.IncludeIDs, ExcludeIds: rs.ExcludeIDs,
		FavoriteIds: rs.FavoriteIDs, UsedIds: rs.UsedIDs, ViewedIds: rs.ViewedIDs,
		SearchTerms: rs.SearchTerms, Limit: rs.Limit, Offset: rs.Offset,
		BoostCategoryID: optInt8(rs.CourseBoostID),
	}
}

// SearchRecipes returns one page of recipes matching the filters, ordered by
// engagement tier then name.
func (s *Service) SearchRecipes(ctx context.Context, arg RecipeSearch) ([]Recipe, error) {
	rows, err := s.q.SearchRecipes(ctx, arg.params())
	if err != nil {
		return nil, fmt.Errorf("search recipes: %w", err)
	}
	out := make([]Recipe, len(rows))
	for i, r := range rows {
		out[i] = toRecipe(r)
	}
	return out, nil
}

// SetRecipeEmbedding stores a recipe's embedding (as a pgvector text
// literal) and records which model produced it.
func (s *Service) SetRecipeEmbedding(ctx context.Context, recipeID int64, embedding, model string) error {
	return s.q.SetRecipeEmbedding(ctx, sqlc.SetRecipeEmbeddingParams{
		Embedding:      embedding,
		EmbeddingModel: textOrNull(model),
		RecipeID:       recipeID,
	})
}

// ClearRecipeEmbedding drops a recipe's embedding. The next backfill sweep
// sees the NULL and re-embeds.
func (s *Service) ClearRecipeEmbedding(ctx context.Context, recipeID int64) error {
	return s.q.ClearRecipeEmbedding(ctx, recipeID)
}

// ListEmbeddingCandidates returns active recipe IDs whose embedding is
// missing or was produced by a different model — the backfill work set.
func (s *Service) ListEmbeddingCandidates(ctx context.Context, model string, limit int32) ([]int64, error) {
	return s.q.ListEmbeddingCandidates(ctx, sqlc.ListEmbeddingCandidatesParams{Model: model, Limit: limit})
}

// SemanticSearch carries the query vector plus the RecipeSearch filters the
// semantic listing shares with keyword mode. QueryVector is a pgvector text
// literal produced by an llm.Embedder; CategoryIDs/Include/Exclude filter,
// Favorite/Used/Viewed feed the small engagement blend.
type SemanticSearch struct {
	Active      bool
	QueryVector string
	CategoryIDs []int64
	IncludeIDs  []int64
	ExcludeIDs  []int64
	FavoriteIDs []int64
	UsedIDs     []int64
	ViewedIDs   []int64
	Limit       int32
	Offset      int32
}

// SemanticResult is one semantic-search hit: the recipe plus its cosine
// distance to the query vector (0 = identical; the tool exposes it as a
// 1-distance score).
type SemanticResult struct {
	Recipe   Recipe
	Distance float64
}

func (ss SemanticSearch) params() sqlc.SearchRecipesSemanticParams {
	return sqlc.SearchRecipesSemanticParams{
		IsActive: ss.Active, QueryVec: ss.QueryVector,
		CategoryIds: ss.CategoryIDs, IncludeIds: ss.IncludeIDs, ExcludeIds: ss.ExcludeIDs,
		FavoriteIds: ss.FavoriteIDs, UsedIds: ss.UsedIDs, ViewedIds: ss.ViewedIDs,
		Limit: ss.Limit, Offset: ss.Offset,
	}
}

// SearchRecipesSemantic returns one page of embedded recipes ranked by
// blended cosine distance + engagement boost.
func (s *Service) SearchRecipesSemantic(ctx context.Context, arg SemanticSearch) ([]SemanticResult, error) {
	rows, err := s.q.SearchRecipesSemantic(ctx, arg.params())
	if err != nil {
		return nil, fmt.Errorf("semantic search recipes: %w", err)
	}
	out := make([]SemanticResult, len(rows))
	for i, r := range rows {
		out[i] = SemanticResult{Recipe: toRecipe(sqlc.RecipeRecipe{
			RecipeID: r.RecipeID, Name: r.Name, Description: r.Description,
			Servings: r.Servings, PrepTimeMinutes: r.PrepTimeMinutes,
			CookTimeMinutes: r.CookTimeMinutes, IsActive: r.IsActive,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt,
			UpdatedBy: r.UpdatedBy, UpdatedAt: r.UpdatedAt,
		}), Distance: r.Distance}
	}
	return out, nil
}

// CountSearchRecipesSemantic returns the un-paged embedded match count for
// the same filters (engagement inputs don't affect the count).
func (s *Service) CountSearchRecipesSemantic(ctx context.Context, arg SemanticSearch) (int64, error) {
	n, err := s.q.CountSearchRecipesSemantic(ctx, sqlc.CountSearchRecipesSemanticParams{
		IsActive:    arg.Active,
		CategoryIds: arg.CategoryIDs, IncludeIds: arg.IncludeIDs, ExcludeIds: arg.ExcludeIDs,
	})
	if err != nil {
		return 0, fmt.Errorf("count semantic search recipes: %w", err)
	}
	return n, nil
}

// CountSearchRecipes returns the un-paged match count for the same filters.
func (s *Service) CountSearchRecipes(ctx context.Context, arg RecipeSearch) (int64, error) {
	n, err := s.q.CountSearchRecipes(ctx, sqlc.CountSearchRecipesParams{
		IsActive: arg.Active, Search: arg.Search,
		CategoryIds: arg.CategoryIDs, IncludeIds: arg.IncludeIDs, ExcludeIds: arg.ExcludeIDs,
	})
	if err != nil {
		return 0, fmt.Errorf("count search recipes: %w", err)
	}
	return n, nil
}
