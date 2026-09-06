package bff

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

var errMealPlanBoom = errors.New("mealplan boom")

const (
	mealPlanUserID = int64(7)
	mealPlanEmail  = "mealplan@example.com"
)

var mealPlanDate = time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)

func mealPlanCtx() context.Context {
	return testenv.WithUser(context.Background(), mealPlanUserID, mealPlanEmail)
}

func mealPlanPtrInt64(v int64) *int64 { return &v }
func mealPlanPtrInt32(v int32) *int32 { return &v }
func mealPlanPtrBool(v bool) *bool    { return &v }

func TestResolver_MealPlan_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)
	rec := mock.NewMockRecipeService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{MealPlanService: mp, InventoryService: inv, RecipeService: rec, UserPrefsService: up}

	mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{
		MealPlanID: 10, UserID: mealPlanUserID, Name: "Week 1", WeekStartDate: mealPlanDate,
		WeekStartDayOfWeek: 1, IsActive: true,
	}, nil)

	res, err := r.MealPlan(mealPlanCtx(), struct{ ID graphql.ID }{ID: "10"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("10"), res.ID())
	assert.Equal(t, "Week 1", res.Name())
	assert.Equal(t, "2025-06-02", res.WeekStartDate())
	assert.True(t, res.IsActive())

	recipeID := int64(20)
	servings := int32(2)
	mp.EXPECT().ListMealSlotsForPlan(gomock.Any(), int64(10)).Return([]mealplan.MealSlot{
		{SlotID: 100, MealPlanID: 10, DayOfWeek: 1, MealType: "dinner", RecipeID: &recipeID, Servings: &servings, ReplacementNote: "note"},
	}, nil)

	slots, err := res.Slots(mealPlanCtx())
	require.NoError(t, err)
	require.Len(t, slots, 1)
	slot := slots[0]
	assert.Equal(t, graphql.ID("100"), slot.ID())
	assert.Equal(t, int32(1), slot.DayOfWeek())
	assert.Equal(t, "dinner", slot.MealType())
	assert.Equal(t, int32(2), *slot.Servings())
	assert.Equal(t, "note", *slot.ReplacementNote())

	mp.EXPECT().ListMealSlotItems(gomock.Any(), int64(100)).Return([]mealplan.MealSlotItem{
		{SlotItemID: 1000, SlotID: 100, ItemID: mealPlanPtrInt64(50), Quantity: 1.5, UnitID: 3, IsFromRecipe: true},
	}, nil)
	inv.EXPECT().GetItemByID(gomock.Any(), int64(50)).Return(inventory.Item{ItemID: 50, Name: "Flour", CategoryID: 1, UnitID: 3}, nil)
	inv.EXPECT().GetUnitByID(gomock.Any(), int64(3)).Return(inventory.Unit{UnitID: 3, Name: "cup"}, nil)

	items, err := slot.Items(mealPlanCtx())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, graphql.ID("1000"), items[0].ID())
	assert.Equal(t, 1.5, items[0].Quantity())
	unit, err := items[0].Unit(mealPlanCtx())
	require.NoError(t, err)
	assert.Equal(t, "cup", unit)
	assert.True(t, items[0].IsFromRecipe())

	itemRes, err := items[0].Item(mealPlanCtx())
	require.NoError(t, err)
	require.NotNil(t, itemRes)
	assert.Equal(t, graphql.ID("50"), itemRes.ID())
	assert.Equal(t, "Flour", itemRes.Name())

	rec.EXPECT().GetRecipeByID(gomock.Any(), int64(20)).Return(recipe.Recipe{RecipeID: 20, Name: "Pasta", Servings: mealPlanPtrInt32(4)}, nil)

	recipeRes, err := slot.Recipe(mealPlanCtx())
	require.NoError(t, err)
	require.NotNil(t, recipeRes)
	assert.Equal(t, graphql.ID("20"), recipeRes.ID())
	assert.Equal(t, "Pasta", recipeRes.Name())
}

