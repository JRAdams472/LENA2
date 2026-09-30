package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
)

// Event fix actions the client can apply through existing mutations.
// shift_serve → updateEventRecipe(targetTime); the rest →
// updateEventRecipeStep on the named step.
const (
	EventFixShiftServe   = "shift_serve"
	EventFixSetAppliance = "set_appliance"
	EventFixSetDuration  = "set_duration"
	EventFixSetDependsOn = "set_dependency"
)

// EventFix is one reviewable schedule fix. Exactly the fields the named
// action needs are populated; applying uses the existing event mutations.
type EventFix struct {
	EventRecipeID int64   `json:"eventRecipeId"`
	RecipeName    string  `json:"-"`
	StepNumber    *int32  `json:"stepNumber,omitempty"`
	Action        string  `json:"action"`
	Minutes       *int32  `json:"minutes,omitempty"`
	Appliance     *string `json:"appliance,omitempty"`
	DurationMin   *int32  `json:"durationMinutes,omitempty"`
	DependsOn     *int32  `json:"dependsOnStepNumber,omitempty"`
	Reason        string  `json:"reason"`
}

const eventFixSystemPrompt = `You are LENA's event-scheduling assistant. The user message contains a food
event's computed cooking timeline. Steps are backwards-scheduled from each
dish's targetTime; conflicting steps name the same appliance at overlapping
times, and unschedulable dishes have broken dependency chains.

Task: propose concrete fixes that resolve every appliance conflict and
unschedulable dish. Allowed actions, applied by the user after review:

- {"action":"shift_serve","eventRecipeId":N,"minutes":M} — move a dish's
  targetTime by M minutes (signed, MUST be a multiple of slotGranularityMinutes).
- {"action":"set_appliance","eventRecipeId":N,"stepNumber":S,"appliance":"x"} —
  move a conflicting step to another appliance.
- {"action":"set_duration","eventRecipeId":N,"stepNumber":S,"durationMinutes":D} —
  shorten/lengthen a step (positive, minutes).
- {"action":"set_dependency","eventRecipeId":N,"stepNumber":S,"dependsOnStepNumber":D} —
  rewire a step's dependency (only to fix a broken/unschedulable chain).

Rules:
- eventRecipeId and stepNumber must exist in the provided timeline.
- Prefer minimal changes: shift the dish that is easier to move, move a
  step to a free appliance before lengthening anything.
- Every suggestion must reduce conflicts or fix unschedulable dishes —
  no cosmetic churn.
- "reason" is one short phrase explaining what conflict it resolves.
- Reply ONLY with a JSON object: {"fixes":[{...}]} — [] if nothing helps.`

type eventFixResponse struct {
	Fixes []EventFix `json:"fixes"`
}

// SuggestEventFixes assembles the event timeline through the read-only
// tool registry and asks the provider for structured fixes. With a clean
// timeline it returns no suggestions without calling the model. Every
// suggestion is validated against the timeline — unknown recipes, steps,
// or out-of-range values are dropped.
func (s *Service) SuggestEventFixes(ctx context.Context, userID, householdID, foodEventID int64, maxSuggestions int) ([]EventFix, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	if maxSuggestions <= 0 {
		maxSuggestions = 6
	}
	if maxSuggestions > 10 {
		maxSuggestions = 10
	}
	scope := tools.Scope{UserID: userID, HouseholdID: householdID}

	tlAny, err := s.reg.Call(ctx, scope, "get_event_timeline", jsonArgs(map[string]any{"foodEventId": foodEventID}))
	if err != nil {
		return nil, err
	}
	tl, ok := tlAny.(tools.EventTimelineOut)
	if !ok {
		return nil, fmt.Errorf("get_event_timeline returned %T", tlAny)
	}
	if tl.FoodEventID == 0 {
		return nil, errors.New("food event not found")
	}
	if !timelineNeedsHelp(tl) {
		return []EventFix{}, nil
	}

	reqBody, err := json.Marshal(tl)
	if err != nil {
		return nil, fmt.Errorf("marshal timeline: %w", err)
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: eventFixSystemPrompt},
		{Role: llm.RoleUser, Content: string(reqBody)},
	}

	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.provider.Chat(ctx, llm.Request{Messages: msgs, JSONMode: true})
		if err != nil {
			return nil, fmt.Errorf("suggest provider: %w", err)
		}
		var parsed eventFixResponse
		if err := json.Unmarshal([]byte(resp.Message.Content), &parsed); err != nil {
			msgs = append(msgs,
				llm.Message{Role: llm.RoleAssistant, Content: resp.Message.Content},
				llm.Message{Role: llm.RoleUser, Content: "That was not valid JSON matching the required schema. Reply ONLY with the JSON object."},
			)
			continue
		}
		return filterEventFixes(parsed.Fixes, tl, maxSuggestions), nil
	}
	return nil, errors.New("assistant returned malformed suggestions")
}

