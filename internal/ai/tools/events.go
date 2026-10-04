package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// EventTimelineReader is the event surface the timeline tool needs.
type EventTimelineReader interface {
	GetFoodEventByID(ctx context.Context, foodEventID, householdID int64) (event.FoodEvent, error)
	ListFoodEvents(ctx context.Context, householdID int64, limit, offset int32) ([]event.FoodEvent, error)
	ListEventRecipesForEvent(ctx context.Context, foodEventID, householdID int64) ([]event.EventRecipe, error)
	ListEventRecipeStepsForEvents(ctx context.Context, foodEventIDs []int64, householdID int64) ([]event.EventRecipeStep, error)
}

// EventStepOut is one scheduled step as the model sees it.
type EventStepOut struct {
	StepNumber  int32    `json:"step"`
	Instruction string   `json:"instruction"`
	Appliance   string   `json:"appliance,omitempty"`
	DurationMin *int32   `json:"durationMinutes,omitempty"`
	Start       string   `json:"start"`
	End         string   `json:"end"`
	Conflicts   []string `json:"conflicts,omitempty"`
}

// EventRecipeOut is one event dish's computed schedule.
type EventRecipeOut struct {
	EventRecipeID int64          `json:"eventRecipeId"`
	Name          string         `json:"name"`
	TargetTime    string         `json:"targetTime"`
	Unschedulable bool           `json:"unschedulable,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	Steps         []EventStepOut `json:"steps"`
}

// EventTimelineOut is the get_event_timeline result.
type EventTimelineOut struct {
	FoodEventID            int64            `json:"foodEventId"`
	Name                   string           `json:"name"`
	EventDate              string           `json:"eventDate"`
	SlotGranularityMinutes int16            `json:"slotGranularityMinutes"`
	Warnings               []string         `json:"warnings,omitempty"`
	Recipes                []EventRecipeOut `json:"recipes"`
}

// RegisterEventTools wires get_event_timeline — the same compute-on-read
// schedule the UI renders, so the model sees exactly the conflicts the
// user is looking at. Without a foodEventId it uses the newest event.
func RegisterEventTools(reg *Registry, events EventTimelineReader, recipes RecipeLookup) {
	reg.Register(llm.ToolSpec{
		Name:        "get_event_timeline",
		Description: "Compute a food event's cooking timeline: per-dish scheduled steps with appliance-conflict flags. Omit foodEventId for the newest event.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"foodEventId": map[string]any{
					"type":        "integer",
					"description": "Event to schedule. Omit for the newest event.",
				},
			},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		var a struct {
			FoodEventID int64 `json:"foodEventId"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_event_timeline args: %w", err)
			}
		}
		if scope.HouseholdID == 0 {
			return EventTimelineOut{}, nil
		}
		if a.FoodEventID <= 0 {
			list, err := events.ListFoodEvents(ctx, scope.HouseholdID, 1, 0)
			if err != nil {
				return nil, fmt.Errorf("list food events: %w", err)
			}
			if len(list) == 0 {
				return EventTimelineOut{}, nil
			}
			a.FoodEventID = list[0].FoodEventID
		}
		return buildEventTimeline(ctx, events, recipes, a.FoodEventID, scope.HouseholdID)
	})
}

