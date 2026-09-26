package bff

import (
	"context"
	"strconv"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// FoodEvent resolves a single event by ID, scoped to the caller's
// household; nil when the event does not exist or belongs elsewhere.
func (r *Resolver) FoodEvent(ctx context.Context, args struct{ ID graphql.ID }) (*foodEventResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	ev, err := r.EventService.GetFoodEventByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	recipes, err := r.EventService.ListEventRecipesForEvent(ctx, ev.FoodEventID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.InventoryService, u.UserID,
		distinctIDs(recipes, func(er event.EventRecipe) *int64 { return er.RecipeID }), nil)
	if err != nil {
		return nil, err
	}
	stepsBy, err := r.eventStepsByEventRecipe(ctx, []int64{ev.FoodEventID}, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	return &foodEventResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, event: ev, recipes: recipes, rc: rc, stepsBy: stepsBy, loaded: true}, nil
}

// FoodEvents resolves a page of the caller's household events.
func (r *Resolver) FoodEvents(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*foodEventPageResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)
	events, err := r.EventService.ListFoodEvents(ctx, u.HouseholdID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.EventService.CountFoodEvents(ctx, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	eventIDs := distinctIDs(events, func(e event.FoodEvent) *int64 { return &e.FoodEventID })
	recipesByEvent := make(map[int64][]event.EventRecipe)
	if len(eventIDs) > 0 {
		recipes, err := r.EventService.ListEventRecipesByEvents(ctx, eventIDs, u.HouseholdID)
		if err != nil {
			return nil, err
		}
		for _, er := range recipes {
			recipesByEvent[er.FoodEventID] = append(recipesByEvent[er.FoodEventID], er)
		}
	}
	stepsBy, err := r.eventStepsByEventRecipe(ctx, eventIDs, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	return &foodEventPageResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, events: events, recipesByEvent: recipesByEvent, stepsBy: stepsBy, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// EventTimeline computes the event's master schedule on read: load the
// slots and their recipe steps, then hand everything to the pure engine.
func (r *Resolver) EventTimeline(ctx context.Context, args struct {
	FoodEventID graphql.ID
}) (*eventTimelineResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.FoodEventID))
	if err != nil {
		return nil, err
	}
	ev, err := r.EventService.GetFoodEventByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	recipes, err := r.EventService.ListEventRecipesForEvent(ctx, ev.FoodEventID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	// Recipe names come off the shared recipe rows; the scheduled steps
	// come off each slot's own snapshot so event-specific edits — and a
	// later recipe edit or deletion — never move a laid-out plan.
	var names map[int64]string
	recipeIDs := distinctIDs(recipes, func(er event.EventRecipe) *int64 { return er.RecipeID })
	if len(recipeIDs) > 0 {
		recs, err := r.RecipeService.GetRecipesByIDs(ctx, recipeIDs)
		if err != nil {
			return nil, err
		}
		names = make(map[int64]string, len(recs))
		for _, rec := range recs {
			names[rec.RecipeID] = rec.Name
		}
	}
	snapshot, err := r.EventService.ListEventRecipeStepsForEvents(ctx, []int64{ev.FoodEventID}, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	stepsBy := make(map[int64][]event.EventRecipeStep)
	for _, st := range snapshot {
		stepsBy[st.EventRecipeID] = append(stepsBy[st.EventRecipeID], st)
	}
	slots := make([]event.TimelineRecipeInput, len(recipes))
	for i, er := range recipes {
		var steps []event.TimelineStepInput
		for _, s := range stepsBy[er.EventRecipeID] {
			steps = append(steps, event.TimelineStepInput{
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
		slots[i] = event.TimelineRecipeInput{
			EventRecipeID: er.EventRecipeID,
			Name:          name,
			TargetTime:    er.TargetTime,
			Servings:      er.Servings,
			Steps:         steps,
		}
	}
	tl := event.BuildTimeline(ev, slots)
	return &eventTimelineResolver{tl: tl}, nil
}

// CreateFoodEvent creates a new event for the caller's household.
func (r *Resolver) CreateFoodEvent(ctx context.Context, args struct{ Input createFoodEventInput }) (*foodEventResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	granularity, err := checkedGranularity(args.Input.SlotGranularityMinutes)
	if err != nil {
		return nil, err
	}
	name, err := checkedName(args.Input.Name, "name")
	if err != nil {
		return nil, err
	}
	d, err := time.Parse("2006-01-02", args.Input.EventDate)
	if err != nil {
		return nil, badInputf("invalid eventDate %q, want YYYY-MM-DD", args.Input.EventDate)
	}
	var ev event.FoodEvent
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		ev, err = r.EventService.CreateFoodEvent(ctx, event.FoodEvent{
			HouseholdID:            u.HouseholdID,
			Name:                   name,
			EventDate:              d,
			SlotGranularityMinutes: granularity,
			IsActive:               true,
		}, u.Email)
		if err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventCreated, &ev.FoodEventID)
	})
	if err != nil {
		return nil, err
	}
	return &foodEventResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, event: ev}, nil
}

// UpdateFoodEvent modifies an existing household event.
func (r *Resolver) UpdateFoodEvent(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateFoodEventInput
}) (*foodEventResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.EventService.GetFoodEventByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name, err = checkedName(*args.Input.Name, "name")
		if err != nil {
			return nil, err
		}
	}
	day := existing.EventDate
	if args.Input.EventDate != nil {
		if d, err := time.Parse("2006-01-02", *args.Input.EventDate); err == nil {
			day = d
		} else {
			return nil, badInputf("invalid eventDate %q, want YYYY-MM-DD", *args.Input.EventDate)
		}
	}
	granularity := existing.SlotGranularityMinutes
	if args.Input.SlotGranularityMinutes != nil {
		granularity, err = checkedGranularity(args.Input.SlotGranularityMinutes)
		if err != nil {
			return nil, err
		}
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.EventService.UpdateFoodEvent(ctx, id, u.HouseholdID, event.FoodEvent{
			Name:                   name,
			EventDate:              day,
			SlotGranularityMinutes: granularity,
			IsActive:               isActive,
		}, u.Email); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &id)
	})
	if err != nil {
		return nil, err
	}
	updated, err := r.EventService.GetFoodEventByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	recipes, err := r.EventService.ListEventRecipesForEvent(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	rc, err := loadRecipeChildren(ctx, r.RecipeService, r.UserPrefsService, r.InventoryService, u.UserID,
		distinctIDs(recipes, func(er event.EventRecipe) *int64 { return er.RecipeID }), nil)
	if err != nil {
		return nil, err
	}
	stepsBy, err := r.eventStepsByEventRecipe(ctx, []int64{updated.FoodEventID}, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	return &foodEventResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, event: updated, recipes: recipes, rc: rc, stepsBy: stepsBy, loaded: true}, nil
}

