package bff

import (
	"context"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/app/recipeimport"
	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/async"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

func TestSplitDataURI(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		mediaType string
		data      string
		wantErr   bool
	}{
		{"raw base64 passes through", "iVBORw0KGgo=", "", "iVBORw0KGgo=", false},
		{"png data uri", "data:image/png;base64,iVBOR", "image/png", "iVBOR", false},
		{"jpeg data uri", "data:image/jpeg;base64,/9j/4", "image/jpeg", "/9j/4", false},
		{"data uri without base64 marker", "data:image/png,iVBOR", "image/png", "iVBOR", false},
		{"media type with params", "data:image/png;charset=utf-8;base64,abc", "image/png", "abc", false},
		{"empty media type", "data:;base64,abc", "", "abc", false},
		{"no comma is malformed", "data:image/png;base64", "", "", true},
		{"bare data prefix is malformed", "data:", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mt, data, err := splitDataURI(tc.in)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.mediaType, mt)
			assert.Equal(t, tc.data, data)
		})
	}
}

func b64PNG(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(testPNG(t, 8, 8))
}

func TestSubmitRecipeScan_Validation(t *testing.T) {
	png := b64PNG(t)

	t.Run("non-admin is forbidden", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.SubmitRecipeScan(testutil.WithUser(context.Background(), 7, "u@example.com"), struct {
			FileBase64 string
		}{FileBase64: png})
		assert.Error(t, err)
	})

	t.Run("empty input", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.SubmitRecipeScan(adminCtx(), struct{ FileBase64 string }{FileBase64: "  "})
		assert.ErrorContains(t, err, "required")
	})

	t.Run("invalid base64", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.SubmitRecipeScan(adminCtx(), struct{ FileBase64 string }{FileBase64: "!!!not-base64!!!"})
		assert.ErrorContains(t, err, "base64")
	})

	t.Run("malformed data uri", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.SubmitRecipeScan(adminCtx(), struct{ FileBase64 string }{FileBase64: "data:image/png;base64"})
		assert.ErrorContains(t, err, "data URI")
	})

	t.Run("oversized payload", func(t *testing.T) {
		r := &Resolver{RecipeScanMaxBytes: 8}
		_, err := r.SubmitRecipeScan(adminCtx(), struct{ FileBase64 string }{FileBase64: png})
		assert.ErrorContains(t, err, "maximum size")
	})

	t.Run("declared pdf but png body is rejected", func(t *testing.T) {
		r := &Resolver{}
		_, err := r.SubmitRecipeScan(adminCtx(), struct{ FileBase64 string }{
			FileBase64: "data:application/pdf;base64," + png,
		})
		assert.ErrorContains(t, err, "rejected")
	})

	t.Run("valid scan reaches the import service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		imports := mock.NewMockRecipeImportService(ctrl)
		imports.EXPECT().
			Submit(gomock.Any(), "image/png", gomock.Not(gomock.Nil()), gomock.Not(gomock.Nil()), "admin@example.com").
			Return(&recipeimport.RecipeImport{ID: 42}, nil)

		r := &Resolver{RecipeImportService: imports}
		got, err := r.SubmitRecipeScan(adminCtx(), struct{ FileBase64 string }{
			FileBase64: "data:image/png;base64," + png,
		})
		require.NoError(t, err)
		require.NotNil(t, got)
	})

	t.Run("per-user upload rate limit returns busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		imports := mock.NewMockRecipeImportService(ctrl)
		imports.EXPECT().Submit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&recipeimport.RecipeImport{ID: 1}, nil).Times(1)

		r := &Resolver{RecipeImportService: imports, uploads: newUserRateLimiter(1)}
		ctx := adminCtx()
		_, err := r.SubmitRecipeScan(ctx, struct{ FileBase64 string }{FileBase64: png})
		require.NoError(t, err)

		_, err = r.SubmitRecipeScan(ctx, struct{ FileBase64 string }{FileBase64: png})
		require.Error(t, err)
		var ce *clientError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, codeBusy, ce.code)
	})
}

// fakeOCRClient satisfies the bff OCRClient interface and signals when the
// background task ran.
type fakeOCRClient struct {
	done chan struct{}
}

func (f *fakeOCRClient) ExtractText(context.Context, []byte) (string, error) {
	select {
	case <-f.done:
	default:
		close(f.done)
	}
	return "unparseable label text", nil
}

