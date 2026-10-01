package grocery

// store.go implements store routing: household-defined stores with ordered
// aisles, item->aisle assignments, and the learned/manual walk order per
// item identity. Route state lives off-list so it survives
// grocery_list_item regeneration and applies to future lists.

import (
	"context"
	"fmt"
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
	LearnedMean  float64
	LearnedCount int32
	ManualRank   *float64
	ManualAt     *time.Time
}

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

// RenameAisle renames an aisle on a store owned by the household.
func (s *Service) RenameAisle(ctx context.Context, aisleID, householdID int64, name string, by string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("rename aisle: name is required")
	}
	n, err := s.q.RenameAisle(ctx, sqlc.RenameAisleParams{
		AisleID: aisleID, HouseholdID: householdID, Name: name, UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("rename aisle: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("rename aisle: %w", domainerr.ErrNotFound)
	}
	return nil
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
	r := ItemRoute{LearnedCount: row.LearnedCount}
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
