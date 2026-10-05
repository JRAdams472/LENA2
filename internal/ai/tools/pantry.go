package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/userprefs"
)

// PantryReader is the userprefs surface the pantry tools need.
type PantryReader interface {
	ListHouseholdItems(ctx context.Context, householdID int64, limit, offset int32) ([]userprefs.HouseholdItem, error)
}

// ItemNamer resolves catalog item IDs to names, unit abbreviations, and
// the generic ingredients items link to — ingredient names are how the
// model matches pantry rows ("Green Giant corn") to recipe lines ("corn").
type ItemNamer interface {
	GetItemsByIDs(ctx context.Context, itemIDs []int64) ([]inventory.Item, error)
	GetUnitsByIDs(ctx context.Context, unitIDs []int64) ([]inventory.Unit, error)
	GetIngredientsByIDs(ctx context.Context, ingredientIDs []int64) ([]inventory.Ingredient, error)
	ResolveItemIngredients(ctx context.Context, householdID int64, itemIDs []int64) (map[int64]*int64, error)
}

// pantryRow is the tool's output shape — compact, name-first, ISO dates.
type pantryRow struct {
	Name       string   `json:"name"`
	Ingredient string   `json:"ingredient,omitempty"`
	Quantity   float64  `json:"quantity"`
	Unit       string   `json:"unit,omitempty"`
	MinQty     *float64 `json:"minQuantity,omitempty"`
	ExpiresAt  string   `json:"expiresAt,omitempty"`
}

// RegisterPantryTools wires the pantry read tools (get_pantry_inventory,
// get_expiring_items) into the registry. Both are scoped to the caller's
// household and return at most a few hundred rows — enough for prompt
// context without blowing the token budget.
func RegisterPantryTools(reg *Registry, pantry PantryReader, items ItemNamer) {
	reg.Register(llm.ToolSpec{
		Name:        "get_pantry_inventory",
		Description: "List the household's pantry items with quantities, units, minimums, and expiration dates.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum rows to return (default 200, max 500)",
				},
			},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		var a struct {
			Limit int `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_pantry_inventory args: %w", err)
			}
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 200
		}
		if limit > 500 {
			limit = 500
		}
		return pantryRows(ctx, scope, pantry, items, int32(limit), nil)
	})

	reg.Register(llm.ToolSpec{
		Name:        "get_expiring_items",
		Description: "List pantry items that are expired or expire within the given number of days — the consume-first candidates.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"days": map[string]any{
					"type":        "integer",
					"description": "Lookahead window in days (default 7, max 365)",
				},
			},
		},
	}, func(ctx context.Context, scope Scope, args json.RawMessage) (any, error) {
		var a struct {
			Days int `json:"days"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("get_expiring_items args: %w", err)
			}
		}
		days := a.Days
		if days <= 0 {
			days = 7
		}
		if days > 365 {
			days = 365
		}
		cutoff := time.Now().Add(time.Duration(days) * 24 * time.Hour)
		return pantryRows(ctx, scope, pantry, items, 500, func(hi userprefs.HouseholdItem) bool {
			return hi.ExpiresAt != nil && !hi.ExpiresAt.After(cutoff)
		})
	})
}

// pantryRows loads the household pantry once, applies an optional row
// filter, and joins catalog names/units.
func pantryRows(ctx context.Context, scope Scope, pantry PantryReader, items ItemNamer, limit int32, keep func(userprefs.HouseholdItem) bool) ([]pantryRow, error) {
	if scope.HouseholdID == 0 {
		return []pantryRow{}, nil
	}
	rows, err := pantry.ListHouseholdItems(ctx, scope.HouseholdID, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("list pantry: %w", err)
	}
	if keep != nil {
		filtered := rows[:0]
		for _, hi := range rows {
			if keep(hi) {
				filtered = append(filtered, hi)
			}
		}
		rows = filtered
	}
	if len(rows) == 0 {
		return []pantryRow{}, nil
	}

	ids := make([]int64, len(rows))
	for i, hi := range rows {
		ids[i] = hi.ItemID
	}
	meta, units, err := itemMetadata(ctx, items, ids)
	if err != nil {
		return nil, err
	}
	// Resolved ingredient names let the model match pantry rows to
	// ingredient-keyed recipe lines regardless of brand.
	ingredientNames, err := itemIngredientNames(ctx, items, scope.HouseholdID, ids)
	if err != nil {
		return nil, err
	}

	out := make([]pantryRow, 0, len(rows))
	for _, hi := range rows {
		out = append(out, toPantryRow(hi, meta, units, ingredientNames))
	}
	return out, nil
}

// toPantryRow maps one household item, falling back to a placeholder name
// when the catalog row is missing.
func toPantryRow(hi userprefs.HouseholdItem, meta map[int64]inventory.Item, units map[int64]inventory.Unit, ingredientNames map[int64]string) pantryRow {
	row := pantryRow{
		Name:       fmt.Sprintf("item #%d", hi.ItemID),
		Ingredient: ingredientNames[hi.ItemID],
		Quantity:   hi.CurrentQty,
		MinQty:     hi.MinQty,
	}
	if it, ok := meta[hi.ItemID]; ok {
		row.Name = it.Name
		if u, ok := units[it.UnitID]; ok {
			row.Unit = u.Abbreviation
		}
	}
	if hi.ExpiresAt != nil {
		row.ExpiresAt = hi.ExpiresAt.Format(time.DateOnly)
	}
	return row
}

// itemMetadata loads catalog items and their units for the given ids.
func itemMetadata(ctx context.Context, items ItemNamer, ids []int64) (map[int64]inventory.Item, map[int64]inventory.Unit, error) {
	meta := map[int64]inventory.Item{}
	units := map[int64]inventory.Unit{}
	if items == nil {
		return meta, units, nil
	}
	list, err := items.GetItemsByIDs(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("item names: %w", err)
	}
	unitIDs := map[int64]bool{}
	for _, it := range list {
		meta[it.ItemID] = it
		unitIDs[it.UnitID] = true
	}
	if len(unitIDs) == 0 {
		return meta, units, nil
	}
	uids := make([]int64, 0, len(unitIDs))
	for id := range unitIDs {
		uids = append(uids, id)
	}
	ulist, err := items.GetUnitsByIDs(ctx, uids)
	if err != nil {
		return nil, nil, fmt.Errorf("unit names: %w", err)
	}
	for _, u := range ulist {
		units[u.UnitID] = u
	}
	return meta, units, nil
}

// itemIngredientNames maps each pantry item id to its resolved generic
// ingredient's name.
func itemIngredientNames(ctx context.Context, items ItemNamer, householdID int64, ids []int64) (map[int64]string, error) {
	names := map[int64]string{}
	if items == nil {
		return names, nil
	}
	resolved, err := items.ResolveItemIngredients(ctx, householdID, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve item ingredients: %w", err)
	}
	ingIDs := []int64{}
	for _, id := range resolved {
		if id != nil {
			ingIDs = append(ingIDs, *id)
		}
	}
	if len(ingIDs) == 0 {
		return names, nil
	}
	ings, err := items.GetIngredientsByIDs(ctx, ingIDs)
	if err != nil {
		return nil, fmt.Errorf("ingredient names: %w", err)
	}
	namesByID := map[int64]string{}
	for _, in := range ings {
		namesByID[in.IngredientID] = in.Name
	}
	for itemID, ingID := range resolved {
		if ingID != nil {
			names[itemID] = namesByID[*ingID]
		}
	}
	return names, nil
}
