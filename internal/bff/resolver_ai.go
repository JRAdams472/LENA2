package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

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

// requireDrinkingAge gates alcohol-related suggestions: the caller needs
// a stored birthdate showing 21+. Birthdate lives on the identity row,
// not the auth context, so this does a fresh read each call.
func (r *Resolver) requireDrinkingAge(ctx context.Context, userID int64) error {
	if r.IdentityService == nil {
		return errUnavailablef("profile lookup is unavailable")
	}
	u, err := r.IdentityService.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Birthdate == nil {
		return badInputf("set your birthdate in your profile to use pairing and cocktail suggestions")
	}
	now := time.Now()
	age := now.Year() - u.Birthdate.Year()
	if now.Month() < u.Birthdate.Month() ||
		(now.Month() == u.Birthdate.Month() && now.Day() < u.Birthdate.Day()) {
		age--
	}
	if age < 21 {
		return badInputf("pairing and cocktail suggestions require an account holder of legal drinking age")
	}
	return nil
}

// SuggestPairings returns AI wine pairings for a recipe — cellar bottles
// and style picks, gated to callers of legal drinking age.
func (r *Resolver) SuggestPairings(ctx context.Context, args struct {
	RecipeID       graphql.ID
	MaxSuggestions int32
}) ([]*pairingResolver, error) {
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
	if err := r.requireDrinkingAge(ctx, u.UserID); err != nil {
		return nil, err
	}
	recipeID, err := parseID(string(args.RecipeID))
	if err != nil {
		return nil, err
	}
	limit := int(args.MaxSuggestions)
	if limit < 1 || limit > 10 {
		return nil, badInputf("maxSuggestions must be 1-10")
	}
	pairings, err := r.AIService.SuggestPairings(ctx, u.UserID, u.HouseholdID, recipeID, limit)
	if err != nil {
		if errors.Is(err, ai.ErrUnavailable) {
			return nil, errUnavailablef("the AI assistant is not configured on this deployment")
		}
		return nil, err
	}
	out := make([]*pairingResolver, len(pairings))
	for i, p := range pairings {
		out[i] = &pairingResolver{sugg: p}
	}
	return out, nil
}

// SuggestCocktails returns AI cocktail picks from the catalog, optionally
// limited to recipes the pantry can make, gated to 21+.
func (r *Resolver) SuggestCocktails(ctx context.Context, args struct {
	MaxSuggestions int32
	InStockOnly    bool
}) ([]*cocktailResolver, error) {
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
	if err := r.requireDrinkingAge(ctx, u.UserID); err != nil {
		return nil, err
	}
	limit := int(args.MaxSuggestions)
	if limit < 1 || limit > 10 {
		return nil, badInputf("maxSuggestions must be 1-10")
	}
	suggs, err := r.AIService.SuggestCocktails(ctx, u.UserID, u.HouseholdID, limit, args.InStockOnly)
	if err != nil {
		if errors.Is(err, ai.ErrUnavailable) {
			return nil, errUnavailablef("the AI assistant is not configured on this deployment")
		}
		return nil, err
	}
	out := make([]*cocktailResolver, len(suggs))
	for i, c := range suggs {
		out[i] = &cocktailResolver{sugg: c, r: r, user: u}
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

// aiToolLimiter throttles the read-only tool surface used by client-side
// inference agents. One askAssistant turn can fan out to several tool
// calls, so it's looser than the inference limiter.
func (r *Resolver) aiToolLimiter() *userRateLimiter {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.aiToolCalls == nil {
		r.aiToolCalls = newUserRateLimiter(30)
	}
	return r.aiToolCalls
}

// AssistantTools exposes the assistant's read-only tool catalog to
// client-side agents — the same specs advertised to the server provider.
// Available even when no LLM provider is configured: the tools are just
// household-scoped reads.
func (r *Resolver) AssistantTools(ctx context.Context) ([]*assistantToolSpecResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	if r.AIService == nil {
		return []*assistantToolSpecResolver{}, nil
	}
	specs := r.AIService.ToolSpecs()
	out := make([]*assistantToolSpecResolver, len(specs))
	for i, s := range specs {
		params, err := json.Marshal(s.Parameters)
		if err != nil {
			return nil, fmt.Errorf("marshal tool schema %q: %w", s.Name, err)
		}
		out[i] = &assistantToolSpecResolver{name: s.Name, description: s.Description, parametersJSON: string(params)}
	}
	return out, nil
}

// CallAssistantTool executes one read-only tool under the caller's
// household scope for a client-side agent. The result is a JSON string;
// tool failures arrive as {"error":...} payloads — mirroring how the
// server loop reports them to the model.
func (r *Resolver) CallAssistantTool(ctx context.Context, args struct {
	Name      string
	Arguments string
}) (string, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return "", err
	}
	if r.AIService == nil {
		return "", errUnavailablef("assistant tools are not configured on this deployment")
	}
	if !r.aiToolLimiter().allow(u.UserID) {
		return "", errUnavailablef("assistant tool rate limit reached — try again shortly")
	}
	out, err := r.AIService.CallTool(ctx, u.UserID, u.HouseholdID, args.Name, json.RawMessage(args.Arguments))
	if err != nil {
		if errors.Is(err, ai.ErrUnavailable) {
			return "", errUnavailablef("assistant tools are not configured on this deployment")
		}
		return "", err
	}
	return out, nil
}

// AssistantPrompt serves the server-owned system prompt for a named
// assistant flow so local agents stay in sync with the server path.
func (r *Resolver) AssistantPrompt(ctx context.Context, args struct{ Name string }) (string, error) {
	if _, err := userFromContext(ctx); err != nil {
		return "", err
	}
	if r.AIService == nil {
		return "", errUnavailablef("assistant prompts are not configured on this deployment")
	}
	p, ok := r.AIService.Prompt(args.Name)
	if !ok {
		return "", badInputf("unknown assistant prompt %q", args.Name)
	}
	return p, nil
}

// PrepareAssistantRequest assembles a one-shot structured AI request
// server-side — context gathered through the same read-only tools — for
// client-side generation. Returns null when nothing needs generating.
func (r *Resolver) PrepareAssistantRequest(ctx context.Context, args struct {
	Name       string
	ParamsJSON string
}) (*preparedAIRequestResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if r.AIService == nil {
		return nil, errUnavailablef("assistant tools are not configured on this deployment")
	}
	// The alcohol-gated features keep their age gate on the server even
	// when inference moves to the client.
	if args.Name == "suggest-pairings" || args.Name == "suggest-cocktails" {
		if err := r.requireDrinkingAge(ctx, u.UserID); err != nil {
			return nil, err
		}
	}
	if !r.aiToolLimiter().allow(u.UserID) {
		return nil, errUnavailablef("assistant tool rate limit reached — try again shortly")
	}
	p, err := r.AIService.PrepareRequest(ctx, u.UserID, u.HouseholdID, args.Name, json.RawMessage(args.ParamsJSON))
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrUnavailable):
			return nil, errUnavailablef("assistant tools are not configured on this deployment")
		case errors.Is(err, ai.ErrUnknownRequest), errors.Is(err, ai.ErrBadParams):
			return nil, badInputf("%s", err)
		default:
			return nil, err
		}
	}
	if p == nil {
		return nil, nil
	}
	return &preparedAIRequestResolver{req: p}, nil
}

