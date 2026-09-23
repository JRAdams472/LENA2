package recipeimport

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

// newIntegrationService builds a Service over the real SQL store and real
// domain services so tests exercise the actual transition guards and
// transaction semantics instead of a hand-rolled in-memory double.
func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return &Service{
		uow:   dbtx.NewUnitOfWork(pool),
		store: NewStore(pool),
		inv:   inventory.NewService(pool),
		rec:   recipe.NewService(pool),
	}, pool
}

// mustImportItem creates a category + item so review ItemID references are
// real FK-valid rows, and returns the item id and the seeded "cup" unit id.
func mustImportItem(t *testing.T, ctx context.Context, svc *inventory.Service, name string) (itemID, unitID int64) {
	t.Helper()
	cat, err := svc.CreateCategory(ctx, name+" Category", "", itBy)
	require.NoError(t, err)
	unit, err := svc.GetUnitByName(ctx, "cup")
	require.NoError(t, err, "unit cup should be seeded by migration 0012")
	item, err := svc.CreateItem(ctx, inventory.Item{
		Name:       name,
		CategoryID: cat.CategoryID,
		UnitID:     unit.UnitID,
		IsMetric:   true,
	}, itBy)
	require.NoError(t, err)
	return item.ItemID, unit.UnitID
}

// walkToReady drives an import through the worker stages to ready with an
// approved review, mirroring the real pipeline order.
func walkToReady(t *testing.T, ctx context.Context, svc *Service, id int64, itemID, unitID int64) {
	t.Helper()
	reviewJSON := fmt.Sprintf(
		`{"name":"IT Recipe","approved":true,"items":[{"draftItem":{"ingredient":"flour"},"itemId":"%d","unit":"cup","unitId":"%d","status":"accepted","approved":true}],"steps":[]}`,
		itemID, unitID)
	_, err := svc.store.Claim(ctx, id)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, id, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, id, []byte("{}")))
	require.NoError(t, svc.store.UpdateReview(ctx, id, []byte(reviewJSON), StatusReady, itBy))
}

func TestIntegrationService_CreateGetRejectRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	created, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, itBy)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, created.Status)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	// Retry while pending is a conflict — only failed/profanity may retry.
	assert.ErrorIs(t, svc.Retry(ctx, created.ID), domainerr.ErrConflict)

	// Move to processing via claim, then fail, then retry works.
	_, err = svc.store.Claim(ctx, created.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.MarkFailed(ctx, created.ID, "boom"))
	require.NoError(t, svc.Retry(ctx, created.ID))

	got, err = svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, got.Status)

	require.NoError(t, svc.Reject(ctx, created.ID))
	got, err = svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRejected, got.Status)

	// Terminal: a second reject is a conflict.
	assert.ErrorIs(t, svc.Reject(ctx, created.ID), domainerr.ErrConflict)
}

func TestIntegrationService_UpdateReview(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)
	invSvc := svc.inv.(*inventory.Service)
	itemID, unitID := mustImportItem(t, ctx, invSvc, "IT Review Item")

	ri, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, itBy)
	require.NoError(t, err)
	_, err = svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, ri.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, ri.ID, []byte("{}")))

	review := &ocrimport.ReviewRecipe{
		Name: "Pancakes",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
				ItemID:    strconv.FormatInt(itemID, 10),
				Unit:      "cup",
				UnitID:    strconv.FormatInt(unitID, 10),
				Status:    "accepted",
			},
		},
	}
	updated, err := svc.UpdateReview(ctx, ri.ID, review, itBy)
	require.NoError(t, err)
	assert.Equal(t, StatusReady, updated.Status)
}

func TestIntegrationService_UpdateReview_SuggestedIsNotReady(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)
	invSvc := svc.inv.(*inventory.Service)
	itemID, unitID := mustImportItem(t, ctx, invSvc, "IT Suggested Item")

	ri, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, itBy)
	require.NoError(t, err)
	_, err = svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, ri.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, ri.ID, []byte("{}")))

	review := &ocrimport.ReviewRecipe{
		Name: "Pancakes",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
				ItemID:    strconv.FormatInt(itemID, 10),
				Unit:      "cup",
				UnitID:    strconv.FormatInt(unitID, 10),
				Status:    "suggested", // fuzzy suggestion: not resolved
			},
		},
	}
	updated, err := svc.UpdateReview(ctx, ri.ID, review, itBy)
	require.NoError(t, err)
	assert.Equal(t, StatusReviewing, updated.Status)
}

