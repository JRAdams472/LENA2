package bff

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipeimport"
)

// RecipeImport resolves a single recipe import by ID.
func (r *Resolver) RecipeImport(ctx context.Context, args struct{ ID graphql.ID }) (*recipeImportResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	ri, err := r.RecipeImportService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeImportResolver{ri: ri, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService}, nil
}

// RecipeImports returns a paged list of recipe imports, optionally filtered by status.
func (r *Resolver) RecipeImports(ctx context.Context, args struct {
	Page     int32
	PageSize int32
	Status   *string
}) (*recipeImportPageResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	status := ""
	if args.Status != nil {
		status = *args.Status
	}
	items, err := r.RecipeImportService.List(ctx, status, args.Page, args.PageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.RecipeImportService.Count(ctx, status)
	if err != nil {
		return nil, err
	}
	return &recipeImportPageResolver{
		items: items, page: args.Page, pageSize: args.PageSize, total: total,
		inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService,
	}, nil
}

// PendingRecipeImports returns recipe imports awaiting admin attention.
func (r *Resolver) PendingRecipeImports(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*recipeImportPageResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	items, err := r.RecipeImportService.ListPending(ctx, args.Page, args.PageSize)
	if err != nil {
		return nil, err
	}
	return &recipeImportPageResolver{
		items: items, page: args.Page, pageSize: args.PageSize, total: int64(len(items)),
		inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService,
	}, nil
}

// UpdateRecipeImport applies admin edits to the review JSON.
func (r *Resolver) UpdateRecipeImport(ctx context.Context, args struct {
	ID    graphql.ID
	Input recipeImportReviewInput
}) (*recipeImportResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	review, err := args.Input.toReviewRecipe()
	if err != nil {
		return nil, err
	}
	updated, err := r.RecipeImportService.UpdateReview(ctx, id, review, u.Email)
	if err != nil {
		return nil, err
	}
	return &recipeImportResolver{ri: updated, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService}, nil
}

// ApproveRecipeImport persists the approved review as a catalog recipe.
func (r *Resolver) ApproveRecipeImport(ctx context.Context, args struct{ ID graphql.ID }) (*recipeResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	recipe, _, err := r.RecipeImportService.Approve(ctx, id, u)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService, user: u, recipe: *recipe}, nil
}

// RejectRecipeImport rejects a recipe import.
func (r *Resolver) RejectRecipeImport(ctx context.Context, args struct{ ID graphql.ID }) (*recipeImportResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	if err := r.RecipeImportService.Reject(ctx, id); err != nil {
		return nil, err
	}
	ri, err := r.RecipeImportService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeImportResolver{ri: ri, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService}, nil
}

// RetryRecipeImport resets and re-enqueues a recipe import.
func (r *Resolver) RetryRecipeImport(ctx context.Context, args struct{ ID graphql.ID }) (*recipeImportResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	if err := r.RecipeImportService.Retry(ctx, id); err != nil {
		return nil, err
	}
	ri, err := r.RecipeImportService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &recipeImportResolver{ri: ri, inv: r.InventoryService, rec: r.RecipeService, up: r.UserPrefsService}, nil
}

type recipeImportResolver struct {
	ri  *recipeimport.RecipeImport
	inv InventoryService
	rec RecipeService
	up  UserPrefsService
}

func (r *recipeImportResolver) ID() graphql.ID          { return graphql.ID(strconv.FormatInt(r.ri.ID, 10)) }
func (r *recipeImportResolver) Status() string          { return r.ri.Status }
func (r *recipeImportResolver) SourceFilename() string  { return r.ri.SourceFilename }
func (r *recipeImportResolver) ProfanityFlag() bool     { return r.ri.ProfanityFlag }
func (r *recipeImportResolver) CreatedAt() graphql.Time { return graphql.Time{Time: r.ri.CreatedAt} }
func (r *recipeImportResolver) CreatedBy() string       { return r.ri.CreatedBy }

func (r *recipeImportResolver) OCRText() *string {
	if r.ri.OCRText == "" {
		return nil
	}
	return &r.ri.OCRText
}

func (r *recipeImportResolver) ProfanityReason() *string { return nilIfEmpty(r.ri.ProfanityReason) }
func (r *recipeImportResolver) ErrorMessage() *string    { return nilIfEmpty(r.ri.ErrorMessage) }
func (r *recipeImportResolver) UpdatedAt() *graphql.Time { return timeToGraphQL(r.ri.UpdatedAt) }

func (r *recipeImportResolver) Draft() *recipeImportDraftResolver {
	if len(r.ri.DraftJSON) == 0 {
		return nil
	}
	var draft ocrimport.RecipeDraft
	if err := json.Unmarshal(r.ri.DraftJSON, &draft); err != nil {
		return nil
	}
	return &recipeImportDraftResolver{draft: draft}
}

func (r *recipeImportResolver) Review() *recipeImportReviewResolver {
	if len(r.ri.ReviewJSON) == 0 {
		return nil
	}
	var review ocrimport.ReviewRecipe
	if err := json.Unmarshal(r.ri.ReviewJSON, &review); err != nil {
		return nil
	}
	return &recipeImportReviewResolver{review: review}
}