// DeleteFoodEvent removes an event owned by the caller's household. The
// notification deep-link goes stale after deletion (SET NULL), so members
// are told about the removal but cannot navigate to the gone row.
func (r *Resolver) DeleteFoodEvent(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.EventService.DeleteFoodEvent(ctx, id, u.HouseholdID); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventDeleted, nil)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// AddEventRecipe schedules a recipe at a serve time within an event.
// targetTime must land on the event's slot granularity boundary.
func (r *Resolver) AddEventRecipe(ctx context.Context, args struct{ Input addEventRecipeInput }) (*eventRecipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	foodEventID, err := parseID(string(args.Input.FoodEventID))
	if err != nil {
		return nil, err
	}
	var recipeID *int64
	if args.Input.RecipeID != nil {
		rid, err := parseID(string(*args.Input.RecipeID))
		if err != nil {
			return nil, err
		}
		recipeID = &rid
	}
	if args.Input.Servings != nil && *args.Input.Servings <= 0 {
		return nil, badInputf("servings must be positive")
	}
	ev, err := r.EventService.GetFoodEventByID(ctx, foodEventID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	if err := checkTargetTime(ev, args.Input.TargetTime.Time); err != nil {
		return nil, err
	}
	var er event.EventRecipe
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		er, err = r.EventService.AddEventRecipe(ctx, event.EventRecipe{
			FoodEventID: foodEventID,
			RecipeID:    recipeID,
			MealType:    args.Input.MealType,
			TargetTime:  args.Input.TargetTime.Time,
			Servings:    args.Input.Servings,
			Notes:       derefString(args.Input.Notes),
		}, u.HouseholdID, u.Email)
		if err != nil {
			return err
		}
		if err := r.snapshotRecipeSteps(ctx, er, u); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &foodEventID)
	})
	if err != nil {
		return nil, err
	}
	return &eventRecipeResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, er: er}, nil
}

