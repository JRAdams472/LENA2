package grocery

// store.go implements store routing: household-defined stores with ordered
// aisles, item->aisle assignments, and the learned/manual walk order per
// item identity. Route state lives off-list so it survives
// grocery_list_item regeneration and applies to future lists.

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JRAdams472/LENA2/internal/grocery/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
	"github.com/jackc/pgx/v5/pgtype"
)

// GenericStoreID marks item_route rows observed while the list had no
// store — store-specific reads fall back to it.
const GenericStoreID int64 = 0

// Store is a household-defined shopping location with its own aisle
// layout. ExternalRef is reserved for a canonical chain+location key so
// aisle data can later be shared across households.
type Store struct {
	StoreID     int64
	HouseholdID int64
	Name        string
	ExternalRef string
}

// StoreAisle is one ordered section of a store's layout.
type StoreAisle struct {
	AisleID  int64
	StoreID  int64
	Name     string
	Position int32
}

// RouteIdentity selects one item identity (item, ingredient, or manual
// name) — the same triple aisles and route stats key on.
type RouteIdentity struct {
	ItemID       *int64
	IngredientID *int64
	ManualName   string
}

func (id RouteIdentity) manual() pgtype.Text { return textOrNull(normalizeManual(id.ManualName)) }

// routeMapKey is the canonical map key for a RouteIdentity — item and
// ingredient IDs are distinct namespaces, manual names normalize.
type routeMapKey string

func (id RouteIdentity) key() routeMapKey {
	switch {
	case id.ItemID != nil:
		return routeMapKey("i:" + strconv.FormatInt(*id.ItemID, 10))
	case id.IngredientID != nil:
		return routeMapKey("g:" + strconv.FormatInt(*id.IngredientID, 10))
	default:
		return routeMapKey("m:" + normalizeManual(id.ManualName))
	}
}

