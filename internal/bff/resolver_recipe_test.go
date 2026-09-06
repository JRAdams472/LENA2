package bff

import (
	"context"
	"errors"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

var errRecBoom = errors.New("boom")

const recTestEmail = "rec@example.com"

func recCtx() context.Context {
	return testenv.WithAdmin(context.Background(), 11, recTestEmail)
}

func recUserCtx() context.Context {
	return testenv.WithUser(context.Background(), 11, recTestEmail)
}

func recStrPtr(s string) *string { return &s }
func recInt32Ptr(v int32) *int32 { return &v }

func newRecMocks(t *testing.T) (*mock.MockRecipeService, *mock.MockInventoryService, *mock.MockUserPrefsService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return mock.NewMockRecipeService(ctrl), mock.NewMockInventoryService(ctrl), mock.NewMockUserPrefsService(ctrl)
}

func TestResolver_Recipe_Recipe(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		rec, inv, up := newRecMocks(t)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{
			RecipeID: 9, Name: "Soup", Description: "Tasty",
			Servings: recInt32Ptr(4), PrepTimeMinutes: recInt32Ptr(10), CookTimeMinutes: recInt32Ptr(20), IsActive: true,
		}, nil)
		rec.EXPECT().ListRecipeItems(gomock.Any(), int64(9)).Return([]recipe.RecipeItem{
			{RecipeItemID: 40, RecipeID: 9, ItemID: 3, Quantity: 2, UnitID: 3, Notes: "diced"},
		}, nil)
		rec.EXPECT().ListRecipeSteps(gomock.Any(), int64(9)).Return([]recipe.RecipeStep{
			{StepID: 7, RecipeID: 9, StepNumber: 1, Instruction: "Boil"},
		}, nil)
		inv.EXPECT().GetItemByID(gomock.Any(), int64(3)).Return(inventory.Item{ItemID: 3, Name: "Broth"}, nil)
		inv.EXPECT().GetUnitByID(gomock.Any(), int64(3)).Return(inventory.Unit{UnitID: 3, Name: "cup"}, nil)
		up.EXPECT().GetRecipeFavorite(gomock.Any(), int64(11), int64(9)).
			Return(userprefs.RecipeFavorite{UserID: 11, RecipeID: 9, IsFavorite: true}, nil)

		r := &Resolver{RecipeService: rec, InventoryService: inv, UserPrefsService: up}
		res, err := r.Recipe(recCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		ctx := recCtx()
		assert.Equal(t, graphql.ID("9"), res.ID())
		assert.Equal(t, "Soup", res.Name())
		require.NotNil(t, res.Description())
		assert.Equal(t, "Tasty", *res.Description())
		require.NotNil(t, res.Servings())
		assert.Equal(t, int32(4), *res.Servings())
		require.NotNil(t, res.PrepTimeMinutes())
		assert.Equal(t, int32(10), *res.PrepTimeMinutes())
		require.NotNil(t, res.CookTimeMinutes())
		assert.Equal(t, int32(20), *res.CookTimeMinutes())

		items, err := res.Items(ctx)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, graphql.ID("40"), items[0].ID())
		assert.Equal(t, 2.0, items[0].Quantity())
		unit, err := items[0].Unit(ctx)
		require.NoError(t, err)
		assert.Equal(t, "cup", unit)
		require.NotNil(t, items[0].Notes())
		assert.Equal(t, "diced", *items[0].Notes())
		assert.False(t, items[0].IsOptional())
		it, err := items[0].Item(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Broth", it.Name())

		steps, err := res.Steps(ctx)
		require.NoError(t, err)
		require.Len(t, steps, 1)
		assert.Equal(t, int32(1), steps[0].StepNumber())
		assert.Equal(t, "Boil", steps[0].Instruction())

		fav, err := res.IsFavorite(ctx)
		require.NoError(t, err)
		assert.True(t, fav)
	})

	t.Run("nullable fields", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).
			Return(recipe.Recipe{RecipeID: 9, Name: "Bare"}, nil)
		r := &Resolver{RecipeService: rec}
		res, err := r.Recipe(recCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		assert.Nil(t, res.Description())
		assert.Nil(t, res.Servings())
		assert.Nil(t, res.PrepTimeMinutes())
		assert.Nil(t, res.CookTimeMinutes())
	})

	t.Run("unauthorized", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.Recipe(context.Background(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.Recipe(recCtx(), struct{ ID graphql.ID }{ID: "abc"})
		require.Error(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{}, errRecBoom)
		r := &Resolver{RecipeService: rec}
		_, err := r.Recipe(recCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorIs(t, err, errRecBoom)
	})
}

func TestResolver_Recipe_Recipes(t *testing.T) {
	type pageArgs = struct {
		Page     int32
		PageSize int32
	}

	t.Run("happy path", func(t *testing.T) {
		rec, inv, up := newRecMocks(t)
		rec.EXPECT().ListRecipes(gomock.Any(), true, int32(10), int32(10)).Return([]recipe.Recipe{
			{RecipeID: 1, Name: "Soup"},
			{RecipeID: 2, Name: "Salad"},
		}, nil)
		rec.EXPECT().CountRecipes(gomock.Any(), true).Return(int64(5), nil)
		rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{1, 2}).Return([]recipe.Recipe{
			{RecipeID: 1, Name: "Soup"},
			{RecipeID: 2, Name: "Salad"},
		}, nil)
		rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{1, 2}).Return(nil, nil)
		rec.EXPECT().ListRecipeStepsByRecipes(gomock.Any(), []int64{1, 2}).Return(nil, nil)
		up.EXPECT().ListRecipeFavorites(gomock.Any(), int64(11), []int64{1, 2}).Return(nil, nil)
		r := &Resolver{RecipeService: rec, InventoryService: inv, UserPrefsService: up}
		res, err := r.Recipes(recCtx(), pageArgs{Page: 2, PageSize: 10})
		require.NoError(t, err)
		require.Len(t, res.Items(), 2)
		assert.Equal(t, "Soup", res.Items()[0].Name())
		pi := res.PageInfo()
		assert.Equal(t, int32(2), pi.PageNumber())
		assert.Equal(t, int32(10), pi.PageSize())
		assert.Equal(t, int32(5), pi.TotalCount())
	})

	t.Run("clamps page and page size", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().ListRecipes(gomock.Any(), true, int32(100), int32(0)).Return([]recipe.Recipe{}, nil)
		rec.EXPECT().CountRecipes(gomock.Any(), true).Return(int64(0), nil)
		r := &Resolver{RecipeService: rec}
		res, err := r.Recipes(recCtx(), pageArgs{Page: -3, PageSize: 500})
		require.NoError(t, err)
		pi := res.PageInfo()
		assert.Equal(t, int32(1), pi.PageNumber())
		assert.Equal(t, int32(100), pi.PageSize())
		assert.Equal(t, int32(0), pi.TotalCount())
	})

	t.Run("unauthorized", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.Recipes(context.Background(), pageArgs{Page: 1, PageSize: 10})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("service error", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().ListRecipes(gomock.Any(), true, int32(10), int32(0)).Return(nil, errRecBoom)
		r := &Resolver{RecipeService: rec}
		_, err := r.Recipes(recCtx(), pageArgs{Page: 1, PageSize: 10})
		require.ErrorIs(t, err, errRecBoom)
	})
}