// UpdateEventRecipe modifies a recipe slot on a household event; omitted
// fields keep their current values.
func (r *Resolver) UpdateEventRecipe(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateEventRecipeInput
}) (*eventRecipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.EventService.GetEventRecipeByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	recipeID := existing.RecipeID
	recipeChanged := false
	if args.Input.RecipeID != nil {
		rid, err := parseID(string(*args.Input.RecipeID))
		if err != nil {
			return nil, err
		}
		if existing.RecipeID == nil || *existing.RecipeID != rid {
			recipeChanged = true
		}
		recipeID = &rid
	}
	mealType := existing.MealType
	if args.Input.MealType != nil {
		mealType = *args.Input.MealType
	}
	target := existing.TargetTime
	if args.Input.TargetTime != nil {
		target = args.Input.TargetTime.Time
	}
	servings := existing.Servings
	if args.Input.Servings != nil {
		if *args.Input.Servings <= 0 {
			return nil, badInputf("servings must be positive")
		}
		servings = args.Input.Servings
	}
	notes := existing.Notes
	if args.Input.Notes != nil {
		notes = *args.Input.Notes
	}
	ev, err := r.EventService.GetFoodEventByID(ctx, existing.FoodEventID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	if err := checkTargetTime(ev, target); err != nil {
		return nil, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.EventService.UpdateEventRecipe(ctx, id, u.HouseholdID, event.EventRecipe{
			RecipeID:   recipeID,
			MealType:   mealType,
			TargetTime: target,
			Servings:   servings,
			Notes:      notes,
		}, u.Email); err != nil {
			return err
		}
		// Linking a different recipe re-materializes the snapshot from the
		// new source; edits made to the old copy are intentionally dropped.
		if recipeChanged {
			slot := event.EventRecipe{EventRecipeID: id, RecipeID: recipeID}
			if err := r.snapshotRecipeSteps(ctx, slot, u); err != nil {
				return err
			}
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &existing.FoodEventID)
	})
	if err != nil {
		return nil, err
	}
	updated, err := r.EventService.GetEventRecipeByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	return &eventRecipeResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, er: updated}, nil
}

// RemoveEventRecipe removes a recipe slot from a household event.
func (r *Resolver) RemoveEventRecipe(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	er, err := r.EventService.GetEventRecipeByID(ctx, id, u.HouseholdID)
	if err != nil {
		return false, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.EventService.DeleteEventRecipe(ctx, id, u.HouseholdID); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &er.FoodEventID)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// AddEventRecipeStep appends a step to a slot's snapshot.
func (r *Resolver) AddEventRecipeStep(ctx context.Context, args struct {
	EventRecipeID graphql.ID
	Input         eventRecipeStepInput
}) (*eventRecipeStepResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	eventRecipeID, err := parseID(string(args.EventRecipeID))
	if err != nil {
		return nil, err
	}
	er, err := r.EventService.GetEventRecipeByID(ctx, eventRecipeID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	var st event.EventRecipeStep
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		st, err = r.EventService.AddEventRecipeStep(ctx, event.EventRecipeStep{
			EventRecipeID:       eventRecipeID,
			Instruction:         args.Input.Instruction,
			DurationMinutes:     args.Input.DurationMinutes,
			StepType:            derefString(args.Input.StepType),
			IsPassive:           derefBool(args.Input.IsPassive),
			DependsOnStepNumber: args.Input.DependsOnStepNumber,
			Appliance:           derefString(args.Input.Appliance),
		}, u.HouseholdID, u.Email)
		if err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &er.FoodEventID)
	})
	if err != nil {
		return nil, err
	}
	return &eventRecipeStepResolver{st: st}, nil
}

