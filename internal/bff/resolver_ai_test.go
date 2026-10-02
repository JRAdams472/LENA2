package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/ai"
	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func aiCtx() context.Context {
	return testutil.WithUser(context.Background(), 7, "ai@example.com")
}

func TestResolver_AIAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Available().Return(true)
	r := &Resolver{AIService: svc}
	ok, err := r.AIAvailable(aiCtx())
	require.NoError(t, err)
	assert.True(t, ok)

	r = &Resolver{}
	ok, err = r.AIAvailable(aiCtx())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestResolver_AIAvailable_Unauth(t *testing.T) {
	r := &Resolver{}
	_, err := r.AIAvailable(context.Background())
	require.Error(t, err)
}

func TestResolver_AskAssistant_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Available().Return(true)
	// userID=7; testutil.WithUser sets HouseholdID=UserID.
	svc.EXPECT().Ask(gomock.Any(), int64(7), int64(7), "what's in stock?").
		Return(ai.Answer{Text: "2 cups flour", Tools: []ai.ToolTrace{{Name: "get_pantry_inventory"}}}, nil)

	r := &Resolver{AIService: svc}
	res, err := r.AskAssistant(aiCtx(), struct{ Question string }{Question: "what's in stock?"})
	require.NoError(t, err)
	assert.Equal(t, "2 cups flour", res.Answer())
	require.Len(t, res.ToolCalls(), 1)
	assert.Equal(t, "get_pantry_inventory", res.ToolCalls()[0].Name())
}