func (r *recipeImportResolver) Recipe(ctx context.Context) (*recipeResolver, error) {
	if r.ri.RecipeID == nil {
		return nil, nil
	}
	recipe, err := r.rec.GetRecipeByID(ctx, *r.ri.RecipeID)
	if err != nil {
		return nil, err
	}
	u, _ := currentuser.FromContext(ctx)
	return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: u, recipe: recipe}, nil
}

type recipeImportDraftResolver struct{ draft ocrimport.RecipeDraft }

func (r *recipeImportDraftResolver) Name() *string {
	if r.draft.Name == "" {
		return nil
	}
	return &r.draft.Name
}
func (r *recipeImportDraftResolver) Description() *string { return r.draft.Description }
func (r *recipeImportDraftResolver) Servings() *int32     { return optIntToInt32(r.draft.Servings) }
func (r *recipeImportDraftResolver) PrepTimeMinutes() *int32 {
	return optIntToInt32(r.draft.PrepTimeMinutes)
}
func (r *recipeImportDraftResolver) CookTimeMinutes() *int32 {
	return optIntToInt32(r.draft.CookTimeMinutes)
}
func (r *recipeImportDraftResolver) SourceHint() *string { return r.draft.SourceHint }

func (r *recipeImportDraftResolver) Items() []*recipeImportDraftItemResolver {
	out := make([]*recipeImportDraftItemResolver, len(r.draft.Items))
	for i := range r.draft.Items {
		out[i] = &recipeImportDraftItemResolver{item: r.draft.Items[i]}
	}
	return out
}

func (r *recipeImportDraftResolver) Steps() []*recipeImportDraftStepResolver {
	out := make([]*recipeImportDraftStepResolver, len(r.draft.Steps))
	for i := range r.draft.Steps {
		out[i] = &recipeImportDraftStepResolver{step: r.draft.Steps[i]}
	}
	return out
}

type recipeImportDraftItemResolver struct{ item ocrimport.DraftItem }

func (r *recipeImportDraftItemResolver) Quantity() *float64 { return r.item.Quantity }
func (r *recipeImportDraftItemResolver) Unit() *string      { return r.item.Unit }
func (r *recipeImportDraftItemResolver) Ingredient() string { return r.item.Ingredient }
func (r *recipeImportDraftItemResolver) Section() *string   { return r.item.Section }
func (r *recipeImportDraftItemResolver) Notes() *string     { return r.item.Notes }
func (r *recipeImportDraftItemResolver) IsOptional() bool   { return r.item.IsOptional }

type recipeImportDraftStepResolver struct{ step ocrimport.DraftStep }

func (r *recipeImportDraftStepResolver) StepNumber() int32   { return toInt32(r.step.StepNumber) }
func (r *recipeImportDraftStepResolver) Instruction() string { return r.step.Instruction }

type recipeImportReviewResolver struct{ review ocrimport.ReviewRecipe }

func (r *recipeImportReviewResolver) PageID() *string { return nilIfEmpty(r.review.PageID) }
func (r *recipeImportReviewResolver) Name() *string {
	if r.review.Name == "" {
		return nil
	}
	return &r.review.Name
}
func (r *recipeImportReviewResolver) Description() *string { return r.review.Description }
func (r *recipeImportReviewResolver) Servings() *int32     { return optIntToInt32(r.review.Servings) }
func (r *recipeImportReviewResolver) PrepTimeMinutes() *int32 {
	return optIntToInt32(r.review.PrepTime)
}
func (r *recipeImportReviewResolver) CookTimeMinutes() *int32 {
	return optIntToInt32(r.review.CookTime)
}
func (r *recipeImportReviewResolver) SourceHint() *string { return r.review.SourceHint }
func (r *recipeImportReviewResolver) Approved() bool      { return r.review.Approved }

func (r *recipeImportReviewResolver) Items() []*recipeImportReviewItemResolver {
	out := make([]*recipeImportReviewItemResolver, len(r.review.Items))
	for i := range r.review.Items {
		out[i] = &recipeImportReviewItemResolver{item: r.review.Items[i]}
	}
	return out
}

func (r *recipeImportReviewResolver) Steps() []*recipeImportReviewStepResolver {
	out := make([]*recipeImportReviewStepResolver, len(r.review.Steps))
	for i := range r.review.Steps {
		out[i] = &recipeImportReviewStepResolver{step: r.review.Steps[i]}
	}
	return out
}

type recipeImportReviewItemResolver struct{ item ocrimport.MatchResult }