// UpdateEventRecipeStep edits a snapshot step; the shared recipe step it
// was copied from is untouched.
func (r *Resolver) UpdateEventRecipeStep(ctx context.Context, args struct {
	ID    graphql.ID
	Input eventRecipeStepInput
}) (*eventRecipeStepResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.EventService.GetEventRecipeStepByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	er, err := r.EventService.GetEventRecipeByID(ctx, existing.EventRecipeID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.EventService.UpdateEventRecipeStep(ctx, id, u.HouseholdID, event.EventRecipeStep{
			Instruction:         args.Input.Instruction,
			DurationMinutes:     args.Input.DurationMinutes,
			StepType:            derefString(args.Input.StepType),
			IsPassive:           derefBool(args.Input.IsPassive),
			DependsOnStepNumber: args.Input.DependsOnStepNumber,
			Appliance:           derefString(args.Input.Appliance),
		}, u.Email); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &er.FoodEventID)
	})
	if err != nil {
		return nil, err
	}
	updated, err := r.EventService.GetEventRecipeStepByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	return &eventRecipeStepResolver{st: updated}, nil
}

// RemoveEventRecipeStep deletes a snapshot step owned by the household.
func (r *Resolver) RemoveEventRecipeStep(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	st, err := r.EventService.GetEventRecipeStepByID(ctx, id, u.HouseholdID)
	if err != nil {
		return false, err
	}
	er, err := r.EventService.GetEventRecipeByID(ctx, st.EventRecipeID, u.HouseholdID)
	if err != nil {
		return false, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.EventService.DeleteEventRecipeStep(ctx, id, u.HouseholdID); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &er.FoodEventID)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// SyncEventRecipeSteps re-copies the linked recipe's current steps over
// the slot's snapshot. Slot-specific edits are intentionally discarded.
func (r *Resolver) SyncEventRecipeSteps(ctx context.Context, args struct {
	EventRecipeID graphql.ID
}) (*eventRecipeResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	eventRecipeID, err := parseID(string(args.EventRecipeID))
	if err != nil {
		return nil, err
	}
	er, err := r.EventService.GetEventRecipeByID(ctx, eventRecipeID, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	if er.RecipeID == nil {
		return nil, badInputf("eventRecipe %d has no linked recipe to sync from", eventRecipeID)
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if err := r.snapshotRecipeSteps(ctx, er, u); err != nil {
			return err
		}
		return r.notifyMembers(ctx, u, household.KindEventUpdated, &er.FoodEventID)
	})
	if err != nil {
		return nil, err
	}
	return &eventRecipeResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, er: er}, nil
}

// snapshotRecipeSteps materializes the slot's step snapshot from its
// linked recipe inside the caller's transaction. A slot with no linked
// recipe has nothing to copy and is left as-is.
func (r *Resolver) snapshotRecipeSteps(ctx context.Context, er event.EventRecipe, u currentuser.User) error {
	if er.RecipeID == nil {
		return nil
	}
	steps, err := r.RecipeService.ListRecipeStepsByRecipes(ctx, []int64{*er.RecipeID})
	if err != nil {
		return err
	}
	snap := make([]event.EventRecipeStep, len(steps))
	for i, s := range steps {
		snap[i] = event.EventRecipeStep{
			StepNumber:          s.StepNumber,
			Instruction:         s.Instruction,
			DurationMinutes:     s.DurationMinutes,
			StepType:            s.StepType,
			IsPassive:           s.IsPassive,
			DependsOnStepNumber: s.DependsOnStepNumber,
			Appliance:           s.Appliance,
		}
	}
	return r.EventService.ReplaceEventRecipeSteps(ctx, er.EventRecipeID, u.HouseholdID, snap, u.Email)
}

