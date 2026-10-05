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
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func noHouseCtx() context.Context {
	return testutil.WithHousehold(context.Background(), 11, 0, recTestEmail)
}

// TestResolver_Recipe_DeltaFields covers the delta-aware Recipe fields:
// effective vs canonical views, the householdDelta object, and stale.
func TestResolver_Recipe_DeltaFields(t *testing.T) {
	base := time.Now().Add(-time.Hour)
	newer := base.Add(30 * time.Minute)

	build := func(d *recipe.RecipeDelta, updatedAt *time.Time) *recipeResolver {
		rec := recipe.Recipe{RecipeID: 9, Name: "Soup", UpdatedAt: updatedAt}
		canon := []recipe.RecipeItem{{RecipeItemID: 40, RecipeID: 9, ItemID: ptrInt64(3), Quantity: 1, UnitID: 3}}
		rc := newRecipeChildren()
		rc.recipes[9] = rec
		rc.itemsBy[9] = canon
		if d != nil {
			eff := recipe.ApplyDelta(canon, nil, d)
			rc.canonItemsBy[9] = canon
			rc.itemsBy[9] = eff.Items
			rc.deltas = map[int64]*recipe.RecipeDelta{9: d}
		}
		return &recipeResolver{recipe: rec, rc: rc}
	}

	t.Run("effective view applies delta, canonical bypasses", func(t *testing.T) {
		d := &recipe.RecipeDelta{Items: []recipe.DeltaItem{
			{DeltaItemID: 1, Kind: recipe.DeltaItemAdjust, RecipeItemID: ptrInt64(40), Quantity: f64p(9)},
		}}
		res := build(d, &base)
		items, err := res.Items(context.TODO(), struct{ View string }{View: "effective"})
		require.NoError(t, err)
		assert.Equal(t, 9.0, items[0].Quantity())
		assert.Equal(t, "adjust", *items[0].DeltaKind())

		items, err = res.Items(context.TODO(), struct{ View string }{View: "canonical"})
		require.NoError(t, err)
		assert.Equal(t, 1.0, items[0].Quantity())
		assert.Nil(t, items[0].DeltaKind())
	})

	t.Run("householdDelta exposes stale + orphans", func(t *testing.T) {
		d := &recipe.RecipeDelta{
			RecipeDeltaID: 5, RecipeID: 9, BaseUpdatedAt: base,
			Items: []recipe.DeltaItem{{DeltaItemID: 2, Kind: recipe.DeltaItemRemove}},
			Steps: []recipe.DeltaStep{{DeltaStepID: 3, Kind: recipe.DeltaStepReplace}},
		}
		res := build(d, &newer)
		dr, err := res.HouseholdDelta(context.TODO())
		require.NoError(t, err)
		require.NotNil(t, dr)
		assert.Equal(t, "5", string(dr.ID()))
		assert.True(t, dr.Stale())
		assert.Equal(t, int32(1), dr.OrphanedItemCount())
		assert.Equal(t, int32(1), dr.OrphanedStepCount())
		assert.True(t, dr.Items()[0].Orphaned())

		fresh := build(d, &base)
		dr, err = fresh.HouseholdDelta(context.TODO())
		require.NoError(t, err)
		assert.False(t, dr.Stale())
	})

	t.Run("no delta returns null", func(t *testing.T) {
		dr, err := build(nil, &base).HouseholdDelta(context.TODO())
		require.NoError(t, err)
		assert.Nil(t, dr)
	})

	t.Run("lazy path applies delta", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().ListRecipeItems(gomock.Any(), int64(9)).Return([]recipe.RecipeItem{
			{RecipeItemID: 40, RecipeID: 9, Quantity: 1, UnitID: 3},
		}, nil)
		rec.EXPECT().GetRecipeDelta(gomock.Any(), int64(9), int64(11)).Return(&recipe.RecipeDelta{
			Items: []recipe.DeltaItem{{Kind: recipe.DeltaItemAdjust, RecipeItemID: ptrInt64(40), Quantity: f64p(7)}},
		}, nil)
		res := &recipeResolver{rec: rec, user: recUser(), recipe: recipe.Recipe{RecipeID: 9}}
		items, err := res.Items(recCtx(), struct{ View string }{View: ""})
		require.NoError(t, err)
		assert.Equal(t, 7.0, items[0].Quantity())

		// canonical lazy path skips the delta entirely
		rec.EXPECT().ListRecipeItems(gomock.Any(), int64(9)).Return([]recipe.RecipeItem{
			{RecipeItemID: 40, RecipeID: 9, Quantity: 1, UnitID: 3},
		}, nil)
		items, err = res.Items(recCtx(), struct{ View string }{View: "canonical"})
		require.NoError(t, err)
		assert.Equal(t, 1.0, items[0].Quantity())
	})
}

