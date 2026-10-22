package recipeimport

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// memoryStore implements Store with the same transition guards as the SQL
// store so unit tests exercise the real state machine. Its fidelity to the
// SQL implementation is pinned by the shared contract suite in
// store_contract_test.go, which runs identical assertions against both.
type memoryStore struct {
	nextID int64
	rows   map[int64]*RecipeImport
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rows: make(map[int64]*RecipeImport), nextID: 1}
}

func (m *memoryStore) transition(id int64, to Status) (*RecipeImport, error) {
	r, ok := m.rows[id]
	if !ok {
		return nil, domainerr.ErrNotFound
	}
	if !canTransition(r.Status, to) {
		return nil, domainerr.ErrConflict
	}
	r.Status = to
	now := time.Now()
	r.UpdatedAt = &now
	return r, nil
}

func (m *memoryStore) Create(_ context.Context, ri RecipeImport) (*RecipeImport, error) {
	ri.ID = m.nextID
	m.nextID++
	ri.Status = StatusPending
	now := time.Now()
	ri.CreatedAt = now
	ri.UpdatedAt = &now
	m.rows[ri.ID] = &ri
	return &ri, nil
}

func (m *memoryStore) Get(_ context.Context, id int64) (*RecipeImport, error) {
	r, ok := m.rows[id]
	if !ok {
		return nil, domainerr.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (m *memoryStore) List(_ context.Context, status string, limit, offset int32) ([]RecipeImport, error) {
	var out []RecipeImport
	for _, r := range m.rows {
		if status != "" && string(r.Status) != status {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		out = append(out, *r)
		if int64(len(out)) >= int64(limit) {
			break
		}
	}
	return out, nil
}

func (m *memoryStore) Count(context.Context, string) (int64, error) { return int64(len(m.rows)), nil }

func (m *memoryStore) ListByStatuses(_ context.Context, statuses []Status, limit, offset int32) ([]RecipeImport, error) {
	want := make(map[Status]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}
	var out []RecipeImport
	for _, r := range m.rows {
		if !want[r.Status] {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		out = append(out, *r)
		if int64(len(out)) >= int64(limit) {
			break
		}
	}
	return out, nil
}

func (m *memoryStore) CountByStatuses(_ context.Context, statuses []Status) (int64, error) {
	want := make(map[Status]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}
	var n int64
	for _, r := range m.rows {
		if want[r.Status] {
			n++
		}
	}
	return n, nil
}

func (m *memoryStore) ListClaimableIDs(context.Context) ([]int64, error) {
	var ids []int64
	for id, r := range m.rows {
		for _, s := range claimableStatuses {
			if r.Status == s {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

func (m *memoryStore) ResetProcessing(_ context.Context, staleFor time.Duration) error {
	for _, r := range m.rows {
		if r.Status != StatusProcessing {
			continue
		}
		if staleFor > 0 && r.UpdatedAt != nil && time.Since(*r.UpdatedAt) <= staleFor {
			continue
		}
		r.Status = StatusPending
	}
	return nil
}

func (m *memoryStore) Claim(_ context.Context, id int64) (*RecipeImport, error) {
	r, ok := m.rows[id]
	if !ok {
		return nil, domainerr.ErrNotFound
	}
	claimable := false
	for _, s := range claimableStatuses {
		if r.Status == s {
			claimable = true
		}
	}
	if !claimable {
		return nil, domainerr.ErrConflict
	}
	r.Status = StatusProcessing
	now := time.Now()
	r.UpdatedAt = &now
	cp := *r
	return &cp, nil
}

func (m *memoryStore) UpdateOCR(_ context.Context, id int64, text string, json []byte) error {
	r, err := m.transition(id, StatusOCRED)
	if err != nil {
		return err
	}
	r.OCRText = text
	r.OCRJSON = json
	return nil
}

func (m *memoryStore) UpdateDraft(_ context.Context, id int64, json []byte) error {
	r, err := m.transition(id, StatusDrafted)
	if err != nil {
		return err
	}
	r.DraftJSON = json
	return nil
}

func (m *memoryStore) UpdateReview(_ context.Context, id int64, json []byte, status Status, updatedBy string, from []Status) error {
	r, ok := m.rows[id]
	if !ok {
		return domainerr.ErrNotFound
	}
	allowed := false
	for _, s := range from {
		if r.Status == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return domainerr.ErrConflict
	}
	r, err := m.transition(id, status)
	if err != nil {
		return err
	}
	r.ReviewJSON = json
	r.UpdatedBy = updatedBy
	return nil
}

func (m *memoryStore) MarkProfanity(_ context.Context, id int64, reason string) error {
	r, err := m.transition(id, StatusProfanity)
	if err != nil {
		return err
	}
	r.ProfanityFlag = true
	r.ProfanityReason = reason
	return nil
}

func (m *memoryStore) MarkFailed(_ context.Context, id int64, msg string) error {
	r, err := m.transition(id, StatusFailed)
	if err != nil {
		return err
	}
	r.ErrorMessage = msg
	return nil
}

func (m *memoryStore) SetPersisted(_ context.Context, id int64, recipeID int64, approver int64) error {
	r, err := m.transition(id, StatusPersisted)
	if err != nil {
		return err
	}
	r.RecipeID = &recipeID
	r.ApprovedByUserID = &approver
	return nil
}

func (m *memoryStore) SetRejected(_ context.Context, id int64) error {
	_, err := m.transition(id, StatusRejected)
	return err
}

func (m *memoryStore) SetPending(_ context.Context, id int64) error {
	r, err := m.transition(id, StatusPending)
	if err != nil {
		return err
	}
	r.ErrorMessage = ""
	r.ProfanityFlag = false
	r.ProfanityReason = ""
	return nil
}

func (m *memoryStore) WithTx(_ pgx.Tx) Store { return m }

// fakeInventory is a read-only InventoryReader double for pure unit tests.
type fakeInventory struct {
	units       []inventory.Unit
	items       []inventory.Item
	ingredients []inventory.Ingredient

	// scans counts snapshot builds so cache tests can assert one scan per
	// burst; ListUnits runs first and unconditionally in
	// newCatalogSnapshot, so it is the reliable probe. gate optionally
	// blocks it to force concurrency; err fails it.
	scans atomic.Int64
	gate  chan struct{}
	err   error
}

func (f *fakeInventory) ListUnits(ctx context.Context) ([]inventory.Unit, error) {
	f.scans.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.units, nil
}
func (f *fakeInventory) ListItems(context.Context, int64, int32, int32) ([]inventory.Item, error) {
	return f.items, nil
}
func (f *fakeInventory) CountItems(context.Context, int64) (int64, error) {
	return int64(len(f.items)), nil
}
func (f *fakeInventory) ListIngredients(context.Context, int32, int32) ([]inventory.Ingredient, error) {
	return f.ingredients, nil
}
func (f *fakeInventory) CountIngredients(context.Context) (int64, error) {
	return int64(len(f.ingredients)), nil
}
func (f *fakeInventory) GetItemByID(_ context.Context, id int64) (inventory.Item, error) {
	for _, it := range f.items {
		if it.ItemID == id {
			return it, nil
		}
	}
	return inventory.Item{}, errors.New("not found")
}
func (f *fakeInventory) GetIngredientByID(_ context.Context, id int64) (inventory.Ingredient, error) {
	for _, in := range f.ingredients {
		if in.IngredientID == id {
			return in, nil
		}
	}
	return inventory.Ingredient{}, errors.New("not found")
}
func (f *fakeInventory) GetUnitByID(_ context.Context, id int64) (inventory.Unit, error) {
	for _, u := range f.units {
		if u.UnitID == id {
			return u, nil
		}
	}
	return inventory.Unit{}, errors.New("not found")
}
func (f *fakeInventory) GetUnitByName(_ context.Context, name string) (inventory.Unit, error) {
	for _, u := range f.units {
		if u.Name == name {
			return u, nil
		}
	}
	return inventory.Unit{}, errors.New("not found")
}

func TestService_buildRecipe(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup", Abbreviation: "c"}},
		items: []inventory.Item{{ItemID: 10, Name: "Flour"}},
	}
	svc := &Service{inv: inv}
	s := "flour"
	q := 2.0
	review := &ocrimport.ReviewRecipe{
		Name: "Cake",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: s, Quantity: &q, Unit: &[]string{"cup"}[0]},
				ItemID:    "10",
				Unit:      "cup",
				UnitID:    "1",
			},
		},
		Steps: []ocrimport.DraftStep{{StepNumber: 1, Instruction: "Mix"}},
	}
	rcp, items, steps, err := svc.buildRecipe(context.Background(), review)
	require.NoError(t, err)
	assert.Equal(t, "Cake", rcp.Name)
	assert.Len(t, items, 1)
	assert.Equal(t, 2.0, items[0].Quantity)
	assert.Equal(t, int64(1), items[0].UnitID)
	assert.Len(t, steps, 1)
}

func TestService_buildRecipe_blankUnitDefaultsToEach(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 2, Name: "each"}},
		items: []inventory.Item{{ItemID: 10, Name: "Salt"}},
	}
	svc := &Service{inv: inv}
	review := &ocrimport.ReviewRecipe{
		Name: "Seasoning",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "salt"},
				ItemID:    "10",
				// no Unit/UnitID — "salt to taste" style rows carry no unit
				Approved: true,
			},
		},
		Steps: []ocrimport.DraftStep{{StepNumber: 1, Instruction: "Season"}},
	}
	_, items, _, err := svc.buildRecipe(context.Background(), review)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(2), items[0].UnitID)
	assert.Equal(t, 1.0, items[0].Quantity)
}