func TestResolver_AskAssistant_Disabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Available().Return(false)
	r := &Resolver{AIService: svc}

	_, err := r.AskAssistant(aiCtx(), struct{ Question string }{Question: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")

	// Nil service behaves identically.
	r = &Resolver{}
	_, err = r.AskAssistant(aiCtx(), struct{ Question string }{Question: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestResolver_AskAssistant_InvalidQuestion(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Available().Return(true)
	svc.EXPECT().Ask(gomock.Any(), int64(7), int64(7), "  ").
		Return(ai.Answer{}, ai.ErrInvalidQuestion)
	r := &Resolver{AIService: svc}
	_, err := r.AskAssistant(aiCtx(), struct{ Question string }{Question: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1-2000")
}

func TestResolver_SuggestMeals_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	recipes := mock.NewMockRecipeService(ctrl)
	aiSvc.EXPECT().Available().Return(true)
	aiSvc.EXPECT().SuggestMeals(gomock.Any(), int64(7), int64(7), int64(10), 4).
		Return([]ai.MealSuggestion{{RecipeID: 20, DayOfWeek: 3, MealType: "Dinner", Reason: "uses milk", Expiring: []string{"milk"}}}, nil)
	recipes.EXPECT().GetRecipeByID(gomock.Any(), int64(20)).
		Return(recipe.Recipe{RecipeID: 20, Name: "Pasta"}, nil)

	r := &Resolver{AIService: aiSvc, RecipeService: recipes}
	res, err := r.SuggestMeals(aiCtx(), struct {
		MealPlanID     graphql.ID
		MaxSuggestions int32
	}{MealPlanID: "10", MaxSuggestions: 4})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, int32(3), res[0].DayOfWeek())
	assert.Equal(t, "Dinner", res[0].MealType())
	assert.Equal(t, "uses milk", res[0].Reason())
	assert.Equal(t, []string{"milk"}, res[0].UsesExpiringItems())
	rec, err := res[0].Recipe(aiCtx())
	require.NoError(t, err)
	assert.NotNil(t, rec)
}

func TestResolver_SuggestMeals_Disabled(t *testing.T) {
	r := &Resolver{}
	_, err := r.SuggestMeals(aiCtx(), struct {
		MealPlanID     graphql.ID
		MaxSuggestions int32
	}{MealPlanID: "10"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestResolver_SuggestMeals_BadMax(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	aiSvc.EXPECT().Available().Return(true)
	r := &Resolver{AIService: aiSvc}
	_, err := r.SuggestMeals(aiCtx(), struct {
		MealPlanID     graphql.ID
		MaxSuggestions int32
	}{MealPlanID: "10", MaxSuggestions: 99})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1-10")
}

func TestResolver_SuggestEventFixes_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	mins := int32(30)
	aiSvc.EXPECT().Available().Return(true)
	aiSvc.EXPECT().SuggestEventFixes(gomock.Any(), int64(7), int64(7), int64(20), 6).
		Return([]ai.EventFix{{EventRecipeID: 101, RecipeName: "Sides", Action: ai.EventFixShiftServe, Minutes: &mins, Reason: "frees the oven"}}, nil)

	r := &Resolver{AIService: aiSvc}
	res, err := r.SuggestEventFixes(aiCtx(), struct {
		FoodEventID    graphql.ID
		MaxSuggestions int32
	}{FoodEventID: "20", MaxSuggestions: 6})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "101", string(res[0].EventRecipeID()))
	assert.Equal(t, "Sides", res[0].RecipeName())
	assert.Equal(t, "shift_serve", res[0].Action())
	assert.Equal(t, int32(30), *res[0].Minutes())
	assert.Nil(t, res[0].Appliance())
	assert.Equal(t, "frees the oven", res[0].Reason())
}

func TestResolver_SuggestEventFixes_Disabled(t *testing.T) {
	r := &Resolver{}
	_, err := r.SuggestEventFixes(aiCtx(), struct {
		FoodEventID    graphql.ID
		MaxSuggestions int32
	}{FoodEventID: "20"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestResolver_SuggestEventFixes_BadMax(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	aiSvc.EXPECT().Available().Return(true)
	r := &Resolver{AIService: aiSvc}
	_, err := r.SuggestEventFixes(aiCtx(), struct {
		FoodEventID    graphql.ID
		MaxSuggestions int32
	}{FoodEventID: "20", MaxSuggestions: 99})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1-10")
}

func TestResolver_SuggestEventFixes_BadID(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	aiSvc.EXPECT().Available().Return(true)
	r := &Resolver{AIService: aiSvc}
	_, err := r.SuggestEventFixes(aiCtx(), struct {
		FoodEventID    graphql.ID
		MaxSuggestions int32
	}{FoodEventID: "abc", MaxSuggestions: 6})
	require.Error(t, err)
}

func TestResolver_SuggestPairings_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	ofAge := time.Now().AddDate(-30, 0, 0)
	bottle := int64(501)
	aiSvc.EXPECT().Available().Return(true)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(7)).
		Return(identity.User{UserID: 7, Birthdate: &ofAge}, nil)
	aiSvc.EXPECT().SuggestPairings(gomock.Any(), int64(7), int64(7), int64(1), 4).
		Return([]ai.PairingSuggestion{
			{BottleID: &bottle, Name: "Estate 501 2021", Reason: "tannins", InCellar: true},
			{Name: "off-dry Riesling", Reason: "acidity"},
		}, nil)

	r := &Resolver{AIService: aiSvc, IdentityService: idSvc}
	res, err := r.SuggestPairings(aiCtx(), struct {
		RecipeID       graphql.ID
		MaxSuggestions int32
	}{RecipeID: "1", MaxSuggestions: 4})
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, "501", string(*res[0].BottleID()))
	assert.True(t, res[0].InCellar())
	assert.Nil(t, res[1].BottleID())
	assert.False(t, res[1].InCellar())
	assert.Equal(t, "off-dry Riesling", res[1].Name())
}

func TestResolver_SuggestPairings_UnderageOrMissingBirthdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	aiSvc.EXPECT().Available().Return(true).Times(2)
	minor := time.Now().AddDate(-17, 0, 0)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(7)).Return(identity.User{UserID: 7, Birthdate: &minor}, nil)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(7)).Return(identity.User{UserID: 7}, nil)

	r := &Resolver{AIService: aiSvc, IdentityService: idSvc}
	args := struct {
		RecipeID       graphql.ID
		MaxSuggestions int32
	}{RecipeID: "1", MaxSuggestions: 4}
	_, err := r.SuggestPairings(aiCtx(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drinking age")
	_, err = r.SuggestPairings(aiCtx(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "birthdate")
}

func TestResolver_SuggestCocktails_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	recipes := mock.NewMockRecipeService(ctrl)
	ofAge := time.Now().AddDate(-40, 0, 0)
	aiSvc.EXPECT().Available().Return(true)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(7)).
		Return(identity.User{UserID: 7, Birthdate: &ofAge}, nil)
	aiSvc.EXPECT().SuggestCocktails(gomock.Any(), int64(7), int64(7), 6, true).
		Return([]ai.CocktailSuggestion{{RecipeID: 1, Reason: "citrus on hand"}}, nil)
	recipes.EXPECT().GetRecipeByID(gomock.Any(), int64(1)).
		Return(recipe.Recipe{RecipeID: 1, Name: "Margarita"}, nil)

	r := &Resolver{AIService: aiSvc, IdentityService: idSvc, RecipeService: recipes}
	res, err := r.SuggestCocktails(aiCtx(), struct {
		MaxSuggestions int32
		InStockOnly    bool
	}{MaxSuggestions: 6, InStockOnly: true})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "citrus on hand", res[0].Reason())
	assert.Empty(t, res[0].MissingIngredients())
	rec, err := res[0].Recipe(aiCtx())
	require.NoError(t, err)
	assert.NotNil(t, rec)
}

func TestResolver_SuggestCocktails_Underage(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	minor := time.Now().AddDate(-20, 0, 0)
	aiSvc.EXPECT().Available().Return(true)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(7)).Return(identity.User{UserID: 7, Birthdate: &minor}, nil)

	r := &Resolver{AIService: aiSvc, IdentityService: idSvc}
	_, err := r.SuggestCocktails(aiCtx(), struct {
		MaxSuggestions int32
		InStockOnly    bool
	}{MaxSuggestions: 6})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drinking age")
}

func TestResolver_AskAssistant_RateLimited(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Available().Return(true).AnyTimes()
	svc.EXPECT().Ask(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(ai.Answer{Text: "ok"}, nil).AnyTimes()

	r := &Resolver{AIService: svc, aiCalls: newUserRateLimiter(1)}
	_, err := r.AskAssistant(aiCtx(), struct{ Question string }{Question: "one"})
	require.NoError(t, err)
	_, err = r.AskAssistant(aiCtx(), struct{ Question string }{Question: "two"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestResolver_AssistantTools(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().ToolSpecs().Return([]llm.ToolSpec{{
		Name:        "get_pantry_inventory",
		Description: "List pantry items",
		Parameters:  map[string]any{"type": "object"},
	}})

	r := &Resolver{AIService: svc}
	res, err := r.AssistantTools(aiCtx())
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "get_pantry_inventory", res[0].Name())
	assert.Equal(t, "List pantry items", res[0].Description())
	assert.JSONEq(t, `{"type":"object"}`, res[0].ParametersJSON())

	// Nil service → empty catalog, not an error: tools are just reads and
	// may exist without a configured provider.
	r = &Resolver{}
	res, err = r.AssistantTools(aiCtx())
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestResolver_CallAssistantTool(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	// Scope flows from the authenticated user — never from arguments.
	svc.EXPECT().CallTool(gomock.Any(), int64(7), int64(7), "get_expiring_items", json.RawMessage(`{"days":7}`)).
		Return(`[{"name":"milk"}]`, nil)

	r := &Resolver{AIService: svc}
	out, err := r.CallAssistantTool(aiCtx(), struct {
		Name      string
		Arguments string
	}{Name: "get_expiring_items", Arguments: `{"days":7}`})
	require.NoError(t, err)
	assert.JSONEq(t, `[{"name":"milk"}]`, out)

	// Tool failures are model fodder — error payloads pass through.
	svc.EXPECT().CallTool(gomock.Any(), int64(7), int64(7), "bogus", json.RawMessage(`{}`)).
		Return(`{"error":"unknown tool: bogus"}`, nil)
	out, err = r.CallAssistantTool(aiCtx(), struct {
		Name      string
		Arguments string
	}{Name: "bogus", Arguments: `{}`})
	require.NoError(t, err)
	assert.Contains(t, out, "unknown tool")
}

func TestResolver_CallAssistantTool_DisabledAndRateLimited(t *testing.T) {
	r := &Resolver{}
	_, err := r.CallAssistantTool(aiCtx(), struct {
		Name      string
		Arguments string
	}{Name: "x", Arguments: `{}`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")

	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().CallTool(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(`{}`, nil).AnyTimes()
	r = &Resolver{AIService: svc, aiToolCalls: newUserRateLimiter(1)}
	_, err = r.CallAssistantTool(aiCtx(), struct {
		Name      string
		Arguments string
	}{Name: "x", Arguments: `{}`})
	require.NoError(t, err)
	_, err = r.CallAssistantTool(aiCtx(), struct {
		Name      string
		Arguments string
	}{Name: "x", Arguments: `{}`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestResolver_AssistantPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().Prompt("ask").Return("You are Dot...", true)
	svc.EXPECT().Prompt("nope").Return("", false)

	r := &Resolver{AIService: svc}
	p, err := r.AssistantPrompt(aiCtx(), struct{ Name string }{Name: "ask"})
	require.NoError(t, err)
	assert.Equal(t, "You are Dot...", p)

	_, err = r.AssistantPrompt(aiCtx(), struct{ Name string }{Name: "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown assistant prompt")
}

func TestResolver_PrepareAssistantRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockAIService(ctrl)
	svc.EXPECT().PrepareRequest(gomock.Any(), int64(7), int64(7), "suggest-meals",
		json.RawMessage(`{"mealPlanId":10}`)).
		Return(&ai.PreparedRequest{
			Prompt:       "You are LENA's meal-planning assistant.",
			Context:      json.RawMessage(`{"plan":{}}`),
			OutputSchema: json.RawMessage(`{"type":"object"}`),
		}, nil)
	svc.EXPECT().PrepareRequest(gomock.Any(), int64(7), int64(7), "suggest-meals",
		json.RawMessage(`{"mealPlanId":99}`)).
		Return(nil, nil)
	svc.EXPECT().PrepareRequest(gomock.Any(), int64(7), int64(7), "bogus", gomock.Any()).
		Return(nil, fmt.Errorf("%w: bogus", ai.ErrUnknownRequest))

	r := &Resolver{AIService: svc}
	res, err := r.PrepareAssistantRequest(aiCtx(), struct {
		Name       string
		ParamsJSON string
	}{Name: "suggest-meals", ParamsJSON: `{"mealPlanId":10}`})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.Prompt(), "meal-planning")
	assert.JSONEq(t, `{"plan":{}}`, res.ContextJSON())
	assert.JSONEq(t, `{"type":"object"}`, res.OutputSchemaJSON())

	// Nothing to generate → null, not an error.
	res, err = r.PrepareAssistantRequest(aiCtx(), struct {
		Name       string
		ParamsJSON string
	}{Name: "suggest-meals", ParamsJSON: `{"mealPlanId":99}`})
	require.NoError(t, err)
	assert.Nil(t, res)

	_, err = r.PrepareAssistantRequest(aiCtx(), struct {
		Name       string
		ParamsJSON string
	}{Name: "bogus", ParamsJSON: `{}`})
	require.Error(t, err)
}

func TestResolver_PrepareAssistantRequest_AgeGate(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	minor := time.Now().AddDate(-19, 0, 0)
	idSvc.EXPECT().GetByID(gomock.Any(), int64(7)).Return(identity.User{UserID: 7, Birthdate: &minor}, nil)
	// PrepareRequest must never be reached for an underage caller.
	r := &Resolver{AIService: aiSvc, IdentityService: idSvc}
	_, err := r.PrepareAssistantRequest(aiCtx(), struct {
		Name       string
		ParamsJSON string
	}{Name: "suggest-pairings", ParamsJSON: `{"recipeId":1}`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drinking age")
}
