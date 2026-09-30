package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// MealPlanner is the mealplan surface the meal-plan tool needs.
type MealPlanner interface {
	ListMealPlans(ctx context.Context, householdID int64, limit, offset int32) ([]mealplan.MealPlan, error)
	GetMealPlanByID(ctx context.Context, mealPlanID, householdID int64) (mealplan.MealPlan, error)
	ListMealSlotsForPlan(ctx context.Context, mealPlanID, householdID int64) ([]mealplan.MealSlot, error)
}

// RecipeLookup resolves recipe IDs — *recipe.Service satisfies it, and the
// suggestion flow uses it to validate model-proposed recipeIds.
type RecipeLookup interface {
	GetRecipesByIDs(ctx context.Context, recipeIDs []int64) ([]recipe.Recipe, error)
}

// MealPlanSlotRow is one plan cell as the model sees it.
type MealPlanSlotRow struct {
	DayOfWeek int16  `json:"dayOfWeek"`
	MealType  string `json:"mealType"`
	RecipeID  *int64 `json:"recipeId,omitempty"`
	Recipe    string `json:"recipe,omitempty"`
}

// MealPlanOut is the get_meal_plan result — the plan plus its filled
// slots. Cells absent from Slots are open suggestion targets.
type MealPlanOut struct {
	MealPlanID    int64             `json:"mealPlanId"`
	Name          string            `json:"name"`
	WeekStartDate string            `json:"weekStartDate"`
	Slots         []MealPlanSlotRow `json:"slots"`
}

// RegisterMealPlanTools wires get_meal_plan. Without a mealPlanId argument
// it returns the household's most recent plan.
func RegisterMealPlanTools(reg *Registry, mp MealPlanner, recipes RecipeLookup) {
	reg.Register(llm.ToolSpec{
		Name:        "get_meal_plan",
		Description: "Get a household meal plan and its assigned slots. Empty dayOfWeek/mealType cells are open slots that suggestions may target.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mealPlanId": map[string]any{
					"type":        "integer",
					"description": "Plan to load. Omit for the most recent plan.",
				},
			},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		var a struct {
			MealPlanID int64 `json:"mealPlanId"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_meal_plan args: %w", err)
			}
		}
		if scope.HouseholdID == 0 {
			return MealPlanOut{}, nil
		}
		if a.MealPlanID <= 0 {
			plans, err := mp.ListMealPlans(ctx, scope.HouseholdID, 1, 0)
			if err != nil {
				return nil, fmt.Errorf("list meal plans: %w", err)
			}
			if len(plans) == 0 {
				return MealPlanOut{}, nil
			}
			a.MealPlanID = plans[0].MealPlanID
		}
		return mealPlanOut(ctx, mp, recipes, a.MealPlanID, scope.HouseholdID)
	})
}

// mealPlanOut loads a household-owned plan with its slots. The ownership
// check is inside GetMealPlanByID — a foreign or missing ID returns the
// domain not-found error.
func mealPlanOut(ctx context.Context, mp MealPlanner, recipes RecipeLookup, mealPlanID, householdID int64) (MealPlanOut, error) {
	plan, err := mp.GetMealPlanByID(ctx, mealPlanID, householdID)
	if err != nil {
		return MealPlanOut{}, fmt.Errorf("get meal plan: %w", err)
	}
	slots, err := mp.ListMealSlotsForPlan(ctx, mealPlanID, householdID)
	if err != nil {
		return MealPlanOut{}, fmt.Errorf("list meal slots: %w", err)
	}
	out := MealPlanOut{
		MealPlanID:    plan.MealPlanID,
		Name:          plan.Name,
		WeekStartDate: plan.WeekStartDate.Format("2006-01-02"),
		Slots:         make([]MealPlanSlotRow, 0, len(slots)),
	}
	ids := make([]int64, 0, len(slots))
	for _, s := range slots {
		if s.RecipeID != nil {
			ids = append(ids, *s.RecipeID)
		}
	}
	names := map[int64]string{}
	if len(ids) > 0 && recipes != nil {
		list, err := recipes.GetRecipesByIDs(ctx, ids)
		if err != nil {
			return MealPlanOut{}, fmt.Errorf("recipe names: %w", err)
		}
		for _, r := range list {
			names[r.RecipeID] = r.Name
		}
	}
	for _, s := range slots {
		row := MealPlanSlotRow{DayOfWeek: s.DayOfWeek, MealType: s.MealType, RecipeID: s.RecipeID}
		if s.RecipeID != nil {
			row.Recipe = names[*s.RecipeID]
		}
		out.Slots = append(out.Slots, row)
	}
	return out, nil
}