// timelineNeedsHelp reports whether the timeline has anything to fix —
// warnings, step conflicts, or unschedulable dishes.
func timelineNeedsHelp(tl tools.EventTimelineOut) bool {
	if len(tl.Warnings) > 0 {
		return true
	}
	for _, r := range tl.Recipes {
		if r.Unschedulable || len(r.Warnings) > 0 {
			return true
		}
		for _, st := range r.Steps {
			if len(st.Conflicts) > 0 {
				return true
			}
		}
	}
	return false
}

// filterEventFixes drops model output violating the contract and stamps
// the recipe name for display.
func filterEventFixes(in []EventFix, tl tools.EventTimelineOut, maxCount int) []EventFix {
	stepsByRecipe := map[int64]map[int32]bool{}
	names := map[int64]string{}
	for _, r := range tl.Recipes {
		names[r.EventRecipeID] = r.Name
		set := map[int32]bool{}
		for _, st := range r.Steps {
			set[st.StepNumber] = true
		}
		stepsByRecipe[r.EventRecipeID] = set
	}
	gran := int64(tl.SlotGranularityMinutes)
	if gran <= 0 {
		gran = 15
	}

	seen := map[string]bool{}
	out := make([]EventFix, 0, len(in))
	for _, f := range in {
		steps, known := stepsByRecipe[f.EventRecipeID]
		if !known {
			continue
		}
		switch f.Action {
		case EventFixShiftServe:
			if f.Minutes == nil || *f.Minutes == 0 || *f.Minutes > 480 || *f.Minutes < -480 ||
				int64(*f.Minutes)%gran != 0 {
				continue
			}
		case EventFixSetAppliance:
			if f.StepNumber == nil || !steps[*f.StepNumber] || f.Appliance == nil ||
				strings.TrimSpace(*f.Appliance) == "" || len(*f.Appliance) > 40 {
				continue
			}
		case EventFixSetDuration:
			if f.StepNumber == nil || !steps[*f.StepNumber] || f.DurationMin == nil ||
				*f.DurationMin <= 0 || *f.DurationMin > 720 {
				continue
			}
		case EventFixSetDependsOn:
			if f.StepNumber == nil || !steps[*f.StepNumber] || f.DependsOn == nil ||
				*f.DependsOn == *f.StepNumber || !steps[*f.DependsOn] {
				continue
			}
		default:
			continue
		}
		step := int32(0)
		if f.StepNumber != nil {
			step = *f.StepNumber
		}
		key := fmt.Sprintf("%d|%s|%d", f.EventRecipeID, f.Action, step)
		if seen[key] {
			continue
		}
		seen[key] = true
		f.RecipeName = names[f.EventRecipeID]
		f.Reason = truncate(strings.TrimSpace(f.Reason), 160)
		out = append(out, f)
	}
	if len(out) > maxCount {
		out = out[:maxCount]
	}
	return out
}