// TestResolver_SetRecipeDelta exercises the member-gated mutations end to
// end through the service mock.
func TestResolver_SetRecipeDelta(t *testing.T) {
	t.Run("member writes a delta", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		qty := 2.5
		rec.EXPECT().SetRecipeDelta(gomock.Any(), int64(9), int64(11),
			[]recipe.DeltaItem{{Kind: "adjust", RecipeItemID: ptrInt64(40), Quantity: &qty}},
			[]recipe.DeltaStep{{Kind: "replace", StepID: ptrInt64(21), Instruction: strPtr("bake low")}},
			recTestEmail,
		).Return(recipe.RecipeDelta{RecipeDeltaID: 5, RecipeID: 9, HouseholdID: 11, BaseUpdatedAt: time.Now()}, nil)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{RecipeID: 9}, nil)
		rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{9}).Return([]recipe.Recipe{{RecipeID: 9}}, nil)
		rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRecipeStepsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListCategoriesForRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRecipeRatings(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		rec.EXPECT().ListRatingSummaries(gomock.Any(), gomock.Any()).Return(nil, nil)

		r := &Resolver{RecipeService: rec}
		out, err := r.SetRecipeDelta(recCtx(), struct {
			RecipeID graphql.ID
			Items    []deltaItemInput
			Steps    []deltaStepInput
		}{
			RecipeID: "9",
			Items: []deltaItemInput{{
				RecipeItemID: gqlIDPtr("40"), Kind: "adjust", Quantity: &qty,
			}},
			Steps: []deltaStepInput{{
				StepID: gqlIDPtr("21"), Kind: "replace", Instruction: strPtr("bake low"),
			}},
		})
		require.NoError(t, err)
		assert.Equal(t, "5", string(out.ID()))
	})

	t.Run("clear + acknowledge", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		rec.EXPECT().ClearRecipeDelta(gomock.Any(), int64(9), int64(11)).Return(nil)
		r := &Resolver{RecipeService: rec}
		ok, err := r.ClearRecipeDelta(recCtx(), struct{ RecipeID graphql.ID }{RecipeID: "9"})
		require.NoError(t, err)
		assert.True(t, ok)

		rec.EXPECT().AcknowledgeRecipeDelta(gomock.Any(), int64(9), int64(11), recTestEmail).Return(nil)
		rec.EXPECT().GetRecipeDelta(gomock.Any(), int64(9), int64(11)).Return(&recipe.RecipeDelta{
			RecipeDeltaID: 5, RecipeID: 9, HouseholdID: 11, BaseUpdatedAt: time.Now(),
		}, nil)
		rec.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(recipe.Recipe{RecipeID: 9}, nil)
		rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{9}).Return([]recipe.Recipe{{RecipeID: 9}}, nil)
		rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRecipeStepsByRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListCategoriesForRecipes(gomock.Any(), []int64{9}).Return(nil, nil)
		rec.EXPECT().ListRecipeRatings(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		rec.EXPECT().ListRatingSummaries(gomock.Any(), gomock.Any()).Return(nil, nil)
		out, err := r.AcknowledgeRecipeDelta(recCtx(), struct{ RecipeID graphql.ID }{RecipeID: "9"})
		require.NoError(t, err)
		assert.Equal(t, "5", string(out.ID()))
		assert.False(t, out.Stale())
	})

	t.Run("household-less user is rejected", func(t *testing.T) {
		rec, _, _ := newRecMocks(t)
		r := &Resolver{RecipeService: rec}
		_, err := r.SetRecipeDelta(noHouseCtx(), struct {
			RecipeID graphql.ID
			Items    []deltaItemInput
			Steps    []deltaStepInput
		}{RecipeID: "9"})
		require.Error(t, err)
	})
}

// TestResolver_PlanRecipes_Delta proves meal-plan/grocery consumers see
// the household version.
func TestResolver_PlanRecipes_Delta(t *testing.T) {
	rec := mock.NewMockRecipeService(gomock.NewController(t))
	rec.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{9}).Return([]recipe.Recipe{{RecipeID: 9}}, nil)
	rec.EXPECT().ListRecipeItemsByRecipes(gomock.Any(), []int64{9}).Return([]recipe.RecipeItem{
		{RecipeItemID: 40, RecipeID: 9, ItemID: ptrInt64(3), Quantity: 2, UnitID: 3},
		{RecipeItemID: 41, RecipeID: 9, ItemID: ptrInt64(4), Quantity: 1, UnitID: 3},
	}, nil)
	rec.EXPECT().ListRecipeDeltas(gomock.Any(), int64(11), []int64{9}).Return(map[int64]*recipe.RecipeDelta{
		9: {Items: []recipe.DeltaItem{
			{Kind: recipe.DeltaItemSubstitute, RecipeItemID: ptrInt64(40), ItemID: ptrInt64(99)},
			{Kind: recipe.DeltaItemRemove, RecipeItemID: ptrInt64(41)},
		}},
	}, nil)

	r := &Resolver{RecipeService: rec}
	recipes, items, err := r.planRecipes(recCtx(), 11, []mealplan.MealSlot{{RecipeID: ptrInt64(9)}})
	_ = recipes
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(99), *items[0].ItemID)
}

func f64p(v float64) *float64 { return &v }

// recUser mirrors the user embedded by recCtx() for resolver structs.
func recUser() currentuser.User {
	u, _ := currentuser.FromContext(recCtx())
	return u
}
