package ai

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

type sugEvents struct{ foreignErr error }

func (s *sugEvents) GetFoodEventByID(_ context.Context, id, hh int64) (event.FoodEvent, error) {
	if s.foreignErr != nil {
		return event.FoodEvent{}, s.foreignErr
	}
	if id != 20 || hh != 9 {
		return event.FoodEvent{}, errors.New("not found")
	}
	return event.FoodEvent{
		FoodEventID:            20,
		HouseholdID:            9,
		Name:                   "Dinner Party",
		EventDate:              time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
		SlotGranularityMinutes: 15,
	}, nil
}

func (s *sugEvents) ListFoodEvents(_ context.Context, hh int64, _, _ int32) ([]event.FoodEvent, error) {
	if hh != 9 {
		return nil, nil
	}
	return []event.FoodEvent{{
		FoodEventID:            20,
		HouseholdID:            9,
		Name:                   "Dinner Party",
		EventDate:              time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
		SlotGranularityMinutes: 15,
	}}, nil
}

func (s *sugEvents) ListEventRecipesForEvent(_ context.Context, id, hh int64) ([]event.EventRecipe, error) {
	if id != 20 || hh != 9 {
		return nil, errors.New("not found")
	}
	target := time.Date(2026, 3, 14, 18, 0, 0, 0, time.UTC)
	return []event.EventRecipe{
		{EventRecipeID: 100, FoodEventID: 20, MealType: "Dinner", TargetTime: target, Notes: "Roast"},
		{EventRecipeID: 101, FoodEventID: 20, MealType: "Dinner", TargetTime: target, Notes: "Sides"},
	}, nil
}

func (s *sugEvents) ListEventRecipeStepsForEvents(_ context.Context, _ []int64, hh int64) ([]event.EventRecipeStep, error) {
	if hh != 9 {
		return nil, errors.New("not found")
	}
	dur := int32(30)
	return []event.EventRecipeStep{
		{EventRecipeStepID: 1, EventRecipeID: 100, StepNumber: 1, Instruction: "Roast in oven", DurationMinutes: &dur, Appliance: "oven"},
		{EventRecipeStepID: 2, EventRecipeID: 101, StepNumber: 1, Instruction: "Bake potatoes", DurationMinutes: &dur, Appliance: "oven"},
	}, nil
}

type sugEventsClean struct{ sugEvents }

func (s *sugEventsClean) ListEventRecipeStepsForEvents(_ context.Context, _ []int64, hh int64) ([]event.EventRecipeStep, error) {
	if hh != 9 {
		return nil, errors.New("not found")
	}
	dur := int32(30)
	return []event.EventRecipeStep{
		{EventRecipeStepID: 1, EventRecipeID: 100, StepNumber: 1, Instruction: "Roast in oven", DurationMinutes: &dur, Appliance: "oven"},
		{EventRecipeStepID: 2, EventRecipeID: 101, StepNumber: 1, Instruction: "Sauté on stove", DurationMinutes: &dur, Appliance: "stove"},
	}, nil
}

func eventFixService(t *testing.T, p llm.Provider, ev tools.EventTimelineReader) *Service {
	t.Helper()
	reg := tools.New()
	tools.RegisterEventTools(reg, ev, sugRecipes{})
	return NewService(p, reg, Config{})
}

func enqueueFixes(p *llm.MockProvider, fixes ...EventFix) {
	b, _ := json.Marshal(eventFixResponse{Fixes: fixes})
	p.EnqueueText(string(b))
}

func TestSuggestEventFixes_Happy(t *testing.T) {
	p := llm.NewMockProvider()
	mins := int32(30)
	enqueueFixes(p, EventFix{EventRecipeID: 101, Action: EventFixShiftServe, Minutes: &mins, Reason: "moves potatoes later"})
	svc := eventFixService(t, p, &sugEvents{})
	got, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(101), got[0].EventRecipeID)
	assert.Equal(t, "Sides", got[0].RecipeName)
	assert.True(t, p.Requests[0].JSONMode)
	assert.Contains(t, p.Requests[0].Messages[1].Content, `"foodEventId":20`)
}

func TestSuggestEventFixes_CleanTimelineSkipsModel(t *testing.T) {
	p := llm.NewMockProvider()
	svc := eventFixService(t, p, &sugEventsClean{})
	got, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, 0, p.CallCount()) // no model call when nothing conflicts
}

