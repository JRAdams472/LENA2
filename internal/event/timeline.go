package event

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// TimelineStepInput is one recipe step as scheduling input. The BFF
// resolves recipe steps and hands them here; the engine never touches the
// database (compute-on-read persistence).
type TimelineStepInput struct {
	StepNumber          int32
	Instruction         string
	DurationMinutes     *int32
	StepType            string
	IsPassive           bool
	DependsOnStepNumber *int32
	Appliance           string
}

// TimelineRecipeInput is one event slot with its resolved recipe steps.
// Slots without a linked recipe simply carry no steps.
type TimelineRecipeInput struct {
	EventRecipeID int64
	Name          string
	TargetTime    time.Time
	Servings      *int32
	BaseServings  *int32
	Steps         []TimelineStepInput
}

// ScheduledStep is one step placed on the master timeline.
type ScheduledStep struct {
	StepNumber     int32
	Instruction    string
	StepType       string
	IsPassive      bool
	Appliance      string
	DurationMinute *int32
	// ScheduledMinutes is the effective slot time the step occupies after
	// estimation and rounding up to the event's slot granularity.
	ScheduledMinutes int32
	Estimated        bool
	Start            time.Time
	End              time.Time
	Conflicts        []string
}

// RecipeTimeline is the computed schedule for one event recipe.
type RecipeTimeline struct {
	EventRecipeID int64
	Name          string
	TargetTime    time.Time
	Servings      *int32
	BaseServings  *int32
	// StartBy is the earliest moment work must begin for the target to be
	// met; nil when the recipe is unschedulable.
	StartBy       *time.Time
	Steps         []ScheduledStep
	Warnings      []string
	Unschedulable bool
}

// Timeline is the master schedule for one food event.
type Timeline struct {
	FoodEventID int64
	Recipes     []RecipeTimeline
	Warnings    []string
}

// BuildTimeline backwards-schedules every recipe's step DAG from its
// target serve time. Each step ends by the latest start of its dependents
// (sinks end at the target); its start is end minus duration. Durations
// round up to the event's slot granularity so all boundaries are aligned,
// and a missing duration is estimated as one slot and flagged. After
// placement, steps that name the same appliance and overlap in time are
// marked in conflict — the engine reports the contention rather than
// resolving it, which is the scheduler-advice phase's job.
func BuildTimeline(ev FoodEvent, slots []TimelineRecipeInput) Timeline {
	tl := Timeline{FoodEventID: ev.FoodEventID}
	sorted := append([]TimelineRecipeInput(nil), slots...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TargetTime.Equal(sorted[j].TargetTime) {
			return sorted[i].EventRecipeID < sorted[j].EventRecipeID
		}
		return sorted[i].TargetTime.Before(sorted[j].TargetTime)
	})
	for _, slot := range sorted {
		tl.Recipes = append(tl.Recipes, scheduleRecipe(ev.SlotGranularityMinutes, slot))
	}
	markApplianceConflicts(&tl)
	return tl
}

func scheduleRecipe(granularity int16, slot TimelineRecipeInput) RecipeTimeline {
	rt := RecipeTimeline{
		EventRecipeID: slot.EventRecipeID,
		Name:          slot.Name,
		TargetTime:    slot.TargetTime,
		Servings:      slot.Servings,
		BaseServings:  slot.BaseServings,
	}
	if len(slot.Steps) == 0 {
		rt.Unschedulable = true
		rt.Warnings = append(rt.Warnings, "no recipe steps to schedule")
		return rt
	}
	order, ok := topoOrder(slot.Steps, &rt)
	if !ok {
		rt.Unschedulable = true
		return rt
	}
	// Index dependents: dependents[d] = steps that need d finished first.
	dependents := make(map[int32][]int32, len(slot.Steps))
	for _, s := range slot.Steps {
		d := depOf(s)
		if d > 0 {
			dependents[d] = append(dependents[d], s.StepNumber)
		}
	}
	g := int64(granularity)
	if g <= 0 {
		g = 15
	}
	ends := make(map[int32]time.Time, len(order))
	starts := make(map[int32]time.Time, len(order))
	byNumber := make(map[int32]TimelineStepInput, len(slot.Steps))
	for _, s := range slot.Steps {
		byNumber[s.StepNumber] = s
	}
	var estimated int
	// Reverse topo order guarantees a step's dependents are placed first.
	for i := len(order) - 1; i >= 0; i-- {
		num := order[i]
		s := byNumber[num]
		end := slot.TargetTime
		for _, d := range dependents[num] {
			if starts[d].Before(end) {
				end = starts[d]
			}
		}
		mins, est := effectiveMinutes(s.DurationMinutes, g)
		if est {
			estimated++
		}
		ends[num] = end
		starts[num] = end.Add(-time.Duration(mins) * time.Minute)
		rt.Steps = append(rt.Steps, ScheduledStep{
			StepNumber:     num,
			Instruction:    s.Instruction,
			StepType:       s.StepType,
			IsPassive:      s.IsPassive,
			Appliance:      strings.TrimSpace(s.Appliance),
			DurationMinute: s.DurationMinutes,
			//nolint:gosec // bounded by an int32 input plus one slot.
			ScheduledMinutes: int32(mins),
			Estimated:        est,
			Start:            starts[num],
			End:              ends[num],
		})
	}
	sort.Slice(rt.Steps, func(i, j int) bool { return rt.Steps[i].Start.Before(rt.Steps[j].Start) })
	earliest := rt.Steps[0].Start
	rt.StartBy = &earliest
	if estimated > 0 {
		rt.Warnings = append(rt.Warnings, fmt.Sprintf("%d step(s) have no duration — estimated", estimated))
	}
	return rt
}

