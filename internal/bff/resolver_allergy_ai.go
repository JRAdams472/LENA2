package bff

import (
	"context"
	"strconv"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/inventory"
)

// allergenSuggestionResolver renders one review-queue row. Names ride on
// the row (the list query joins them) so the admin UI needs no lookups.
type allergenSuggestionResolver struct {
	r *Resolver
	s inventory.AllergenSuggestion
}

func (r *allergenSuggestionResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.s.AllergenSuggestionID, 10))
}

func (r *allergenSuggestionResolver) RecipeID() *graphql.ID {
	if r.s.RecipeID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.s.RecipeID, 10))
	return &id
}

func (r *allergenSuggestionResolver) RecipeName() *string {
	if r.s.RecipeName == "" {
		return nil
	}
	return &r.s.RecipeName
}

func (r *allergenSuggestionResolver) TargetKind() string { return r.s.TargetKind }

func (r *allergenSuggestionResolver) IngredientID() *graphql.ID {
	if r.s.IngredientID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.s.IngredientID, 10))
	return &id
}

func (r *allergenSuggestionResolver) IngredientName() *string {
	if r.s.IngredientName == "" {
		return nil
	}
	return &r.s.IngredientName
}

func (r *allergenSuggestionResolver) ItemID() *graphql.ID {
	if r.s.ItemID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.s.ItemID, 10))
	return &id
}

func (r *allergenSuggestionResolver) ItemName() *string {
	if r.s.ItemName == "" {
		return nil
	}
	return &r.s.ItemName
}

func (r *allergenSuggestionResolver) Allergen() *allergenResolver {
	return &allergenResolver{a: inventory.Allergen{AllergenID: r.s.AllergenID, Name: r.s.AllergenName, IsActive: true}}
}

func (r *allergenSuggestionResolver) Kind() string { return r.s.Kind }

func (r *allergenSuggestionResolver) Rationale() *string {
	if r.s.Rationale == "" {
		return nil
	}
	return &r.s.Rationale
}

func (r *allergenSuggestionResolver) Status() string { return r.s.Status }

func (r *allergenSuggestionResolver) ReviewedAt() *string {
	if r.s.ReviewedAt == nil {
		return nil
	}
	t := r.s.ReviewedAt.Format(time.RFC3339)
	return &t
}

// AllergenSuggestions lists the review queue — admin only. status narrows
// to pending/accepted/dismissed; unset returns every row.
func (r *Resolver) AllergenSuggestions(ctx context.Context, args struct {
	Status *string
}) ([]*allergenSuggestionResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	status := ""
	if args.Status != nil {
		status = *args.Status
		if status != "pending" && status != "accepted" && status != "dismissed" {
			return nil, badInputf("status must be pending, accepted, or dismissed")
		}
	}
	rows, err := r.InventoryService.ListAllergenSuggestions(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make([]*allergenSuggestionResolver, len(rows))
	for i, s := range rows {
		out[i] = &allergenSuggestionResolver{r: r, s: s}
	}
	return out, nil
}

// SuggestRecipeAllergens runs the AI flag suggester over one recipe and
// enqueues the validated proposals — admin only. Returns the rows that
// were actually created; proposals duplicating an open suggestion or an
// already-curated flag drop out server-side.
func (r *Resolver) SuggestRecipeAllergens(ctx context.Context, args struct {
	RecipeID       graphql.ID
	MaxSuggestions *int32
}) ([]*allergenSuggestionResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	limit := 0
	if args.MaxSuggestions != nil {
		limit = int(*args.MaxSuggestions)
		if limit < 1 || limit > 20 {
			return nil, badInputf("maxSuggestions must be 1-20")
		}
	}
	proposals, err := r.AIService.SuggestAllergens(ctx, u.UserID, u.HouseholdID, recipeID, limit)
	if err != nil {
		return nil, err
	}
	newRows := make([]inventory.NewAllergenSuggestion, len(proposals))
	for i, p := range proposals {
		newRows[i] = inventory.NewAllergenSuggestion{
			RecipeID:    &recipeID,
			TargetKind:  p.TargetKind,
			TargetID:    p.TargetID,
			AllergenID:  p.AllergenID,
			Kind:        p.Kind,
			Rationale:   p.Reason,
			SuggestedBy: &u.UserID,
		}
	}
	created, err := r.InventoryService.CreateAllergenSuggestions(ctx, newRows, u.Email)
	if err != nil {
		return nil, err
	}
	out := make([]*allergenSuggestionResolver, 0, len(created))
	for _, c := range created {
		full, err := r.InventoryService.GetAllergenSuggestion(ctx, c.AllergenSuggestionID)
		if err != nil {
			return nil, err
		}
		out = append(out, &allergenSuggestionResolver{r: r, s: full})
	}
	return out, nil
}

// AcceptAllergenSuggestion writes the proposed flag under the reviewer's
// attribution and marks the row accepted — one transaction.
func (r *Resolver) AcceptAllergenSuggestion(ctx context.Context, args struct {
	ID graphql.ID
}) (*allergenSuggestionResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	if err := r.InventoryService.AcceptAllergenSuggestion(ctx, id, u.UserID, u.Email); err != nil {
		return nil, err
	}
	return r.suggestionByID(ctx, id)
}

// DismissAllergenSuggestion marks a pending row dismissed.
func (r *Resolver) DismissAllergenSuggestion(ctx context.Context, args struct {
	ID graphql.ID
}) (*allergenSuggestionResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	if err := r.InventoryService.DismissAllergenSuggestion(ctx, id, u.UserID, u.Email); err != nil {
		return nil, err
	}
	return r.suggestionByID(ctx, id)
}

func (r *Resolver) suggestionByID(ctx context.Context, id int64) (*allergenSuggestionResolver, error) {
	s, err := r.InventoryService.GetAllergenSuggestion(ctx, id)
	if err != nil {
		return nil, err
	}
	return &allergenSuggestionResolver{r: r, s: s}, nil
}
