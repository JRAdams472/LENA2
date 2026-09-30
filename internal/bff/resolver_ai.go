package bff

import (
	"context"
	"errors"
	"strconv"

	"github.com/JRAdams472/LENA2/internal/ai"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/graph-gophers/graphql-go"
)

// AIAvailable reports whether the assistant is configured. Clients use it
// to hide AI entry points instead of erroring on first use.
func (r *Resolver) AIAvailable(ctx context.Context) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	return r.AIService != nil && r.AIService.Available(), nil
}

// AskAssistant runs one free-form assistant request. The model may use the
// read-only household tools; it can never write — actions always go through
// the normal mutations.
func (r *Resolver) AskAssistant(ctx context.Context, args struct{ Question string }) (*assistantAnswerResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if r.AIService == nil || !r.AIService.Available() {
		return nil, errUnavailablef("the AI assistant is not configured on this deployment")
	}
	if !r.aiLimiter().allow(u.UserID) {
		return nil, errUnavailablef("assistant rate limit reached — try again shortly")
	}
	ans, err := r.AIService.Ask(ctx, u.UserID, u.HouseholdID, args.Question)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrUnavailable):
			return nil, errUnavailablef("the AI assistant is not configured on this deployment")
		case errors.Is(err, ai.ErrInvalidQuestion):
			return nil, badInputf("question must be 1-2000 characters")
		default:
			return nil, err
		}
	}
	out := &assistantAnswerResolver{answer: ans.Text}
	for _, t := range ans.Tools {
		out.toolCalls = append(out.toolCalls, &assistantToolCallResolver{name: t.Name})
	}
	return out, nil
}

// SuggestMeals returns AI meal suggestions for a plan's open cells —
// reviewable cards the client applies through addMealSlot.
func (r *Resolver) SuggestMeals(ctx context.Context, args struct {
	MealPlanID     graphql.ID
	MaxSuggestions int32
}) ([]*mealSuggestionResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if r.AIService == nil || !r.AIService.Available() {
		return nil, errUnavailablef("the AI assistant is not configured on this deployment")
	}
	if !r.aiLimiter().allow(u.UserID) {
		return nil, errUnavailablef("assistant rate limit reached — try again shortly")
	}
	mealPlanID, err := parseID(string(args.MealPlanID))
	if err != nil {
		return nil, err
	}
	limit := int(args.MaxSuggestions)
	if limit < 1 || limit > 10 {
		return nil, badInputf("maxSuggestions must be 1-10")
	}
	suggs, err := r.AIService.SuggestMeals(ctx, u.UserID, u.HouseholdID, mealPlanID, limit)
	if err != nil {
		if errors.Is(err, ai.ErrUnavailable) {
			return nil, errUnavailablef("the AI assistant is not configured on this deployment")
		}
		return nil, err
	}
	out := make([]*mealSuggestionResolver, len(suggs))
	for i, sg := range suggs {
		out[i] = &mealSuggestionResolver{r: r, user: u, sugg: sg}
	}
	return out, nil
}

// SuggestEventFixes returns AI fixes for an event timeline's conflicts —
// reviewable cards the client applies through the existing mutations.
func (r *Resolver) SuggestEventFixes(ctx context.Context, args struct {
	FoodEventID    graphql.ID
	MaxSuggestions int32
}) ([]*eventFixResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if r.AIService == nil || !r.AIService.Available() {
		return nil, errUnavailablef("the AI assistant is not configured on this deployment")
	}
	if !r.aiLimiter().allow(u.UserID) {
		return nil, errUnavailablef("assistant rate limit reached — try again shortly")
	}
	foodEventID, err := parseID(string(args.FoodEventID))
	if err != nil {
		return nil, err
	}
	limit := int(args.MaxSuggestions)
	if limit < 1 || limit > 10 {
		return nil, badInputf("maxSuggestions must be 1-10")
	}
	fixes, err := r.AIService.SuggestEventFixes(ctx, u.UserID, u.HouseholdID, foodEventID, limit)
	if err != nil {
		if errors.Is(err, ai.ErrUnavailable) {
			return nil, errUnavailablef("the AI assistant is not configured on this deployment")
		}
		return nil, err
	}
	out := make([]*eventFixResolver, len(fixes))
	for i, f := range fixes {
		out[i] = &eventFixResolver{fix: f}
	}
	return out, nil
}

// aiLimiter returns the per-user assistant rate limiter, lazily built so
// Resolver literals in tests still work. LLM calls are expensive, so the
// burst is tighter than the upload limiter.
func (r *Resolver) aiLimiter() *userRateLimiter {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.aiCalls == nil {
		r.aiCalls = newUserRateLimiter(10)
	}
	return r.aiCalls
}

type assistantAnswerResolver struct {
	answer    string
	toolCalls []*assistantToolCallResolver
}

// Answer is the assistant's final text.
func (a *assistantAnswerResolver) Answer() string { return a.answer }

// ToolCalls lists the household-data lookups the assistant made, in order.
func (a *assistantAnswerResolver) ToolCalls() []*assistantToolCallResolver { return a.toolCalls }

type assistantToolCallResolver struct {
	name string
}

// Name is the tool the assistant invoked (e.g. "get_pantry_inventory").
func (t *assistantToolCallResolver) Name() string { return t.name }

// mealSuggestionResolver resolves MealPlanSuggestion fields.
type mealSuggestionResolver struct {
	r    *Resolver
	user currentuser.User
	sugg ai.MealSuggestion
}

// Recipe is the suggested recipe — always a validated catalog recipe.
func (s *mealSuggestionResolver) Recipe(ctx context.Context) (*recipeResolver, error) {
	rec, err := s.r.RecipeService.GetRecipeByID(ctx, s.sugg.RecipeID)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: s.r.InventoryService, rec: s.r.RecipeService, up: s.r.UserPrefsService, user: s.user, recipe: rec}, nil
}

// DayOfWeek is the open cell's day (0=Sunday … 6=Saturday).
func (s *mealSuggestionResolver) DayOfWeek() int32 { return int32(s.sugg.DayOfWeek) }

// MealType is the open cell's meal type (e.g. "Dinner").
func (s *mealSuggestionResolver) MealType() string { return s.sugg.MealType }

// Reason is the model's short justification for the pick.
func (s *mealSuggestionResolver) Reason() string { return s.sugg.Reason }

// UsesExpiringItems lists expiring pantry items the recipe would consume.
func (s *mealSuggestionResolver) UsesExpiringItems() []string {
	if s.sugg.Expiring == nil {
		return []string{}
	}
	return s.sugg.Expiring
}

// eventFixResolver resolves EventFixSuggestion fields.
type eventFixResolver struct {
	fix ai.EventFix
}

func (f *eventFixResolver) EventRecipeID() graphql.ID {
	return graphql.ID(strconv.FormatInt(f.fix.EventRecipeID, 10))
}

func (f *eventFixResolver) RecipeName() string { return f.fix.RecipeName }

func (f *eventFixResolver) StepNumber() *int32 { return f.fix.StepNumber }

func (f *eventFixResolver) Action() string { return f.fix.Action }

func (f *eventFixResolver) Minutes() *int32 { return f.fix.Minutes }

func (f *eventFixResolver) Appliance() *string { return f.fix.Appliance }

func (f *eventFixResolver) DurationMinutes() *int32 { return f.fix.DurationMin }

func (f *eventFixResolver) DependsOnStepNumber() *int32 { return f.fix.DependsOn }

func (f *eventFixResolver) Reason() string { return f.fix.Reason }
