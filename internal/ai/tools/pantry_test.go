package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

type stubPantry struct {
	rows     []userprefs.HouseholdItem
	gotHH    int64
	gotLimit int32
}

func (s *stubPantry) ListHouseholdItems(_ context.Context, householdID int64, limit, _ int32) ([]userprefs.HouseholdItem, error) {
	s.gotHH = householdID
	s.gotLimit = limit
	return s.rows, nil
}

type stubNamer struct {
	items map[int64]inventory.Item
	units map[int64]inventory.Unit
}

func (s *stubNamer) GetItemsByIDs(_ context.Context, ids []int64) ([]inventory.Item, error) {
	out := make([]inventory.Item, 0, len(ids))
	for _, id := range ids {
		if it, ok := s.items[id]; ok {
			out = append(out, it)
		}
	}
	return out, nil
}

func (s *stubNamer) GetIngredientsByIDs(_ context.Context, ids []int64) ([]inventory.Ingredient, error) {
	out := make([]inventory.Ingredient, 0, len(ids))
	for _, id := range ids {
		out = append(out, inventory.Ingredient{IngredientID: id, Name: fmt.Sprintf("ingredient%d", id)})
	}
	return out, nil
}

func (s *stubNamer) ResolveItemIngredients(_ context.Context, _ int64, ids []int64) (map[int64]*int64, error) {
	out := make(map[int64]*int64, len(ids))
	for _, id := range ids {
		if it, ok := s.items[id]; ok && it.IngredientID != nil {
			v := *it.IngredientID
			out[id] = &v
		} else {
			out[id] = nil
		}
	}
	return out, nil
}

func (s *stubNamer) GetUnitsByIDs(_ context.Context, ids []int64) ([]inventory.Unit, error) {
	out := make([]inventory.Unit, 0, len(ids))
	for _, id := range ids {
		if u, ok := s.units[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

func pantryFixture() (*stubPantry, *stubNamer, *Registry) {
	minQty := 1.0
	soon := time.Now().Add(48 * time.Hour)
	later := time.Now().Add(90 * 24 * time.Hour)
	p := &stubPantry{rows: []userprefs.HouseholdItem{
		{HouseholdItemID: 1, HouseholdID: 9, ItemID: 10, CurrentQty: 2, MinQty: &minQty, ExpiresAt: &soon},
		{HouseholdItemID: 2, HouseholdID: 9, ItemID: 11, CurrentQty: 6, ExpiresAt: &later},
		{HouseholdItemID: 3, HouseholdID: 9, ItemID: 12, CurrentQty: 1},
	}}
	n := &stubNamer{
		items: map[int64]inventory.Item{
			10: {ItemID: 10, Name: "Flour", UnitID: 1},
			11: {ItemID: 11, Name: "Milk", UnitID: 2},
			12: {ItemID: 12, Name: "Salt", UnitID: 1},
		},
		units: map[int64]inventory.Unit{
			1: {UnitID: 1, Name: "cup", Abbreviation: "c"},
			2: {UnitID: 2, Name: "gallon", Abbreviation: "gal"},
		},
	}
	reg := New()
	RegisterPantryTools(reg, p, n)
	return p, n, reg
}

func TestPantryInventory(t *testing.T) {
	p, _, reg := pantryFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_pantry_inventory", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(9), p.gotHH)

	rows, ok := out.([]pantryRow)
	require.True(t, ok)
	require.Len(t, rows, 3)
	assert.Equal(t, "Flour", rows[0].Name)
	assert.Equal(t, 2.0, rows[0].Quantity)
	assert.Equal(t, "c", rows[0].Unit)
	assert.Equal(t, soonish(rows[0].ExpiresAt), true)
}

func soonish(s string) bool { return s != "" }

func TestPantryInventoryLimit(t *testing.T) {
	p, _, reg := pantryFixture()
	_, err := reg.Call(context.Background(), Scope{HouseholdID: 9}, "get_pantry_inventory", json.RawMessage(`{"limit":5}`))
	require.NoError(t, err)
	assert.Equal(t, int32(5), p.gotLimit)

	// Over the cap → clamped to 500.
	_, err = reg.Call(context.Background(), Scope{HouseholdID: 9}, "get_pantry_inventory", json.RawMessage(`{"limit":9999}`))
	require.NoError(t, err)
	assert.Equal(t, int32(500), p.gotLimit)
}

func TestExpiringItems(t *testing.T) {
	_, _, reg := pantryFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7, HouseholdID: 9}, "get_expiring_items", json.RawMessage(`{"days":7}`))
	require.NoError(t, err)
	rows, ok := out.([]pantryRow)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, "Flour", rows[0].Name)
}

func TestPantryNoHousehold(t *testing.T) {
	_, _, reg := pantryFixture()
	out, err := reg.Call(context.Background(), Scope{UserID: 7}, "get_pantry_inventory", nil)
	require.NoError(t, err)
	rows, ok := out.([]pantryRow)
	require.True(t, ok)
	assert.Empty(t, rows)
}

func TestPantryBadArgs(t *testing.T) {
	_, _, reg := pantryFixture()
	_, err := reg.Call(context.Background(), Scope{HouseholdID: 9}, "get_pantry_inventory", json.RawMessage(`{"limit":"x"}`))
	require.Error(t, err)
}