func TestResolver_Recipe_CreateRecipe(t *testing.T) {
	t.Run("happy path with items and steps", func(t *testing.T) {
		rec, inv, _ := newRecMocks(t)
		expectedRecipe := recipe.Recipe{
			Name: "Soup", Description: "Tasty",
			Servings: recInt32Ptr(4), PrepTimeMinutes: recInt32Ptr(10), CookTimeMinutes: recInt32Ptr(20),
			IsActive: true,
		}
		expectedItems := []recipe.RecipeItem{
			{ItemID: 3, Quantity: 2, UnitID: 3, Notes: "diced", IsOptional: true},
		}
		expectedSteps := []recipe.RecipeStep{
			{StepNumber: 1, Instruction: "Boil"},
		}
		inv.EXPECT().GetUnitByName(gomock.Any(), "cups").Return(inventory.Unit{UnitID: 3, Name: "cup"}, nil)
		rec.EXPECT().CreateRecipeWithChildren(gomock.Any(), gomock.Eq(expectedRecipe), gomock.Eq(expectedItems), gomock.Eq(expectedSteps), recTestEmail).
			Return(recipe.Recipe{RecipeID: 9, Name: "Soup", Servings: recInt32Ptr(4), IsActive: true}, nil)

		r := &Resolver{RecipeService: rec, InventoryService: inv}
		res, err := r.CreateRecipe(recCtx(), struct{ Input createRecipeInput }{
			Input: createRecipeInput{
				Name:            "Soup",
				Description:     recStrPtr("Tasty"),
				Servings:        recInt32Ptr(4),
				PrepTimeMinutes: recInt32Ptr(10),
				CookTimeMinutes: recInt32Ptr(20),
				Items: []recipeItemInput{
					{ItemID: "3", Quantity: 2, Unit: "cups", Notes: recStrPtr("diced"), IsOptional: recBoolPtr(true)},
				},
				Steps: []recipeStepInput{
					{StepNumber: 1, Instruction: "Boil"},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("9"), res.ID())
		assert.Equal(t, "Soup", res.Name())
	})

	t.Run("unauthorized", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.CreateRecipe(context.Background(), struct{ Input createRecipeInput }{})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.CreateRecipe(recUserCtx(), struct{ Input createRecipeInput }{
			Input: createRecipeInput{Name: "Soup"},
		})
		require.ErrorContains(t, err, "forbidden")
	})

	t.Run("invalid item id", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.CreateRecipe(recCtx(), struct{ Input createRecipeInput }{
			Input: createRecipeInput{
				Name:  "Soup",
				Items: []recipeItemInput{{ItemID: "abc"}},
			},
		})
		require.Error(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().CreateRecipeWithChildren(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), recTestEmail).Return(recipe.Recipe{}, errRecBoom)
		r := &Resolver{RecipeService: rec}
		_, err := r.CreateRecipe(recCtx(), struct{ Input createRecipeInput }{
			Input: createRecipeInput{Name: "Soup"},
		})
		require.ErrorIs(t, err, errRecBoom)
	})
}

func recBoolPtr(b bool) *bool { return &b }

func TestResolver_Recipe_UpdateRecipe(t *testing.T) {
	type args = struct {
		ID    graphql.ID
		Input createRecipeInput
	}

	t.Run("happy path replaces items and steps", func(t *testing.T) {
		rec, inv, _ := newRecMocks(t)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).
			Return(recipe.Recipe{RecipeID: 9, Name: "Old", Description: "D", Servings: recInt32Ptr(2), IsActive: true}, nil)

		expectedRecipe := recipe.Recipe{
			Name: "New", Description: "D", Servings: recInt32Ptr(6), IsActive: true,
		}
		expectedItems := []recipe.RecipeItem{
			{ItemID: 3, Quantity: 1, UnitID: 3},
		}
		expectedSteps := []recipe.RecipeStep{
			{StepNumber: 1, Instruction: "Stir"},
		}
		inv.EXPECT().GetUnitByName(gomock.Any(), "cup").Return(inventory.Unit{UnitID: 3, Name: "cup"}, nil)
		rec.EXPECT().UpdateRecipeWithChildren(gomock.Any(), int64(9), gomock.Eq(expectedRecipe), gomock.Eq(expectedItems), gomock.Eq(expectedSteps), recTestEmail).Return(nil)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).
			Return(recipe.Recipe{RecipeID: 9, Name: "New", Servings: recInt32Ptr(6), IsActive: true}, nil)

		r := &Resolver{RecipeService: rec, InventoryService: inv}
		res, err := r.UpdateRecipe(recCtx(), args{
			ID: "9",
			Input: createRecipeInput{
				Name:     "New",
				Servings: recInt32Ptr(6),
				Items:    []recipeItemInput{{ItemID: "3", Quantity: 1, Unit: "cup"}},
				Steps:    []recipeStepInput{{StepNumber: 1, Instruction: "Stir"}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "New", res.Name())
		require.NotNil(t, res.Servings())
		assert.Equal(t, int32(6), *res.Servings())
	})

	t.Run("unauthorized", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.UpdateRecipe(context.Background(), args{ID: "9"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.UpdateRecipe(recCtx(), args{ID: "abc"})
		require.Error(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{}, errRecBoom)
		r := &Resolver{RecipeService: rec}
		_, err := r.UpdateRecipe(recCtx(), args{ID: "9"})
		require.ErrorIs(t, err, errRecBoom)
	})
}

func TestResolver_Recipe_DeleteRecipe(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().DeleteRecipe(gomock.Any(), int64(9)).Return(nil)
		r := &Resolver{RecipeService: rec}
		ok, err := r.DeleteRecipe(recCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unauthorized", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.DeleteRecipe(context.Background(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("invalid id", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.DeleteRecipe(recCtx(), struct{ ID graphql.ID }{ID: "abc"})
		require.Error(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().DeleteRecipe(gomock.Any(), int64(9)).Return(errRecBoom)
		r := &Resolver{RecipeService: rec}
		ok, err := r.DeleteRecipe(recCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.ErrorIs(t, err, errRecBoom)
		assert.False(t, ok)
	})
}
