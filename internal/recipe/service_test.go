package recipe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/recipe/sqlc"
	"github.com/JRAdams472/LENA2/internal/recipe/sqlc/mock"
)

var errDB = errors.New("db failure")

func i32(v int32) *int32 { return &v }

func mustNum(v float64) pgtype.Numeric {
	n, err := numericFromFloat64(v)
	if err != nil {
		panic(err)
	}
	return n
}

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mq := mock.NewMockQuerier(ctrl)
	return &Service{q: mq}, mq
}

func recipeRow() sqlc.RecipeRecipe {
	return sqlc.RecipeRecipe{
		RecipeID:        7,
		Name:            "Pancakes",
		Description:     pgtype.Text{String: "fluffy", Valid: true},
		Servings:        pgtype.Int4{Int32: 4, Valid: true},
		PrepTimeMinutes: pgtype.Int4{Int32: 10, Valid: true},
		CookTimeMinutes: pgtype.Int4{Int32: 15, Valid: true},
		IsActive:        true,
		CreatedBy:       "alice",
		CreatedAt:       time.Now(),
		UpdatedBy:       pgtype.Text{String: "alice", Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func assertRecipe(t *testing.T, r Recipe) {
	t.Helper()
	assert.Equal(t, int64(7), r.RecipeID)
	assert.Equal(t, "Pancakes", r.Name)
	assert.Equal(t, "fluffy", r.Description)
	require.NotNil(t, r.Servings)
	assert.Equal(t, int32(4), *r.Servings)
	require.NotNil(t, r.PrepTimeMinutes)
	assert.Equal(t, int32(10), *r.PrepTimeMinutes)
	require.NotNil(t, r.CookTimeMinutes)
	assert.Equal(t, int32(15), *r.CookTimeMinutes)
	assert.True(t, r.IsActive)
}

func TestCreateRecipe(t *testing.T) {
	t.Run("success maps fields and audit params", func(t *testing.T) {
		svc, mq := newService(t)
		arg := Recipe{
			Name:            "Pancakes",
			Description:     "fluffy",
			Servings:        i32(4),
			PrepTimeMinutes: i32(10),
			CookTimeMinutes: i32(15),
			IsActive:        true,
		}
		want := sqlc.CreateRecipeParams{
			Name:            "Pancakes",
			Description:     textOrNull("fluffy"),
			Servings:        optInt4(i32(4)),
			PrepTimeMinutes: optInt4(i32(10)),
			CookTimeMinutes: optInt4(i32(15)),
			IsActive:        true,
			CreatedBy:       "alice",
			UpdatedBy:       textOrNull("alice"),
		}
		mq.EXPECT().CreateRecipe(gomock.Any(), want).Return(recipeRow(), nil)

		got, err := svc.CreateRecipe(context.Background(), arg, "alice")
		require.NoError(t, err)
		assertRecipe(t, got)
	})

	t.Run("empty optional fields become NULL", func(t *testing.T) {
		svc, mq := newService(t)
		want := sqlc.CreateRecipeParams{
			Name:            "Minimal",
			Description:     pgtype.Text{},
			Servings:        pgtype.Int4{},
			PrepTimeMinutes: pgtype.Int4{},
			CookTimeMinutes: pgtype.Int4{},
			IsActive:        false,
			CreatedBy:       "bob",
			UpdatedBy:       textOrNull("bob"),
		}
		mq.EXPECT().CreateRecipe(gomock.Any(), want).
			Return(sqlc.RecipeRecipe{RecipeID: 1, Name: "Minimal"}, nil)

		got, err := svc.CreateRecipe(context.Background(), Recipe{Name: "Minimal"}, "bob")
		require.NoError(t, err)
		assert.Equal(t, int64(1), got.RecipeID)
		assert.Equal(t, "", got.Description)
		assert.Nil(t, got.Servings)
		assert.Nil(t, got.PrepTimeMinutes)
		assert.Nil(t, got.CookTimeMinutes)
		assert.False(t, got.IsActive)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateRecipe(gomock.Any(), gomock.Any()).Return(sqlc.RecipeRecipe{}, errDB)

		_, err := svc.CreateRecipe(context.Background(), Recipe{Name: "x"}, "alice")
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "create recipe:")
	})
}

func TestGetRecipeByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipeByID(gomock.Any(), int64(7)).Return(recipeRow(), nil)

		got, err := svc.GetRecipeByID(context.Background(), 7)
		require.NoError(t, err)
		assertRecipe(t, got)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipeByID(gomock.Any(), int64(9)).Return(sqlc.RecipeRecipe{}, errDB)

		_, err := svc.GetRecipeByID(context.Background(), 9)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "get recipe by id:")
	})
}

