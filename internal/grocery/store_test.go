package grocery

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/grocery/sqlc"
	"github.com/JRAdams472/LENA2/internal/grocery/sqlc/mock"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

func TestCreateStore(t *testing.T) {
	ctx := context.Background()

	t.Run("success trims name", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateStore(ctx, sqlc.CreateStoreParams{
			HouseholdID: 42, Name: "Costco", CreatedBy: "tester",
			UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.GroceryStore{StoreID: 9, HouseholdID: 42, Name: "Costco"}, nil)

		got, err := s.CreateStore(ctx, 42, "  Costco  ", "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(9), got.StoreID)
		assert.Equal(t, "Costco", got.Name)
	})

	t.Run("empty name rejected without query", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.CreateStore(ctx, 42, "   ", "tester")
		assert.Error(t, err)
	})

	t.Run("duplicate name surfaces conflict", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().CreateStore(ctx, gomock.Any()).Return(sqlc.GroceryStore{}, errDB)
		_, err := s.CreateStore(ctx, 42, "Costco", "tester")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestSetGroceryListStore(t *testing.T) {
	ctx := context.Background()
	storeID := int64(7)

	t.Run("verifies store ownership then sets", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetStoreByID(ctx, sqlc.GetStoreByIDParams{StoreID: 7, HouseholdID: 42}).
			Return(sqlc.GroceryStore{StoreID: 7, HouseholdID: 42}, nil)
		mq.EXPECT().SetGroceryListStore(ctx, sqlc.SetGroceryListStoreParams{
			GroceryListID: 5, HouseholdID: 42,
			StoreID:   pgtype.Int8{Int64: 7, Valid: true},
			UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(int64(1), nil)

		require.NoError(t, s.SetGroceryListStore(ctx, 5, 42, &storeID, "tester"))
	})

	t.Run("foreign store rejected before update", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetStoreByID(ctx, gomock.Any()).Return(sqlc.GroceryStore{}, pgx.ErrNoRows)
		err := s.SetGroceryListStore(ctx, 5, 42, &storeID, "tester")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("nil store clears without lookup", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().SetGroceryListStore(ctx, sqlc.SetGroceryListStoreParams{
			GroceryListID: 5, HouseholdID: 42, StoreID: pgtype.Int8{},
			UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(int64(1), nil)
		require.NoError(t, s.SetGroceryListStore(ctx, 5, 42, nil, "tester"))
	})

	t.Run("missing list is not found", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetStoreByID(ctx, gomock.Any()).Return(sqlc.GroceryStore{StoreID: 7}, nil)
		mq.EXPECT().SetGroceryListStore(ctx, gomock.Any()).Return(int64(0), nil)
		err := s.SetGroceryListStore(ctx, 5, 42, &storeID, "tester")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})
}

func TestAisleOps(t *testing.T) {
	ctx := context.Background()

	t.Run("create verifies store and trims", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetStoreByID(ctx, sqlc.GetStoreByIDParams{StoreID: 7, HouseholdID: 42}).
			Return(sqlc.GroceryStore{StoreID: 7}, nil)
		mq.EXPECT().CreateAisle(ctx, sqlc.CreateAisleParams{
			StoreID: 7, Name: "Produce", Position: 0, CreatedBy: "tester",
			UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.GroceryStoreAisle{AisleID: 3, StoreID: 7, Name: "Produce", Position: 0}, nil)

		got, err := s.CreateAisle(ctx, 7, 42, " Produce ", 0, "tester")
		require.NoError(t, err)
		assert.Equal(t, int64(3), got.AisleID)
	})

	t.Run("create on foreign store rejected", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetStoreByID(ctx, gomock.Any()).Return(sqlc.GroceryStore{}, pgx.ErrNoRows)
		_, err := s.CreateAisle(ctx, 7, 42, "Produce", 0, "tester")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("rename missing aisle is not found", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().RenameAisle(ctx, gomock.Any()).Return(int64(0), nil)
		err := s.RenameAisle(ctx, 3, 42, "Deli", "tester")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("reorder sends ordered ids", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().GetStoreByID(ctx, gomock.Any()).Return(sqlc.GroceryStore{StoreID: 7}, nil)
		mq.EXPECT().ReorderAisles(ctx, sqlc.ReorderAislesParams{
			StoreID: 7, HouseholdID: 42, AisleIds: []int64{3, 1, 2}, UpdatedBy: "tester",
		}).Return(nil)
		require.NoError(t, s.ReorderAisles(ctx, 7, 42, []int64{3, 1, 2}, "tester"))
	})
}

func TestAssignToAisle(t *testing.T) {
	ctx := context.Background()
	itemID := int64(11)
	aisleID := int64(3)

	expectStore := func(mq *mock.MockQuerier) {
		mq.EXPECT().GetStoreByID(ctx, sqlc.GetStoreByIDParams{StoreID: 7, HouseholdID: 42}).
			Return(sqlc.GroceryStore{StoreID: 7}, nil)
	}

	t.Run("item assignment", func(t *testing.T) {
		s, mq := newService(t)
		expectStore(mq)
		mq.EXPECT().AssignItemToAisle(ctx, sqlc.AssignItemToAisleParams{
			StoreID: 7, AisleID: 3, ItemID: pgtype.Int8{Int64: 11, Valid: true},
			CreatedBy: "tester", UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.GroceryAisleAssignment{}, nil)
		require.NoError(t, s.AssignToAisle(ctx, 7, 42, RouteIdentity{ItemID: &itemID}, &aisleID, "tester"))
	})

	t.Run("manual name normalized", func(t *testing.T) {
		s, mq := newService(t)
		expectStore(mq)
		mq.EXPECT().AssignManualToAisle(ctx, sqlc.AssignManualToAisleParams{
			StoreID: 7, AisleID: 3, ManualName: "cilantro",
			CreatedBy: "tester", UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.GroceryAisleAssignment{}, nil)
		require.NoError(t, s.AssignToAisle(ctx, 7, 42, RouteIdentity{ManualName: "  Cilantro "}, &aisleID, "tester"))
	})

	t.Run("nil aisle unassigns", func(t *testing.T) {
		s, mq := newService(t)
		expectStore(mq)
		mq.EXPECT().UnassignItem(ctx, sqlc.UnassignItemParams{
			StoreID: 7, HouseholdID: 42,
			ItemID:       pgtype.Int8{Int64: 11, Valid: true},
			IngredientID: pgtype.Int8{},
			ManualName:   pgtype.Text{},
		}).Return(int64(1), nil)
		require.NoError(t, s.AssignToAisle(ctx, 7, 42, RouteIdentity{ItemID: &itemID}, nil, "tester"))
	})

	t.Run("unassign miss is not found", func(t *testing.T) {
		s, mq := newService(t)
		expectStore(mq)
		mq.EXPECT().UnassignItem(ctx, gomock.Any()).Return(int64(0), nil)
		err := s.AssignToAisle(ctx, 7, 42, RouteIdentity{ItemID: &itemID}, nil, "tester")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("no identity rejected", func(t *testing.T) {
		s, mq := newService(t)
		expectStore(mq)
		err := s.AssignToAisle(ctx, 7, 42, RouteIdentity{}, &aisleID, "tester")
		assert.Error(t, err)
	})
}

func TestRouteLearning(t *testing.T) {
	ctx := context.Background()
	itemID := int64(11)

	t.Run("item observation upserts learned sum", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpsertRouteObservationItem(ctx, sqlc.UpsertRouteObservationItemParams{
			HouseholdID: 42, StoreID: 7, ItemID: pgtype.Int8{Int64: 11, Valid: true},
			LearnedSum: 0.25, CreatedBy: "tester", UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.GroceryItemRoute{}, nil)

		err := s.RecordRouteObservation(ctx, 42, 7,
			GroceryListItem{ItemID: &itemID}, 0.25, "tester")
		require.NoError(t, err)
	})

	t.Run("manual name normalized", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpsertRouteObservationManual(ctx, sqlc.UpsertRouteObservationManualParams{
			HouseholdID: 42, StoreID: 0, ManualName: "cilantro",
			LearnedSum: 0.8, CreatedBy: "tester", UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(sqlc.GroceryItemRoute{}, nil)

		err := s.RecordRouteObservation(ctx, 42, GenericStoreID,
			GroceryListItem{ManualItemName: "Cilantro "}, 0.8, "tester")
		require.NoError(t, err)
	})

	t.Run("identity-less item rejected", func(t *testing.T) {
		s, _ := newService(t)
		err := s.RecordRouteObservation(ctx, 42, 0, GroceryListItem{}, 0.5, "tester")
		assert.Error(t, err)
	})
}

func TestWriteManualRank(t *testing.T) {
	ctx := context.Background()
	itemID := int64(11)

	t.Run("item rank upsert", func(t *testing.T) {
		s, mq := newService(t)
		mq.EXPECT().UpsertManualRankItem(ctx, sqlc.UpsertManualRankItemParams{
			HouseholdID: 42, StoreID: 7, ItemID: pgtype.Int8{Int64: 11, Valid: true},
			ManualRank: pgtype.Float8{Float64: 0.5, Valid: true},
			CreatedBy:  "tester", UpdatedBy: pgtype.Text{String: "tester", Valid: true},
		}).Return(nil)
		require.NoError(t, s.WriteManualRank(ctx, 42, 7, RouteIdentity{ItemID: &itemID}, 0.5, "tester"))
	})

	t.Run("no identity rejected", func(t *testing.T) {
		s, _ := newService(t)
		assert.Error(t, s.WriteManualRank(ctx, 42, 7, RouteIdentity{}, 0.5, "tester"))
	})
}

func TestEffectiveRank(t *testing.T) {
	rank, learned := 0.9, 0.3

	t.Run("manual wins over learned", func(t *testing.T) {
		r := ItemRoute{ManualRank: &rank, LearnedMean: learned, LearnedCount: 4}
		assert.Equal(t, rank, *r.EffectiveRank())
	})

	t.Run("learned used when no manual", func(t *testing.T) {
		r := ItemRoute{LearnedMean: learned, LearnedCount: 4}
		assert.Equal(t, learned, *r.EffectiveRank())
	})

	t.Run("nil when neither", func(t *testing.T) {
		assert.Nil(t, ItemRoute{}.EffectiveRank())
	})
}

func TestToItemRoute(t *testing.T) {
	now := time.Now()
	r := toItemRoute(sqlc.GroceryItemRoute{
		HouseholdID:  42,
		StoreID:      7,
		ItemID:       pgtype.Int8{Int64: 11, Valid: true},
		LearnedSum:   1.5,
		LearnedCount: 3,
		ManualRank:   pgtype.Float8{Float64: 0.2, Valid: true},
		ManualAt:     pgtype.Timestamptz{Time: now, Valid: true},
	})
	assert.Equal(t, int64(11), *r.Identity.ItemID)
	assert.InDelta(t, 0.5, r.LearnedMean, 1e-9)
	assert.Equal(t, 0.2, *r.ManualRank)
	assert.Equal(t, now, *r.ManualAt)
}