func TestService_UpdateReview_blankUnitDefaultsToEach(t *testing.T) {
	store := newMemoryStore()
	ri, err := store.Create(context.Background(), RecipeImport{})
	require.NoError(t, err)
	store.rows[ri.ID].Status = StatusReviewing

	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 2, Name: "each"}, {UnitID: 3, Name: "to taste"}},
		items: []inventory.Item{{ItemID: 10, Name: "Salt"}},
	}
	svc := &Service{inv: inv, store: store}

	review := &ocrimport.ReviewRecipe{
		Name: "Seasoning",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "salt"},
				ItemID:    "10",
				Approved:  true,
			},
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "pepper"},
				ItemID:    "10",
				Unit:      "to taste",
				Approved:  true,
			},
		},
		Steps: []ocrimport.DraftStep{{StepNumber: 1, Instruction: "Season"}},
	}
	updated, err := svc.UpdateReview(context.Background(), ri.ID, review, "tester")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, updated.Status)

	// blank unit -> 'each'; named unit -> resolved by name
	assert.Equal(t, "2", review.Items[0].UnitID)
	assert.Equal(t, "3", review.Items[1].UnitID)
}

// dupNameWriter simulates a recipe_name_key unique violation at persist time.
type dupNameWriter struct{}

func (dupNameWriter) CreateRecipeWithChildren(context.Context, recipe.Recipe, []recipe.RecipeItem, []recipe.RecipeStep, string) (recipe.Recipe, error) {
	return recipe.Recipe{}, &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "recipe_name_key",
		Message:        `duplicate key value violates unique constraint "recipe_name_key"`,
	}
}