func TestScaleRecipe(t *testing.T) {
	t.Run("success scales quantities and servings", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipeByID(gomock.Any(), int64(7)).Return(recipeRow(), nil)
		mq.EXPECT().ListRecipeItems(gomock.Any(), int64(7)).Return([]sqlc.RecipeRecipeItem{itemRow()}, nil)
		mq.EXPECT().ListRecipeSteps(gomock.Any(), int64(7)).Return([]sqlc.RecipeRecipeStep{stepRow()}, nil)

		got, err := svc.ScaleRecipe(context.Background(), 7, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.Recipe.RecipeID)
		require.NotNil(t, got.Recipe.Servings)
		assert.Equal(t, int32(2), *got.Recipe.Servings)
		require.Len(t, got.Items, 1)
		assert.InDelta(t, 0.75, got.Items[0].Quantity, 1e-9)
		assert.Equal(t, int64(42), got.Items[0].ItemID)
		require.Len(t, got.Steps, 1)
		assert.Equal(t, int32(1), got.Steps[0].StepNumber)
	})

	t.Run("target servings must be positive", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipeByID(gomock.Any(), int64(7)).Return(recipeRow(), nil)

		_, err := svc.ScaleRecipe(context.Background(), 7, 0)
		require.Error(t, err)
		assert.ErrorContains(t, err, "target servings")
	})

	t.Run("error when recipe has no base servings", func(t *testing.T) {
		svc, mq := newService(t)
		row := recipeRow()
		row.Servings = pgtype.Int4{}
		mq.EXPECT().GetRecipeByID(gomock.Any(), int64(7)).Return(row, nil)

		_, err := svc.ScaleRecipe(context.Background(), 7, 2)
		require.Error(t, err)
		assert.ErrorContains(t, err, "base servings")
	})
}

func TestListRecipes(t *testing.T) {
	tests := []struct {
		name   string
		active bool
		limit  int32
		offset int32
		rows   []sqlc.RecipeRecipe
		want   []Recipe
	}{
		{
			name:   "active recipes",
			active: true,
			limit:  10,
			offset: 0,
			rows:   []sqlc.RecipeRecipe{recipeRow()},
			want: []Recipe{{
				RecipeID: 7, Name: "Pancakes", Description: "fluffy",
				Servings: i32(4), PrepTimeMinutes: i32(10), CookTimeMinutes: i32(15), IsActive: true,
			}},
		},
		{
			name:   "inactive recipes",
			active: false,
			limit:  5,
			offset: 20,
			rows:   []sqlc.RecipeRecipe{{RecipeID: 3, Name: "Old", IsActive: false}},
			want:   []Recipe{{RecipeID: 3, Name: "Old"}},
		},
		{
			name:   "empty page",
			active: true,
			limit:  10,
			offset: 100,
			rows:   []sqlc.RecipeRecipe{},
			want:   []Recipe{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, mq := newService(t)
			mq.EXPECT().ListRecipes(gomock.Any(), sqlc.ListRecipesParams{
				IsActive: tc.active, Limit: tc.limit, Offset: tc.offset,
			}).Return(tc.rows, nil)

			got, err := svc.ListRecipes(context.Background(), tc.active, tc.limit, tc.offset)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRecipes(gomock.Any(), gomock.Any()).Return(nil, errDB)

		_, err := svc.ListRecipes(context.Background(), true, 10, 0)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "list recipes:")
	})
}