func TestResolver_MealPlans_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)
	rec := mock.NewMockRecipeService(ctrl)
	up := mock.NewMockUserPrefsService(ctrl)
	r := &Resolver{MealPlanService: mp, InventoryService: inv, RecipeService: rec, UserPrefsService: up}

	mp.EXPECT().ListMealPlans(gomock.Any(), mealPlanUserID, int32(10), int32(10)).Return([]mealplan.MealPlan{
		{MealPlanID: 10, UserID: mealPlanUserID, Name: "Week 1", WeekStartDate: mealPlanDate, WeekStartDayOfWeek: 1, IsActive: true},
		{MealPlanID: 11, UserID: mealPlanUserID, Name: "Week 2", WeekStartDate: mealPlanDate, WeekStartDayOfWeek: 1, IsActive: true},
	}, nil)
	mp.EXPECT().CountMealPlans(gomock.Any(), mealPlanUserID).Return(int64(5), nil)
	mp.EXPECT().ListMealSlotsByPlans(gomock.Any(), []int64{10, 11}).Return(nil, nil)
	mp.EXPECT().ListMealSlotItemsByPlans(gomock.Any(), []int64{10, 11}).Return(nil, nil)

	res, err := r.MealPlans(mealPlanCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 2, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, res)

	items := res.Items()
	require.Len(t, items, 2)
	assert.Equal(t, graphql.ID("10"), items[0].ID())
	assert.Equal(t, graphql.ID("11"), items[1].ID())

	pi := res.PageInfo()
	assert.Equal(t, int32(2), pi.PageNumber())
	assert.Equal(t, int32(10), pi.PageSize())
	assert.Equal(t, int32(5), pi.TotalCount())
}

func TestResolver_MealPlan_Nutrition_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)
	rec := mock.NewMockRecipeService(ctrl)
	r := &Resolver{MealPlanService: mp, InventoryService: inv, RecipeService: rec}

	mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{
		MealPlanID: 10, UserID: mealPlanUserID, Name: "Week 1", WeekStartDate: mealPlanDate,
		WeekStartDayOfWeek: 1, IsActive: true,
	}, nil)

	mp.EXPECT().ListMealSlotsForPlan(gomock.Any(), int64(10)).Return([]mealplan.MealSlot{
		{SlotID: 100, MealPlanID: 10, DayOfWeek: 1, MealType: "dinner", RecipeID: mealPlanPtrInt64(20), Servings: mealPlanPtrInt32(2)},
	}, nil)

	mp.EXPECT().ListMealSlotItemsByPlan(gomock.Any(), int64(10)).Return([]mealplan.MealSlotItem{
		{SlotItemID: 1000, SlotID: 100, ItemID: mealPlanPtrInt64(50), Quantity: 1, UnitID: 3, IsFromRecipe: true},
	}, nil)

	rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{20}).Return([]recipe.Recipe{
		{RecipeID: 20, Name: "Pasta", Servings: mealPlanPtrInt32(4)},
	}, nil)
	rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{20}).Return([]recipe.RecipeItem{
		{RecipeID: 20, ItemID: 50, Quantity: 0.5, UnitID: 3},
		{RecipeID: 20, ItemID: 60, Quantity: 1, UnitID: 12},
	}, nil)

	inv.EXPECT().ListFoodNutrientsByItems(gomock.Any(), gomock.Any()).Return([]inventory.FoodNutrient{
		{ItemID: 50, NutrientID: 1, Name: "Protein", Unit: "g", Amount: 5},
		{ItemID: 50, NutrientID: 2, Name: "Carbs", Unit: "g", Amount: 10},
		{ItemID: 60, NutrientID: 2, Name: "Carbs", Unit: "g", Amount: 3},
	}, nil)

	res, err := r.Nutrition(mealPlanCtx(), struct{ MealPlanID graphql.ID }{MealPlanID: "10"})
	require.NoError(t, err)
	require.Len(t, res, 2)

	assert.Equal(t, "Carbs", res[0].Name())
	assert.Equal(t, "g", res[0].Unit())
	assert.InDelta(t, 11.5, res[0].Amount(), 0.0001)

	assert.Equal(t, "Protein", res[1].Name())
	assert.Equal(t, "g", res[1].Unit())
	assert.InDelta(t, 5.0, res[1].Amount(), 0.0001)
}

