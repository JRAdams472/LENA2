package bff

import (
	"context"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func evCtx() context.Context {
	return testutil.WithUser(context.Background(), 7, "ev-caller@example.com")
}

var (
	evDay    = time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	evTarget = time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)
)

func evRow(id int64) event.FoodEvent {
	return event.FoodEvent{
		FoodEventID:            id,
		HouseholdID:            7,
		Name:                   "Halloween Party",
		EventDate:              evDay,
		SlotGranularityMinutes: 15,
		IsActive:               true,
	}
}

func TestResolver_FoodEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	rec := mock.NewMockRecipeService(ctrl)
	r := &Resolver{EventService: ev, RecipeService: rec}

	recipeID := int64(11)
	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(evRow(3), nil)
	ev.EXPECT().ListEventRecipesForEvent(gomock.Any(), int64(3), int64(7)).Return([]event.EventRecipe{
		{EventRecipeID: 9, FoodEventID: 3, RecipeID: &recipeID, MealType: "dinner", TargetTime: evTarget},
	}, nil)
	rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{11}).Return([]recipe.Recipe{{RecipeID: 11, Name: "Casserole"}}, nil)
	rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{11}).Return([]recipe.RecipeItem{}, nil)
	rec.EXPECT().ListRecipeStepsByRecipes(gomock.Any(), []int64{11}).Return([]recipe.RecipeStep{}, nil)
	rec.EXPECT().ListRecipeRatings(gomock.Any(), int64(7), []int64{11}).Return([]recipe.RecipeRating{}, nil)
	rec.EXPECT().ListRatingSummaries(gomock.Any(), []int64{11}).Return([]recipe.RatingSummary{}, nil)

	res, err := r.FoodEvent(evCtx(), struct{ ID graphql.ID }{ID: "3"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("3"), res.ID())
	assert.Equal(t, "Halloween Party", res.Name())
	assert.Equal(t, "2026-10-31", res.EventDate())
	assert.Equal(t, int32(15), res.SlotGranularityMinutes())
	assert.True(t, res.IsActive())

	recipes, err := res.Recipes(evCtx())
	require.NoError(t, err)
	require.Len(t, recipes, 1)
	got, err := recipes[0].Recipe(evCtx())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Casserole", got.Name())
}

func TestResolver_FoodEvent_WrongHousehold(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	r := &Resolver{EventService: ev}

	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).
		Return(event.FoodEvent{}, domainerr.ErrNotFound)

	res, err := r.FoodEvent(evCtx(), struct{ ID graphql.ID }{ID: "3"})
	assert.ErrorIs(t, err, domainerr.ErrNotFound)
	assert.Nil(t, res)
}

func TestResolver_FoodEvents_Page(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	r := &Resolver{EventService: ev}

	ev.EXPECT().ListFoodEvents(gomock.Any(), int64(7), int32(10), int32(10)).
		Return([]event.FoodEvent{evRow(3), evRow(4)}, nil)
	ev.EXPECT().CountFoodEvents(gomock.Any(), int64(7)).Return(int64(25), nil)
	ev.EXPECT().ListEventRecipesByEvents(gomock.Any(), gomock.InAnyOrder([]int64{3, 4}), int64(7)).
		Return([]event.EventRecipe{
			{EventRecipeID: 9, FoodEventID: 3, MealType: "dinner", TargetTime: evTarget},
			{EventRecipeID: 10, FoodEventID: 3, MealType: "dessert", TargetTime: evTarget},
		}, nil)

	res, err := r.FoodEvents(evCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 2, PageSize: 10})
	require.NoError(t, err)
	items := res.Items()
	require.Len(t, items, 2)
	recipes, err := items[0].Recipes(evCtx())
	require.NoError(t, err)
	assert.Len(t, recipes, 2)
	recipes, err = items[1].Recipes(evCtx())
	require.NoError(t, err)
	assert.Empty(t, recipes)
	pi := res.PageInfo()
	assert.Equal(t, int32(25), pi.TotalCount())
	assert.Equal(t, int32(2), pi.PageNumber())
}