func TestService_Approve_DuplicateNameIsValidationError(t *testing.T) {
	store := newMemoryStore()
	ri, err := store.Create(context.Background(), RecipeImport{})
	require.NoError(t, err)

	review := ocrimport.ReviewRecipe{
		Name: "Cake",
		Items: []ocrimport.MatchResult{{
			DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
			ItemID:    "10",
			UnitID:    "1",
			Status:    "accepted",
			Approved:  true,
		}},
		Steps:    []ocrimport.DraftStep{{StepNumber: 1, Instruction: "Mix"}},
		Approved: true,
	}
	rj, err := json.Marshal(review)
	require.NoError(t, err)
	store.rows[ri.ID].Status = StatusReady
	store.rows[ri.ID].ReviewJSON = rj

	svc := &Service{
		inv:   &fakeInventory{units: []inventory.Unit{{UnitID: 1, Name: "cup"}}, items: []inventory.Item{{ItemID: 10, Name: "Flour"}}},
		store: store,
		rec:   dupNameWriter{},
	}
	_, _, err = svc.Approve(context.Background(), ri.ID, currentuser.User{UserID: 1, Email: "a@b.c"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domainerr.ErrValidation), "want ValidationError, got %v", err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestService_UpdateReview_NegativeQuantityRejected(t *testing.T) {
	store := newMemoryStore()
	ri, err := store.Create(context.Background(), RecipeImport{})
	require.NoError(t, err)
	store.rows[ri.ID].Status = StatusReviewing

	svc := &Service{
		inv:   &fakeInventory{units: []inventory.Unit{{UnitID: 2, Name: "each"}}, items: []inventory.Item{{ItemID: 10, Name: "Salt"}}},
		store: store,
	}
	neg := -2.0
	review := &ocrimport.ReviewRecipe{
		Name: "Bad",
		Items: []ocrimport.MatchResult{{
			DraftItem: ocrimport.DraftItem{Ingredient: "flour", Quantity: &neg},
			ItemID:    "10",
			Approved:  true,
		}},
		Steps: []ocrimport.DraftStep{{StepNumber: 1, Instruction: "x"}},
	}
	_, err = svc.UpdateReview(context.Background(), ri.ID, review, "tester")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domainerr.ErrValidation), "want ValidationError, got %v", err)
	assert.Contains(t, err.Error(), "greater than zero")
}