// notifyMembers fans an event notification out to every household member
// except the actor, inside the caller's transaction.
func (r *Resolver) notifyMembers(ctx context.Context, u currentuser.User, kind household.NotificationKind, foodEventID *int64) error {
	members, err := r.IdentityService.ListUsersByHousehold(ctx, u.HouseholdID)
	if err != nil {
		return err
	}
	var others []int64
	for _, m := range members {
		if m.UserID != u.UserID {
			others = append(others, m.UserID)
		}
	}
	return r.notifyEvent(ctx, others, kind, u.HouseholdID, u.UserID, nil, foodEventID)
}

// checkedGranularity validates the 15/30-minute slot boundary.
func checkedGranularity(v *int32) (int16, error) {
	g := int32(15)
	if v != nil {
		g = *v
	}
	if g != 15 && g != 30 {
		return 0, badInputf("slotGranularityMinutes must be 15 or 30")
	}
	//nolint:gosec // bounded above to 15/30.
	return int16(g), nil
}

// checkTargetTime rejects serve times that do not fall on the event's
// granularity boundary or its date.
func checkTargetTime(ev event.FoodEvent, t time.Time) error {
	if t.Format("2006-01-02") != ev.EventDate.Format("2006-01-02") {
		return badInputf("targetTime must fall on the event date %s", ev.EventDate.Format("2006-01-02"))
	}
	if ev.SlotGranularityMinutes > 0 && t.Minute()%int(ev.SlotGranularityMinutes) != 0 {
		return badInputf("targetTime must fall on a %d-minute boundary", ev.SlotGranularityMinutes)
	}
	return nil
}

// checkedName rejects blank event names.
func checkedName(s, field string) (string, error) {
	if s == "" {
		return "", badInputf("%s must not be empty", field)
	}
	return s, nil
}

// foodEventResolver resolves FoodEvent fields. When loaded is true the
// recipes list was preloaded (single-event and page paths) and is served
// without a query per row — even when empty.
type foodEventResolver struct {
	ev      EventService
	rec     RecipeService
	up      UserPrefsService
	inv     ItemReader
	user    currentuser.User
	event   event.FoodEvent
	recipes []event.EventRecipe
	rc      *recipeChildren
	stepsBy map[int64][]event.EventRecipeStep
	loaded  bool
}

func (r *foodEventResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.event.FoodEventID, 10))
}

func (r *foodEventResolver) Name() string { return r.event.Name }

func (r *foodEventResolver) EventDate() string { return r.event.EventDate.Format("2006-01-02") }

func (r *foodEventResolver) SlotGranularityMinutes() int32 {
	return int32(r.event.SlotGranularityMinutes)
}

func (r *foodEventResolver) IsActive() bool { return r.event.IsActive }

func (r *foodEventResolver) Recipes(ctx context.Context) ([]*eventRecipeResolver, error) {
	var recipes []event.EventRecipe
	if r.loaded {
		recipes = r.recipes
	} else {
		var err error
		recipes, err = r.ev.ListEventRecipesForEvent(ctx, r.event.FoodEventID, r.user.HouseholdID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*eventRecipeResolver, len(recipes))
	for i := range recipes {
		out[i] = &eventRecipeResolver{ev: r.ev, rec: r.rec, up: r.up, inv: r.inv, user: r.user, er: recipes[i], rc: r.rc, steps: r.stepsBy[recipes[i].EventRecipeID], stepsLoaded: r.stepsBy != nil}
	}
	return out, nil
}

// eventRecipeResolver resolves EventRecipe fields. steps holds the slot's
// snapshot when it was batch-loaded with the parent event; otherwise the
// Steps field loads it lazily.
type eventRecipeResolver struct {
	ev          EventService
	rec         RecipeService
	up          UserPrefsService
	inv         ItemReader
	user        currentuser.User
	er          event.EventRecipe
	rc          *recipeChildren
	steps       []event.EventRecipeStep
	stepsLoaded bool
}

func (r *eventRecipeResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.er.EventRecipeID, 10))
}

func (r *eventRecipeResolver) MealType() string { return r.er.MealType }

