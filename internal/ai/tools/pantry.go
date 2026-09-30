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

// ItemNamer resolves catalog item IDs to names and unit abbreviations for
// prompt-friendly tool output.
type ItemNamer interface {
	GetItemsByIDs(ctx context.Context, itemIDs []int64) ([]inventory.Item, error)
	GetUnitsByIDs(ctx context.Context, unitIDs []int64) ([]inventory.Unit, error)
}

// pantryRow is the tool's output shape — compact, name-first, ISO dates.
type pantryRow struct {
	Name      string   `json:"name"`
	Quantity  float64  `json:"quantity"`
	Unit      string   `json:"unit,omitempty"`
	MinQty    *float64 `json:"minQuantity,omitempty"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
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
	meta := map[int64]inventory.Item{}
	unitIDs := map[int64]bool{}
	if items != nil {
		list, err := items.GetItemsByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("item names: %w", err)
		}
		for _, it := range list {
			meta[it.ItemID] = it
			unitIDs[it.UnitID] = true
		}
	}
	units := map[int64]inventory.Unit{}
	if len(unitIDs) > 0 {
		uids := make([]int64, 0, len(unitIDs))
		for id := range unitIDs {
			uids = append(uids, id)
		}
		list, err := items.GetUnitsByIDs(ctx, uids)
		if err != nil {
			return nil, fmt.Errorf("unit names: %w", err)
		}
		for _, u := range list {
			units[u.UnitID] = u
		}
	}

	out := make([]pantryRow, 0, len(rows))
	for _, hi := range rows {
		row := pantryRow{
			Name:     fmt.Sprintf("item #%d", hi.ItemID),
			Quantity: hi.CurrentQty,
			MinQty:   hi.MinQty,
		}
		if it, ok := meta[hi.ItemID]; ok {
			row.Name = it.Name
			if u, ok := units[it.UnitID]; ok {
				row.Unit = u.Abbreviation
			}
		}
		if hi.ExpiresAt != nil {
			row.ExpiresAt = hi.ExpiresAt.Format("2006-01-02")
		}
		out = append(out, row)
	}
	return out, nil
}