func TestResolver_MealPlan_CreateMealPlan_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	r := &Resolver{MealPlanService: mp}

	mp.EXPECT().CreateMealPlan(gomock.Any(), gomock.Eq(mealplan.MealPlan{
		UserID: mealPlanUserID, Name: "Week 1", WeekStartDate: mealPlanDate,
		WeekStartDayOfWeek: 1, IsActive: true,
	}), mealPlanEmail).Return(mealplan.MealPlan{
		MealPlanID: 10, UserID: mealPlanUserID, Name: "Week 1", WeekStartDate: mealPlanDate,
		WeekStartDayOfWeek: 1, IsActive: true,
	}, nil)

	res, err := r.CreateMealPlan(mealPlanCtx(), struct{ Input createMealPlanInput }{
		Input: createMealPlanInput{Name: "Week 1", WeekStartDate: "2025-06-02"},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("10"), res.ID())
	assert.Equal(t, "Week 1", res.Name())
}

func TestResolver_MealPlan_UpdateMealPlan_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	r := &Resolver{MealPlanService: mp}

	gomock.InOrder(
		mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{
			MealPlanID: 10, UserID: mealPlanUserID, Name: "Old", WeekStartDate: mealPlanDate,
			WeekStartDayOfWeek: 1, IsActive: true,
		}, nil),
		mp.EXPECT().UpdateMealPlan(gomock.Any(), int64(10), mealPlanUserID, gomock.Eq(mealplan.MealPlan{
			Name: "New", WeekStartDate: mealPlanDate, WeekStartDayOfWeek: 2, IsActive: true,
		}), mealPlanEmail).Return(nil),
		mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{
			MealPlanID: 10, UserID: mealPlanUserID, Name: "New", WeekStartDate: mealPlanDate,
			WeekStartDayOfWeek: 2, IsActive: true,
		}, nil),
	)

	day := int32(2)
	res, err := r.UpdateMealPlan(mealPlanCtx(), struct {
		ID    graphql.ID
		Input createMealPlanInput
	}{
		ID:    "10",
		Input: createMealPlanInput{Name: "New", WeekStartDate: "2025-06-02", WeekStartDayOfWeek: &day},
	})
	require.NoError(t, err)
	assert.Equal(t, "New", res.Name())
}