func TestUpdateRecipe(t *testing.T) {
	t.Run("success passes mapped params", func(t *testing.T) {
		svc, mq := newService(t)
		arg := Recipe{
			Name:            "Waffles",
			Description:     "crispy",
			Servings:        i32(2),
			PrepTimeMinutes: i32(5),
			CookTimeMinutes: i32(8),
			IsActive:        true,
		}
		want := sqlc.UpdateRecipeParams{
			RecipeID:        7,
			Name:            "Waffles",
			Description:     textOrNull("crispy"),
			Servings:        optInt4(i32(2)),
			PrepTimeMinutes: optInt4(i32(5)),
			CookTimeMinutes: optInt4(i32(8)),
			IsActive:        true,
			UpdatedBy:       textOrNull("carol"),
		}
		mq.EXPECT().UpdateRecipe(gomock.Any(), want).Return(nil)

		require.NoError(t, svc.UpdateRecipe(context.Background(), 7, arg, "carol"))
	})

	t.Run("deactivate and clear optional fields", func(t *testing.T) {
		svc, mq := newService(t)
		want := sqlc.UpdateRecipeParams{
			RecipeID:        7,
			Name:            "Waffles",
			IsActive:        false,
			UpdatedBy:       textOrNull("carol"),
			Servings:        pgtype.Int4{},
			PrepTimeMinutes: pgtype.Int4{},
			CookTimeMinutes: pgtype.Int4{},
			Description:     pgtype.Text{},
		}
		mq.EXPECT().UpdateRecipe(gomock.Any(), want).Return(nil)

		require.NoError(t, svc.UpdateRecipe(context.Background(), 7, Recipe{Name: "Waffles"}, "carol"))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateRecipe(gomock.Any(), gomock.Any()).Return(errDB)

		err := svc.UpdateRecipe(context.Background(), 7, Recipe{}, "carol")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestDeleteRecipe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRecipe(gomock.Any(), int64(7)).Return(nil)

		require.NoError(t, svc.DeleteRecipe(context.Background(), 7))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRecipe(gomock.Any(), int64(7)).Return(errDB)

		assert.ErrorIs(t, svc.DeleteRecipe(context.Background(), 7), errDB)
	})
}

func itemRow() sqlc.RecipeRecipeItem {
	return sqlc.RecipeRecipeItem{
		RecipeItemID: 55,
		RecipeID:     7,
		ItemID:       42,
		Quantity:     mustNum(1.5),
		UnitID:       3,
		SectionName:  pgtype.Text{String: "filling", Valid: true},
		DisplayOrder: 2,
		Notes:        pgtype.Text{String: "sifted", Valid: true},
		IsOptional:   true,
	}
}

func TestAddRecipeItem(t *testing.T) {
	t.Run("success passes mapped params", func(t *testing.T) {
		svc, mq := newService(t)
		arg := RecipeItem{
			RecipeID:     7,
			ItemID:       42,
			Quantity:     1.5,
			UnitID:       3,
			SectionName:  "filling",
			DisplayOrder: 2,
			Notes:        "sifted",
			IsOptional:   true,
		}
		want := sqlc.AddRecipeItemParams{
			RecipeID:     7,
			ItemID:       42,
			Quantity:     mustNum(1.5),
			UnitID:       3,
			SectionName:  textOrNull("filling"),
			DisplayOrder: 2,
			Notes:        textOrNull("sifted"),
			IsOptional:   true,
		}
		mq.EXPECT().AddRecipeItem(gomock.Any(), want).Return(nil)

		require.NoError(t, svc.AddRecipeItem(context.Background(), arg))
	})

	t.Run("empty notes become NULL", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().AddRecipeItem(gomock.Any(), sqlc.AddRecipeItemParams{
			RecipeID: 7,
			ItemID:   42,
			Quantity: mustNum(2),
			UnitID:   10,
			Notes:    pgtype.Text{},
		}).Return(nil)

		require.NoError(t, svc.AddRecipeItem(context.Background(),
			RecipeItem{RecipeID: 7, ItemID: 42, Quantity: 2, UnitID: 10}))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().AddRecipeItem(gomock.Any(), gomock.Any()).Return(errDB)

		assert.ErrorIs(t, svc.AddRecipeItem(context.Background(), RecipeItem{}), errDB)
	})
}

