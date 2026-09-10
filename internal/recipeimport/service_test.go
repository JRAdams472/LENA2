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
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

type memoryStore struct {
	nextID int64
	rows   map[int64]*RecipeImport
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rows: make(map[int64]*RecipeImport), nextID: 1}
}

func (m *memoryStore) Create(_ context.Context, ri RecipeImport) (*RecipeImport, error) {
	ri.ID = m.nextID
	m.nextID++
	ri.Status = "pending"
	m.rows[ri.ID] = &ri
	return &ri, nil
}

func (m *memoryStore) Get(_ context.Context, id int64) (*RecipeImport, error) {
	r, ok := m.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *r
	return &cp, nil
}

func (m *memoryStore) List(_ context.Context, status string, limit, offset int32) ([]RecipeImport, error) {
	var out []RecipeImport
	for _, r := range m.rows {
		if status != "" && r.Status != status {
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

func (m *memoryStore) UpdateOCR(_ context.Context, id int64, text string, json []byte) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.OCRText = text
	r.OCRJSON = json
	r.Status = "ocred"
	return nil
}

func (m *memoryStore) UpdateDraft(_ context.Context, id int64, json []byte) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.DraftJSON = json
	r.Status = "drafted"
	return nil
}

func (m *memoryStore) UpdateReview(_ context.Context, id int64, json []byte, status string, updatedBy string) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.ReviewJSON = json
	r.Status = status
	r.UpdatedBy = updatedBy
	return nil
}

func (m *memoryStore) MarkProfanity(_ context.Context, id int64, reason string) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.ProfanityFlag = true
	r.ProfanityReason = reason
	r.Status = "profanity"
	return nil
}

func (m *memoryStore) MarkFailed(_ context.Context, id int64, msg string) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.Status = "failed"
	r.ErrorMessage = msg
	return nil
}

func (m *memoryStore) SetPersisted(_ context.Context, id int64, recipeID int64, approver int64) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.RecipeID = &recipeID
	r.ApprovedByUserID = &approver
	r.Status = "persisted"
	return nil
}

func (m *memoryStore) SetRejected(_ context.Context, id int64) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.Status = "rejected"
	return nil
}

func (m *memoryStore) SetPending(_ context.Context, id int64) error {
	r, ok := m.rows[id]
	if !ok {
		return errors.New("not found")
	}
	r.Status = "pending"
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
	assert.Equal(t, "pending", created.Status)
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
	assert.Equal(t, "rejected", got.Status)
}

func TestService_UpdateReview(t *testing.T) {
	inv := &fakeInventory{
		units: []inventory.Unit{{UnitID: 1, Name: "cup", Abbreviation: "c"}},
		items: []inventory.Item{{ItemID: 10, Name: "Flour"}},
	}
	svc := &Service{store: newMemoryStore(), inv: inv}
	ctx := context.Background()
	ri, _ := svc.Create(ctx, "scan.pdf", "/inbox/scan.pdf", "abc", nil, "admin@example.com")

	review := &ocrimport.ReviewRecipe{
		Name: "Pancakes",
		Items: []ocrimport.MatchResult{
			{
				DraftItem: ocrimport.DraftItem{Ingredient: "flour"},
				ItemID:    "10",
				Unit:      "cup",
				UnitID:    "1",
			},
		},
	}
	updated, err := svc.UpdateReview(ctx, ri.ID, review, "admin")
	require.NoError(t, err)
	assert.Equal(t, "ready", updated.Status)
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

	reviewJSON := `{"name":"Pancakes","items":[{"draftItem":{"ingredient":"flour"},"itemId":"10","unit":"cup","unitId":"1"}],"steps":[]}`
	require.NoError(t, svc.store.UpdateReview(ctx, ri.ID, []byte(reviewJSON), "ready", "admin"))

	admin := currentuser.User{UserID: 1, Email: "admin@example.com", IsAdmin: true}
	recipe, updated, err := svc.Approve(ctx, ri.ID, admin)
	require.NoError(t, err)
	assert.Equal(t, "Pancakes", recipe.Name)
	assert.Equal(t, "persisted", updated.Status)
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
