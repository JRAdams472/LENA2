package recipe

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe/sqlc"
)

func s2p(v string) *string { return &v }

func f2p(v float64) *float64 { return &v }

func baseItems() []RecipeItem {
	return []RecipeItem{
		{RecipeItemID: 11, RecipeID: 7, IngredientID: i64(100), ItemID: i64(5), Quantity: 2, UnitID: 3, DisplayOrder: 1, Notes: "fresh"},
		{RecipeItemID: 12, RecipeID: 7, IngredientID: i64(101), Quantity: 1, UnitID: 3, DisplayOrder: 2},
		{RecipeItemID: 13, RecipeID: 7, ItemID: i64(9), Quantity: 4, UnitID: 4, DisplayOrder: 3, IsOptional: true},
	}
}

func baseSteps() []RecipeStep {
	return []RecipeStep{
		{StepID: 21, RecipeID: 7, StepNumber: 1, Instruction: "mix", DurationMinutes: i32(5)},
		{StepID: 22, RecipeID: 7, StepNumber: 2, Instruction: "bake", IsPassive: true},
		{StepID: 23, RecipeID: 7, StepNumber: 3, Instruction: "serve"},
	}
}

func TestApplyDeltaNil(t *testing.T) {
	items, steps := baseItems(), baseSteps()
	out := ApplyDelta(items, steps, nil)
	assert.Equal(t, items, out.Items)
	assert.Equal(t, steps, out.Steps)
	assert.Zero(t, out.OrphanedItems)
	assert.Zero(t, out.OrphanedSteps)
}

func TestApplyDeltaSubstitute(t *testing.T) {
	out := ApplyDelta(baseItems(), nil, &RecipeDelta{Items: []DeltaItem{
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(12), IngredientID: i64(202), Quantity: f2p(3)},
	}})
	require.Len(t, out.Items, 3)
	got := out.Items[1]
	assert.Equal(t, "substitute", got.DeltaKind)
	assert.Equal(t, i64(202), got.IngredientID)
	assert.Nil(t, got.ItemID) // substitute clears the old preferred-brand item
	assert.Equal(t, float64(3), got.Quantity)
	assert.Equal(t, int64(12), got.RecipeItemID)
}

func TestApplyDeltaSubstituteToItem(t *testing.T) {
	out := ApplyDelta(baseItems(), nil, &RecipeDelta{Items: []DeltaItem{
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(12), ItemID: i64(55)},
	}})
	got := out.Items[1]
	assert.Equal(t, i64(55), got.ItemID)
	assert.Nil(t, got.IngredientID)
	assert.Equal(t, float64(1), got.Quantity) // untouched columns keep base
}

func TestApplyDeltaAdjust(t *testing.T) {
	out := ApplyDelta(baseItems(), nil, &RecipeDelta{Items: []DeltaItem{
		{Kind: DeltaItemAdjust, RecipeItemID: i64(11), Quantity: f2p(6), Notes: s2p("")},
	}})
	got := out.Items[0]
	assert.Equal(t, "adjust", got.DeltaKind)
	assert.Equal(t, float64(6), got.Quantity)
	assert.Equal(t, "", got.Notes) // empty string clears
	assert.Equal(t, i64(5), got.ItemID)
	assert.Equal(t, i64(100), got.IngredientID)
}

func TestApplyDeltaRemove(t *testing.T) {
	out := ApplyDelta(baseItems(), nil, &RecipeDelta{Items: []DeltaItem{
		{Kind: DeltaItemRemove, RecipeItemID: i64(12)},
	}})
	require.Len(t, out.Items, 2)
	assert.Equal(t, int64(11), out.Items[0].RecipeItemID)
	assert.Equal(t, int64(13), out.Items[1].RecipeItemID)
}

func TestApplyDeltaAdd(t *testing.T) {
	out := ApplyDelta(baseItems(), nil, &RecipeDelta{Items: []DeltaItem{
		{Kind: DeltaItemAdd, IngredientID: i64(300), Quantity: f2p(1), UnitID: i64(3), DisplayOrder: i32(2)},
	}})
	require.Len(t, out.Items, 4)
	assert.Equal(t, i64(300), out.Items[2].IngredientID) // ordered in by display_order
	assert.Equal(t, "add", out.Items[2].DeltaKind)
	assert.Zero(t, out.Items[2].RecipeItemID)
}