type assistantToolSpecResolver struct {
	name           string
	description    string
	parametersJSON string
}

// Name is the tool name the model invokes (e.g. "get_pantry_inventory").
func (t *assistantToolSpecResolver) Name() string { return t.name }

// Description is the human/model-facing purpose text.
func (t *assistantToolSpecResolver) Description() string { return t.description }

// ParametersJSON is the JSON Schema for the tool's arguments.
func (t *assistantToolSpecResolver) ParametersJSON() string { return t.parametersJSON }

type preparedAIRequestResolver struct {
	req *ai.PreparedRequest
}

// Prompt is the system prompt.
func (p *preparedAIRequestResolver) Prompt() string { return p.req.Prompt }

// ContextJSON is the assembled user message as a JSON string.
func (p *preparedAIRequestResolver) ContextJSON() string { return string(p.req.Context) }

// OutputSchemaJSON is the JSON Schema the model output must satisfy.
func (p *preparedAIRequestResolver) OutputSchemaJSON() string { return string(p.req.OutputSchema) }

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
	return &recipeResolver{inv: s.r.InventoryService, rec: s.r.RecipeService, up: s.r.UserPrefsService, user: s.user, recipe: rec, as: s.r.allergySrc(s.user)}, nil
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

// pairingResolver resolves PairingSuggestion fields.
type pairingResolver struct {
	sugg ai.PairingSuggestion
}

func (p *pairingResolver) BottleID() *graphql.ID {
	if p.sugg.BottleID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*p.sugg.BottleID, 10))
	return &id
}

func (p *pairingResolver) Name() string { return p.sugg.Name }

func (p *pairingResolver) Reason() string { return p.sugg.Reason }

func (p *pairingResolver) InCellar() bool { return p.sugg.InCellar }

// cocktailResolver resolves CocktailSuggestion fields.
type cocktailResolver struct {
	sugg ai.CocktailSuggestion
	r    *Resolver
	user currentuser.User
}

// Recipe loads the suggested cocktail recipe — always a validated catalog
// recipe tagged with the Cocktail dish type.
func (c *cocktailResolver) Recipe(ctx context.Context) (*recipeResolver, error) {
	rec, err := c.r.RecipeService.GetRecipeByID(ctx, c.sugg.RecipeID)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: c.r.InventoryService, rec: c.r.RecipeService, up: c.r.UserPrefsService, user: c.user, recipe: rec, as: c.r.allergySrc(c.user)}, nil
}

func (c *cocktailResolver) Reason() string { return c.sugg.Reason }

func (c *cocktailResolver) MissingIngredients() []string {
	if c.sugg.Missing == nil {
		return []string{}
	}
	return c.sugg.Missing
}