func TestResolver_MealPlan_DeleteMealPlan_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	r := &Resolver{MealPlanService: mp}

	mp.EXPECT().DeleteMealPlan(gomock.Any(), int64(10), mealPlanUserID).Return(nil)

	ok, err := r.DeleteMealPlan(mealPlanCtx(), struct{ ID graphql.ID }{ID: "10"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_MealPlan_AddMealSlot_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	rec := mock.NewMockRecipeService(ctrl)
	r := &Resolver{MealPlanService: mp, RecipeService: rec}

	recipeID := graphql.ID("20")
	servings := int32(2)
	note := "note"

	mp.EXPECT().AddMealSlot(gomock.Any(), gomock.Any(), mealPlanEmail).Return(mealplan.MealSlot{
		SlotID: 100, MealPlanID: 10, DayOfWeek: 1, MealType: "dinner",
		RecipeID: mealPlanPtrInt64(20), Servings: mealPlanPtrInt32(2), ReplacementNote: "note",
	}, nil)

	res, err := r.AddMealSlot(mealPlanCtx(), struct{ Input addMealSlotInput }{
		Input: addMealSlotInput{
			MealPlanID:      "10",
			DayOfWeek:       1,
			MealType:        "dinner",
			RecipeID:        &recipeID,
			Servings:        &servings,
			ReplacementNote: &note,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("100"), res.ID())
	assert.Equal(t, int32(1), res.DayOfWeek())

	rec.EXPECT().GetRecipeByID(gomock.Any(), int64(20)).Return(recipe.Recipe{RecipeID: 20, Name: "Pasta", Servings: mealPlanPtrInt32(4)}, nil)

	recipeRes, err := res.Recipe(mealPlanCtx())
	require.NoError(t, err)
	require.NotNil(t, recipeRes)
	assert.Equal(t, graphql.ID("20"), recipeRes.ID())
}

func TestResolver_MealPlan_RemoveMealSlot_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	r := &Resolver{MealPlanService: mp}

	mp.EXPECT().DeleteMealSlot(gomock.Any(), int64(100)).Return(nil)

	ok, err := r.RemoveMealSlot(mealPlanCtx(), struct{ SlotID graphql.ID }{SlotID: "100"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_MealPlan_AddMealSlotItem_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)
	r := &Resolver{MealPlanService: mp, InventoryService: inv}

	inv.EXPECT().GetUnitByName(gomock.Any(), "cup").Return(inventory.Unit{UnitID: 3, Name: "cup"}, nil)
	mp.EXPECT().AddMealSlotItem(gomock.Any(), gomock.Eq(mealplan.MealSlotItem{
		SlotID: 100, ItemID: mealPlanPtrInt64(50), Quantity: 1.5, UnitID: 3, IsFromRecipe: true,
	}), mealPlanEmail).Return(mealplan.MealSlotItem{
		SlotItemID: 1000, SlotID: 100, ItemID: mealPlanPtrInt64(50), Quantity: 1.5, UnitID: 3, IsFromRecipe: true,
	}, nil)

	res, err := r.AddMealSlotItem(mealPlanCtx(), struct{ Input addMealSlotItemInput }{
		Input: addMealSlotItemInput{
			SlotID:       "100",
			ItemID:       "50",
			Quantity:     1.5,
			Unit:         "cup",
			IsFromRecipe: mealPlanPtrBool(true),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("1000"), res.ID())

	inv.EXPECT().GetItemByID(gomock.Any(), int64(50)).Return(inventory.Item{ItemID: 50, Name: "Flour", CategoryID: 1, UnitID: 3}, nil)

	itemRes, err := res.Item(mealPlanCtx())
	require.NoError(t, err)
	require.NotNil(t, itemRes)
	assert.Equal(t, graphql.ID("50"), itemRes.ID())
	assert.Equal(t, "Flour", itemRes.Name())
}

func TestResolver_MealPlan_RemoveMealSlotItem_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mp := mock.NewMockMealPlanService(ctrl)
	r := &Resolver{MealPlanService: mp}

	mp.EXPECT().DeleteMealSlotItem(gomock.Any(), int64(1000)).Return(nil)

	ok, err := r.RemoveMealSlotItem(mealPlanCtx(), struct{ SlotItemID graphql.ID }{SlotItemID: "1000"})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestResolver_MealPlan_Unauthorized(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context) (any, error)
	}{
		{"MealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).MealPlan(ctx, struct{ ID graphql.ID }{ID: "10"})
		}},
		{"MealPlans", func(ctx context.Context) (any, error) {
			return (&Resolver{}).MealPlans(ctx, struct{ Page, PageSize int32 }{Page: 1, PageSize: 10})
		}},
		{"CreateMealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateMealPlan(ctx, struct{ Input createMealPlanInput }{
				Input: createMealPlanInput{Name: "Week 1", WeekStartDate: "2025-06-02"},
			})
		}},
		{"UpdateMealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateMealPlan(ctx, struct {
				ID    graphql.ID
				Input createMealPlanInput
			}{ID: "10", Input: createMealPlanInput{Name: "Week 1", WeekStartDate: "2025-06-02"}})
		}},
		{"DeleteMealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteMealPlan(ctx, struct{ ID graphql.ID }{ID: "10"})
		}},
		{"AddMealSlot", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddMealSlot(ctx, struct{ Input addMealSlotInput }{
				Input: addMealSlotInput{MealPlanID: "10", DayOfWeek: 1, MealType: "dinner"},
			})
		}},
		{"RemoveMealSlot", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveMealSlot(ctx, struct{ SlotID graphql.ID }{SlotID: "100"})
		}},
		{"AddMealSlotItem", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddMealSlotItem(ctx, struct{ Input addMealSlotItemInput }{
				Input: addMealSlotItemInput{SlotID: "100", ItemID: "50", Quantity: 1, Unit: "cup"},
			})
		}},
		{"RemoveMealSlotItem", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveMealSlotItem(ctx, struct{ SlotItemID graphql.ID }{SlotItemID: "1000"})
		}},
		{"Nutrition", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Nutrition(ctx, struct{ MealPlanID graphql.ID }{MealPlanID: "10"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.call(context.Background())
			if b, ok := res.(bool); ok {
				assert.False(t, b)
			} else {
				assert.Nil(t, res)
			}
			assert.EqualError(t, err, "unauthorized")
		})
	}
}