func TestApplyDeltaOrphans(t *testing.T) {
	out := ApplyDelta(baseItems(), nil, &RecipeDelta{Items: []DeltaItem{
		{Kind: DeltaItemRemove, RecipeItemID: nil},                                 // orphaned by base edit
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(12)},                         // refs SET NULL'd → degenerate
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(13), IngredientID: i64(999)}, // healthy
	}})
	require.Len(t, out.Items, 3)
	assert.Equal(t, 2, out.OrphanedItems)
	assert.Equal(t, "", out.Items[1].DeltaKind) // line 12 survives — degenerate sub skipped
	assert.Equal(t, i64(999), out.Items[2].IngredientID)
}

func TestApplyDeltaSteps(t *testing.T) {
	out := ApplyDelta(nil, baseSteps(), &RecipeDelta{Steps: []DeltaStep{
		{Kind: DeltaStepReplace, StepID: i64(22), Instruction: s2p("bake 30 min"), DurationMinutes: i32(30)},
		{Kind: DeltaStepRemove, StepID: i64(23)},
		{Kind: DeltaStepAdd, StepNumber: i32(2), Instruction: s2p("rest")},
	}})
	require.Len(t, out.Steps, 3)
	assert.Equal(t, "mix", out.Steps[0].Instruction)
	assert.Equal(t, "rest", out.Steps[1].Instruction) // add slots in at position 2
	assert.Equal(t, "add", out.Steps[1].DeltaKind)
	assert.Equal(t, int32(2), out.Steps[1].StepNumber) // renumbered sequentially
	assert.Equal(t, "bake 30 min", out.Steps[2].Instruction)
	assert.Equal(t, "replace", out.Steps[2].DeltaKind)
	assert.Equal(t, int32(3), out.Steps[2].StepNumber)
	assert.Equal(t, 0, out.OrphanedSteps)

	orphan := ApplyDelta(nil, baseSteps(), &RecipeDelta{Steps: []DeltaStep{
		{Kind: DeltaStepRemove, StepID: nil},
	}})
	assert.Equal(t, 3, len(orphan.Steps))
	assert.Equal(t, 1, orphan.OrphanedSteps)
}

func TestDeltaStale(t *testing.T) {
	base := time.Now()
	d := RecipeDelta{BaseUpdatedAt: base}
	assert.False(t, d.Stale(nil))
	earlier := base.Add(-time.Hour)
	assert.False(t, d.Stale(&earlier))
	later := base.Add(time.Hour)
	assert.True(t, d.Stale(&later))
}

// ---------- service ----------