// pendingItemOwnedBy returns a mock InventoryService exposing one pending
// item owned by userID, so canModifyItem lets a non-admin through.
func pendingItemOwnedBy(ctrl *gomock.Controller, itemID, userID int64) *mock.MockInventoryService {
	inv := mock.NewMockInventoryService(ctrl)
	inv.EXPECT().GetItemByID(gomock.Any(), itemID).Return(inventory.Item{
		ItemID:            itemID,
		Status:            inventory.ItemStatusPending,
		SubmittedByUserID: &userID,
	}, nil).AnyTimes()
	return inv
}

func nutritionArgs(itemID int64, photo string) struct {
	ItemID      graphql.ID
	PhotoBase64 string
} {
	return struct {
		ItemID      graphql.ID
		PhotoBase64 string
	}{ItemID: graphql.ID(strconv.FormatInt(itemID, 10)), PhotoBase64: photo}
}

func TestSubmitItemNutritionPhoto_Validation(t *testing.T) {
	ctx := testutil.WithUser(context.Background(), 7, "u@example.com")
	png := b64PNG(t)

	t.Run("invalid base64", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		r := &Resolver{InventoryService: pendingItemOwnedBy(ctrl, 5, 7)}
		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, "!!!"))
		assert.ErrorContains(t, err, "base64")
	})

	t.Run("oversized photo", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		r := &Resolver{InventoryService: pendingItemOwnedBy(ctrl, 5, 7), NutritionPhotoMaxBytes: 8}
		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		assert.ErrorContains(t, err, "maximum size")
	})

	t.Run("pdf is rejected as not an image", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		r := &Resolver{InventoryService: pendingItemOwnedBy(ctrl, 5, 7)}
		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, base64.StdEncoding.EncodeToString(testPDF(1))))
		assert.ErrorContains(t, err, "not a pdf")
	})

	t.Run("other user's pending item is forbidden", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		inv := mock.NewMockInventoryService(ctrl)
		owner := int64(99)
		inv.EXPECT().GetItemByID(gomock.Any(), int64(5)).Return(inventory.Item{
			ItemID:            5,
			Status:            inventory.ItemStatusPending,
			SubmittedByUserID: &owner,
		}, nil)
		r := &Resolver{InventoryService: inv}
		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		assert.Error(t, err)
	})
}

func TestSubmitItemNutritionPhoto_Queueing(t *testing.T) {
	ctx := testutil.WithUser(context.Background(), 7, "u@example.com")
	png := b64PNG(t)

	t.Run("accepted job runs the OCR pipeline", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ocr := &fakeOCRClient{done: make(chan struct{})}
		r := &Resolver{InventoryService: pendingItemOwnedBy(ctrl, 5, 7), OCRClient: ocr}
		ok, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		require.NoError(t, err)
		assert.True(t, ok)
		select {
		case <-ocr.done:
		case <-time.After(5 * time.Second):
			t.Fatal("OCR task did not run")
		}
		require.NoError(t, r.Shutdown(context.Background()))
	})

	t.Run("second in-flight job per user is busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		r := &Resolver{InventoryService: pendingItemOwnedBy(ctrl, 5, 7)}
		// Hold the user's only OCR slot.
		release := r.acquireOCRJob(7)
		require.NotNil(t, release)
		defer release()
		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		require.Error(t, err)
		var ce *clientError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, codeBusy, ce.code)
		assert.Contains(t, ce.msg, "already being processed")
	})

	t.Run("saturated worker pool returns busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		r := &Resolver{InventoryService: pendingItemOwnedBy(ctrl, 5, 7)}
		r.ensureBG()
		// Swap in a capacity-1 runner and occupy its only slot.
		r.bgRunner = async.New(1)
		release := make(chan struct{})
		require.True(t, r.bgRunner.Submit("blocker", time.Minute, func(context.Context) error {
			<-release
			return nil
		}))
		defer func() {
			close(release)
			require.NoError(t, r.Shutdown(context.Background()))
		}()

		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		require.Error(t, err)
		var ce *clientError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, codeBusy, ce.code)
		assert.Contains(t, ce.msg, "server busy")
	})

	t.Run("per-user upload rate limit returns busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		r := &Resolver{
			InventoryService: pendingItemOwnedBy(ctrl, 5, 7),
			OCRClient:        &fakeOCRClient{done: make(chan struct{})},
			uploads:          newUserRateLimiter(1),
		}
		// First call consumes the single-token bucket and queues fine.
		_, err := r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		require.NoError(t, err)
		defer func() { require.NoError(t, r.Shutdown(context.Background())) }()

		// Second call is rate limited before queueing.
		_, err = r.SubmitItemNutritionPhoto(ctx, nutritionArgs(5, png))
		require.Error(t, err)
		var ce *clientError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, codeBusy, ce.code)
	})
}