func (r *eventRecipeResolver) TargetTime() graphql.Time { return graphql.Time{Time: r.er.TargetTime} }

func (r *eventRecipeResolver) Servings() *int32 { return r.er.Servings }

func (r *eventRecipeResolver) Notes() *string { return nilIfEmpty(r.er.Notes) }

// Steps returns the slot's snapshot steps — preloaded with the event, or
// lazily when the resolver was built outside the preload paths.
func (r *eventRecipeResolver) Steps(ctx context.Context) ([]*eventRecipeStepResolver, error) {
	steps := r.steps
	if !r.stepsLoaded {
		var err error
		steps, err = r.ev.ListEventRecipeSteps(ctx, r.er.EventRecipeID, r.user.HouseholdID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*eventRecipeStepResolver, len(steps))
	for i := range steps {
		out[i] = &eventRecipeStepResolver{st: steps[i]}
	}
	return out, nil
}

func (r *eventRecipeResolver) Recipe(ctx context.Context) (*recipeResolver, error) {
	if r.er.RecipeID == nil {
		return nil, nil
	}
	if r.rc != nil {
		rec, ok := r.rc.recipes[*r.er.RecipeID]
		if !ok {
			return nil, nil
		}
		return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: rec, rc: r.rc}, nil
	}
	rec, err := r.rec.GetRecipeByID(ctx, *r.er.RecipeID)
	if err != nil {
		return nil, err
	}
	return &recipeResolver{inv: r.inv, rec: r.rec, up: r.up, user: r.user, recipe: rec}, nil
}

// eventStepsByEventRecipe batch-loads snapshot steps for every slot of
// the given events and groups them by event_recipe_id.
func (r *Resolver) eventStepsByEventRecipe(ctx context.Context, foodEventIDs []int64, householdID int64) (map[int64][]event.EventRecipeStep, error) {
	stepsBy := make(map[int64][]event.EventRecipeStep)
	if len(foodEventIDs) == 0 {
		return stepsBy, nil
	}
	steps, err := r.EventService.ListEventRecipeStepsForEvents(ctx, foodEventIDs, householdID)
	if err != nil {
		return nil, err
	}
	for _, st := range steps {
		stepsBy[st.EventRecipeID] = append(stepsBy[st.EventRecipeID], st)
	}
	return stepsBy, nil
}

// foodEventPageResolver resolves FoodEventPage.
type foodEventPageResolver struct {
	ev             EventService
	rec            RecipeService
	up             UserPrefsService
	inv            ItemReader
	user           currentuser.User
	events         []event.FoodEvent
	recipesByEvent map[int64][]event.EventRecipe
	stepsBy        map[int64][]event.EventRecipeStep
	page           int32
	pageSize       int32
	total          int32
}

func (r *foodEventPageResolver) Items() []*foodEventResolver {
	out := make([]*foodEventResolver, len(r.events))
	for i := range r.events {
		out[i] = &foodEventResolver{ev: r.ev, rec: r.rec, up: r.up, inv: r.inv, user: r.user, event: r.events[i], recipes: r.recipesByEvent[r.events[i].FoodEventID], stepsBy: r.stepsBy, loaded: true}
	}
	return out
}

func (r *foodEventPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type createFoodEventInput struct {
	Name                   string
	EventDate              string
	SlotGranularityMinutes *int32
}

type updateFoodEventInput struct {
	Name                   *string
	EventDate              *string
	SlotGranularityMinutes *int32
	IsActive               *bool
}

type addEventRecipeInput struct {
	FoodEventID graphql.ID
	RecipeID    *graphql.ID
	MealType    string
	TargetTime  graphql.Time
	Servings    *int32
	Notes       *string
}

type updateEventRecipeInput struct {
	RecipeID   *graphql.ID
	MealType   *string
	TargetTime *graphql.Time
	Servings   *int32
	Notes      *string
}

// eventTimelineResolver resolves the computed EventTimeline.
type eventTimelineResolver struct {
	tl event.Timeline
}

func (r *eventTimelineResolver) FoodEventID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.tl.FoodEventID, 10))
}