// topoOrder returns step numbers in dependency order (each step after its
// dependencies). The default edge is the previous step number; an explicit
// depends_on_step_number overrides it. A missing dependency is ignored
// with a warning; a cycle marks the recipe unschedulable.
func topoOrder(steps []TimelineStepInput, rt *RecipeTimeline) ([]int32, bool) {
	nums := make(map[int32]bool, len(steps))
	for _, s := range steps {
		nums[s.StepNumber] = true
	}
	indeg := make(map[int32]int, len(steps))
	next := make(map[int32][]int32, len(steps))
	for _, s := range steps {
		if _, ok := indeg[s.StepNumber]; !ok {
			indeg[s.StepNumber] = 0
		}
		d := depOf(s)
		if d == 0 {
			continue
		}
		if !nums[d] {
			// A default edge onto a non-contiguous numbering is silently
			// dropped; an explicit dep on a missing step is worth noting.
			if s.DependsOnStepNumber != nil {
				rt.Warnings = append(rt.Warnings, fmt.Sprintf("step %d depends on missing step %d — ignored", s.StepNumber, d))
			}
			continue
		}
		indeg[s.StepNumber]++
		next[d] = append(next[d], s.StepNumber)
	}
	var queue []int32
	for n, deg := range indeg {
		if deg == 0 {
			queue = append(queue, n)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	var order []int32
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		order = append(order, n)
		var freed []int32
		for _, m := range next[n] {
			indeg[m]--
			if indeg[m] == 0 {
				freed = append(freed, m)
			}
		}
		if len(freed) > 0 {
			queue = append(queue, freed...)
			sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
		}
	}
	if len(order) != len(steps) {
		rt.Warnings = append(rt.Warnings, "step dependencies form a cycle — recipe cannot be scheduled")
		return nil, false
	}
	return order, true
}

// depOf returns the step's effective dependency: the explicit
// depends_on_step_number when set, else the previous step number, else 0.
func depOf(s TimelineStepInput) int32 {
	if s.DependsOnStepNumber != nil {
		return *s.DependsOnStepNumber
	}
	if s.StepNumber > 1 {
		return s.StepNumber - 1
	}
	return 0
}

// effectiveMinutes rounds a duration up to a whole number of slots so
// every scheduled boundary lands on the event's granularity. A missing
// duration is estimated as one slot.
func effectiveMinutes(d *int32, granularity int64) (int64, bool) {
	if d == nil || *d <= 0 {
		return granularity, true
	}
	m := int64(*d)
	if r := m % granularity; r != 0 {
		m += granularity - r
	}
	return m, false
}

// markApplianceConflicts finds overlapping scheduled steps that name the
// same appliance and records the contention on both steps plus recipe and
// event warnings.
func markApplianceConflicts(tl *Timeline) {
	type use struct {
		recipe int
		step   int
	}
	byAppliance := make(map[string][]use)
	for i := range tl.Recipes {
		for j := range tl.Recipes[i].Steps {
			a := strings.ToLower(tl.Recipes[i].Steps[j].Appliance)
			if a != "" {
				byAppliance[a] = append(byAppliance[a], use{recipe: i, step: j})
			}
		}
	}
	seen := make(map[string]bool)
	for appliance, uses := range byAppliance {
		for i := 0; i < len(uses); i++ {
			for j := i + 1; j < len(uses); j++ {
				a := tl.Recipes[uses[i].recipe].Steps[uses[i].step]
				b := tl.Recipes[uses[j].recipe].Steps[uses[j].step]
				if !a.Start.Before(b.End) || !b.Start.Before(a.End) {
					continue
				}
				msg := fmt.Sprintf("%s is needed by %q step %d and %q step %d at the same time",
					appliance, tl.Recipes[uses[i].recipe].Name, a.StepNumber,
					tl.Recipes[uses[j].recipe].Name, b.StepNumber)
				a.Conflicts = append(a.Conflicts, msg)
				b.Conflicts = append(b.Conflicts, msg)
				tl.Recipes[uses[i].recipe].Steps[uses[i].step] = a
				tl.Recipes[uses[j].recipe].Steps[uses[j].step] = b
				if !seen[msg] {
					seen[msg] = true
					tl.Warnings = append(tl.Warnings, "appliance conflict: "+msg)
					tl.Recipes[uses[i].recipe].Warnings = append(tl.Recipes[uses[i].recipe].Warnings, msg)
					if uses[i].recipe != uses[j].recipe {
						tl.Recipes[uses[j].recipe].Warnings = append(tl.Recipes[uses[j].recipe].Warnings, msg)
					}
				}
			}
		}
	}
}
