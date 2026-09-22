package recipeimport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

// memoryStore implements Store with the same transition guards as the SQL
// store so unit tests exercise the real state machine.
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
	return r, nil
}

func (m *memoryStore) Create(_ context.Context, ri RecipeImport) (*RecipeImport, error) {
	ri.ID = m.nextID
	m.nextID++
	ri.Status = StatusPending
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

func (m *memoryStore) ResetProcessing(context.Context) error {
	for _, r := range m.rows {
		if r.Status == StatusProcessing {
			r.Status = StatusPending
		}
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

func (m *memoryStore) UpdateReview(_ context.Context, id int64, json []byte, status Status, updatedBy string) error {
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

type fakeInventory struct {
	units       []inventory.Unit
	items       []inventory.Item
	ingredients []inventory.Ingredient
}

func (f *fakeInventory) ListUnits(context.Context) ([]inventory.Unit, error) { return f.units, nil }
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

type fakeRecipeWriter struct{ created recipe.Recipe }

func (f *fakeRecipeWriter) CreateRecipeWithChildren(_ context.Context, arg recipe.Recipe, _ []recipe.RecipeItem, _ []recipe.RecipeStep, _ string) (recipe.Recipe, error) {
	f.created = arg
	f.created.RecipeID = 99
	return f.created, nil
}

func TestService_CreateAndGet(t *testing.T) {
	svc := &Service{store: newMemoryStore()}
	ctx := context.Background()

	created, err := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, StatusPending, created.Status)
	assert.Equal(t, "scan.pdf", created.SourceFilename)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestService_Reject(t *testing.T) {
	svc := &Service{store: newMemoryStore()}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")

	require.NoError(t, svc.Reject(ctx, ri.ID))
	got, _ := svc.Get(ctx, ri.ID)
	assert.Equal(t, StatusRejected, got.Status)

	// Terminal: a second reject is a conflict.
	assert.ErrorIs(t, svc.Reject(ctx, ri.ID), domainerr.ErrConflict)
}

func TestService_Retry(t *testing.T) {
	svc := &Service{store: newMemoryStore()}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")

	// Retry while pending is a conflict — only failed/profanity may retry.
	assert.ErrorIs(t, svc.Retry(ctx, ri.ID), domainerr.ErrConflict)

	// Move to processing via claim, then fail it, then retry works.
	_, err := svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.MarkFailed(ctx, ri.ID, "boom"))
	require.NoError(t, svc.Retry(ctx, ri.ID))

	got, _ := svc.Get(ctx, ri.ID)
	assert.Equal(t, StatusPending, got.Status)
}

func TestService_UpdateReview(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup", Abbreviation: "c"}},
		items: []inventory.Item{{ItemID: 10, Name: "Flour"}},
	}
	svc := &Service{store: newMemoryStore(), inv: inv}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")

	// Worker path: claim and draft first so the review transition is legal.
	_, err := svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, ri.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, ri.ID, []byte("{}")))

	review := &ocrimport.ReviewRecipe{
		Name: "Pancakes",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
				ItemID:    "10",
				Unit:      "cup",
				UnitID:    "1",
				Status:    "accepted",
			},
		},
	}
	updated, err := svc.UpdateReview(ctx, ri.ID, review, "admin")
	require.NoError(t, err)
	assert.Equal(t, StatusReady, updated.Status)

	// The stored review is approved once fully resolved.
	var stored ocrimport.ReviewRecipe
	require.NoError(t, json.Unmarshal(updated.ReviewJSON, &stored))
	assert.True(t, stored.Approved)
}

func TestService_UpdateReview_SuggestedIsNotReady(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup", Abbreviation: "c"}},
		items: []inventory.Item{{ItemID: 10, Name: "Flour"}},
	}
	svc := &Service{store: newMemoryStore(), inv: inv}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")
	_, err := svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, ri.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, ri.ID, []byte("{}")))

	review := &ocrimport.ReviewRecipe{
		Name: "Pancakes",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
				ItemID:    "10",
				Unit:      "cup",
				UnitID:    "1",
				Status:    "suggested", // fuzzy suggestion: not resolved
			},
		},
	}
	updated, err := svc.UpdateReview(ctx, ri.ID, review, "admin")
	require.NoError(t, err)
	assert.Equal(t, StatusReviewing, updated.Status)

	var stored ocrimport.ReviewRecipe
	require.NoError(t, json.Unmarshal(updated.ReviewJSON, &stored))
	assert.False(t, stored.Approved)
}

func TestService_Approve(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup", Abbreviation: "c"}},
		items: []inventory.Item{{ItemID: 10, Name: "Flour"}},
	}
	recWriter := &fakeRecipeWriter{}
	svc := &Service{store: newMemoryStore(), inv: inv, rec: recWriter}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")

	reviewJSON := `{"name":"Pancakes","approved":true,"items":[{"draftItem":{"ingredient":"flour"},"itemId":"10","unit":"cup","unitId":"1","status":"accepted","approved":true}],"steps":[]}`
	_, err := svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, ri.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, ri.ID, []byte("{}")))
	require.NoError(t, svc.store.UpdateReview(ctx, ri.ID, []byte(reviewJSON), StatusReady, "admin"))

	admin := currentuser.User{UserID: 1, Email: "admin@example.com", IsAdmin: true}
	recipe, updated, err := svc.Approve(ctx, ri.ID, admin)
	require.NoError(t, err)
	assert.Equal(t, "Pancakes", recipe.Name)
	assert.Equal(t, StatusPersisted, updated.Status)

	// A second approve is a conflict, not a duplicate recipe.
	_, _, err = svc.Approve(ctx, ri.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrConflict)
}

func TestService_Approve_RequiresApproved(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup", Abbreviation: "c"}},
		items: []inventory.Item{{ItemID: 10, Name: "Flour"}},
	}
	svc := &Service{store: newMemoryStore(), inv: inv, rec: &fakeRecipeWriter{}}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")

	// Fully resolved but the review itself is not approved.
	reviewJSON := `{"name":"Pancakes","items":[{"draftItem":{"ingredient":"flour"},"itemId":"10","unit":"cup","unitId":"1","status":"accepted"}],"steps":[]}`
	_, err := svc.store.Claim(ctx, ri.ID)
	require.NoError(t, err)
	require.NoError(t, svc.store.UpdateOCR(ctx, ri.ID, "text", nil))
	require.NoError(t, svc.store.UpdateDraft(ctx, ri.ID, []byte("{}")))
	require.NoError(t, svc.store.UpdateReview(ctx, ri.ID, []byte(reviewJSON), StatusReady, "admin"))

	admin := currentuser.User{UserID: 1, Email: "admin@example.com", IsAdmin: true}
	_, _, err = svc.Approve(ctx, ri.ID, admin)
	assert.ErrorIs(t, err, domainerr.ErrValidation)
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