func normalizeManual(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func f8(v float64) pgtype.Float8 {
	return pgtype.Float8{Float64: v, Valid: true}
}

// routeKey resolves a list row to its route identity: catalog item first,
// then ingredient, then normalized manual name.
func routeKey(it GroceryListItem) RouteIdentity {
	switch {
	case it.ItemID != nil:
		return RouteIdentity{ItemID: it.ItemID}
	case it.IngredientID != nil:
		return RouteIdentity{IngredientID: it.IngredientID}
	default:
		return RouteIdentity{ManualName: it.ManualItemName}
	}
}

// ItemRoute is the route record for one item identity: the learned
// walk-position mean plus the optional manual override that outranks it.
type ItemRoute struct {
	Identity     RouteIdentity
	StoreID      int64
	LearnedMean  float64
	LearnedCount int32
	ManualRank   *float64
	ManualAt     *time.Time
}

// StoreSpecific reports whether the row belongs to the given store rather
// than the generic household route (store_id 0).
func (r ItemRoute) StoreSpecific(storeID int64) bool { return r.StoreID == storeID && storeID != 0 }

// EffectiveRank is the sort key — manual arrangement wins over learned.
func (r ItemRoute) EffectiveRank() *float64 {
	if r.ManualRank != nil {
		return r.ManualRank
	}
	if r.LearnedCount > 0 {
		return &r.LearnedMean
	}
	return nil
}

// AisleAssignment maps one item identity to a store aisle.
type AisleAssignment struct {
	Identity RouteIdentity
	AisleID  int64
}

// CreateStore adds a household store (name trimmed, unique per household).
func (s *Service) CreateStore(ctx context.Context, householdID int64, name string, by string) (Store, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Store{}, fmt.Errorf("create store: name is required")
	}
	row, err := s.q.CreateStore(ctx, sqlc.CreateStoreParams{
		HouseholdID: householdID, Name: name, CreatedBy: by, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return Store{}, fmt.Errorf("create store: %w", domainerr.FromStorage(err))
	}
	return toStore(row), nil
}

// GetStoreByID returns a store owned by the household.
func (s *Service) GetStoreByID(ctx context.Context, storeID, householdID int64) (Store, error) {
	row, err := s.q.GetStoreByID(ctx, sqlc.GetStoreByIDParams{StoreID: storeID, HouseholdID: householdID})
	if err != nil {
		return Store{}, fmt.Errorf("get store: %w", domainerr.FromStorage(err))
	}
	return toStore(row), nil
}

// ListStores returns the household's stores by name.
func (s *Service) ListStores(ctx context.Context, householdID int64) ([]Store, error) {
	rows, err := s.q.ListStores(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list stores: %w", err)
	}
	out := make([]Store, len(rows))
	for i := range rows {
		out[i] = toStore(rows[i])
	}
	return out, nil
}

// RenameStore renames a store owned by the household.
func (s *Service) RenameStore(ctx context.Context, storeID, householdID int64, name string, by string) (Store, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Store{}, fmt.Errorf("rename store: name is required")
	}
	row, err := s.q.RenameStore(ctx, sqlc.RenameStoreParams{
		StoreID: storeID, HouseholdID: householdID, Name: name, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return Store{}, fmt.Errorf("rename store: %w", domainerr.FromStorage(err))
	}
	return toStore(row), nil
}

// DeleteStore removes a store; its aisles, assignments and route rows cascade and lists keep their rows via SET NULL.
func (s *Service) DeleteStore(ctx context.Context, storeID, householdID int64) error {
	return s.q.DeleteStore(ctx, sqlc.DeleteStoreParams{StoreID: storeID, HouseholdID: householdID})
}

// LatestListStore returns the store on the household's most recent list —
// the deterministic default new lists inherit so every client picks the
// same store. Zero when no list has ever carried a store.
func (s *Service) LatestListStore(ctx context.Context, householdID int64) (int64, error) {
	id, err := s.q.GetLatestListStore(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("latest list store: %w", domainerr.FromStorage(err))
	}
	if !id.Valid {
		return 0, nil
	}
	return id.Int64, nil
}

// SetGroceryListStore attaches a store to a list (nil clears). The store
// must belong to the household.
func (s *Service) SetGroceryListStore(ctx context.Context, groceryListID, householdID int64, storeID *int64, by string) error {
	if storeID != nil {
		if _, err := s.GetStoreByID(ctx, *storeID, householdID); err != nil {
			return fmt.Errorf("set list store: %w", err)
		}
	}
	n, err := s.q.SetGroceryListStore(ctx, sqlc.SetGroceryListStoreParams{
		GroceryListID: groceryListID, HouseholdID: householdID, StoreID: optInt8(storeID), UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set list store: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("set list store: %w", domainerr.ErrNotFound)
	}
	return nil
}

// CreateAisle adds an aisle to a store owned by the household at the
// given walk-order position.
func (s *Service) CreateAisle(ctx context.Context, storeID, householdID int64, name string, position int32, by string) (StoreAisle, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return StoreAisle{}, fmt.Errorf("create aisle: name is required")
	}
	if _, err := s.GetStoreByID(ctx, storeID, householdID); err != nil {
		return StoreAisle{}, fmt.Errorf("create aisle: %w", err)
	}
	row, err := s.q.CreateAisle(ctx, sqlc.CreateAisleParams{
		StoreID: storeID, Name: name, Position: position, CreatedBy: by, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return StoreAisle{}, fmt.Errorf("create aisle: %w", domainerr.FromStorage(err))
	}
	return toAisle(row), nil
}

// ListAisles returns a store's aisles in walk order.
func (s *Service) ListAisles(ctx context.Context, storeID, householdID int64) ([]StoreAisle, error) {
	rows, err := s.q.ListAisles(ctx, sqlc.ListAislesParams{StoreID: storeID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list aisles: %w", err)
	}
	out := make([]StoreAisle, len(rows))
	for i := range rows {
		out[i] = toAisle(rows[i])
	}
	return out, nil
}

// RenameAisle renames an aisle on a store owned by the household and
// returns the updated row.
func (s *Service) RenameAisle(ctx context.Context, aisleID, householdID int64, name string, by string) (StoreAisle, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return StoreAisle{}, fmt.Errorf("rename aisle: name is required")
	}
	row, err := s.q.RenameAisle(ctx, sqlc.RenameAisleParams{
		AisleID: aisleID, HouseholdID: householdID, Name: name, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return StoreAisle{}, fmt.Errorf("rename aisle: %w", domainerr.FromStorage(err))
	}
	return toAisle(row), nil
}

// DeleteAisle removes an aisle; its assignments cascade.
func (s *Service) DeleteAisle(ctx context.Context, aisleID, householdID int64) error {
	return s.q.DeleteAisle(ctx, sqlc.DeleteAisleParams{AisleID: aisleID, HouseholdID: householdID})
}

// ReorderAisles rewrites positions so aisles sort in the submitted order.
// The caller-supplied list must cover every aisle on the store.
func (s *Service) ReorderAisles(ctx context.Context, storeID, householdID int64, aisleIDs []int64, by string) error {
	if _, err := s.GetStoreByID(ctx, storeID, householdID); err != nil {
		return fmt.Errorf("reorder aisles: %w", err)
	}
	return s.q.ReorderAisles(ctx, sqlc.ReorderAislesParams{
		StoreID: storeID, HouseholdID: householdID, AisleIds: aisleIDs, UpdatedBy: by,
	})
}

// AssignToAisle records which aisle an item identity lives in; aisleID nil
// removes the assignment.
func (s *Service) AssignToAisle(ctx context.Context, storeID, householdID int64, id RouteIdentity, aisleID *int64, by string) error {
	if _, err := s.GetStoreByID(ctx, storeID, householdID); err != nil {
		return fmt.Errorf("assign aisle: %w", err)
	}
	if aisleID == nil {
		n, err := s.q.UnassignItem(ctx, sqlc.UnassignItemParams{
			StoreID: storeID, HouseholdID: householdID,
			ItemID: optInt8(id.ItemID), IngredientID: optInt8(id.IngredientID),
			ManualName: id.manual(),
		})
		if err != nil {
			return fmt.Errorf("unassign aisle: %w", domainerr.FromStorage(err))
		}
		if n == 0 {
			return fmt.Errorf("unassign aisle: %w", domainerr.ErrNotFound)
		}
		return nil
	}
	var err error
	switch {
	case id.ItemID != nil:
		_, err = s.q.AssignItemToAisle(ctx, sqlc.AssignItemToAisleParams{
			StoreID: storeID, AisleID: *aisleID, ItemID: optInt8(id.ItemID), CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	case id.IngredientID != nil:
		_, err = s.q.AssignIngredientToAisle(ctx, sqlc.AssignIngredientToAisleParams{
			StoreID: storeID, AisleID: *aisleID, IngredientID: optInt8(id.IngredientID), CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	case id.ManualName != "":
		_, err = s.q.AssignManualToAisle(ctx, sqlc.AssignManualToAisleParams{
			StoreID: storeID, AisleID: *aisleID, ManualName: normalizeManual(id.ManualName), CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	default:
		return fmt.Errorf("assign aisle: item identity required")
	}
	if err != nil {
		return fmt.Errorf("assign aisle: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ListAssignments returns every item->aisle mapping on the store.
func (s *Service) ListAssignments(ctx context.Context, storeID, householdID int64) ([]AisleAssignment, error) {
	rows, err := s.q.ListAssignments(ctx, sqlc.ListAssignmentsParams{StoreID: storeID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	out := make([]AisleAssignment, len(rows))
	for i := range rows {
		out[i] = toAssignment(rows[i])
	}
	return out, nil
}

// RecordRouteObservation folds one check-off into the item's learned
// walk-position mean at the given store (GenericStoreID when the list has
// none). position should be the item's checked_seq normalized to 0..1.
func (s *Service) RecordRouteObservation(ctx context.Context, householdID, storeID int64, it GroceryListItem, position float64, by string) error {
	id := routeKey(it)
	var err error
	switch {
	case id.ItemID != nil:
		_, err = s.q.UpsertRouteObservationItem(ctx, sqlc.UpsertRouteObservationItemParams{
			HouseholdID: householdID, StoreID: storeID, ItemID: optInt8(id.ItemID),
			LearnedSum: position, CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	case id.IngredientID != nil:
		_, err = s.q.UpsertRouteObservationIngredient(ctx, sqlc.UpsertRouteObservationIngredientParams{
			HouseholdID: householdID, StoreID: storeID, IngredientID: optInt8(id.IngredientID),
			LearnedSum: position, CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	case id.ManualName != "":
		_, err = s.q.UpsertRouteObservationManual(ctx, sqlc.UpsertRouteObservationManualParams{
			HouseholdID: householdID, StoreID: storeID, ManualName: normalizeManual(id.ManualName),
			LearnedSum: position, CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	default:
		return fmt.Errorf("record route observation: item identity required")
	}
	if err != nil {
		return fmt.Errorf("record route observation: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ListItemRoutes returns route records for the store plus the generic
// household rows. Callers prefer a store-specific record when both exist.
func (s *Service) ListItemRoutes(ctx context.Context, storeID, householdID int64) ([]ItemRoute, error) {
	rows, err := s.q.ListItemRoutes(ctx, sqlc.ListItemRoutesParams{HouseholdID: householdID, StoreID: storeID})
	if err != nil {
		return nil, fmt.Errorf("list item routes: %w", err)
	}
	out := make([]ItemRoute, len(rows))
	for i := range rows {
		out[i] = toItemRoute(rows[i])
	}
	return out, nil
}

// WriteManualRank persists the user's arrangement: rank is the item's
// fractional position in the submitted order. Rows are upserted so a
// first-time drag needs no prior learned history.
func (s *Service) WriteManualRank(ctx context.Context, householdID, storeID int64, id RouteIdentity, rank float64, by string) error {
	var err error
	switch {
	case id.ItemID != nil:
		err = s.q.UpsertManualRankItem(ctx, sqlc.UpsertManualRankItemParams{
			HouseholdID: householdID, StoreID: storeID, ItemID: optInt8(id.ItemID),
			ManualRank: f8(rank), CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	case id.IngredientID != nil:
		err = s.q.UpsertManualRankIngredient(ctx, sqlc.UpsertManualRankIngredientParams{
			HouseholdID: householdID, StoreID: storeID, IngredientID: optInt8(id.IngredientID),
			ManualRank: f8(rank), CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	case id.ManualName != "":
		err = s.q.UpsertManualRankManual(ctx, sqlc.UpsertManualRankManualParams{
			HouseholdID: householdID, StoreID: storeID, ManualName: normalizeManual(id.ManualName),
			ManualRank: f8(rank), CreatedBy: by, UpdatedBy: textOrNull(by),
		})
	default:
		return fmt.Errorf("write manual rank: item identity required")
	}
	if err != nil {
		return fmt.Errorf("write manual rank: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ResetStoreRoute clears learned stats, manual ranks and aisle
// assignments for a store — the remedy when a store remodels its layout.
func (s *Service) ResetStoreRoute(ctx context.Context, storeID, householdID int64) error {
	if _, err := s.GetStoreByID(ctx, storeID, householdID); err != nil {
		return fmt.Errorf("reset store route: %w", err)
	}
	return s.q.ResetStoreRoute(ctx, sqlc.ResetStoreRouteParams{StoreID: storeID, HouseholdID: householdID})
}

// RouteItem is one list row inside a route group. Suggested marks an item
// placed in an aisle by the learned-order heuristic rather than an
// explicit assignment.
type RouteItem struct {
	Item      GroceryListItem
	Suggested bool
}

// RouteGroup is one stop on the store walk: an aisle (in position order)
// or the trailing bucket for items with no aisle signal (Aisle nil).
type RouteGroup struct {
	Aisle *StoreAisle
	Items []RouteItem
}

// RouteGroups is the server-side source of truth for grocery ordering:
// both clients render this verbatim so web and mobile always agree.
// Items group by their aisle assignment, sort by effective rank (manual
// arrangement outranks the learned check-off mean), and unassigned items
// either slot into the aisle of their nearest ranked neighbor as a
// suggestion or fall into a trailing unsorted bucket.
func (s *Service) RouteGroups(ctx context.Context, groceryListID, householdID int64) ([]RouteGroup, error) {
	list, err := s.GetGroceryListByID(ctx, groceryListID, householdID)
	if err != nil {
		return nil, err
	}
	items, err := s.ListGroceryListItems(ctx, groceryListID, householdID)
	if err != nil {
		return nil, err
	}

	storeID := GenericStoreID
	if list.StoreID != nil {
		storeID = *list.StoreID
	}
	routes, err := s.ListItemRoutes(ctx, storeID, householdID)
	if err != nil {
		return nil, err
	}
	// Store-specific rows win over generic (store_id 0) fallbacks.
	rankByKey := make(map[routeMapKey]*float64, len(routes))
	storeSpecific := make(map[routeMapKey]bool, len(routes))
	for _, rt := range routes {
		k := rt.Identity.key()
		if rt.StoreSpecific(storeID) {
			storeSpecific[k] = true
			rankByKey[k] = rt.EffectiveRank()
			continue
		}
		if !storeSpecific[k] {
			rankByKey[k] = rt.EffectiveRank()
		}
	}

	var aisles []StoreAisle
	aisleByItem := make(map[int64]int64)
	if list.StoreID != nil {
		if aisles, err = s.ListAisles(ctx, *list.StoreID, householdID); err != nil {
			return nil, err
		}
		assignments, err := s.ListAssignments(ctx, *list.StoreID, householdID)
		if err != nil {
			return nil, err
		}
		assignByKey := make(map[routeMapKey]int64, len(assignments))
		for _, a := range assignments {
			assignByKey[a.Identity.key()] = a.AisleID
		}
		// Ranked neighbor lookup for aisle suggestions: the ranked assigned
		// items sorted by rank.
		type rankedItem struct {
			rank    float64
			aisleID int64
		}
		var ranked []rankedItem
		for _, it := range items {
			k := routeKey(it).key()
			aID, ok := assignByKey[k]
			if !ok {
				continue
			}
			if r := rankByKey[k]; r != nil {
				ranked = append(ranked, rankedItem{rank: *r, aisleID: aID})
				aisleByItem[it.GroceryListItemID] = aID
			} else {
				aisleByItem[it.GroceryListItemID] = aID
			}
		}
		sort.Slice(ranked, func(i, j int) bool { return ranked[i].rank < ranked[j].rank })

		for _, it := range items {
			if _, ok := aisleByItem[it.GroceryListItemID]; ok {
				continue
			}
			r := rankByKey[routeKey(it).key()]
			if r == nil || len(ranked) == 0 {
				continue
			}
			// Nearest ranked assigned item suggests this item's aisle.
			i := sort.Search(len(ranked), func(i int) bool { return ranked[i].rank >= *r })
			if i == len(ranked) {
				i--
			}
			if i > 0 && *r-ranked[i-1].rank < ranked[i].rank-*r {
				i--
			}
			aisleByItem[it.GroceryListItemID] = -ranked[i].aisleID // negative marks suggested
		}
	}

	sortItems := func(items []RouteItem) {
		sort.SliceStable(items, func(i, j int) bool {
			ri := rankByKey[routeKey(items[i].Item).key()]
			rj := rankByKey[routeKey(items[j].Item).key()]
			switch {
			case ri == nil && rj == nil:
				return items[i].Item.GroceryListItemID < items[j].Item.GroceryListItemID
			case ri == nil:
				return false
			case rj == nil:
				return true
			case *ri != *rj:
				return *ri < *rj
			default:
				return items[i].Item.GroceryListItemID < items[j].Item.GroceryListItemID
			}
		})
	}

	groups := make([]RouteGroup, 0, len(aisles)+1)
	byAisle := make(map[int64]int, len(aisles))
	for i := range aisles {
		byAisle[aisles[i].AisleID] = i
		groups = append(groups, RouteGroup{Aisle: &aisles[i]})
	}
	var unsorted []RouteItem
	for _, it := range items {
		aID, ok := aisleByItem[it.GroceryListItemID]
		switch {
		case !ok || aID == 0:
			unsorted = append(unsorted, RouteItem{Item: it})
		case aID < 0: // suggested placement
			gi, known := byAisle[-aID]
			if !known {
				unsorted = append(unsorted, RouteItem{Item: it})
				continue
			}
			groups[gi].Items = append(groups[gi].Items, RouteItem{Item: it, Suggested: true})
		default:
			gi, known := byAisle[aID]
			if !known {
				unsorted = append(unsorted, RouteItem{Item: it})
				continue
			}
			groups[gi].Items = append(groups[gi].Items, RouteItem{Item: it})
		}
	}
	out := groups[:0]
	for _, g := range groups {
		if len(g.Items) == 0 {
			continue
		}
		sortItems(g.Items)
		out = append(out, g)
	}
	if len(unsorted) > 0 {
		sortItems(unsorted)
		out = append(out, RouteGroup{Items: unsorted})
	}
	return out, nil
}

// ReorderEntry is one item's slot in a submitted list order. AisleID
// additionally reassigns the item's aisle at the list's store when set.
type ReorderEntry struct {
	GroceryListItemID int64
	AisleID           *int64
}

// ReorderListItems records a user arrangement: every submitted item gets a
// manual rank equal to its fractional position in the order (index n of N
// becomes (n+1)/(N+1), leaving headroom for items added later). Entries
// carrying an aisle ID also update the item's aisle assignment at the
// list's store, so a cross-aisle drag lands in one mutation. Callers wrap
// this in a unit of work so ranks and assignments commit atomically.
func (s *Service) ReorderListItems(ctx context.Context, groceryListID, householdID int64, entries []ReorderEntry, by string) error {
	list, err := s.GetGroceryListByID(ctx, groceryListID, householdID)
	if err != nil {
		return fmt.Errorf("reorder list items: %w", err)
	}
	items, err := s.ListGroceryListItems(ctx, groceryListID, householdID)
	if err != nil {
		return fmt.Errorf("reorder list items: %w", err)
	}
	byID := make(map[int64]GroceryListItem, len(items))
	for _, it := range items {
		byID[it.GroceryListItemID] = it
	}
	storeID := GenericStoreID
	if list.StoreID != nil {
		storeID = *list.StoreID
	}
	var aisleSet map[int64]bool
	for _, e := range entries {
		if e.AisleID == nil {
			continue
		}
		if list.StoreID == nil {
			return fmt.Errorf("reorder list items: aisle moves require a store on the list")
		}
		if aisleSet == nil {
			aisles, err := s.ListAisles(ctx, *list.StoreID, householdID)
			if err != nil {
				return fmt.Errorf("reorder list items: %w", err)
			}
			aisleSet = make(map[int64]bool, len(aisles))
			for _, a := range aisles {
				aisleSet[a.AisleID] = true
			}
		}
		if !aisleSet[*e.AisleID] {
			return fmt.Errorf("reorder list items: %w", domainerr.ErrNotFound)
		}
	}
	for i, e := range entries {
		it, ok := byID[e.GroceryListItemID]
		if !ok {
			return fmt.Errorf("reorder list items: %w", domainerr.ErrNotFound)
		}
		rank := float64(i+1) / float64(len(entries)+1)
		if err := s.WriteManualRank(ctx, householdID, storeID, routeKey(it), rank, by); err != nil {
			return err
		}
		if e.AisleID != nil {
			if err := s.AssignToAisle(ctx, storeID, householdID, routeKey(it), e.AisleID, by); err != nil {
				return err
			}
		}
	}
	return nil
}

func toStore(row sqlc.GroceryStore) Store {
	return Store{
		StoreID:     row.StoreID,
		HouseholdID: row.HouseholdID,
		Name:        row.Name,
		ExternalRef: row.ExternalRef.String,
	}
}

func toAisle(row sqlc.GroceryStoreAisle) StoreAisle {
	return StoreAisle{
		AisleID:  row.AisleID,
		StoreID:  row.StoreID,
		Name:     row.Name,
		Position: row.Position,
	}
}

func toAssignment(row sqlc.GroceryAisleAssignment) AisleAssignment {
	a := AisleAssignment{AisleID: row.AisleID}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		a.Identity.ItemID = &v
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		a.Identity.IngredientID = &v
	}
	a.Identity.ManualName = row.ManualName.String
	return a
}

func toItemRoute(row sqlc.GroceryItemRoute) ItemRoute {
	r := ItemRoute{StoreID: row.StoreID, LearnedCount: row.LearnedCount}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		r.Identity.ItemID = &v
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		r.Identity.IngredientID = &v
	}
	r.Identity.ManualName = row.ManualName.String
	if row.LearnedCount > 0 {
		r.LearnedMean = row.LearnedSum / float64(row.LearnedCount)
	}
	if row.ManualRank.Valid {
		v := row.ManualRank.Float64
		r.ManualRank = &v
	}
	if row.ManualAt.Valid {
		t := row.ManualAt.Time
		r.ManualAt = &t
	}
	return r
}
