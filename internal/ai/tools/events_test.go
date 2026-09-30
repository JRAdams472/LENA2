package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

type stubEvents struct {
	events []event.FoodEvent
	slots  []event.EventRecipe
	steps  []event.EventRecipeStep
	err    error
}

func (s *stubEvents) GetFoodEventByID(_ context.Context, id, hh int64) (event.FoodEvent, error) {
	if s.err != nil {
		return event.FoodEvent{}, s.err
	}
	for _, ev := range s.events {
		if ev.FoodEventID == id && ev.HouseholdID == hh {
			return ev, nil
		}
	}
	return event.FoodEvent{}, errors.New("not found")
}

func (s *stubEvents) ListFoodEvents(_ context.Context, hh int64, limit, _ int32) ([]event.FoodEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []event.FoodEvent
	for _, ev := range s.events {
		if ev.HouseholdID == hh {
			out = append(out, ev)
		}
	}
	if int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *stubEvents) ListEventRecipesForEvent(_ context.Context, id, hh int64) ([]event.EventRecipe, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []event.EventRecipe
	for _, er := range s.slots {
		if er.FoodEventID == id && hh == 9 {
			out = append(out, er)
		}
	}
	return out, nil
}

func (s *stubEvents) ListEventRecipeStepsForEvents(_ context.Context, ids []int64, hh int64) ([]event.EventRecipeStep, error) {
	if s.err != nil {
		return nil, s.err
	}
	if hh != 9 {
		return nil, errors.New("not found")
	}
	want := map[int64]bool{}
	for _, s := range s.slots {
		for _, id := range ids {
			if s.FoodEventID == id {
				want[s.EventRecipeID] = true
			}
		}
	}
	var out []event.EventRecipeStep
	for _, st := range s.steps {
		if want[st.EventRecipeID] {
			out = append(out, st)
		}
	}
	return out, nil
}

func eventFixture() *Registry {
	rid := int64(55)
	target := time.Date(2026, 3, 14, 18, 0, 0, 0, time.UTC)
	dur := int32(30)
	ev := &stubEvents{
		events: []event.FoodEvent{{
			FoodEventID:            20,
			HouseholdID:            9,
			Name:                   "Dinner",
			EventDate:              time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			SlotGranularityMinutes: 15,
		}},
		slots: []event.EventRecipe{
			{EventRecipeID: 100, FoodEventID: 20, RecipeID: &rid, MealType: "Dinner", TargetTime: target},
			{EventRecipeID: 101, FoodEventID: 20, RecipeID: &rid, MealType: "Dinner", TargetTime: target},
		},
		steps: []event.EventRecipeStep{
			{EventRecipeStepID: 1, EventRecipeID: 100, StepNumber: 1, Instruction: "Roast", DurationMinutes: &dur, Appliance: "Oven"},
			{EventRecipeStepID: 2, EventRecipeID: 101, StepNumber: 1, Instruction: "Bake", DurationMinutes: &dur, Appliance: "oven"},
		},
	}
	rc := &stubRecipes{recipes: map[int64]recipe.Recipe{55: {RecipeID: 55, Name: "Chicken"}}}
	reg := New()
	RegisterEventTools(reg, ev, rc)
	return reg
}

func TestGetEventTimeline_ExplicitID(t *testing.T) {
	reg := eventFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9},
		"get_event_timeline", json.RawMessage(`{"foodEventId":20}`))
	require.NoError(t, err)
	tl, ok := out.(EventTimelineOut)
	require.True(t, ok)
	assert.Equal(t, int64(20), tl.FoodEventID)
	assert.Equal(t, "Dinner", tl.Name)
	assert.Equal(t, "2026-03-14", tl.EventDate)
	assert.Equal(t, int16(15), tl.SlotGranularityMinutes)
	require.Len(t, tl.Recipes, 2)
	assert.Equal(t, "Chicken", tl.Recipes[0].Name)
	// Both dishes want "oven" 17:30–18:00 — a conflict is flagged on both
	// sides and in the event warnings.
	require.Len(t, tl.Recipes[0].Steps, 1)
	assert.NotEmpty(t, tl.Recipes[0].Steps[0].Conflicts)
	assert.NotEmpty(t, tl.Recipes[1].Steps[0].Conflicts)
	assert.NotEmpty(t, tl.Warnings)
}

func TestGetEventTimeline_DefaultsToNewest(t *testing.T) {
	reg := eventFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_event_timeline", nil)
	require.NoError(t, err)
	tl, ok := out.(EventTimelineOut)
	require.True(t, ok)
	assert.Equal(t, int64(20), tl.FoodEventID)
}

func TestGetEventTimeline_ForeignEventFails(t *testing.T) {
	reg := eventFixture()
	_, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 99},
		"get_event_timeline", json.RawMessage(`{"foodEventId":20}`))
	require.Error(t, err)
}

func TestGetEventTimeline_NoEventsEmpty(t *testing.T) {
	reg := New()
	RegisterEventTools(reg, &stubEvents{}, &stubRecipes{})
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_event_timeline", nil)
	require.NoError(t, err)
	tl, ok := out.(EventTimelineOut)
	require.True(t, ok)
	assert.Equal(t, int64(0), tl.FoodEventID)
}
