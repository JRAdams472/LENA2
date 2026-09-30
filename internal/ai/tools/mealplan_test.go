package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

type stubPlanner struct {
	plans []mealplan.MealPlan
	slots []mealplan.MealSlot
	err   error
}

func (s *stubPlanner) ListMealPlans(_ context.Context, _ int64, limit, _ int32) ([]mealplan.MealPlan, error) {
	if s.err != nil {
		return nil, s.err
	}
	if int(limit) < len(s.plans) {
		return s.plans[:limit], nil
	}
	return s.plans, nil
}

func (s *stubPlanner) GetMealPlanByID(_ context.Context, id, hh int64) (mealplan.MealPlan, error) {
	if s.err != nil {
		return mealplan.MealPlan{}, s.err
	}
	for _, p := range s.plans {
		if p.MealPlanID == id && p.HouseholdID == hh {
			return p, nil
		}
	}
	return mealplan.MealPlan{}, errors.New("not found")
}

func (s *stubPlanner) ListMealSlotsForPlan(_ context.Context, mealPlanID, _ int64) ([]mealplan.MealSlot, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []mealplan.MealSlot
	for _, sl := range s.slots {
		if sl.MealPlanID == mealPlanID {
			out = append(out, sl)
		}
	}
	return out, nil
}

type stubRecipes struct {
	recipes map[int64]recipe.Recipe
	cats    map[int64][]recipe.Category
	items   []recipe.RecipeItem
}

func (s *stubRecipes) GetRecipesByIDs(_ context.Context, ids []int64) ([]recipe.Recipe, error) {
	out := make([]recipe.Recipe, 0, len(ids))
	for _, id := range ids {
		if r, ok := s.recipes[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *stubRecipes) ListRecipes(_ context.Context, _ bool, limit, _ int32) ([]recipe.Recipe, error) {
	out := make([]recipe.Recipe, 0, len(s.recipes))
	for _, r := range s.recipes {
		out = append(out, r)
	}
	if int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *stubRecipes) ListCategoriesForRecipes(_ context.Context, _ []int64) (map[int64][]recipe.Category, error) {
	return s.cats, nil
}

func (s *stubRecipes) ListRecipeItemsByRecipes(_ context.Context, _ []int64) ([]recipe.RecipeItem, error) {
	return s.items, nil
}

func mealPlanFixture() *Registry {
	rid := int64(20)
	mp := &stubPlanner{
		plans: []mealplan.MealPlan{
			{MealPlanID: 10, HouseholdID: 9, Name: "Week", WeekStartDate: time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)},
		},
		slots: []mealplan.MealSlot{
			{SlotID: 100, MealPlanID: 10, DayOfWeek: 1, MealType: "Dinner", RecipeID: &rid},
			{SlotID: 101, MealPlanID: 10, DayOfWeek: 2, MealType: "Lunch"},
		},
	}
	rc := &stubRecipes{recipes: map[int64]recipe.Recipe{20: {RecipeID: 20, Name: "Pasta"}}}
	reg := New()
	RegisterMealPlanTools(reg, mp, rc)
	return reg
}

func TestGetMealPlan_ExplicitID(t *testing.T) {
	reg := mealPlanFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9},
		"get_meal_plan", json.RawMessage(`{"mealPlanId":10}`))
	require.NoError(t, err)
	plan, ok := out.(MealPlanOut)
	require.True(t, ok)
	assert.Equal(t, int64(10), plan.MealPlanID)
	assert.Equal(t, "2025-06-02", plan.WeekStartDate)
	require.Len(t, plan.Slots, 2)
	assert.Equal(t, "Pasta", plan.Slots[0].Recipe)
	assert.Equal(t, int64(20), *plan.Slots[0].RecipeID)
	assert.Nil(t, plan.Slots[1].RecipeID)
}

func TestGetMealPlan_DefaultsToLatest(t *testing.T) {
	reg := mealPlanFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_meal_plan", nil)
	require.NoError(t, err)
	plan, ok := out.(MealPlanOut)
	require.True(t, ok)
	assert.Equal(t, int64(10), plan.MealPlanID)
}

func TestGetMealPlan_ForeignPlanFails(t *testing.T) {
	reg := mealPlanFixture()
	// HouseholdID 99 doesn't own plan 10.
	_, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 99},
		"get_meal_plan", json.RawMessage(`{"mealPlanId":10}`))
	require.Error(t, err)
}
