package recipeimport

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
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

// fakeInventory is a read-only InventoryReader double for pure unit tests.
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