func TestResolver_MealPlan_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context) (any, error)
	}{
		{"MealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).MealPlan(mealPlanCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"DeleteMealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteMealPlan(mealPlanCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"UpdateMealPlan", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateMealPlan(mealPlanCtx(), struct {
				ID    graphql.ID
				Input createMealPlanInput
			}{ID: "abc", Input: createMealPlanInput{Name: "Week 1", WeekStartDate: "2025-06-02"}})
		}},
		{"AddMealSlot_MealPlanID", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddMealSlot(mealPlanCtx(), struct{ Input addMealSlotInput }{
				Input: addMealSlotInput{MealPlanID: "abc", DayOfWeek: 1, MealType: "dinner"},
			})
		}},
		{"AddMealSlot_RecipeID", func(ctx context.Context) (any, error) {
			recipeID := graphql.ID("abc")
			return (&Resolver{}).AddMealSlot(mealPlanCtx(), struct{ Input addMealSlotInput }{
				Input: addMealSlotInput{MealPlanID: "10", DayOfWeek: 1, MealType: "dinner", RecipeID: &recipeID},
			})
		}},
		{"RemoveMealSlot", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveMealSlot(mealPlanCtx(), struct{ SlotID graphql.ID }{SlotID: "abc"})
		}},
		{"AddMealSlotItem_SlotID", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddMealSlotItem(mealPlanCtx(), struct{ Input addMealSlotItemInput }{
				Input: addMealSlotItemInput{SlotID: "abc", ItemID: "50", Quantity: 1, Unit: "cup"},
			})
		}},
		{"AddMealSlotItem_ItemID", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddMealSlotItem(mealPlanCtx(), struct{ Input addMealSlotItemInput }{
				Input: addMealSlotItemInput{SlotID: "100", ItemID: "abc", Quantity: 1, Unit: "cup"},
			})
		}},
		{"RemoveMealSlotItem", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveMealSlotItem(mealPlanCtx(), struct{ SlotItemID graphql.ID }{SlotItemID: "abc"})
		}},
		{"Nutrition", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Nutrition(mealPlanCtx(), struct{ MealPlanID graphql.ID }{MealPlanID: "abc"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call(mealPlanCtx())
			assert.Error(t, err)
		})
	}
}