func TestIntegrationService_Approve(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)
	invSvc := svc.inv.(*inventory.Service)
	itemID, unitID := mustImportItem(t, ctx, invSvc, "IT Approve Item")
	adminID := testutil.MustUser(ctx, t, pool, "approve-admin@example.com")
	admin := currentuser.User{UserID: adminID, Email: "approve-admin@example.com", IsAdmin: true}

	ri, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, itBy)
	require.NoError(t, err)
	walkToReady(t, ctx, svc, ri.ID, itemID, unitID)

	created, updated, err := svc.Approve(ctx, ri.ID, admin)
	require.NoError(t, err)
	assert.Equal(t, "IT Recipe", created.Name)
	assert.Equal(t, StatusPersisted, updated.Status)
	require.NotNil(t, updated.RecipeID)
	assert.Equal(t, created.RecipeID, *updated.RecipeID)

	// A second approve is a conflict, not a duplicate recipe.
	_, _, err = svc.Approve(ctx, ri.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict)
}

func TestIntegrationService_Approve_NegativeStates(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, pool := newIntegrationService(t, ctx)
	invSvc := svc.inv.(*inventory.Service)
	itemID, unitID := mustImportItem(t, ctx, invSvc, "IT Negative Item")
	adminID := testutil.MustUser(ctx, t, pool, "neg-admin@example.com")
	admin := currentuser.User{UserID: adminID, Email: "neg-admin@example.com", IsAdmin: true}

	newImport := func() *RecipeImport {
		ri, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, itBy)
		require.NoError(t, err)
		return ri
	}

	// pending: never processed.
	pending := newImport()
	_, _, err := svc.Approve(ctx, pending.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict, "pending import must not approve")

	// processing: claimed by a worker, no review yet.
	processing := newImport()
	_, err = svc.store.Claim(ctx, processing.ID)
	require.NoError(t, err)
	_, _, err = svc.Approve(ctx, processing.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict, "processing import must not approve")

	// rejected: terminal.
	rejected := newImport()
	require.NoError(t, svc.Reject(ctx, rejected.ID))
	_, _, err = svc.Approve(ctx, rejected.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict, "rejected import must not approve")

	// persisted: already approved once.
	persisted := newImport()
	walkToReady(t, ctx, svc, persisted.ID, itemID, unitID)
	_, _, err = svc.Approve(ctx, persisted.ID, admin)
	require.NoError(t, err)
	_, _, err = svc.Approve(ctx, persisted.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict, "persisted import must not approve again")

	// ready but review not approved.
	notApproved := newImport()
	_, err = svc.store.Claim(ctx, notApproved.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, notApproved.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, notApproved.ID, []byte("{}")))
	unapproved := fmt.Sprintf(
		`{"name":"IT Recipe","items":[{"draftItem":{"ingredient":"flour"},"itemId":"%d","unit":"cup","unitId":"%d","status":"accepted","approved":true}],"steps":[]}`,
		itemID, unitID)
	require.NoError(t, svc.store.UpdateReview(ctx, notApproved.ID, []byte(unapproved), StatusReady, itBy))
	_, _, err = svc.Approve(ctx, notApproved.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrValidation, "unapproved review must not persist")
}

// failPersistStore wraps the real SQL store and fails only SetPersisted, to
// prove the approve transaction rolls back the recipe insert as well.
type failPersistStore struct {
	Store
	err error
}

func (f failPersistStore) SetPersisted(context.Context, int64, int64, int64) error {
	return f.err
}

func (f failPersistStore) WithTx(pgx.Tx) Store { return f }

func TestIntegrationService_Approve_RollbackOnPersistFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	svc := &Service{
		uow:   dbtx.NewUnitOfWork(pool),
		store: NewStore(pool),
		inv:   inventory.NewService(pool),
		rec:   recipe.NewService(pool),
	}
	invSvc := svc.inv.(*inventory.Service)
	itemID, unitID := mustImportItem(t, ctx, invSvc, "IT Rollback Item")
	adminID := testutil.MustUser(ctx, t, pool, "rollback-admin@example.com")
	admin := currentuser.User{UserID: adminID, Email: "rollback-admin@example.com", IsAdmin: true}

	ri, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, itBy)
	require.NoError(t, err)
	walkToReady(t, ctx, svc, ri.ID, itemID, unitID)

	// Swap in the store that fails SetPersisted inside the transaction.
	svc.store = failPersistStore{Store: svc.store, err: errors.New("persist boom")}

	_, _, err = svc.Approve(ctx, ri.ID, admin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "persist boom")

	// The import row must still be ready — the transaction rolled back.
	got, err := NewStore(pool).Get(ctx, ri.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Status)
	assert.Nil(t, got.RecipeID)

	// The recipe insert must have rolled back too: no recipe named for this run.
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM recipe.recipe WHERE name = 'IT Recipe'`).Scan(&count))
	assert.Equal(t, 0, count, "recipe insert must roll back when persist fails")
}
