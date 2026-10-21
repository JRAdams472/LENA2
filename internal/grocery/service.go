// Package grocery owns household-scoped grocery lists and their items. Meal-plan
// expansion and pantry-stock subtraction are composed by the BFF, which
// is the only layer allowed to read across domains.
package grocery

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/grocery/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Service provides grocery list operations.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates a grocery Service using the given connection pool. The
// querier resolves a ctx-carried transaction first (see dbtx.ContextExecer)
// so calls made inside a UnitOfWork join that transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "grocery")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "grocery"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "grocery")) }
	}
	c := *s
	c.q = newQ(tx)
	c.tx = tx
	c.newQ = newQ
	return &c
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise. If the service is already bound to a transaction, or
// ctx already carries a UnitOfWork transaction, fn runs in that transaction
// instead of starting a new one.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	if s.tx != nil || dbtx.HasTx(ctx) {
		return fn(s)
	}
	if s.pool == nil {
		return fmt.Errorf("grocery: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// GroceryList is a household's generated shopping list. StoreID selects
// which store layout routes the list; new lists inherit the household's
// most recent store so web and mobile route identically.
type GroceryList struct {
	GroceryListID int64
	HouseholdID   int64
	MealPlanID    *int64
	StoreID       *int64
	GeneratedAt   time.Time
}

// CreateGroceryList creates an empty grocery list for a household.
func (s *Service) CreateGroceryList(ctx context.Context, householdID int64, mealPlanID *int64, by string) (GroceryList, error) {
	row, err := s.q.CreateGroceryList(ctx, sqlc.CreateGroceryListParams{
		HouseholdID: householdID,
		MealPlanID:  optInt8(mealPlanID),
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return GroceryList{}, fmt.Errorf("create grocery list: %w", err)
	}
	return toGroceryList(row), nil
}

// GetGroceryListByID returns a grocery list owned by the household.
func (s *Service) GetGroceryListByID(ctx context.Context, groceryListID, householdID int64) (GroceryList, error) {
	row, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: groceryListID, HouseholdID: householdID})
	if err != nil {
		return GroceryList{}, fmt.Errorf("get grocery list: %w", domainerr.FromStorage(err))
	}
	return toGroceryList(row), nil
}

// GetLatestGroceryListByPlan returns the most recently generated list
// linked to a meal plan owned by the household.
func (s *Service) GetLatestGroceryListByPlan(ctx context.Context, mealPlanID, householdID int64) (GroceryList, error) {
	row, err := s.q.GetLatestGroceryListByPlan(ctx, sqlc.GetLatestGroceryListByPlanParams{HouseholdID: householdID, MealPlanID: optInt8(&mealPlanID)})
	if err != nil {
		return GroceryList{}, fmt.Errorf("get grocery list by plan: %w", domainerr.FromStorage(err))
	}
	return toGroceryList(row), nil
}

// ReplaceGeneratedItems swaps a list's generated lines (source <> 'manual')
// for the supplied set and stamps the list's generated_at, leaving manual
// lines and the list itself untouched. Callers compose it inside their
// unit of work.
func (s *Service) ReplaceGeneratedItems(ctx context.Context, groceryListID, householdID int64, items []GroceryListItem, by string) ([]GroceryListItem, error) {
	if _, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: groceryListID, HouseholdID: householdID}); err != nil {
		return nil, fmt.Errorf("replace generated items: %w", domainerr.FromStorage(err))
	}
	if err := s.q.DeleteGeneratedGroceryListItems(ctx, groceryListID); err != nil {
		return nil, fmt.Errorf("replace generated items: %w", err)
	}
	out, err := s.AddGroceryListItems(ctx, items, householdID, by)
	if err != nil {
		return nil, err
	}
	if _, err := s.q.TouchGroceryListGeneratedAt(ctx, sqlc.TouchGroceryListGeneratedAtParams{GroceryListID: groceryListID, HouseholdID: householdID, UpdatedBy: textOrNull(by)}); err != nil {
		return nil, fmt.Errorf("replace generated items: %w", err)
	}
	return out, nil
}