func TestResolver_MealPlan_ServiceError(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*mock.MockMealPlanService)
		call     func(*Resolver, context.Context) (any, error)
		wantBool bool
	}{
		{
			name: "MealPlan",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{}, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.MealPlan(ctx, struct{ ID graphql.ID }{ID: "10"})
			},
		},
		{
			name: "MealPlans",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().ListMealPlans(gomock.Any(), mealPlanUserID, int32(10), int32(0)).Return(nil, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.MealPlans(ctx, struct{ Page, PageSize int32 }{Page: 1, PageSize: 10})
			},
		},
		{
			name: "CreateMealPlan",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().CreateMealPlan(gomock.Any(), gomock.Any(), mealPlanEmail).Return(mealplan.MealPlan{}, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.CreateMealPlan(ctx, struct{ Input createMealPlanInput }{
					Input: createMealPlanInput{Name: "Week 1", WeekStartDate: "2025-06-02"},
				})
			},
		},
		{
			name: "UpdateMealPlan",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{}, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.UpdateMealPlan(ctx, struct {
					ID    graphql.ID
					Input createMealPlanInput
				}{ID: "10", Input: createMealPlanInput{Name: "Week 1", WeekStartDate: "2025-06-02"}})
			},
		},
		{
			name: "DeleteMealPlan",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().DeleteMealPlan(gomock.Any(), int64(10), mealPlanUserID).Return(errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.DeleteMealPlan(ctx, struct{ ID graphql.ID }{ID: "10"})
			},
			wantBool: true,
		},
		{
			name: "AddMealSlot",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().AddMealSlot(gomock.Any(), gomock.Any(), mealPlanEmail).Return(mealplan.MealSlot{}, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.AddMealSlot(ctx, struct{ Input addMealSlotInput }{
					Input: addMealSlotInput{MealPlanID: "10", DayOfWeek: 1, MealType: "dinner"},
				})
			},
		},
		{
			name: "RemoveMealSlot",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().DeleteMealSlot(gomock.Any(), int64(100)).Return(errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.RemoveMealSlot(ctx, struct{ SlotID graphql.ID }{SlotID: "100"})
			},
			wantBool: true,
		},
		{
			name: "AddMealSlotItem",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().AddMealSlotItem(gomock.Any(), gomock.Any(), mealPlanEmail).Return(mealplan.MealSlotItem{}, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.AddMealSlotItem(ctx, struct{ Input addMealSlotItemInput }{
					Input: addMealSlotItemInput{SlotID: "100", ItemID: "50", Quantity: 1, Unit: "cup"},
				})
			},
		},
		{
			name: "RemoveMealSlotItem",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().DeleteMealSlotItem(gomock.Any(), int64(1000)).Return(errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.RemoveMealSlotItem(ctx, struct{ SlotItemID graphql.ID }{SlotItemID: "1000"})
			},
			wantBool: true,
		},
		{
			name: "Nutrition_GetMealPlanByID",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{}, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.Nutrition(ctx, struct{ MealPlanID graphql.ID }{MealPlanID: "10"})
			},
		},
		{
			name: "Nutrition_ListMealSlots",
			setup: func(mp *mock.MockMealPlanService) {
				mp.EXPECT().GetMealPlanByID(gomock.Any(), int64(10), mealPlanUserID).Return(mealplan.MealPlan{MealPlanID: 10}, nil)
				mp.EXPECT().ListMealSlotsForPlan(gomock.Any(), int64(10)).Return(nil, errMealPlanBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.Nutrition(ctx, struct{ MealPlanID graphql.ID }{MealPlanID: "10"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mp := mock.NewMockMealPlanService(ctrl)
			inv := mock.NewMockInventoryService(ctrl)
			rec := mock.NewMockRecipeService(ctrl)
			r := &Resolver{MealPlanService: mp, InventoryService: inv, RecipeService: rec}
			// AddMealSlotItem resolves the unit name before calling the
			// service; permit that lookup for any case that reaches it.
			inv.EXPECT().GetUnitByName(gomock.Any(), gomock.Any()).
				Return(inventory.Unit{UnitID: 1, Name: "each"}, nil).AnyTimes()
			tt.setup(mp)

			res, err := tt.call(r, mealPlanCtx())
			if tt.wantBool {
				assert.False(t, res.(bool))
			} else {
				assert.Nil(t, res)
			}
			assert.ErrorIs(t, err, errMealPlanBoom)
		})
	}
}