func TestService_catalogSnapshot_cachesAcrossCalls(t *testing.T) {
	inv := &fakeInventory{units: []inventory.Unit{{UnitID: 1, Name: "cup"}}}
	svc := &Service{inv: inv}

	first, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)
	second, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)

	assert.Same(t, first, second)
	assert.Equal(t, int64(1), inv.scans.Load(), "two calls should scan the catalog once")
}

func TestService_catalogSnapshot_expiresAfterTTL(t *testing.T) {
	inv := &fakeInventory{units: []inventory.Unit{{UnitID: 1, Name: "cup"}}}
	now := time.Now()
	svc := &Service{inv: inv, now: func() time.Time { return now }}

	_, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)

	now = now.Add(catalogSnapshotTTL + time.Second)
	stale, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), inv.scans.Load(), "expired snapshot should rebuild")
	assert.NotNil(t, stale)
}

func TestService_InvalidateCatalog_forcesRebuild(t *testing.T) {
	inv := &fakeInventory{units: []inventory.Unit{{UnitID: 1, Name: "cup"}}}
	svc := &Service{inv: inv}

	_, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)
	svc.InvalidateCatalog()
	_, err = svc.catalogSnapshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), inv.scans.Load())
}

func TestService_catalogSnapshot_singleflight(t *testing.T) {
	gate := make(chan struct{})
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup"}},
		gate:  gate,
	}
	svc := &Service{inv: inv}

	const callers = 8
	var wg sync.WaitGroup
	snaps := make([]*ocrimport.CatalogSnapshot, callers)
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			snaps[i], errs[i] = svc.catalogSnapshot(context.Background())
		}(i)
	}
	// Let all callers reach the gate, then release the build.
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()

	for i := range callers {
		require.NoError(t, errs[i])
		assert.Same(t, snaps[0], snaps[i])
	}
	assert.Equal(t, int64(1), inv.scans.Load(), "concurrent callers share one build")
}

func TestService_UpdateReview_IngredientKindResolves(t *testing.T) {
	store := newMemoryStore()
	ri, err := store.Create(context.Background(), RecipeImport{})
	require.NoError(t, err)
	store.rows[ri.ID].Status = StatusReviewing

	// The selected id is an ingredient, not a branded item — items stays
	// empty so a GetItemByID lookup would fail where GetIngredientByID wins.
	inv := &fakeInventory{
		units:       []inventory.Unit{{UnitID: 2, Name: "each"}},
		ingredients: []inventory.Ingredient{{IngredientID: 7, Name: "flour"}},
	}
	svc := &Service{inv: inv, store: store}

	review := &ocrimport.ReviewRecipe{
		Name: "Simple",
		Items: []ocrimport.MatchResult{{
			DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
			ItemID:    "7",
			ItemKind:  "ingredient",
			Approved:  true,
		}},
		Steps: []ocrimport.DraftStep{{StepNumber: 1, Instruction: "x"}},
	}
	updated, err := svc.UpdateReview(context.Background(), ri.ID, review, "tester")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, updated.Status)
}