func TestListRecipeItems(t *testing.T) {
	t.Run("success maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRecipeItems(gomock.Any(), int64(7)).
			Return([]sqlc.RecipeRecipeItem{itemRow()}, nil)

		got, err := svc.ListRecipeItems(context.Background(), 7)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, int64(55), got[0].RecipeItemID)
		assert.Equal(t, int64(7), got[0].RecipeID)
		assert.Equal(t, int64(42), got[0].ItemID)
		assert.InDelta(t, 1.5, got[0].Quantity, 1e-9)
		assert.Equal(t, int64(3), got[0].UnitID)
		assert.Equal(t, "filling", got[0].SectionName)
		assert.Equal(t, int32(2), got[0].DisplayOrder)
		assert.Equal(t, "sifted", got[0].Notes)
		assert.True(t, got[0].IsOptional)
	})

	t.Run("null quantity maps to zero", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRecipeItems(gomock.Any(), int64(7)).
			Return([]sqlc.RecipeRecipeItem{{RecipeID: 7, ItemID: 9, UnitID: 15}}, nil)

		got, err := svc.ListRecipeItems(context.Background(), 7)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, float64(0), got[0].Quantity)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRecipeItems(gomock.Any(), int64(7)).Return(nil, errDB)

		_, err := svc.ListRecipeItems(context.Background(), 7)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "list recipe items:")
	})
}

func TestRemoveRecipeItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().RemoveRecipeItem(gomock.Any(), int64(55)).Return(nil)

		require.NoError(t, svc.RemoveRecipeItem(context.Background(), 55))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().RemoveRecipeItem(gomock.Any(), int64(55)).Return(errDB)

		assert.ErrorIs(t, svc.RemoveRecipeItem(context.Background(), 55), errDB)
	})
}

func stepRow() sqlc.RecipeRecipeStep {
	return sqlc.RecipeRecipeStep{
		StepID:      11,
		RecipeID:    7,
		StepNumber:  1,
		Instruction: "Mix dry ingredients",
		CreatedBy:   "alice",
		CreatedAt:   time.Now(),
		UpdatedBy:   pgtype.Text{String: "alice", Valid: true},
	}
}

func TestAddRecipeStep(t *testing.T) {
	t.Run("success maps fields and audit params", func(t *testing.T) {
		svc, mq := newService(t)
		want := sqlc.AddRecipeStepParams{
			RecipeID:    7,
			StepNumber:  1,
			Instruction: "Mix dry ingredients",
			CreatedBy:   "alice",
			UpdatedBy:   textOrNull("alice"),
		}
		mq.EXPECT().AddRecipeStep(gomock.Any(), want).Return(stepRow(), nil)

		got, err := svc.AddRecipeStep(context.Background(), 7, 1, "Mix dry ingredients", "alice")
		require.NoError(t, err)
		assert.Equal(t, int64(11), got.StepID)
		assert.Equal(t, int64(7), got.RecipeID)
		assert.Equal(t, int32(1), got.StepNumber)
		assert.Equal(t, "Mix dry ingredients", got.Instruction)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().AddRecipeStep(gomock.Any(), gomock.Any()).
			Return(sqlc.RecipeRecipeStep{}, errDB)

		_, err := svc.AddRecipeStep(context.Background(), 7, 1, "x", "alice")
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "add recipe step:")
	})
}