func TestSuggestEventFixes_FiltersInvalid(t *testing.T) {
	p := llm.NewMockProvider()
	m := func(v int32) *int32 { return &v }
	step := func(v int32) *int32 { return &v }
	app := func(v string) *string { return &v }
	enqueueFixes(p,
		EventFix{EventRecipeID: 999, Action: EventFixShiftServe, Minutes: m(15), Reason: "unknown recipe"},
		EventFix{EventRecipeID: 100, Action: EventFixShiftServe, Minutes: m(0), Reason: "zero shift"},
		EventFix{EventRecipeID: 100, Action: EventFixShiftServe, Minutes: m(20), Reason: "off-granularity"},
		EventFix{EventRecipeID: 100, Action: EventFixShiftServe, Minutes: m(600), Reason: "too far"},
		EventFix{EventRecipeID: 100, Action: EventFixSetAppliance, StepNumber: step(1), Appliance: app(""), Reason: "empty appliance"},
		EventFix{EventRecipeID: 100, Action: EventFixSetAppliance, StepNumber: step(9), Appliance: app("stove"), Reason: "unknown step"},
		EventFix{EventRecipeID: 100, Action: EventFixSetDuration, StepNumber: step(1), DurationMin: m(0), Reason: "zero duration"},
		EventFix{EventRecipeID: 100, Action: EventFixSetDependsOn, StepNumber: step(1), DependsOn: step(1), Reason: "self dep"},
		EventFix{EventRecipeID: 100, Action: "delete_event", Reason: "bogus action"},
		EventFix{EventRecipeID: 101, Action: EventFixSetAppliance, StepNumber: step(1), Appliance: app("grill"), Reason: "free appliance"},
	)
	svc := eventFixService(t, p, &sugEvents{})
	got, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, EventFixSetAppliance, got[0].Action)
	assert.Equal(t, "grill", *got[0].Appliance)
	assert.Equal(t, "Sides", got[0].RecipeName)
}

func TestSuggestEventFixes_DedupesSameAction(t *testing.T) {
	p := llm.NewMockProvider()
	m, step := int32(30), int32(1)
	app := func(v string) *string { return &v }
	enqueueFixes(p,
		EventFix{EventRecipeID: 101, Action: EventFixShiftServe, Minutes: &m, Reason: "a"},
		EventFix{EventRecipeID: 101, Action: EventFixShiftServe, Minutes: &m, Reason: "dup"},
		EventFix{EventRecipeID: 101, Action: EventFixSetAppliance, StepNumber: &step, Appliance: app("grill"), Reason: "b"},
		EventFix{EventRecipeID: 101, Action: EventFixSetAppliance, StepNumber: &step, Appliance: app("stove"), Reason: "dup"},
	)
	svc := eventFixService(t, p, &sugEvents{})
	got, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestSuggestEventFixes_RetriesMalformed(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("have you considered cooking earlier?")
	m := int32(-30)
	enqueueFixes(p, EventFix{EventRecipeID: 101, Action: EventFixShiftServe, Minutes: &m, Reason: "earlier"})
	svc := eventFixService(t, p, &sugEvents{})
	got, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int32(-30), *got[0].Minutes)
	assert.Equal(t, 2, p.CallCount())
}

func TestSuggestEventFixes_MalformedTwice(t *testing.T) {
	p := llm.NewMockProvider()
	p.EnqueueText("nope")
	p.EnqueueText("nope")
	svc := eventFixService(t, p, &sugEvents{})
	_, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed")
}

func TestSuggestEventFixes_EventNotFound(t *testing.T) {
	p := llm.NewMockProvider()
	svc := eventFixService(t, p, &sugEvents{})
	_, err := svc.SuggestEventFixes(context.Background(), 7, 99, 20, 6) // wrong household
	require.Error(t, err)
	assert.Equal(t, 0, p.CallCount())
}

func TestSuggestEventFixes_Unavailable(t *testing.T) {
	svc := eventFixService(t, nil, &sugEvents{})
	_, err := svc.SuggestEventFixes(context.Background(), 7, 9, 20, 6)
	assert.ErrorIs(t, err, ErrUnavailable)
}