// buildEventTimeline mirrors the BFF's EventTimeline assembly: slot
// snapshots (not the shared recipe rows) drive the schedule so the model
// reasons about the same data the user sees.
func buildEventTimeline(ctx context.Context, events EventTimelineReader, recipes RecipeLookup, foodEventID, householdID int64) (EventTimelineOut, error) {
	ev, err := events.GetFoodEventByID(ctx, foodEventID, householdID)
	if err != nil {
		return EventTimelineOut{}, fmt.Errorf("get food event: %w", err)
	}
	ers, err := events.ListEventRecipesForEvent(ctx, foodEventID, householdID)
	if err != nil {
		return EventTimelineOut{}, fmt.Errorf("list event recipes: %w", err)
	}
	names, err := eventRecipeNames(ctx, recipes, ers)
	if err != nil {
		return EventTimelineOut{}, err
	}
	steps, err := events.ListEventRecipeStepsForEvents(ctx, []int64{foodEventID}, householdID)
	if err != nil {
		return EventTimelineOut{}, fmt.Errorf("list event steps: %w", err)
	}
	stepsBy := make(map[int64][]event.EventRecipeStep)
	for _, st := range steps {
		stepsBy[st.EventRecipeID] = append(stepsBy[st.EventRecipeID], st)
	}

	slots := make([]event.TimelineRecipeInput, len(ers))
	for i, er := range ers {
		slots[i] = timelineSlot(er, stepsBy[er.EventRecipeID], names)
	}

	tl := event.BuildTimeline(ev, slots)
	out := EventTimelineOut{
		FoodEventID:            tl.FoodEventID,
		Name:                   ev.Name,
		EventDate:              ev.EventDate.Format("2006-01-02"),
		SlotGranularityMinutes: ev.SlotGranularityMinutes,
		Warnings:               tl.Warnings,
		Recipes:                make([]EventRecipeOut, 0, len(tl.Recipes)),
	}
	for _, rt := range tl.Recipes {
		r := EventRecipeOut{
			EventRecipeID: rt.EventRecipeID,
			Name:          rt.Name,
			TargetTime:    rt.TargetTime.Format("15:04"),
			Unschedulable: rt.Unschedulable,
			Warnings:      rt.Warnings,
			Steps:         make([]EventStepOut, 0, len(rt.Steps)),
		}
		for _, st := range rt.Steps {
			instr := st.Instruction
			if len(instr) > 160 {
				instr = instr[:160] + "…"
			}
			r.Steps = append(r.Steps, EventStepOut{
				StepNumber:  st.StepNumber,
				Instruction: instr,
				Appliance:   st.Appliance,
				DurationMin: st.DurationMinute,
				Start:       st.Start.Format("15:04"),
				End:         st.End.Format("15:04"),
				Conflicts:   st.Conflicts,
			})
		}
		out.Recipes = append(out.Recipes, r)
	}
	return out, nil
}

// eventRecipeNames resolves display names for the event's linked recipes.
func eventRecipeNames(ctx context.Context, recipes RecipeLookup, ers []event.EventRecipe) (map[int64]string, error) {
	names := map[int64]string{}
	var recipeIDs []int64
	for _, er := range ers {
		if er.RecipeID != nil {
			recipeIDs = append(recipeIDs, *er.RecipeID)
		}
	}
	if len(recipeIDs) == 0 || recipes == nil {
		return names, nil
	}
	list, err := recipes.GetRecipesByIDs(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("recipe names: %w", err)
	}
	for _, r := range list {
		names[r.RecipeID] = r.Name
	}
	return names, nil
}

// timelineSlot converts one event recipe into a scheduling input, falling
// back to notes/meal type for a name.
func timelineSlot(er event.EventRecipe, steps []event.EventRecipeStep, names map[int64]string) event.TimelineRecipeInput {
	in := make([]event.TimelineStepInput, 0, len(steps))
	for _, s := range steps {
		in = append(in, event.TimelineStepInput{
			StepNumber:          s.StepNumber,
			Instruction:         s.Instruction,
			DurationMinutes:     s.DurationMinutes,
			StepType:            s.StepType,
			IsPassive:           s.IsPassive,
			DependsOnStepNumber: s.DependsOnStepNumber,
			Appliance:           s.Appliance,
		})
	}
	name := ""
	if er.RecipeID != nil {
		name = names[*er.RecipeID]
	}
	if name == "" {
		name = er.Notes
	}
	if name == "" {
		name = er.MealType
	}
	if name == "" {
		name = "Unnamed dish"
	}
	return event.TimelineRecipeInput{
		EventRecipeID: er.EventRecipeID,
		Name:          name,
		TargetTime:    er.TargetTime,
		Servings:      er.Servings,
		BaseServings:  er.BaseServings,
		Steps:         in,
	}
}
