package bff

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

var recTestCategory = recipe.Category{
	CategoryID: 21, CategoryGroupID: 3, Name: "Mexican",
	GroupName: "Cuisine", GroupExclusive: true, GroupDisplayOrder: 6,
}

func TestResolver_Recipe_Categories(t *testing.T) {
	cat := recTestCategory

	t.Run("groups query returns groups with categories", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().ListCategoryGroups(gomock.Any()).Return([]recipe.CategoryGroup{
			{CategoryGroupID: 3, Name: "Cuisine", Exclusive: true, DisplayOrder: 6},
		}, nil)
		rec.EXPECT().ListCategoriesByGroup(gomock.Any(), int64(3)).
			Return([]recipe.Category{cat}, nil)

		r := &Resolver{RecipeService: rec}
		groups, err := r.RecipeCategoryGroups(recCtx())
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, "3", string(groups[0].ID()))
		assert.Equal(t, "Cuisine", groups[0].Name())
		assert.True(t, groups[0].Exclusive())
		cats := groups[0].Categories()
		require.Len(t, cats, 1)
		assert.Equal(t, "Mexican", cats[0].Name())
	})

	t.Run("groups query requires auth", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.RecipeCategoryGroups(context.Background())
		require.ErrorContains(t, err, "unauthorized")
	})

	t.Run("recipe.categories resolves from preload", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{RecipeID: 9, Name: "Tacos"}, nil)
		rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{9}).Return([]recipe.Recipe{{RecipeID: 9}}, nil)
		rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRecipeStepsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListCategoriesForRecipes(gomock.Any(), []int64{9}).
			Return(map[int64][]recipe.Category{9: {cat}}, nil)
		rec.EXPECT().ListRecipeRatings(gomock.Any(), int64(11), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRatingSummaries(gomock.Any(), []int64{9}).Return(nil, nil)

		r := &Resolver{RecipeService: rec}
		res, err := r.Recipe(recCtx(), struct{ ID graphql.ID }{ID: "9"})
		require.NoError(t, err)
		cats := res.Categories()
		require.Len(t, cats, 1)
		assert.Equal(t, "Mexican", cats[0].Name())
		assert.Equal(t, "Cuisine", cats[0].Group().Name())
		assert.True(t, cats[0].Group().Exclusive())
	})

	t.Run("setRecipeCategories replaces assignments", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().SetRecipeCategories(gomock.Any(), int64(9), []int64{21, 40}, recTestEmail).Return(nil)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{RecipeID: 9, Name: "Tacos"}, nil)
		rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{9}).Return([]recipe.Recipe{{RecipeID: 9}}, nil)
		rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRecipeStepsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListCategoriesForRecipes(gomock.Any(), []int64{9}).
			Return(map[int64][]recipe.Category{9: {cat}}, nil)
		rec.EXPECT().ListRecipeRatings(gomock.Any(), int64(11), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRatingSummaries(gomock.Any(), []int64{9}).Return(nil, nil)

		r := &Resolver{RecipeService: rec}
		res, err := r.SetRecipeCategories(recCtx(), struct {
			RecipeID    graphql.ID
			CategoryIDs []graphql.ID
		}{RecipeID: "9", CategoryIDs: []graphql.ID{"21", "40"}})
		require.NoError(t, err)
		require.Len(t, res.Categories(), 1)
	})

	t.Run("setRecipeCategories surfaces exclusivity violation", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().SetRecipeCategories(gomock.Any(), int64(9), []int64{21, 22}, recTestEmail).
			Return(&domainerr.ValidationError{Field: "categoryIds", Msg: `a recipe can't be both "Mexican" and "Italian" (Cuisine)`})

		r := &Resolver{RecipeService: rec}
		_, err := r.SetRecipeCategories(recCtx(), struct {
			RecipeID    graphql.ID
			CategoryIDs []graphql.ID
		}{RecipeID: "9", CategoryIDs: []graphql.ID{"21", "22"}})
		require.Error(t, err)
		assert.ErrorContains(t, err, "Mexican")
	})

	t.Run("setRecipeCategories rejects non-admin members", func(t *testing.T) {
		// Recipes are a global catalog — member categorization would leak
		// across households (LEN-29 finding 2). No store call is made.
		r := &Resolver{}
		_, err := r.SetRecipeCategories(recUserCtx(), struct {
			RecipeID    graphql.ID
			CategoryIDs []graphql.ID
		}{RecipeID: "9", CategoryIDs: []graphql.ID{"21"}})
		require.Error(t, err)
	})

	t.Run("createRecipe honors input categoryIds", func(t *testing.T) {
		rec, inv, _ := newRecMocks(t)
		an := newAnalyticsMock(t)
		// Analytics calls fire on the detached background runner.
		an.EXPECT().RecordEvent(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		an.EXPECT().ComputeIngredientOverlapSuggestions(gomock.Any(), gomock.Any()).AnyTimes()
		rec.EXPECT().CreateRecipeWithChildren(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), recTestEmail).
			Return(recipe.Recipe{RecipeID: 9, Name: "New"}, nil)
		rec.EXPECT().SetRecipeCategories(gomock.Any(), int64(9), []int64{21}, recTestEmail).Return(nil)

		r := &Resolver{RecipeService: rec, InventoryService: inv, AnalyticsService: an}
		_, err := r.CreateRecipe(recCtx(), struct{ Input createRecipeInput }{
			Input: createRecipeInput{Name: "New", CategoryIDs: &[]graphql.ID{"21"}},
		})
		require.NoError(t, err)
	})
}

