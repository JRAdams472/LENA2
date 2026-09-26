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
	return &foodEventResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, event: ev, recipes: recipes, rc: rc, loaded: true}, nil
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
	return &foodEventPageResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, events: events, recipesByEvent: recipesByEvent, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
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
	return &foodEventResolver{ev: r.EventService, rec: r.RecipeService, up: r.UserPrefsService, inv: r.InventoryService, user: u, event: updated, recipes: recipes, rc: rc, loaded: true}, nil
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
	if args.Input.RecipeID != nil {
		rid, err := parseID(string(*args.Input.RecipeID))
		if err != nil {
			return nil, err
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
		out[i] = &eventRecipeResolver{ev: r.ev, rec: r.rec, up: r.up, inv: r.inv, user: r.user, er: recipes[i], rc: r.rc}
	}
	return out, nil
}

// eventRecipeResolver resolves EventRecipe fields.
type eventRecipeResolver struct {
	ev   EventService
	rec  RecipeService
	up   UserPrefsService
	inv  ItemReader
	user currentuser.User
	er   event.EventRecipe
	rc   *recipeChildren
}

func (r *eventRecipeResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.er.EventRecipeID, 10))
}

func (r *eventRecipeResolver) MealType() string { return r.er.MealType }

func (r *eventRecipeResolver) TargetTime() graphql.Time { return graphql.Time{Time: r.er.TargetTime} }

func (r *eventRecipeResolver) Servings() *int32 { return r.er.Servings }

func (r *eventRecipeResolver) Notes() *string { return nilIfEmpty(r.er.Notes) }

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

// foodEventPageResolver resolves FoodEventPage.
type foodEventPageResolver struct {
	ev             EventService
	rec            RecipeService
	up             UserPrefsService
	inv            ItemReader
	user           currentuser.User
	events         []event.FoodEvent
	recipesByEvent map[int64][]event.EventRecipe
	page           int32
	pageSize       int32
	total          int32
}

func (r *foodEventPageResolver) Items() []*foodEventResolver {
	out := make([]*foodEventResolver, len(r.events))
	for i := range r.events {
		out[i] = &foodEventResolver{ev: r.ev, rec: r.rec, up: r.up, inv: r.inv, user: r.user, event: r.events[i], recipes: r.recipesByEvent[r.events[i].FoodEventID], loaded: true}
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
