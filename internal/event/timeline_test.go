package event

import (
	"testing"
	"time"
)

var tlTarget = time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)

func tlEvent(gran int16) FoodEvent {
	return FoodEvent{FoodEventID: 1, SlotGranularityMinutes: gran}
}

func tlStep(num int32, mins int32) TimelineStepInput {
	return TimelineStepInput{StepNumber: num, Instruction: "step", DurationMinutes: &mins}
}

func TestBuildTimelineLinearChain(t *testing.T) {
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{{
		EventRecipeID: 10, Name: "Roast", TargetTime: tlTarget,
		Steps: []TimelineStepInput{
			tlStep(1, 30),
			tlStep(2, 15),
			tlStep(3, 15),
		},
	}})
	if len(tl.Recipes) != 1 {
		t.Fatalf("recipes = %d, want 1", len(tl.Recipes))
	}
	rt := tl.Recipes[0]
	if rt.Unschedulable {
		t.Fatalf("unexpected unschedulable: %v", rt.Warnings)
	}
	if rt.StartBy == nil || !rt.StartBy.Equal(tlTarget.Add(-60*time.Minute)) {
		t.Fatalf("startBy = %v, want %v", rt.StartBy, tlTarget.Add(-60*time.Minute))
	}
	if len(rt.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(rt.Steps))
	}
	// Steps come back start-ordered: 1 then 2 then 3.
	want := [][2]time.Time{
		{tlTarget.Add(-60 * time.Minute), tlTarget.Add(-30 * time.Minute)},
		{tlTarget.Add(-30 * time.Minute), tlTarget.Add(-15 * time.Minute)},
		{tlTarget.Add(-15 * time.Minute), tlTarget},
	}
	for i, s := range rt.Steps {
		if !s.Start.Equal(want[i][0]) || !s.End.Equal(want[i][1]) {
			t.Errorf("step %d = %v→%v, want %v→%v", s.StepNumber, s.Start, s.End, want[i][0], want[i][1])
		}
	}
}

func TestBuildTimelineRoundsToGranularity(t *testing.T) {
	tl := BuildTimeline(tlEvent(30), []TimelineRecipeInput{{
		EventRecipeID: 10, Name: "R", TargetTime: tlTarget,
		Steps: []TimelineStepInput{tlStep(1, 25)},
	}})
	s := tl.Recipes[0].Steps[0]
	if s.ScheduledMinutes != 30 {
		t.Fatalf("scheduled = %d, want 30 (25 rounded up)", s.ScheduledMinutes)
	}
	if !s.Start.Equal(tlTarget.Add(-30 * time.Minute)) {
		t.Fatalf("start = %v, want %v", s.Start, tlTarget.Add(-30*time.Minute))
	}
}

func TestBuildTimelineEstimatesMissingDuration(t *testing.T) {
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{{
		EventRecipeID: 10, Name: "R", TargetTime: tlTarget,
		Steps: []TimelineStepInput{{StepNumber: 1, Instruction: "mystery"}},
	}})
	rt := tl.Recipes[0]
	if !rt.Steps[0].Estimated || rt.Steps[0].ScheduledMinutes != 15 {
		t.Fatalf("step = %+v, want estimated 15", rt.Steps[0])
	}
	if len(rt.Warnings) == 0 {
		t.Fatal("want a missing-duration warning")
	}
}

func TestBuildTimelineExplicitDependency(t *testing.T) {
	// Step 3 explicitly depends on step 1 — it can run while step 2 still
	// needs step 1 too, so 2 and 3 are parallel branches off 1.
	dep := int32(1)
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{{
		EventRecipeID: 10, Name: "R", TargetTime: tlTarget,
		Steps: []TimelineStepInput{
			tlStep(1, 15),
			{StepNumber: 2, Instruction: "b", DurationMinutes: tlPtr(15)},
			{StepNumber: 3, Instruction: "c", DurationMinutes: tlPtr(15), DependsOnStepNumber: &dep},
		},
	}})
	rt := tl.Recipes[0]
	if rt.Unschedulable {
		t.Fatalf("unexpected unschedulable: %v", rt.Warnings)
	}
	// Step 1 must finish before the latest-starting dependent (both 2 and
	// 3 start at target-15), so step 1 = [target-30, target-15).
	var s1 ScheduledStep
	for _, s := range rt.Steps {
		if s.StepNumber == 1 {
			s1 = s
		}
	}
	if !s1.End.Equal(tlTarget.Add(-15 * time.Minute)) {
		t.Fatalf("step1 end = %v, want %v", s1.End, tlTarget.Add(-15*time.Minute))
	}
}

func TestBuildTimelineCycleUnschedulable(t *testing.T) {
	one, two := int32(1), int32(2)
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{{
		EventRecipeID: 10, Name: "R", TargetTime: tlTarget,
		Steps: []TimelineStepInput{
			{StepNumber: 1, Instruction: "a", DependsOnStepNumber: &two},
			{StepNumber: 2, Instruction: "b", DependsOnStepNumber: &one},
		},
	}})
	if !tl.Recipes[0].Unschedulable {
		t.Fatal("cycle should be unschedulable")
	}
}

func TestBuildTimelineNoSteps(t *testing.T) {
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{{
		EventRecipeID: 10, Name: "Free-form dish", TargetTime: tlTarget,
	}})
	if !tl.Recipes[0].Unschedulable || len(tl.Recipes[0].Warnings) == 0 {
		t.Fatalf("want unschedulable with warning, got %+v", tl.Recipes[0])
	}
}

func TestBuildTimelineApplianceConflict(t *testing.T) {
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{
		{EventRecipeID: 10, Name: "Turkey", TargetTime: tlTarget,
			Steps: []TimelineStepInput{{StepNumber: 1, Instruction: "roast", Appliance: "Oven", DurationMinutes: tlPtr(60)}}},
		{EventRecipeID: 11, Name: "Pie", TargetTime: tlTarget,
			Steps: []TimelineStepInput{{StepNumber: 1, Instruction: "bake", Appliance: "oven", DurationMinutes: tlPtr(30)}}},
	})
	if len(tl.Warnings) == 0 {
		t.Fatal("want an appliance conflict warning")
	}
	for _, rt := range tl.Recipes {
		if len(rt.Steps[0].Conflicts) == 0 {
			t.Errorf("recipe %q step should carry the conflict", rt.Name)
		}
	}
}

func TestBuildTimelineNoConflictWhenAppliancesDiffer(t *testing.T) {
	tl := BuildTimeline(tlEvent(15), []TimelineRecipeInput{
		{EventRecipeID: 10, Name: "A", TargetTime: tlTarget,
			Steps: []TimelineStepInput{{StepNumber: 1, Instruction: "roast", Appliance: "oven", DurationMinutes: tlPtr(60)}}},
		{EventRecipeID: 11, Name: "B", TargetTime: tlTarget,
			Steps: []TimelineStepInput{{StepNumber: 1, Instruction: "fry", Appliance: "stovetop", DurationMinutes: tlPtr(30)}}},
	})
	if len(tl.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", tl.Warnings)
	}
}

func tlPtr(v int32) *int32 { return &v }