func TestResolver_CreateFoodEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{EventService: ev, HouseholdService: h, IdentityService: idSvc}

	ev.EXPECT().CreateFoodEvent(gomock.Any(), event.FoodEvent{
		HouseholdID:            7,
		Name:                   "Halloween Party",
		EventDate:              evDay,
		SlotGranularityMinutes: 15,
		IsActive:               true,
	}, "ev-caller@example.com").Return(evRow(3), nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{
		{UserID: 7}, {UserID: 9}, {UserID: 10},
	}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindEventCreated,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(10), household.KindEventCreated,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	res, err := r.CreateFoodEvent(evCtx(), struct{ Input createFoodEventInput }{
		Input: createFoodEventInput{Name: "Halloween Party", EventDate: "2026-10-31"},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("3"), res.ID())
}

func TestResolver_CreateFoodEvent_Validation(t *testing.T) {
	r := &Resolver{EventService: mock.NewMockEventService(gomock.NewController(t))}

	_, err := r.CreateFoodEvent(evCtx(), struct{ Input createFoodEventInput }{
		Input: createFoodEventInput{Name: "x", EventDate: "not-a-date"},
	})
	assert.ErrorContains(t, err, "invalid eventDate")

	_, err = r.CreateFoodEvent(evCtx(), struct{ Input createFoodEventInput }{
		Input: createFoodEventInput{Name: "x", EventDate: "2026-10-31", SlotGranularityMinutes: int32Ptr(20)},
	})
	assert.ErrorContains(t, err, "must be 15 or 30")

	_, err = r.CreateFoodEvent(evCtx(), struct{ Input createFoodEventInput }{
		Input: createFoodEventInput{Name: "", EventDate: "2026-10-31"},
	})
	assert.ErrorContains(t, err, "must not be empty")
}

func TestResolver_UpdateFoodEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{EventService: ev, HouseholdService: h, IdentityService: idSvc}

	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(evRow(3), nil)
	ev.EXPECT().UpdateFoodEvent(gomock.Any(), int64(3), int64(7), gomock.Any(), "ev-caller@example.com").Return(nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{{UserID: 7}, {UserID: 9}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindEventUpdated,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	updated := evRow(3)
	updated.Name = "Renamed"
	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(updated, nil)
	ev.EXPECT().ListEventRecipesForEvent(gomock.Any(), int64(3), int64(7)).Return([]event.EventRecipe{}, nil)

	res, err := r.UpdateFoodEvent(evCtx(), struct {
		ID    graphql.ID
		Input updateFoodEventInput
	}{ID: "3", Input: updateFoodEventInput{Name: strPtr("Renamed")}})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", res.Name())
	assert.Equal(t, int32(15), res.SlotGranularityMinutes())
}

func TestResolver_DeleteFoodEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{EventService: ev, HouseholdService: h, IdentityService: idSvc}

	ev.EXPECT().DeleteFoodEvent(gomock.Any(), int64(3), int64(7)).Return(nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{{UserID: 7}, {UserID: 9}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindEventDeleted,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	ok, err := r.DeleteFoodEvent(evCtx(), struct{ ID graphql.ID }{ID: "3"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_AddEventRecipe(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{EventService: ev, HouseholdService: h, IdentityService: idSvc}

	recipeID := graphql.ID("11")
	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(evRow(3), nil)
	ev.EXPECT().AddEventRecipe(gomock.Any(), event.EventRecipe{
		FoodEventID: 3,
		RecipeID:    int64Ptr(11),
		MealType:    "dinner",
		TargetTime:  evTarget,
	}, int64(7), "ev-caller@example.com").Return(event.EventRecipe{
		EventRecipeID: 9, FoodEventID: 3, MealType: "dinner", TargetTime: evTarget,
	}, nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{{UserID: 7}, {UserID: 9}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindEventUpdated,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	res, err := r.AddEventRecipe(evCtx(), struct{ Input addEventRecipeInput }{
		Input: addEventRecipeInput{
			FoodEventID: "3", RecipeID: &recipeID, MealType: "dinner",
			TargetTime: graphql.Time{Time: evTarget},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("9"), res.ID())
	assert.Equal(t, "dinner", res.MealType())
}

func TestResolver_AddEventRecipe_OffBoundary(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	r := &Resolver{EventService: ev}

	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(evRow(3), nil)

	_, err := r.AddEventRecipe(evCtx(), struct{ Input addEventRecipeInput }{
		Input: addEventRecipeInput{
			FoodEventID: "3", MealType: "dinner",
			TargetTime: graphql.Time{Time: time.Date(2026, 10, 31, 18, 7, 0, 0, time.UTC)},
		},
	})
	assert.ErrorContains(t, err, "15-minute boundary")
}

func TestResolver_AddEventRecipe_WrongDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	r := &Resolver{EventService: ev}

	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(evRow(3), nil)

	_, err := r.AddEventRecipe(evCtx(), struct{ Input addEventRecipeInput }{
		Input: addEventRecipeInput{
			FoodEventID: "3", MealType: "dinner",
			TargetTime: graphql.Time{Time: time.Date(2026, 11, 1, 18, 30, 0, 0, time.UTC)},
		},
	})
	assert.ErrorContains(t, err, "event date")
}

func TestResolver_UpdateEventRecipe(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{EventService: ev, HouseholdService: h, IdentityService: idSvc}

	existing := event.EventRecipe{
		EventRecipeID: 9, FoodEventID: 3, MealType: "dinner", TargetTime: evTarget,
	}
	ev.EXPECT().GetEventRecipeByID(gomock.Any(), int64(9), int64(7)).Return(existing, nil)
	newTarget := time.Date(2026, 10, 31, 19, 0, 0, 0, time.UTC)
	ev.EXPECT().GetFoodEventByID(gomock.Any(), int64(3), int64(7)).Return(evRow(3), nil)
	ev.EXPECT().UpdateEventRecipe(gomock.Any(), int64(9), int64(7), gomock.Any(), "ev-caller@example.com").Return(nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{{UserID: 7}, {UserID: 9}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindEventUpdated,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	updated := existing
	updated.TargetTime = newTarget
	ev.EXPECT().GetEventRecipeByID(gomock.Any(), int64(9), int64(7)).Return(updated, nil)

	res, err := r.UpdateEventRecipe(evCtx(), struct {
		ID    graphql.ID
		Input updateEventRecipeInput
	}{ID: "9", Input: updateEventRecipeInput{TargetTime: &graphql.Time{Time: newTarget}}})
	require.NoError(t, err)
	assert.Equal(t, graphql.Time{Time: newTarget}, res.TargetTime())
	assert.Equal(t, "dinner", res.MealType())
}

func TestResolver_RemoveEventRecipe(t *testing.T) {
	ctrl := gomock.NewController(t)
	ev := mock.NewMockEventService(ctrl)
	h := mock.NewMockHouseholdService(ctrl)
	idSvc := mock.NewMockIdentityService(ctrl)
	r := &Resolver{EventService: ev, HouseholdService: h, IdentityService: idSvc}

	ev.EXPECT().GetEventRecipeByID(gomock.Any(), int64(9), int64(7)).Return(event.EventRecipe{
		EventRecipeID: 9, FoodEventID: 3, MealType: "dinner", TargetTime: evTarget,
	}, nil)
	ev.EXPECT().DeleteEventRecipe(gomock.Any(), int64(9), int64(7)).Return(nil)
	idSvc.EXPECT().ListUsersByHousehold(gomock.Any(), int64(7)).Return([]identity.User{{UserID: 7}, {UserID: 9}}, nil)
	h.EXPECT().CreateNotification(gomock.Any(), int64(9), household.KindEventUpdated,
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	ok, err := r.RemoveEventRecipe(evCtx(), struct{ ID graphql.ID }{ID: "9"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func int64Ptr(v int64) *int64 { return &v }
func strPtr(v string) *string { return &v }