// ListGroceryLists returns a household's grocery lists.
func (s *Service) ListGroceryLists(ctx context.Context, householdID int64, limit, offset int32) ([]GroceryList, error) {
	rows, err := s.q.ListGroceryLists(ctx, sqlc.ListGroceryListsParams{HouseholdID: householdID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list grocery lists: %w", err)
	}
	out := make([]GroceryList, len(rows))
	for i := range rows {
		out[i] = toGroceryList(rows[i])
	}
	return out, nil
}

// CountGroceryLists returns the total number of lists owned by the household.
func (s *Service) CountGroceryLists(ctx context.Context, householdID int64) (int64, error) {
	n, err := s.q.CountGroceryLists(ctx, householdID)
	if err != nil {
		return 0, fmt.Errorf("count grocery lists: %w", err)
	}
	return n, nil
}

// DeleteGroceryList removes a grocery list owned by the household.
func (s *Service) DeleteGroceryList(ctx context.Context, groceryListID, householdID int64) error {
	n, err := s.q.DeleteGroceryList(ctx, sqlc.DeleteGroceryListParams{GroceryListID: groceryListID, HouseholdID: householdID})
	if err != nil {
		return fmt.Errorf("delete grocery list: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("delete grocery list: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ReassignHousehold repoints every grocery list owned by the source
// household to the target household as part of an invite-accept merge.
// Callers run it inside the merge unit of work so the reassignment commits
// atomically with the household switch; a source with no lists is not an
// error.
func (s *Service) ReassignHousehold(ctx context.Context, fromHouseholdID, toHouseholdID int64, by string) error {
	if fromHouseholdID == toHouseholdID {
		return nil
	}
	if err := s.q.ReassignGroceryListsToHousehold(ctx, sqlc.ReassignGroceryListsToHouseholdParams{
		ToHouseholdID:   toHouseholdID,
		UpdatedBy:       by,
		FromHouseholdID: fromHouseholdID,
	}); err != nil {
		return fmt.Errorf("reassign grocery lists: %w", domainerr.FromStorage(err))
	}
	return nil
}

// DeleteHouseholdLists removes every grocery list owned by a dissolving
// household (LEN-146 account deletion). grocery_list.household_id is a
// NO ACTION edge, so lists must go before the households row can delete;
// grocery_list_item children cascade.
func (s *Service) DeleteHouseholdLists(ctx context.Context, householdID int64) error {
	if err := s.q.DeleteGroceryListsByHousehold(ctx, householdID); err != nil {
		return fmt.Errorf("delete grocery lists: %w", domainerr.FromStorage(err))
	}
	return nil
}

// GroceryListItem is a single item on a shopping list. ItemID is the
// branded catalog item; IngredientID optionally points at the
// brand-agnostic ingredient abstraction. CheckedSeq records the order it
// was checked off in — the observation the store router learns from.
type GroceryListItem struct {
	GroceryListItemID int64
	GroceryListID     int64
	ItemID            *int64
	IngredientID      *int64
	ManualItemName    string
	QuantityNeeded    float64
	UnitID            *int64
	Source            string
	IsChecked         bool
	CheckedAt         *time.Time
	CheckedSeq        *int32
}

// AddGroceryListItem adds an item to a grocery list owned by the household;
// the ownership check fails with the same not-found error as
// GetGroceryListByID.
func (s *Service) AddGroceryListItem(ctx context.Context, arg GroceryListItem, householdID int64, by string) (GroceryListItem, error) {
	if _, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: arg.GroceryListID, HouseholdID: householdID}); err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", domainerr.FromStorage(err))
	}
	qty, err := numericFromFloat64(arg.QuantityNeeded)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	row, err := s.q.AddGroceryListItem(ctx, sqlc.AddGroceryListItemParams{
		GroceryListID:  arg.GroceryListID,
		ItemID:         optInt8(arg.ItemID),
		IngredientID:   optInt8(arg.IngredientID),
		ManualItemName: textOrNull(arg.ManualItemName),
		QuantityNeeded: qty,
		UnitID:         optInt8(arg.UnitID),
		Source:         arg.Source,
		IsChecked:      arg.IsChecked,
		CreatedBy:      by,
		UpdatedBy:      textOrNull(by),
	})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("add grocery list item: %w", err)
	}
	return gli, nil
}

