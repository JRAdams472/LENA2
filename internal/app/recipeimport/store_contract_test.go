package recipeimport

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
)

// storeFixture builds a fresh Store plus FK-valid recipe and user ids — the
// SQL store enforces those FKs on SetPersisted; the memory double ignores
// them.
type storeFixture func(t *testing.T) (Store, int64, int64)

// runStoreContract pins the behavior every Store implementation must
// satisfy: the transition table, single-claim semantics, terminal states,
// and status filtering. It runs against both the in-memory double (unit)
// and the real SQL store (integration) so the double cannot silently
// diverge from the SQL guards it simulates.
func runStoreContract(t *testing.T, fixture storeFixture) {
	t.Helper()
	ctx := context.Background()

	t.Run("get missing is not found", func(t *testing.T) {
		store, _, _ := fixture(t)
		_, err := store.Get(ctx, 424242)
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("claim is single-winner", func(t *testing.T) {
		store, _, _ := fixture(t)
		ri := mustCreateImport(t, ctx, store)
		claimed, err := store.Claim(ctx, ri.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusProcessing, claimed.Status)
		_, err = store.Claim(ctx, ri.ID)
		assert.ErrorIs(t, err, domainerr.ErrConflict)
	})

	t.Run("claim missing is not claimable", func(t *testing.T) {
		store, _, _ := fixture(t)
		_, err := store.Claim(ctx, 424242)
		assert.True(t, errors.Is(err, domainerr.ErrConflict) || errors.Is(err, domainerr.ErrNotFound),
			"missing row must not be claimable, got %v", err)
	})

	// Full transition matrix: for every source status, every mutator must
	// succeed exactly when the transition table allows it.
	mutators := []struct {
		name string
		to   Status
		exec func(Store, int64, int64, int64) error
	}{
		{"claim", StatusProcessing, func(s Store, id, _, _ int64) error {
			_, err := s.Claim(ctx, id)
			return err
		}},
		{"updateOCR", StatusOCRED, func(s Store, id, _, _ int64) error {
			return s.UpdateOCR(ctx, id, "ocr", nil)
		}},
		{"updateDraft", StatusDrafted, func(s Store, id, _, _ int64) error {
			return s.UpdateDraft(ctx, id, []byte("{}"))
		}},
		{"updateReviewReady", StatusReady, func(s Store, id, _, _ int64) error {
			return s.UpdateReview(ctx, id, []byte("{}"), StatusReady, "contract")
		}},
		{"updateReviewReviewing", StatusReviewing, func(s Store, id, _, _ int64) error {
			return s.UpdateReview(ctx, id, []byte("{}"), StatusReviewing, "contract")
		}},
		{"markFailed", StatusFailed, func(s Store, id, _, _ int64) error {
			return s.MarkFailed(ctx, id, "boom")
		}},
		{"markProfanity", StatusProfanity, func(s Store, id, _, _ int64) error {
			return s.MarkProfanity(ctx, id, "bad words")
		}},
		{"setPending", StatusPending, func(s Store, id, _, _ int64) error {
			return s.SetPending(ctx, id)
		}},
		{"setRejected", StatusRejected, func(s Store, id, _, _ int64) error {
			return s.SetRejected(ctx, id)
		}},
		{"setPersisted", StatusPersisted, func(s Store, id, recipeID, userID int64) error {
			return s.SetPersisted(ctx, id, recipeID, userID)
		}},
	}

	sources := []Status{
		StatusPending, StatusProcessing, StatusOCRED, StatusDrafted,
		StatusReviewing, StatusReady, StatusProfanity, StatusFailed,
		StatusRejected, StatusPersisted,
	}
	for _, from := range sources {
		for _, m := range mutators {
			from, m := from, m
			t.Run(fmt.Sprintf("transition %s via %s", from, m.name), func(t *testing.T) {
				store, recipeID, userID := fixture(t)
				ri := mustCreateImport(t, ctx, store)
				walkImportTo(t, ctx, store, ri.ID, from, recipeID, userID)

				err := m.exec(store, ri.ID, recipeID, userID)
				if canTransition(from, m.to) {
					require.NoError(t, err)
					got, gerr := store.Get(ctx, ri.ID)
					require.NoError(t, gerr)
					assert.Equal(t, m.to, got.Status)
				} else {
					assert.ErrorIs(t, err, domainerr.ErrConflict,
						"%s -> %s must be rejected", from, m.to)
				}
			})
		}
	}

	t.Run("claimable ids exclude terminal and in-flight", func(t *testing.T) {
		store, recipeID, userID := fixture(t)
		pending := mustCreateImport(t, ctx, store)
		ocred := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, ocred.ID, StatusOCRED, recipeID, userID)
		ready := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, ready.ID, StatusReady, recipeID, userID)
		rejected := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, rejected.ID, StatusRejected, recipeID, userID)
		persisted := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, persisted.ID, StatusPersisted, recipeID, userID)
		processing := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, processing.ID, StatusProcessing, recipeID, userID)

		ids, err := store.ListClaimableIDs(ctx)
		require.NoError(t, err)
		assert.Contains(t, ids, pending.ID)
		assert.Contains(t, ids, ocred.ID)
		assert.NotContains(t, ids, ready.ID)
		assert.NotContains(t, ids, rejected.ID)
		assert.NotContains(t, ids, persisted.ID)
		assert.NotContains(t, ids, processing.ID)
	})

	t.Run("reset processing returns rows to pending", func(t *testing.T) {
		store, recipeID, userID := fixture(t)
		processing := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, processing.ID, StatusProcessing, recipeID, userID)
		ready := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, ready.ID, StatusReady, recipeID, userID)

		require.NoError(t, store.ResetProcessing(ctx))

		got, err := store.Get(ctx, processing.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, got.Status)
		got, err = store.Get(ctx, ready.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusReady, got.Status, "non-processing rows untouched")
	})

	t.Run("list by statuses filters and paginates", func(t *testing.T) {
		store, recipeID, userID := fixture(t)
		a := mustCreateImport(t, ctx, store)
		b := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, b.ID, StatusProcessing, recipeID, userID)
		c := mustCreateImport(t, ctx, store)
		walkImportTo(t, ctx, store, c.ID, StatusRejected, recipeID, userID)

		// The SQL store shares the database with other contract cases, so
		// assert on membership and on count/list agreement rather than an
		// absolute row count.
		rows, err := store.ListByStatuses(ctx, []Status{StatusPending, StatusProcessing}, 10, 0)
		require.NoError(t, err)
		gotIDs := map[int64]bool{}
		for _, r := range rows {
			gotIDs[r.ID] = true
		}
		assert.True(t, gotIDs[a.ID])
		assert.True(t, gotIDs[b.ID])
		assert.False(t, gotIDs[c.ID], "rejected row must be filtered out")

		all, err := store.ListByStatuses(ctx, []Status{StatusPending, StatusProcessing}, 10000, 0)
		require.NoError(t, err)
		n, err := store.CountByStatuses(ctx, []Status{StatusPending, StatusProcessing})
		require.NoError(t, err)
		assert.Equal(t, int64(len(all)), n, "count must match the filtered list")

		page, err := store.ListByStatuses(ctx, []Status{StatusPending, StatusProcessing}, 1, 1)
		require.NoError(t, err)
		assert.Len(t, page, 1, "limit/offset must page the merged result set")
	})
}

