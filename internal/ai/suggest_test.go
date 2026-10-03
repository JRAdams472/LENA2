package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

type sugPlanner struct{ foreignErr error }

func (s *sugPlanner) ListMealPlans(context.Context, int64, int32, int32) ([]mealplan.MealPlan, error) {
	return []mealplan.MealPlan{{MealPlanID: 10, HouseholdID: 9, Name: "W"}}, nil
}

func (s *sugPlanner) GetMealPlanByID(_ context.Context, id, hh int64) (mealplan.MealPlan, error) {
	if s.foreignErr != nil {
		return mealplan.MealPlan{}, s.foreignErr
	}
	if id != 10 || hh != 9 {
		return mealplan.MealPlan{}, errors.New("not found")
	}
	return mealplan.MealPlan{MealPlanID: 10, HouseholdID: 9, Name: "W"}, nil
}

func (s *sugPlanner) ListMealSlotsForPlan(_ context.Context, _, hh int64) ([]mealplan.MealSlot, error) {
	if hh != 9 {
		return nil, errors.New("not found")
	}
	rid := int64(1)
	return []mealplan.MealSlot{
		{SlotID: 1, MealPlanID: 10, DayOfWeek: 0, MealType: "Dinner", RecipeID: &rid},
		{SlotID: 2, MealPlanID: 10, DayOfWeek: 1, MealType: "Lunch"},
	}, nil
}

type sugRecipes struct{}

func (sugRecipes) GetRecipesByIDs(_ context.Context, ids []int64) ([]recipe.Recipe, error) {
	var out []recipe.Recipe
	for _, id := range ids {
		out = append(out, recipe.Recipe{RecipeID: id, Name: fmt.Sprintf("R%d", id)})
	}
	return out, nil
}

func (sugRecipes) ListRecipes(context.Context, bool, int32, int32) ([]recipe.Recipe, error) {
	return []recipe.Recipe{{RecipeID: 1, Name: "R1"}, {RecipeID: 2, Name: "R2"}, {RecipeID: 3, Name: "R3"}}, nil
}

func (sugRecipes) ListCategoriesForRecipes(context.Context, []int64) (map[int64][]recipe.Category, error) {
	return map[int64][]recipe.Category{}, nil
}

func (sugRecipes) ListRecipeItemsByRecipes(context.Context, []int64) ([]recipe.RecipeItem, error) {
	return nil, nil
}

type sugPantry struct{}

func (sugPantry) ListHouseholdItems(_ context.Context, hh int64, _, _ int32) ([]userprefs.HouseholdItem, error) {
	if hh != 9 {
		return nil, nil
	}
	exp := time.Now().Add(24 * time.Hour)
	return []userprefs.HouseholdItem{
		{HouseholdItemID: 1, ItemID: 100, CurrentQty: 2, ExpiresAt: &exp},
	}, nil
}

type sugNamer struct{}

func (sugNamer) GetItemsByIDs(_ context.Context, ids []int64) ([]inventory.Item, error) {
	var out []inventory.Item
	for _, id := range ids {
		out = append(out, inventory.Item{ItemID: id, Name: fmt.Sprintf("item%d", id)})
	}
	return out, nil
}

func (sugNamer) GetIngredientsByIDs(_ context.Context, ids []int64) ([]inventory.Ingredient, error) {
	var out []inventory.Ingredient
	for _, id := range ids {
		out = append(out, inventory.Ingredient{IngredientID: id, Name: fmt.Sprintf("ingredient%d", id)})
	}
	return out, nil
}

func (sugNamer) GetUnitsByIDs(context.Context, []int64) ([]inventory.Unit, error) {
	return nil, nil
}

func (sugNamer) ResolveItemIngredients(_ context.Context, _ int64, ids []int64) (map[int64]*int64, error) {
	out := make(map[int64]*int64, len(ids))
	for _, id := range ids {
		out[id] = nil
	}
	return out, nil
}

type sugTastes struct{}

func (sugTastes) TopUserSelections(context.Context, int64, string, int32) ([]analytics.SelectionCount, error) {
	return nil, nil
}

func (sugTastes) HouseholdRecipeUsage(context.Context, int64) (map[int64]int64, error) {
	return map[int64]int64{2: 4}, nil
}

func (sugTastes) HouseholdRecipeVelocities(context.Context, int64, int32) ([]analytics.RecipeVelocity, error) {
	return nil, nil
}