func deltaRow() sqlc.RecipeRecipeDeltum {
	return sqlc.RecipeRecipeDeltum{
		RecipeDeltaID: 40,
		RecipeID:      7,
		HouseholdID:   3,
		BaseUpdatedAt: time.Now(),
		CreatedBy:     "alice",
		CreatedAt:     time.Now(),
		UpdatedBy:     pgtype.Text{String: "alice", Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func TestSetRecipeDelta(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().GetRecipeByID(gomock.Any(), int64(7)).Return(recipeRow(), nil)
	mq.EXPECT().UpsertRecipeDelta(gomock.Any(), sqlc.UpsertRecipeDeltaParams{
		RecipeID: 7, HouseholdID: 3, CreatedBy: "alice",
	}).Return(deltaRow(), nil)
	mq.EXPECT().ReplaceDeltaItems(gomock.Any(), int64(40)).Return(nil)
	mq.EXPECT().AddDeltaItem(gomock.Any(), gomock.Any()).Return(sqlc.RecipeRecipeDeltaItem{}, nil)
	mq.EXPECT().ReplaceDeltaSteps(gomock.Any(), int64(40)).Return(nil)

	d, err := svc.SetRecipeDelta(context.Background(), 7, 3, []DeltaItem{
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(11), IngredientID: i64(202)},
	}, nil, "alice")
	require.NoError(t, err)
	assert.Equal(t, int64(40), d.RecipeDeltaID)
	assert.Equal(t, int64(7), d.RecipeID)
}

func TestSetRecipeDeltaValidation(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().GetRecipeByID(gomock.Any(), int64(7)).Return(recipeRow(), nil).Times(3)

	// substitute needs exactly one target
	_, err := svc.SetRecipeDelta(context.Background(), 7, 3, []DeltaItem{
		{Kind: DeltaItemSubstitute, RecipeItemID: i64(11), ItemID: i64(1), IngredientID: i64(2)},
	}, nil, "alice")
	var ve *domainerr.ValidationError
	require.ErrorAs(t, err, &ve)

	// adjust can't carry refs
	_, err = svc.SetRecipeDelta(context.Background(), 7, 3, []DeltaItem{
		{Kind: DeltaItemAdjust, RecipeItemID: i64(11), ItemID: i64(1)},
	}, nil, "alice")
	require.ErrorAs(t, err, &ve)

	// add needs qty + unit + ref
	_, err = svc.SetRecipeDelta(context.Background(), 7, 3, []DeltaItem{
		{Kind: DeltaItemAdd, IngredientID: i64(2)},
	}, nil, "alice")
	require.ErrorAs(t, err, &ve)
}

func TestClearRecipeDelta(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().DeleteRecipeDelta(gomock.Any(), sqlc.DeleteRecipeDeltaParams{RecipeID: 7, HouseholdID: 3}).Return(int64(1), nil)
	require.NoError(t, svc.ClearRecipeDelta(context.Background(), 7, 3))

	mq2svc, mq2 := newService(t)
	mq2.EXPECT().DeleteRecipeDelta(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	err := mq2svc.ClearRecipeDelta(context.Background(), 7, 3)
	require.ErrorIs(t, err, domainerr.ErrNotFound)
}

func TestAcknowledgeRecipeDelta(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().AcknowledgeRecipeDelta(gomock.Any(), sqlc.AcknowledgeRecipeDeltaParams{
		RecipeID: 7, HouseholdID: 3, UpdatedBy: pgtype.Text{String: "alice", Valid: true},
	}).Return(int64(1), nil)
	require.NoError(t, svc.AcknowledgeRecipeDelta(context.Background(), 7, 3, "alice"))
}

func TestListRecipeDeltasAssembles(t *testing.T) {
	svc, mq := newService(t)
	mq.EXPECT().ListRecipeDeltas(gomock.Any(), sqlc.ListRecipeDeltasParams{
		HouseholdID: 3, Column2: []int64{7},
	}).Return([]sqlc.RecipeRecipeDeltum{deltaRow()}, nil)
	mq.EXPECT().ListDeltaItemsByDeltas(gomock.Any(), []int64{40}).Return([]sqlc.RecipeRecipeDeltaItem{
		{DeltaItemID: 50, RecipeDeltaID: 40, RecipeItemID: pgtype.Int8{Int64: 11, Valid: true}, Kind: "adjust",
			Quantity: mustNum(6)},
	}, nil)
	mq.EXPECT().ListDeltaStepsByDeltas(gomock.Any(), []int64{40}).Return([]sqlc.RecipeRecipeDeltaStep{
		{DeltaStepID: 60, RecipeDeltaID: 40, StepID: pgtype.Int8{Int64: 21, Valid: true}, Kind: "remove"},
	}, nil)

	deltas, err := svc.ListRecipeDeltas(context.Background(), 3, []int64{7})
	require.NoError(t, err)
	d := deltas[7]
	require.NotNil(t, d)
	require.Len(t, d.Items, 1)
	assert.Equal(t, "adjust", d.Items[0].Kind)
	assert.Equal(t, i64(11), d.Items[0].RecipeItemID)
	require.NotNil(t, d.Items[0].Quantity)
	assert.Equal(t, float64(6), *d.Items[0].Quantity)
	require.Len(t, d.Steps, 1)
	assert.Equal(t, "remove", d.Steps[0].Kind)
}