func TestListRecipeSteps(t *testing.T) {
	t.Run("success returns rows in query order", func(t *testing.T) {
		svc, mq := newService(t)
		rows := []sqlc.RecipeRecipeStep{
			{StepID: 11, RecipeID: 7, StepNumber: 1, Instruction: "Mix"},
			{StepID: 12, RecipeID: 7, StepNumber: 2, Instruction: "Cook"},
			{StepID: 13, RecipeID: 7, StepNumber: 3, Instruction: "Serve"},
		}
		mq.EXPECT().ListRecipeSteps(gomock.Any(), int64(7)).Return(rows, nil)

		got, err := svc.ListRecipeSteps(context.Background(), 7)
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, []int32{1, 2, 3},
			[]int32{got[0].StepNumber, got[1].StepNumber, got[2].StepNumber})
		assert.Equal(t, "Cook", got[1].Instruction)
	})

	t.Run("empty list", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRecipeSteps(gomock.Any(), int64(7)).
			Return([]sqlc.RecipeRecipeStep{}, nil)

		got, err := svc.ListRecipeSteps(context.Background(), 7)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRecipeSteps(gomock.Any(), int64(7)).Return(nil, errDB)

		_, err := svc.ListRecipeSteps(context.Background(), 7)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "list recipe steps:")
	})
}

func TestUpdateRecipeStep(t *testing.T) {
	t.Run("success passes mapped params", func(t *testing.T) {
		svc, mq := newService(t)
		want := sqlc.UpdateRecipeStepParams{
			StepID:      11,
			StepNumber:  2,
			Instruction: "Fold gently",
			UpdatedBy:   textOrNull("bob"),
		}
		mq.EXPECT().UpdateRecipeStep(gomock.Any(), want).Return(nil)

		require.NoError(t, svc.UpdateRecipeStep(context.Background(), 11, 2, "Fold gently", "bob"))
	})

	t.Run("reorder step number", func(t *testing.T) {
		svc, mq := newService(t)
		want := sqlc.UpdateRecipeStepParams{
			StepID:      13,
			StepNumber:  1,
			Instruction: "Serve",
			UpdatedBy:   textOrNull("bob"),
		}
		mq.EXPECT().UpdateRecipeStep(gomock.Any(), want).Return(nil)

		require.NoError(t, svc.UpdateRecipeStep(context.Background(), 13, 1, "Serve", "bob"))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateRecipeStep(gomock.Any(), gomock.Any()).Return(errDB)

		assert.ErrorIs(t, svc.UpdateRecipeStep(context.Background(), 11, 2, "x", "bob"), errDB)
	})
}

func TestDeleteRecipeStep(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRecipeStep(gomock.Any(), int64(11)).Return(nil)

		require.NoError(t, svc.DeleteRecipeStep(context.Background(), 11))
	})

	t.Run("error propagates", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRecipeStep(gomock.Any(), int64(11)).Return(errDB)

		assert.ErrorIs(t, svc.DeleteRecipeStep(context.Background(), 11), errDB)
	})
}

func TestCountRecipes(t *testing.T) {
	t.Run("success returns count", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountRecipes(gomock.Any(), true).Return(int64(42), nil)

		n, err := svc.CountRecipes(context.Background(), true)
		require.NoError(t, err)
		assert.Equal(t, int64(42), n)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountRecipes(gomock.Any(), false).Return(int64(0), errDB)

		_, err := svc.CountRecipes(context.Background(), false)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "count recipes:")
	})
}

func TestGetRecipesByIDs(t *testing.T) {
	t.Run("success maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipesByIDs(gomock.Any(), []int64{7, 8}).Return([]sqlc.RecipeRecipe{
			{RecipeID: 7, Name: "A", IsActive: true},
			{RecipeID: 8, Name: "B", IsActive: false},
		}, nil)

		got, err := svc.GetRecipesByIDs(context.Background(), []int64{7, 8})
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(7), got[0].RecipeID)
		assert.Equal(t, "A", got[0].Name)
		assert.Equal(t, int64(8), got[1].RecipeID)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRecipesByIDs(gomock.Any(), gomock.Any()).Return(nil, errDB)

		_, err := svc.GetRecipesByIDs(context.Background(), []int64{7})
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.ErrorContains(t, err, "get recipes by ids:")
	})
}
