package bff

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/ai"
	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

func allergenAdminCtx() context.Context {
	return currentuser.WithUser(context.Background(), currentuser.User{
		UserID: 1, Email: "admin@example.com", IsAdmin: true, HouseholdID: 9,
	})
}

func TestResolver_AllergenSuggestions(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockInventoryService(ctrl)
	inv.EXPECT().ListAllergenSuggestions(gomock.Any(), "pending").Return([]inventory.AllergenSuggestion{
		{
			AllergenSuggestionID: 5,
			RecipeID:             i64p(10),
			RecipeName:           "Enchilada",
			TargetKind:           "ingredient",
			IngredientID:         i64p(51),
			IngredientName:       "flour",
			AllergenID:           2,
			AllergenName:         "wheat",
			Kind:                 "contains",
			Rationale:            "flour is wheat",
			Status:               "pending",
		},
	}, nil)

	r := &Resolver{InventoryService: inv}
	got, err := r.AllergenSuggestions(allergenAdminCtx(), struct {
		Status *string
	}{Status: strp("pending")})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, graphql.ID("5"), got[0].ID())
	assert.Equal(t, "wheat", got[0].Allergen().Name())
	assert.Equal(t, "flour", *got[0].IngredientName())
	assert.Equal(t, "pending", got[0].Status())
}

func TestResolver_AllergenSuggestions_NonAdmin(t *testing.T) {
	r := &Resolver{}
	_, err := r.AllergenSuggestions(aiCtx(), struct {
		Status *string
	}{})
	require.Error(t, err)
}

func TestResolver_SuggestRecipeAllergens(t *testing.T) {
	ctrl := gomock.NewController(t)
	aiSvc := mock.NewMockAIService(ctrl)
	inv := mock.NewMockInventoryService(ctrl)

	aiSvc.EXPECT().SuggestAllergens(gomock.Any(), int64(1), int64(9), int64(10), 8).
		Return([]ai.AllergenFlagProposal{
			{TargetKind: "ingredient", TargetID: 51, AllergenID: 2, Kind: "contains", Reason: "flour"},
		}, nil)
	inv.EXPECT().CreateAllergenSuggestions(gomock.Any(), gomock.Any(), "admin@example.com").
		DoAndReturn(func(_ context.Context, rows []inventory.NewAllergenSuggestion, _ string) ([]inventory.AllergenSuggestion, error) {
			require.Len(t, rows, 1)
			assert.Equal(t, int64(51), rows[0].TargetID)
			assert.Equal(t, "ingredient", rows[0].TargetKind)
			assert.Equal(t, int64(1), *rows[0].SuggestedBy)
			return []inventory.AllergenSuggestion{{AllergenSuggestionID: 5}}, nil
		})
	inv.EXPECT().GetAllergenSuggestion(gomock.Any(), int64(5)).Return(inventory.AllergenSuggestion{
		AllergenSuggestionID: 5, TargetKind: "ingredient", IngredientID: i64p(51),
		AllergenID: 2, AllergenName: "wheat", Kind: "contains", Status: "pending",
	}, nil)

	r := &Resolver{AIService: aiSvc, InventoryService: inv}
	got, err := r.SuggestRecipeAllergens(allergenAdminCtx(), struct {
		RecipeID       graphql.ID
		MaxSuggestions *int32
	}{RecipeID: "10", MaxSuggestions: i32p(8)})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, graphql.ID("5"), got[0].ID())
}

func TestResolver_SuggestRecipeAllergens_NonAdmin(t *testing.T) {
	r := &Resolver{}
	_, err := r.SuggestRecipeAllergens(aiCtx(), struct {
		RecipeID       graphql.ID
		MaxSuggestions *int32
	}{RecipeID: "10"})
	require.Error(t, err)
}

func TestResolver_AcceptAllergenSuggestion(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockInventoryService(ctrl)
	inv.EXPECT().AcceptAllergenSuggestion(gomock.Any(), int64(5), int64(1), "admin@example.com").Return(nil)
	inv.EXPECT().GetAllergenSuggestion(gomock.Any(), int64(5)).Return(inventory.AllergenSuggestion{
		AllergenSuggestionID: 5, TargetKind: "item", ItemID: i64p(200), ItemName: "cheese",
		AllergenID: 1, AllergenName: "milk", Kind: "contains", Status: "accepted",
	}, nil)

	r := &Resolver{InventoryService: inv}
	got, err := r.AcceptAllergenSuggestion(allergenAdminCtx(), struct {
		ID graphql.ID
	}{ID: "5"})
	require.NoError(t, err)
	assert.Equal(t, "accepted", got.Status())
	assert.Equal(t, "cheese", *got.ItemName())
}

func TestResolver_DismissAllergenSuggestion(t *testing.T) {
	ctrl := gomock.NewController(t)
	inv := mock.NewMockInventoryService(ctrl)
	inv.EXPECT().DismissAllergenSuggestion(gomock.Any(), int64(5), int64(1), "admin@example.com").Return(nil)
	inv.EXPECT().GetAllergenSuggestion(gomock.Any(), int64(5)).Return(inventory.AllergenSuggestion{
		AllergenSuggestionID: 5, TargetKind: "ingredient", IngredientID: i64p(51),
		AllergenID: 2, AllergenName: "wheat", Kind: "contains", Status: "dismissed",
	}, nil)

	r := &Resolver{InventoryService: inv}
	got, err := r.DismissAllergenSuggestion(allergenAdminCtx(), struct {
		ID graphql.ID
	}{ID: "5"})
	require.NoError(t, err)
	assert.Equal(t, "dismissed", got.Status())
}

func i64p(v int64) *int64   { return &v }
func strp(v string) *string { return &v }
func i32p(v int32) *int32   { return &v }