func suggestService(t *testing.T, p llm.Provider) *Service {
	t.Helper()
	reg := tools.New()
	tools.RegisterPantryTools(reg, sugPantry{}, sugNamer{})
	tools.RegisterMealPlanTools(reg, &sugPlanner{}, sugRecipes{})
	tools.RegisterRecipeTools(reg, sugRecipes{}, sugNamer{})
	tools.RegisterTasteTools(reg, sugTastes{}, sugRecipes{})
	return NewService(p, reg, Config{})
}

func enqueueSuggestions(p *llm.MockProvider, suggs ...MealSuggestion) {
	b, _ := json.Marshal(suggestResponse{Suggestions: suggs})
	p.EnqueueText(string(b))
}

func TestSuggestMeals_Happy(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueSuggestions(p,
		MealSuggestion{RecipeID: 2, DayOfWeek: 2, MealType: "Dinner", Reason: "household favorite"},
		MealSuggestion{RecipeID: 3, DayOfWeek: 1, MealType: "Dinner", Reason: "uses expiring item100", Expiring: []string{"item100"}},
	)
	svc := suggestService(t, p)
	got, err := svc.SuggestMeals(context.Background(), 7, 9, 10, 6)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Sorted by day.
	assert.Equal(t, int16(1), got[0].DayOfWeek)
	assert.Equal(t, int64(3), got[0].RecipeID)
	assert.Equal(t, []string{"item100"}, got[0].Expiring)
	// Request ran in JSON mode with the context embedded.
	assert.True(t, p.Requests[0].JSONMode)
	assert.Contains(t, p.Requests[0].Messages[1].Content, `"mealPlanId":10`)
	assert.Contains(t, p.Requests[0].Messages[1].Content, "item100")
}

func TestSuggestMeals_FiltersInvalid(t *testing.T) {
	p := llm.NewMockProvider()
	enqueueSuggestions(p,
		MealSuggestion{RecipeID: 99, DayOfWeek: 1, MealType: "Dinner", Reason: "unknown recipe"},
		MealSuggestion{RecipeID: 2, DayOfWeek: 0, MealType: "Dinner", Reason: "occupied cell"},
		MealSuggestion{RecipeID: 2, DayOfWeek: 9, MealType: "Dinner", Reason: "bad day"},
		MealSuggestion{RecipeID: 2, DayOfWeek: 1, MealType: "", Reason: "empty type"},
		MealSuggestion{RecipeID: 2, DayOfWeek: 3, MealType: "Dinner", Reason: "good"},
		MealSuggestion{RecipeID: 2, DayOfWeek: 4, MealType: "Dinner", Reason: "same recipe twice"},
		MealSuggestion{RecipeID: 3, DayOfWeek: 3, MealType: "dinner", Reason: "dup cell different case"},
	)
	svc := suggestService(t, p)
	got, err := svc.SuggestMeals(context.Background(), 7, 9, 10, 6)
	require.NoError(t, err)
	// Only the "good" one survives: occupied day-0 Dinner is on the plan,
	// the case-variant dup at day-3 is seen by cellKey, recipe repeat
	// dropped, and all schema violations filtered.
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].RecipeID)
	assert.Equal(t, int16(3), got[0].DayOfWeek)
	assert.Equal(t, "good", got[0].Reason)
}

func TestSuggestMeals_RetriesMalformedJSON(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("sure! here's what I think:")
	enqueueSuggestions(p, MealSuggestion{RecipeID: 2, DayOfWeek: 3, MealType: "Dinner", Reason: "ok"})
	svc := suggestService(t, p)
	got, err := svc.SuggestMeals(context.Background(), 7, 9, 10, 6)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, p.CallCount())
	assert.Contains(t, p.Requests[1].Messages[len(p.Requests[1].Messages)-1].Content, "valid JSON")
}

func TestSuggestMeals_MalformedTwice(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("oops")
	p.EnqueueText("still not json")
	svc := suggestService(t, p)
	_, err := svc.SuggestMeals(context.Background(), 7, 9, 10, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed")
	assert.Equal(t, 2, p.CallCount())
}

func TestSuggestMeals_PlanNotFound(t *testing.T) {
	p := llm.NewMockProvider()
	svc := suggestService(t, p)
	_, err := svc.SuggestMeals(context.Background(), 7, 99, 10, 6) // wrong household
	require.Error(t, err)
	assert.Equal(t, 0, p.CallCount())
}

func TestSuggestMeals_Unavailable(t *testing.T) {
	svc := suggestService(t, nil)
	_, err := svc.SuggestMeals(context.Background(), 7, 9, 10, 6)
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestSuggestMeals_ProviderError(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueError(errors.New("provider down"))
	svc := suggestService(t, p)
	_, err := svc.SuggestMeals(context.Background(), 7, 9, 10, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider down")
}
