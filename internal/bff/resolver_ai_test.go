package bff

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/ai"
	"github.com/JRAdams472/LENA2/internal/bff/mock"
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