// ListGroceryListItems returns all items on a list owned by the household.
func (s *Service) ListGroceryListItems(ctx context.Context, groceryListID, householdID int64) ([]GroceryListItem, error) {
	rows, err := s.q.ListGroceryListItems(ctx, sqlc.ListGroceryListItemsParams{GroceryListID: groceryListID, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list grocery list items: %w", err)
	}
	out := make([]GroceryListItem, len(rows))
	for i := range rows {
		gli, err := toGroceryListItem(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list grocery list items: %w", err)
		}
		out[i] = gli
	}
	return out, nil
}

// ListGroceryListItemsByLists returns all items across a set of lists
// owned by the household in a single query.
func (s *Service) ListGroceryListItemsByLists(ctx context.Context, groceryListIDs []int64, householdID int64) ([]GroceryListItem, error) {
	rows, err := s.q.ListGroceryListItemsByLists(ctx, sqlc.ListGroceryListItemsByListsParams{GroceryListIds: groceryListIDs, HouseholdID: householdID})
	if err != nil {
		return nil, fmt.Errorf("list grocery list items by lists: %w", err)
	}
	out := make([]GroceryListItem, len(rows))
	for i := range rows {
		gli, err := toGroceryListItem(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list grocery list items by lists: %w", err)
		}
		out[i] = gli
	}
	return out, nil
}

// GetGroceryListItemByID returns an item on a list owned by the household.
func (s *Service) GetGroceryListItemByID(ctx context.Context, groceryListItemID, householdID int64) (GroceryListItem, error) {
	row, err := s.q.GetGroceryListItemByID(ctx, sqlc.GetGroceryListItemByIDParams{GroceryListItemID: groceryListItemID, HouseholdID: householdID})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("get grocery list item: %w", domainerr.FromStorage(err))
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("get grocery list item: %w", err)
	}
	return gli, nil
}

