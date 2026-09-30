package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

type stubTastes struct {
	usage  map[int64]int64
	vel    []analytics.RecipeVelocity
	user   []analytics.SelectionCount
	gotHH  int64
	gotUID int64
}

func (s *stubTastes) TopUserSelections(_ context.Context, userID int64, _ string, _ int32) ([]analytics.SelectionCount, error) {
	s.gotUID = userID
	return s.user, nil
}

func (s *stubTastes) HouseholdRecipeUsage(_ context.Context, householdID int64) (map[int64]int64, error) {
	s.gotHH = householdID
	return s.usage, nil
}

func (s *stubTastes) HouseholdRecipeVelocities(_ context.Context, _ int64, _ int32) ([]analytics.RecipeVelocity, error) {
	return s.vel, nil
}

func TestGetHouseholdTastes(t *testing.T) {
	stats := &stubTastes{
		usage: map[int64]int64{1: 8, 2: 3},
		vel:   []analytics.RecipeVelocity{{RecipeID: 2, RecentCount: 4, TotalCount: 9}},
		user:  []analytics.SelectionCount{{EntityType: "recipe", EntityID: 3, SelectCount: 6}},
	}
	rc := &stubRecipes{recipes: map[int64]recipe.Recipe{
		1: {RecipeID: 1, Name: "Pancakes"},
		2: {RecipeID: 2, Name: "Pasta"},
		3: {RecipeID: 3, Name: "Soup"},
	}}
	reg := New()
	RegisterTasteTools(reg, stats, rc)

	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_household_tastes", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(9), stats.gotHH)
	assert.Equal(t, int64(7), stats.gotUID)

	res, ok := out.(TastesOut)
	require.True(t, ok)
	require.Len(t, res.HouseholdTop, 2)
	assert.Equal(t, int64(1), res.HouseholdTop[0].RecipeID) // sorted by picks desc
	assert.Equal(t, "Pancakes", res.HouseholdTop[0].Name)
	require.Len(t, res.Trending, 1)
	assert.Equal(t, "Pasta", res.Trending[0].Name)
	require.Len(t, res.UserTop, 1)
	assert.Equal(t, "Soup", res.UserTop[0].Name)
}

func TestGetHouseholdTastes_NoHousehold(t *testing.T) {
	stats := &stubTastes{user: []analytics.SelectionCount{{EntityID: 3, SelectCount: 2}}}
	reg := New()
	RegisterTasteTools(reg, stats, &stubRecipes{})

	out, err := reg.Call(context.Background(), Scope{UserID: 7}, "get_household_tastes", nil)
	require.NoError(t, err)
	res, ok := out.(TastesOut)
	require.True(t, ok)
	assert.Empty(t, res.HouseholdTop)
	assert.Len(t, res.UserTop, 1)
}