func (r *recipeImportReviewItemResolver) DraftItem() *recipeImportDraftItemResolver {
	return &recipeImportDraftItemResolver{item: r.item.DraftItem}
}
func (r *recipeImportReviewItemResolver) ItemID() *graphql.ID {
	if r.item.ItemID == "" {
		return nil
	}
	id := graphql.ID(r.item.ItemID)
	return &id
}
func (r *recipeImportReviewItemResolver) ItemName() *string { return nilIfEmpty(r.item.ItemName) }
func (r *recipeImportReviewItemResolver) Unit() *string     { return nilIfEmpty(r.item.Unit) }
func (r *recipeImportReviewItemResolver) UnitID() *graphql.ID {
	if r.item.UnitID == "" {
		return nil
	}
	id := graphql.ID(r.item.UnitID)
	return &id
}
func (r *recipeImportReviewItemResolver) Confidence() float64 { return r.item.Confidence }

func (r *recipeImportReviewItemResolver) Suggestions() []*recipeImportSuggestionResolver {
	out := make([]*recipeImportSuggestionResolver, len(r.item.Suggestions))
	for i := range r.item.Suggestions {
		out[i] = &recipeImportSuggestionResolver{s: r.item.Suggestions[i]}
	}
	return out
}

func (r *recipeImportReviewItemResolver) Status() string { return r.item.Status }
func (r *recipeImportReviewItemResolver) Notes() *string { return nilIfEmpty(r.item.Notes) }
func (r *recipeImportReviewItemResolver) Approved() bool { return r.item.Approved }

type recipeImportSuggestionResolver struct{ s ocrimport.Suggestion }

func (r *recipeImportSuggestionResolver) ID() graphql.ID { return graphql.ID(r.s.ID) }
func (r *recipeImportSuggestionResolver) Name() string   { return r.s.Name }
func (r *recipeImportSuggestionResolver) Kind() string   { return r.s.Kind }
func (r *recipeImportSuggestionResolver) Score() float64 { return r.s.Score }

type recipeImportReviewStepResolver struct{ step ocrimport.DraftStep }

func (r *recipeImportReviewStepResolver) StepNumber() int32   { return toInt32(r.step.StepNumber) }
func (r *recipeImportReviewStepResolver) Instruction() string { return r.step.Instruction }

type recipeImportPageResolver struct {
	items    []recipeimport.RecipeImport
	page     int32
	pageSize int32
	total    int64
	inv      InventoryService
	rec      RecipeService
	up       UserPrefsService
}

func (r *recipeImportPageResolver) Items() []*recipeImportResolver {
	out := make([]*recipeImportResolver, len(r.items))
	for i := range r.items {
		out[i] = &recipeImportResolver{ri: &r.items[i], inv: r.inv, rec: r.rec, up: r.up}
	}
	return out
}
func (r *recipeImportPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: int64ToInt32(r.total)}
}

// Input types mirror the GraphQL input definitions.

type recipeImportReviewInput struct {
	Name            *string
	Description     *string
	Servings        *int32
	PrepTimeMinutes *int32
	CookTimeMinutes *int32
	SourceHint      *string
	Items           []recipeImportReviewItemInput
	Steps           []recipeImportReviewStepInput
}

func (r recipeImportReviewInput) toReviewRecipe() (*ocrimport.ReviewRecipe, error) {
	var servings, prep, cook *int
	if r.Servings != nil {
		s := int(*r.Servings)
		servings = &s
	}
	if r.PrepTimeMinutes != nil {
		p := int(*r.PrepTimeMinutes)
		prep = &p
	}
	if r.CookTimeMinutes != nil {
		c := int(*r.CookTimeMinutes)
		cook = &c
	}
	name := ""
	if r.Name != nil {
		name = *r.Name
	}

	items := make([]ocrimport.MatchResult, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, ocrimport.MatchResult{
			DraftItem: ocrimport.DraftItem{
				Quantity:   it.Quantity,
				Unit:       it.Unit,
				Ingredient: it.Ingredient,
				Section:    it.Section,
				Notes:      it.Notes,
				IsOptional: it.IsOptional,
			},
			ItemID:   stringPtrID(it.ItemID),
			ItemName: stringPtr(it.ItemName),
			Unit:     stringPtr(it.Unit),
			UnitID:   stringPtrID(it.UnitID),
			Approved: it.Approved,
		})
	}

	steps := make([]ocrimport.DraftStep, 0, len(r.Steps))
	for _, s := range r.Steps {
		steps = append(steps, ocrimport.DraftStep{
			StepNumber:  int(s.StepNumber),
			Instruction: s.Instruction,
		})
	}

	return &ocrimport.ReviewRecipe{
		Name:        name,
		Description: r.Description,
		Servings:    servings,
		PrepTime:    prep,
		CookTime:    cook,
		SourceHint:  r.SourceHint,
		Items:       items,
		Steps:       steps,
	}, nil
}

type recipeImportReviewItemInput struct {
	Ingredient string
	Quantity   *float64
	Unit       *string
	Section    *string
	Notes      *string
	IsOptional bool
	ItemID     *graphql.ID
	ItemName   *string
	UnitID     *graphql.ID
	Approved   bool
}

type recipeImportReviewStepInput struct {
	StepNumber  int32
	Instruction string
}

func toInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtrID(s *graphql.ID) string {
	if s == nil {
		return ""
	}
	return string(*s)
}

func optIntToInt32(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := toInt32(*v)
	return &i
}
