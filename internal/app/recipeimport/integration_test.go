package recipeimport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/platform/profanity"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

const itBy = "recipeimport-it"

func newIntegrationStore(t *testing.T, ctx context.Context) (Store, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewStore(pool), pool
}

func TestIntegrationStoreTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newIntegrationStore(t, ctx)

	// recipe_id and approved_by_user_id are real FKs — create real rows.
	adminID := testutil.MustUser(ctx, t, pool, "transitions-admin@example.com")
	recipeSvc := recipe.NewService(pool)
	existing, err := recipeSvc.CreateRecipeWithChildren(ctx, recipe.Recipe{Name: "IT Transition Recipe", IsActive: true}, nil, nil, itBy)
	require.NoError(t, err)

	ri, err := store.Create(ctx, RecipeImport{
		SourceFilename: "scan.pdf", SourcePath: "/inbox/scan.pdf",
		Status: StatusPending, CreatedBy: itBy,
	})
	require.NoError(t, err)

	// pending -> persisted directly is illegal.
	assert.ErrorIs(t, store.SetPersisted(ctx, ri.ID, existing.RecipeID, adminID), domainerr.ErrConflict)

	// Claim moves pending -> processing exactly once.
	claimed, err := store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusProcessing, claimed.Status)
	_, err = store.Claim(ctx, ri.ID)
	assert.ErrorIs(t, err, domainerr.ErrConflict, "second claim must lose")

	// Walk the worker stages.
	require.NoError(t, store.UpdateOCR(ctx, ri.ID, "ocr text", []byte("{}")))
	require.NoError(t, store.UpdateDraft(ctx, ri.ID, []byte("{}")))
	require.NoError(t, store.UpdateReview(ctx, ri.ID, []byte(`{"items":[]}`), StatusReady, itBy))

	// Admin-owned states cannot be regressed by worker transitions.
	assert.ErrorIs(t, store.MarkFailed(ctx, ri.ID, "boom"), domainerr.ErrConflict,
		"ready -> failed is not a worker transition")

	require.NoError(t, store.SetPersisted(ctx, ri.ID, existing.RecipeID, adminID))
	got, err := store.Get(ctx, ri.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPersisted, got.Status)
	require.NotNil(t, got.RecipeID)
	assert.Equal(t, existing.RecipeID, *got.RecipeID)

	// Persisted is terminal.
	assert.ErrorIs(t, store.SetRejected(ctx, ri.ID), domainerr.ErrConflict)
	assert.ErrorIs(t, store.SetPending(ctx, ri.ID), domainerr.ErrConflict)
}

func TestIntegrationStoreRejectAndRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, _ := newIntegrationStore(t, ctx)

	ri, err := store.Create(ctx, RecipeImport{
		SourceFilename: "scan.pdf", SourcePath: "/inbox/scan.pdf",
		Status: StatusPending, CreatedBy: itBy,
	})
	require.NoError(t, err)

	// Retry while pending is a conflict.
	assert.ErrorIs(t, store.SetPending(ctx, ri.ID), domainerr.ErrConflict)

	require.NoError(t, store.SetRejected(ctx, ri.ID))
	got, _ := store.Get(ctx, ri.ID)
	assert.Equal(t, StatusRejected, got.Status)

	// Rejected is terminal: retry does not resurrect it.
	assert.ErrorIs(t, store.SetPending(ctx, ri.ID), domainerr.ErrConflict)
}

func TestIntegrationStoreFailureThenRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, _ := newIntegrationStore(t, ctx)

	ri, err := store.Create(ctx, RecipeImport{
		SourceFilename: "scan.pdf", SourcePath: "/inbox/scan.pdf",
		Status: StatusPending, CreatedBy: itBy,
	})
	require.NoError(t, err)
	_, err = store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, store.MarkFailed(ctx, ri.ID, "ocr blew up"))

	// failed -> pending retry is legal.
	require.NoError(t, store.SetPending(ctx, ri.ID))
	got, _ := store.Get(ctx, ri.ID)
	assert.Equal(t, StatusPending, got.Status)
	assert.Empty(t, got.ErrorMessage)
}

func TestIntegrationListByStatusesPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, pool := newIntegrationStore(t, ctx)

	// Create imports in a mix of statuses by walking transitions.
	var ids []int64
	for i := 0; i < 7; i++ {
		ri, err := store.Create(ctx, RecipeImport{
			SourceFilename: fmt.Sprintf("s%d.pdf", i), SourcePath: "/x",
			Status: StatusPending, CreatedBy: itBy,
		})
		require.NoError(t, err)
		ids = append(ids, ri.ID)
	}
	_, err := store.Claim(ctx, ids[1])
	require.NoError(t, err)
	require.NoError(t, store.UpdateOCR(ctx, ids[1], "t", nil))
	_, err = store.Claim(ctx, ids[2])
	require.NoError(t, err)
	require.NoError(t, store.MarkFailed(ctx, ids[2], "x"))
	require.NoError(t, store.SetRejected(ctx, ids[3]))

	statuses := []Status{StatusPending, StatusOCRED, StatusFailed}
	total, err := store.CountByStatuses(ctx, statuses)
	require.NoError(t, err)
	// ids 0,4,5,6 pending; 1 ocred; 2 failed = 6 rows (3 rejected excluded).
	assert.Equal(t, int64(6), total)

	// Page through the merged set; no row may repeat or be skipped.
	seen := map[int64]bool{}
	page, err := store.ListByStatuses(ctx, statuses, 4, 0)
	require.NoError(t, err)
	assert.Len(t, page, 4)
	for _, r := range page {
		seen[r.ID] = true
	}
	page2, err := store.ListByStatuses(ctx, statuses, 4, 4)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	for _, r := range page2 {
		assert.False(t, seen[r.ID], "row %d returned on two pages", r.ID)
		seen[r.ID] = true
	}
	assert.Len(t, seen, 6)

	// Deterministic order: created_at DESC matches creation order reversed.
	all, err := store.ListByStatuses(ctx, statuses, 25, 0)
	require.NoError(t, err)
	for i := 1; i < len(all); i++ {
		assert.False(t, all[i-1].CreatedAt.Before(all[i].CreatedAt))
	}
	_ = pool
}