func TestResolver_Recipe_CategoryAdmin(t *testing.T) {
	t.Run("createRecipeCategoryGroup requires admin", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.CreateRecipeCategoryGroup(recUserCtx(), struct {
			Input struct {
				Name         string
				Exclusive    *bool
				DisplayOrder *int32
			}
		}{})
		require.Error(t, err)
	})

	t.Run("createRecipeCategoryGroup creates", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().CreateCategoryGroup(gomock.Any(), recipe.CategoryGroup{
			Name: "Season", Exclusive: true, DisplayOrder: 7,
		}, recTestEmail).Return(recipe.CategoryGroup{
			CategoryGroupID: 10, Name: "Season", Exclusive: true, DisplayOrder: 7,
		}, nil)

		r := &Resolver{RecipeService: rec}
		got, err := r.CreateRecipeCategoryGroup(recCtx(), struct {
			Input struct {
				Name         string
				Exclusive    *bool
				DisplayOrder *int32
			}
		}{Input: struct {
			Name         string
			Exclusive    *bool
			DisplayOrder *int32
		}{Name: "Season", Exclusive: recBoolPtr(true), DisplayOrder: recInt32Ptr(7)}})
		require.NoError(t, err)
		assert.Equal(t, "Season", got.Name())
	})

	t.Run("createRecipeCategory creates", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().CreateCategory(gomock.Any(), recipe.Category{
			CategoryGroupID: 3, Name: "Thai",
		}, recTestEmail).Return(recipe.Category{
			CategoryID: 30, CategoryGroupID: 3, Name: "Thai", GroupName: "Cuisine", GroupExclusive: true,
		}, nil)

		r := &Resolver{RecipeService: rec}
		got, err := r.CreateRecipeCategory(recCtx(), struct {
			Input struct {
				GroupID graphql.ID
				Name    string
			}
		}{Input: struct {
			GroupID graphql.ID
			Name    string
		}{GroupID: "3", Name: "Thai"}})
		require.NoError(t, err)
		assert.Equal(t, "Thai", got.Name())
	})

	t.Run("deleteRecipeCategory requires admin", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.DeleteRecipeCategory(recUserCtx(), struct{ ID graphql.ID }{ID: "21"})
		require.Error(t, err)
	})

	t.Run("deleteRecipeCategory deletes", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().DeleteCategory(gomock.Any(), int64(21)).Return(nil)
		r := &Resolver{RecipeService: rec}
		ok, err := r.DeleteRecipeCategory(recCtx(), struct{ ID graphql.ID }{ID: "21"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_RecordView(t *testing.T) {
	t.Run("queues analytics event", func(t *testing.T) {
		an := newAnalyticsMock(t)
		an.EXPECT().RecordView(gomock.Any(), analytics.Event{
			UserID:     7,
			EventType:  analytics.EventRecipeViewed,
			EntityType: analytics.EntityRecipe,
			EntityID:   9,
		}, analyticsTestEmail).Return(nil)

		r := &Resolver{AnalyticsService: an}
		ok, err := r.RecordView(analyticsCtx(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: analytics.EntityRecipe, EntityID: "9"})
		require.NoError(t, err)
		assert.True(t, ok)
		// RecordView dispatches on the detached background runner — drain
		// it so the mock expectation is verified deterministically.
		require.NoError(t, r.Shutdown(context.Background()))
	})

	t.Run("derives event name for any entity type", func(t *testing.T) {
		an := newAnalyticsMock(t)
		an.EXPECT().RecordView(gomock.Any(), analytics.Event{
			UserID:     7,
			EventType:  analytics.EventBottleViewed,
			EntityType: analytics.EntityBottle,
			EntityID:   12,
		}, analyticsTestEmail).Return(nil)

		r := &Resolver{AnalyticsService: an}
		ok, err := r.RecordView(analyticsCtx(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: analytics.EntityBottle, EntityID: "12"})
		require.NoError(t, err)
		assert.True(t, ok)
		require.NoError(t, r.Shutdown(context.Background()))
	})

	t.Run("requires auth", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.RecordView(context.Background(), struct {
			EntityType string
			EntityID   graphql.ID
		}{EntityType: "recipe", EntityID: "9"})
		require.Error(t, err)
	})
}