// mustCreateImport creates a pending import row through the Store under test.
func mustCreateImport(t *testing.T, ctx context.Context, store Store) *RecipeImport {
	t.Helper()
	ri, err := store.Create(ctx, RecipeImport{
		SourceFilename: "contract.pdf",
		SourcePath:     "/inbox/contract.pdf",
		Status:         StatusPending,
		CreatedBy:      "contract",
		UpdatedBy:      "contract",
	})
	require.NoError(t, err)
	return ri
}

// walkImportTo drives a row to the requested status through only legal
// transitions, so contract cases start from real states rather than seeded
// shortcuts.
func walkImportTo(t *testing.T, ctx context.Context, store Store, id int64, to Status, recipeID, userID int64) {
	t.Helper()
	must := func(err error) { require.NoError(t, err) }
	switch to {
	case StatusPending:
	case StatusProcessing:
		_, err := store.Claim(ctx, id)
		must(err)
	case StatusOCRED:
		walkImportTo(t, ctx, store, id, StatusProcessing, recipeID, userID)
		must(store.UpdateOCR(ctx, id, "ocr", nil))
	case StatusDrafted:
		walkImportTo(t, ctx, store, id, StatusOCRED, recipeID, userID)
		must(store.UpdateDraft(ctx, id, []byte("{}")))
	case StatusReviewing, StatusReady:
		walkImportTo(t, ctx, store, id, StatusDrafted, recipeID, userID)
		must(store.UpdateReview(ctx, id, []byte("{}"), to, "contract"))
	case StatusFailed:
		walkImportTo(t, ctx, store, id, StatusProcessing, recipeID, userID)
		must(store.MarkFailed(ctx, id, "boom"))
	case StatusProfanity:
		walkImportTo(t, ctx, store, id, StatusProcessing, recipeID, userID)
		must(store.MarkProfanity(ctx, id, "bad words"))
	case StatusRejected:
		must(store.SetRejected(ctx, id))
	case StatusPersisted:
		walkImportTo(t, ctx, store, id, StatusReady, recipeID, userID)
		must(store.SetPersisted(ctx, id, recipeID, userID))
	default:
		t.Fatalf("walkImportTo: unhandled status %s", to)
	}
}

func TestStoreContract_Memory(t *testing.T) {
	runStoreContract(t, func(*testing.T) (Store, int64, int64) {
		return newMemoryStore(), 1, 1
	})
}

func TestIntegrationStoreContract_SQL(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	recipeSvc := recipe.NewService(pool)
	rcp, err := recipeSvc.CreateRecipeWithChildren(ctx,
		recipe.Recipe{Name: "Contract Recipe", IsActive: true}, nil, nil, "contract")
	require.NoError(t, err)
	userID := testutil.MustUser(ctx, t, pool, "store-contract@example.com")

	runStoreContract(t, func(*testing.T) (Store, int64, int64) {
		return NewStore(pool), rcp.RecipeID, userID
	})
}