// UpdateGroceryListItem modifies an item on a list owned by the household.
func (s *Service) UpdateGroceryListItem(ctx context.Context, groceryListItemID, householdID int64, arg GroceryListItem, by string) error {
	qty, err := numericFromFloat64(arg.QuantityNeeded)
	if err != nil {
		return fmt.Errorf("update grocery list item: %w", err)
	}
	n, err := s.q.UpdateGroceryListItem(ctx, sqlc.UpdateGroceryListItemParams{
		GroceryListItemID: groceryListItemID,
		HouseholdID:       householdID,
		ItemID:            optInt8(arg.ItemID),
		IngredientID:      optInt8(arg.IngredientID),
		ManualItemName:    textOrNull(arg.ManualItemName),
		QuantityNeeded:    qty,
		UnitID:            optInt8(arg.UnitID),
		Source:            arg.Source,
		IsChecked:         arg.IsChecked,
		UpdatedBy:         textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("update grocery list item: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("update grocery list item: %w", domainerr.ErrNotFound)
	}
	return nil
}

// DeleteGroceryListItem removes an item from a list owned by the household.
func (s *Service) DeleteGroceryListItem(ctx context.Context, groceryListItemID, householdID int64) error {
	n, err := s.q.DeleteGroceryListItem(ctx, sqlc.DeleteGroceryListItemParams{GroceryListItemID: groceryListItemID, HouseholdID: householdID})
	if err != nil {
		return fmt.Errorf("delete grocery list item: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("delete grocery list item: %w", domainerr.ErrNotFound)
	}
	return nil
}

// lockGroceryList takes the list-row lock that serializes check-off
// sequencing — checked_seq is max+1 under READ COMMITTED, so concurrent
// toggles would otherwise publish duplicate seqs. Effective only inside
// the ambient unit of work callers wrap around the check/uncheck write.
func (s *Service) lockGroceryList(ctx context.Context, groceryListID, householdID int64) error {
	if _, err := s.q.LockGroceryList(ctx, sqlc.LockGroceryListParams{GroceryListID: groceryListID, HouseholdID: householdID}); err != nil {
		return fmt.Errorf("lock grocery list: %w", domainerr.FromStorage(err))
	}
	return nil
}

// BindItemToGroceryListItem sets only the catalog-item link on a list
// line — the previous whole-row writeback could clobber concurrent edits
// (and resurrect a stale unchecked state) on the brand-pick path.
func (s *Service) BindItemToGroceryListItem(ctx context.Context, groceryListItemID, householdID, itemID int64, by string) error {
	n, err := s.q.BindItemToGroceryListItem(ctx, sqlc.BindItemToGroceryListItemParams{
		GroceryListItemID: groceryListItemID,
		HouseholdID:       householdID,
		ItemID:            optInt8(&itemID),
		UpdatedBy:         textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("bind item to grocery list item: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("bind item to grocery list item: %w", domainerr.ErrNotFound)
	}
	return nil
}

// CheckGroceryListItemOn marks the line checked unless it already is.
// transitioned reports whether THIS call performed the flip — pantry
// credit must follow only on transition so concurrent check-offs credit
// exactly once. The list row lock serializes checked_seq, and the
// transition feeds the store router like a plain toggle does.
func (s *Service) CheckGroceryListItemOn(ctx context.Context, groceryListItemID, householdID int64, by string) (GroceryListItem, bool, error) {
	cur, err := s.q.GetGroceryListItemByID(ctx, sqlc.GetGroceryListItemByIDParams{GroceryListItemID: groceryListItemID, HouseholdID: householdID})
	if err != nil {
		return GroceryListItem{}, false, fmt.Errorf("check grocery list item: %w", domainerr.FromStorage(err))
	}
	if err := s.lockGroceryList(ctx, cur.GroceryListID, householdID); err != nil {
		return GroceryListItem{}, false, err
	}
	row, err := s.q.CheckGroceryListItemOn(ctx, sqlc.CheckGroceryListItemOnParams{
		GroceryListItemID: groceryListItemID,
		HouseholdID:       householdID,
		UpdatedBy:         textOrNull(by),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent check-off won the transition — return the current
		// row with transitioned=false so callers skip the pantry credit.
		cur, err = s.q.GetGroceryListItemByID(ctx, sqlc.GetGroceryListItemByIDParams{GroceryListItemID: groceryListItemID, HouseholdID: householdID})
		if err != nil {
			return GroceryListItem{}, false, fmt.Errorf("check grocery list item: %w", domainerr.FromStorage(err))
		}
		gli, err := toGroceryListItem(cur)
		if err != nil {
			return GroceryListItem{}, false, fmt.Errorf("check grocery list item: %w", err)
		}
		return gli, false, nil
	}
	if err != nil {
		return GroceryListItem{}, false, fmt.Errorf("check grocery list item: %w", domainerr.FromStorage(err))
	}
	before, err := toGroceryListItem(cur)
	if err != nil {
		return GroceryListItem{}, false, fmt.Errorf("check grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, false, fmt.Errorf("check grocery list item: %w", err)
	}
	out, err := s.observeCheckChange(ctx, householdID, before, gli, by)
	if err != nil {
		return GroceryListItem{}, false, err
	}
	return out, true, nil
}

// ToggleGroceryListItemChecked flips the checked state of an item and
// returns the post-toggle row in one atomic statement. A check-off also
// feeds the store router: the item's normalized check position is folded
// into its learned-order record for the list's store (the generic route
// when the list has none); unchecking retracts that observation again.
// Callers wrap the toggle in a unit of work, so the observation commits
// with the flip.
func (s *Service) ToggleGroceryListItemChecked(ctx context.Context, groceryListItemID, householdID int64, by string) (GroceryListItem, error) {
	// Pre-read for the list lock and the pre-flip checked_seq (needed to
	// retract the right observation when the toggle goes off).
	cur, err := s.q.GetGroceryListItemByID(ctx, sqlc.GetGroceryListItemByIDParams{GroceryListItemID: groceryListItemID, HouseholdID: householdID})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("toggle grocery list item: %w", domainerr.FromStorage(err))
	}
	if err := s.lockGroceryList(ctx, cur.GroceryListID, householdID); err != nil {
		return GroceryListItem{}, err
	}
	row, err := s.q.ToggleGroceryListItemChecked(ctx, sqlc.ToggleGroceryListItemCheckedParams{
		GroceryListItemID: groceryListItemID,
		HouseholdID:       householdID,
		UpdatedBy:         textOrNull(by),
	})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("toggle grocery list item: %w", domainerr.FromStorage(err))
	}
	before, err := toGroceryListItem(cur)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("toggle grocery list item: %w", err)
	}
	gli, err := toGroceryListItem(row)
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("toggle grocery list item: %w", err)
	}
	return s.observeCheckChange(ctx, householdID, before, gli, by)
}

// observeCheckChange folds a check transition into the item's learned
// route position — or pulls it back out when the check is retracted.
// The position is checked_seq normalized to 0..1 by the list's item
// count; retraction uses the pre-flip seq, the best available mirror of
// what the check originally contributed.
func (s *Service) observeCheckChange(ctx context.Context, householdID int64, before, after GroceryListItem, by string) (GroceryListItem, error) {
	var position float64
	var observing bool
	switch {
	case after.IsChecked && after.CheckedSeq != nil:
		position = float64(*after.CheckedSeq)
		observing = true
	case !after.IsChecked && before.IsChecked && before.CheckedSeq != nil:
		position = float64(*before.CheckedSeq)
	default:
		return after, nil
	}
	list, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: after.GroceryListID, HouseholdID: householdID})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("route observation: %w", domainerr.FromStorage(err))
	}
	total, err := s.q.CountGroceryListItems(ctx, sqlc.CountGroceryListItemsParams{GroceryListID: after.GroceryListID, HouseholdID: householdID})
	if err != nil {
		return GroceryListItem{}, fmt.Errorf("route observation: %w", err)
	}
	if total <= 0 {
		return after, nil
	}
	storeID := GenericStoreID
	if list.StoreID.Valid {
		storeID = list.StoreID.Int64
	}
	position = position / float64(total)
	if observing {
		err = s.RecordRouteObservation(ctx, householdID, storeID, after, position, by)
	} else {
		err = s.RetractRouteObservation(ctx, householdID, storeID, before, position, by)
	}
	if err != nil {
		return GroceryListItem{}, err
	}
	return after, nil
}