func TestService_UpdateReview_IngredientKindMissing(t *testing.T) {
	store := newMemoryStore()
	ri, err := store.Create(context.Background(), RecipeImport{})
	require.NoError(t, err)
	store.rows[ri.ID].Status = StatusReviewing

	svc := &Service{
		inv:   &fakeInventory{units: []inventory.Unit{{UnitID: 2, Name: "each"}}, ingredients: []inventory.Ingredient{{IngredientID: 7, Name: "flour"}}},
		store: store,
	}
	review := &ocrimport.ReviewRecipe{
		Name: "Simple",
		Items: []ocrimport.MatchResult{{
			DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
			ItemID:    "999",
			ItemKind:  "ingredient",
			Approved:  true,
		}},
		Steps: []ocrimport.DraftStep{{StepNumber: 1, Instruction: "x"}},
	}
	_, err = svc.UpdateReview(context.Background(), ri.ID, review, "tester")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domainerr.ErrValidation), "want ValidationError, got %v", err)
	assert.Contains(t, err.Error(), "no catalog ingredient")
}

func TestService_sweepClaimable_enqueuesPending(t *testing.T) {
	mem := newMemoryStore()
	ri, err := mem.Create(context.Background(), RecipeImport{Status: StatusPending})
	require.NoError(t, err)
	svc := &Service{store: mem, jobs: make(chan int64, 4)}

	svc.sweepClaimable(context.Background())
	select {
	case id := <-svc.jobs:
		assert.Equal(t, ri.ID, id)
	default:
		t.Fatal("sweep did not enqueue the pending job")
	}
}

func TestService_sweepClaimable_drainsDroppedEnqueue(t *testing.T) {
	mem := newMemoryStore()
	ri, err := mem.Create(context.Background(), RecipeImport{Status: StatusPending})
	require.NoError(t, err)
	svc := &Service{store: mem, jobs: make(chan int64, 1)}

	svc.jobs <- 9999          // saturate the bounded channel
	svc.EnqueueProcess(ri.ID) // dropped: queue full, row stays claimable
	<-svc.jobs                // worker drained the filler

	svc.sweepClaimable(context.Background())
	select {
	case id := <-svc.jobs:
		assert.Equal(t, ri.ID, id, "dropped enqueue must be re-offered by the sweep")
	case <-time.After(2 * time.Second):
		t.Fatal("sweep did not re-enqueue the dropped job")
	}
}

func TestService_catalogSnapshot_invalidateDuringBuildDiscards(t *testing.T) {
	gate := make(chan struct{})
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup"}},
		gate:  gate,
	}
	svc := &Service{inv: inv}

	type buildRes struct {
		snap *ocrimport.CatalogSnapshot
		err  error
	}
	done := make(chan buildRes, 1)
	go func() {
		snap, err := svc.catalogSnapshot(context.Background())
		done <- buildRes{snap, err}
	}()
	// Wait for the build to reach the catalog scan, then land a catalog
	// write mid-build.
	require.Eventually(t, func() bool { return inv.scans.Load() == 1 },
		2*time.Second, 5*time.Millisecond)
	svc.InvalidateCatalog()
	close(gate)
	got := <-done
	require.NoError(t, got.err)
	require.NotNil(t, got.snap)

	// The raced snapshot is returned to its caller but never installed —
	// the next call rebuilds instead of serving data that missed the write.
	svc.snapMu.RLock()
	installed := svc.snap
	svc.snapMu.RUnlock()
	assert.Nil(t, installed, "snapshot built across an invalidation must not install")

	second, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, int64(2), inv.scans.Load(), "next call must rebuild")
}

func TestService_catalogSnapshot_errorsNotCached(t *testing.T) {
	boom := errors.New("db down")
	inv := &fakeInventory{units: []inventory.Unit{{UnitID: 1, Name: "cup"}}, err: boom}
	svc := &Service{inv: inv}

	_, err := svc.catalogSnapshot(context.Background())
	require.ErrorIs(t, err, boom)

	inv.err = nil
	snap, err := svc.catalogSnapshot(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, int64(2), inv.scans.Load(), "failed builds must not be cached")
}