func TestIntegrationRecoveryReclaimsOrphans(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	store, _ := newIntegrationStore(t, ctx)

	// Simulate a worker that claimed a job then died mid-stage.
	ri, err := store.Create(ctx, RecipeImport{
		SourceFilename: "scan.pdf", SourcePath: "/x",
		Status: StatusPending, CreatedBy: itBy,
	})
	require.NoError(t, err)
	_, err = store.Claim(ctx, ri.ID)
	require.NoError(t, err)

	// Startup recovery: processing orphans reset to pending and become
	// claimable again.
	require.NoError(t, store.ResetProcessing(ctx))
	ids, err := store.ListClaimableIDs(ctx)
	require.NoError(t, err)
	assert.Contains(t, ids, ri.ID)
}

// TestIntegrationPipelineEndToEnd walks create -> OCR -> draft -> review ->
// approve on the real store, then proves a double approve is a conflict that
// creates no duplicate recipe.
func TestIntegrationPipelineEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	defer cleanup()

	invSvc := inventory.NewService(pool)
	recipeSvc := recipe.NewService(pool)

	brand, err := invSvc.CreateBrand(ctx, "IT Import Brand", itBy)
	require.NoError(t, err)
	cat, err := invSvc.CreateCategory(ctx, "IT Import Category", "", itBy)
	require.NoError(t, err)
	unit, err := invSvc.GetUnitByName(ctx, "each")
	require.NoError(t, err)
	w := 1.0
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name: "IT Import Flour", BrandID: &brand.BrandID, CategoryID: cat.CategoryID,
		UnitID: unit.UnitID, NetWeight: &w,
	}, itBy)
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "scan.png")
	require.NoError(t, os.WriteFile(path, []byte("fake"), 0o600))

	svc := &Service{
		uow:       dbtx.NewUnitOfWork(pool),
		store:     NewStore(pool),
		ocr:       &fakeOCR{res: okOCR("IT Import Flour 1 each")},
		ollama:    &fakeLLM{content: `{"name":"IT Pancakes","items":[{"ingredient":"it import flour","quantity":1,"unit":"each"}],"steps":[{"stepNumber":1,"instruction":"Mix."}]}`},
		inv:       invSvc,
		rec:       recipeSvc,
		profanity: profanity.New(""),
		cfg: Config{
			OCRConfidenceThreshold:     50,
			ImportAutoAcceptConfidence: 0.92,
			ImportReviewThreshold:      0.75,
			ImportStageTimeout:         3 * time.Minute,
		},
	}

	ri, err := svc.Create(ctx, "scan.png", path, "hash", nil, itBy)
	require.NoError(t, err)
	// Create enqueues via the nil jobs channel (no workers in this literal);
	// drive the pipeline synchronously instead.
	require.NoError(t, svc.Process(ctx, ri.ID))

	got, err := svc.Get(ctx, ri.ID)
	require.NoError(t, err)
	assert.Contains(t, []Status{StatusReviewing, StatusReady}, got.Status)

	// Whatever the mapper decided, have the admin review and approve each
	// item explicitly — the human gate.
	var review ocrimport.ReviewRecipe
	require.NoError(t, json.Unmarshal(got.ReviewJSON, &review))
	for i := range review.Items {
		review.Items[i].ItemID = fmt.Sprintf("%d", item.ItemID)
		review.Items[i].Unit = "each"
		review.Items[i].UnitID = fmt.Sprintf("%d", unit.UnitID)
		review.Items[i].Approved = true
	}
	updated, err := svc.UpdateReview(ctx, ri.ID, &review, itBy)
	require.NoError(t, err)
	assert.Equal(t, StatusReady, updated.Status)

	// approved_by_user_id is a real FK — create the admin user.
	adminID := testutil.MustUser(ctx, t, pool, "import-admin@example.com")
	admin := currentuser.User{UserID: adminID, Email: "import-admin@example.com", IsAdmin: true}
	rcp, done, err := svc.Approve(ctx, ri.ID, admin)
	require.NoError(t, err)
	assert.Equal(t, "IT Pancakes", rcp.Name)
	assert.Equal(t, StatusPersisted, done.Status)

	// Double approve: conflict, and still exactly one recipe row.
	_, _, err = svc.Approve(ctx, ri.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict)

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM recipe.recipe WHERE name = 'IT Pancakes'`).Scan(&n))
	assert.Equal(t, 1, n)
}