// AddGroceryListItems inserts a batch of items onto a single list owned
// by the household. All rows share one GroceryListID; callers composing the
// batch with other writes wrap the call in a unit of work, which the
// ctx-carried transaction joins automatically.
func (s *Service) AddGroceryListItems(ctx context.Context, items []GroceryListItem, householdID int64, by string) ([]GroceryListItem, error) {
	if len(items) == 0 {
		return nil, nil
	}
	listID := items[0].GroceryListID
	for _, it := range items[1:] {
		if it.GroceryListID != listID {
			return nil, fmt.Errorf("add grocery list items: mixed list ids %d and %d", listID, it.GroceryListID)
		}
	}
	if _, err := s.q.GetGroceryListByID(ctx, sqlc.GetGroceryListByIDParams{GroceryListID: listID, HouseholdID: householdID}); err != nil {
		return nil, fmt.Errorf("add grocery list items: %w", domainerr.FromStorage(err))
	}
	out := make([]GroceryListItem, 0, len(items))
	for _, it := range items {
		qty, err := numericFromFloat64(it.QuantityNeeded)
		if err != nil {
			return nil, fmt.Errorf("add grocery list items: %w", err)
		}
		row, err := s.q.AddGroceryListItem(ctx, sqlc.AddGroceryListItemParams{
			GroceryListID:  it.GroceryListID,
			ItemID:         optInt8(it.ItemID),
			IngredientID:   optInt8(it.IngredientID),
			ManualItemName: textOrNull(it.ManualItemName),
			QuantityNeeded: qty,
			UnitID:         optInt8(it.UnitID),
			Source:         it.Source,
			IsChecked:      it.IsChecked,
			CreatedBy:      by,
			UpdatedBy:      textOrNull(by),
		})
		if err != nil {
			return nil, fmt.Errorf("add grocery list items: %w", err)
		}
		gli, err := toGroceryListItem(row)
		if err != nil {
			return nil, fmt.Errorf("add grocery list items: %w", err)
		}
		out = append(out, gli)
	}
	return out, nil
}

func toGroceryList(row sqlc.GroceryGroceryList) GroceryList {
	gl := GroceryList{
		GroceryListID: row.GroceryListID,
		HouseholdID:   row.HouseholdID,
		GeneratedAt:   row.GeneratedAt,
	}
	if row.MealPlanID.Valid {
		v := row.MealPlanID.Int64
		gl.MealPlanID = &v
	}
	if row.StoreID.Valid {
		v := row.StoreID.Int64
		gl.StoreID = &v
	}
	return gl
}

func toGroceryListItem(row sqlc.GroceryGroceryListItem) (GroceryListItem, error) {
	gli := GroceryListItem{
		GroceryListItemID: row.GroceryListItemID,
		GroceryListID:     row.GroceryListID,
		ManualItemName:    row.ManualItemName.String,
		Source:            row.Source,
		IsChecked:         row.IsChecked,
	}
	if row.ItemID.Valid {
		v := row.ItemID.Int64
		gli.ItemID = &v
	}
	if row.IngredientID.Valid {
		v := row.IngredientID.Int64
		gli.IngredientID = &v
	}
	if row.UnitID.Valid {
		v := row.UnitID.Int64
		gli.UnitID = &v
	}
	if row.QuantityNeeded.Valid {
		f8, err := row.QuantityNeeded.Float64Value()
		if err != nil {
			return GroceryListItem{}, fmt.Errorf("grocery list item %d quantity: %w", row.GroceryListItemID, err)
		}
		gli.QuantityNeeded = f8.Float64
	}
	if row.CheckedAt.Valid {
		t := row.CheckedAt.Time
		gli.CheckedAt = &t
	}
	if row.CheckedSeq.Valid {
		v := row.CheckedSeq.Int32
		gli.CheckedSeq = &v
	}
	return gli, nil
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func numericFromFloat64(f float64) (pgtype.Numeric, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: value is not finite", f)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("convert %v to numeric: %w", f, err)
	}
	n.Valid = true
	return n, nil
}

func optInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