func (r *eventTimelineResolver) Warnings() []string { return r.tl.Warnings }

func (r *eventTimelineResolver) Recipes() []*eventTimelineRecipeResolver {
	out := make([]*eventTimelineRecipeResolver, len(r.tl.Recipes))
	for i := range r.tl.Recipes {
		out[i] = &eventTimelineRecipeResolver{rt: r.tl.Recipes[i]}
	}
	return out
}

// eventTimelineRecipeResolver resolves EventTimelineRecipe.
type eventTimelineRecipeResolver struct {
	rt event.RecipeTimeline
}

func (r *eventTimelineRecipeResolver) EventRecipeID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.rt.EventRecipeID, 10))
}

func (r *eventTimelineRecipeResolver) Name() string { return r.rt.Name }

func (r *eventTimelineRecipeResolver) TargetTime() graphql.Time {
	return graphql.Time{Time: r.rt.TargetTime}
}

func (r *eventTimelineRecipeResolver) Servings() *int32 { return r.rt.Servings }

func (r *eventTimelineRecipeResolver) StartBy() *graphql.Time {
	if r.rt.StartBy == nil {
		return nil
	}
	return &graphql.Time{Time: *r.rt.StartBy}
}

func (r *eventTimelineRecipeResolver) Unschedulable() bool { return r.rt.Unschedulable }

func (r *eventTimelineRecipeResolver) Warnings() []string { return r.rt.Warnings }

func (r *eventTimelineRecipeResolver) Steps() []*timelineStepResolver {
	out := make([]*timelineStepResolver, len(r.rt.Steps))
	for i := range r.rt.Steps {
		out[i] = &timelineStepResolver{st: r.rt.Steps[i]}
	}
	return out
}

// timelineStepResolver resolves TimelineStep.
type timelineStepResolver struct {
	st event.ScheduledStep
}

func (r *timelineStepResolver) StepNumber() int32 { return r.st.StepNumber }

func (r *timelineStepResolver) Instruction() string { return r.st.Instruction }

func (r *timelineStepResolver) StepType() *string { return nilIfEmpty(r.st.StepType) }

func (r *timelineStepResolver) IsPassive() bool { return r.st.IsPassive }

func (r *timelineStepResolver) Appliance() *string { return nilIfEmpty(r.st.Appliance) }

func (r *timelineStepResolver) DurationMinutes() *int32 { return r.st.DurationMinute }

func (r *timelineStepResolver) ScheduledMinutes() int32 { return r.st.ScheduledMinutes }

func (r *timelineStepResolver) Estimated() bool { return r.st.Estimated }

func (r *timelineStepResolver) StartTime() graphql.Time {
	return graphql.Time{Time: r.st.Start}
}

func (r *timelineStepResolver) EndTime() graphql.Time {
	return graphql.Time{Time: r.st.End}
}

func (r *timelineStepResolver) Conflicts() []string { return r.st.Conflicts }

// eventRecipeStepResolver resolves EventRecipeStep.
type eventRecipeStepResolver struct {
	st event.EventRecipeStep
}

func (r *eventRecipeStepResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.st.EventRecipeStepID, 10))
}

func (r *eventRecipeStepResolver) StepNumber() int32 { return r.st.StepNumber }

func (r *eventRecipeStepResolver) Instruction() string { return r.st.Instruction }

func (r *eventRecipeStepResolver) DurationMinutes() *int32 { return r.st.DurationMinutes }

func (r *eventRecipeStepResolver) StepType() *string { return nilIfEmpty(r.st.StepType) }

func (r *eventRecipeStepResolver) IsPassive() bool { return r.st.IsPassive }

func (r *eventRecipeStepResolver) DependsOnStepNumber() *int32 { return r.st.DependsOnStepNumber }

func (r *eventRecipeStepResolver) Appliance() *string { return nilIfEmpty(r.st.Appliance) }

type eventRecipeStepInput struct {
	Instruction         string
	DurationMinutes     *int32
	StepType            *string
	IsPassive           *bool
	DependsOnStepNumber *int32
	Appliance           *string
}
