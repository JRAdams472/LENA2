// Package inventory owns the catalog of food items: brands, categories,
// items, flavor profiles and nutrient types. It contains no per-user state.
package inventory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/inventory/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// Service provides catalog operations for the inventory domain.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
	tx   pgx.Tx
	// newQ builds the querier bound to a transaction. Tests inject a
	// factory returning their mock so InTx still exercises the real
	// Begin/Commit flow while statements land on the mock.
	newQ func(pgx.Tx) sqlc.Querier
}

// NewService creates an inventory Service using the given connection pool.
// The querier resolves a ctx-carried transaction first (see
// dbtx.ContextExecer) so calls made inside a UnitOfWork join that
// transaction automatically.
func NewService(pool dbtx.Pool) *Service {
	return &Service{
		q:    sqlc.New(dbtx.NewTimedExecer(dbtx.ContextExecer(pool), "inventory")),
		pool: pool,
		newQ: func(tx pgx.Tx) sqlc.Querier {
			return sqlc.New(dbtx.NewTimedExecer(tx, "inventory"))
		},
	}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	newQ := s.newQ
	if newQ == nil {
		newQ = func(t pgx.Tx) sqlc.Querier { return sqlc.New(dbtx.NewTimedExecer(t, "inventory")) }
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
		return fmt.Errorf("inventory: InTx requires a connection pool")
	}
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// Brand approval statuses. User-submitted brands start pending and stay
// visible only to their creator until an admin approves them; rejected brands
// are hidden from everyone.
const (
	BrandStatusPending  = "pending"
	BrandStatusApproved = "approved"
	BrandStatusRejected = "rejected"
)

// Brand is a catalog brand.
type Brand struct {
	BrandID           int64
	Name              string
	CreatedAt         time.Time
	Status            string
	SubmittedByUserID *int64
	ApprovedByUserID  *int64
	ApprovedAt        *time.Time
}

// CreateBrand adds a new approved brand (admin fast path).
func (s *Service) CreateBrand(ctx context.Context, name, by string) (Brand, error) {
	row, err := s.q.CreateBrand(ctx, sqlc.CreateBrandParams{
		Name:      name,
		CreatedBy: by,
	})
	if err != nil {
		return Brand{}, fmt.Errorf("create brand: %w", domainerr.FromStorage(err))
	}
	return toBrand(row), nil
}

// SubmitBrand returns an existing brand whose normalized name matches, or
// creates a new pending brand on behalf of the submitting user. The write is
// race-free: a single INSERT ... ON CONFLICT ensures only one non-rejected
// brand exists per normalized name.
func (s *Service) SubmitBrand(ctx context.Context, name string, userID int64, by string) (Brand, error) {
	row, err := s.q.UpsertBrand(ctx, sqlc.UpsertBrandParams{
		Name:              name,
		Status:            BrandStatusPending,
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		CreatedBy:         by,
	})
	if err == nil {
		return toBrand(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Brand{}, fmt.Errorf("submit brand: %w", domainerr.FromStorage(err))
	}

	// A non-rejected normalized name already exists; resolve it and reject
	// foreign-pending or rejected brands that the caller should not see.
	existing, err := s.q.FindBrandByNormalizedName(ctx, name)
	if err != nil {
		return Brand{}, fmt.Errorf("submit brand: %w", domainerr.FromStorage(err))
	}
	if existing.Status == BrandStatusPending && (!existing.SubmittedByUserID.Valid || existing.SubmittedByUserID.Int64 != userID) {
		return Brand{}, fmt.Errorf("submit brand: %w", domainerr.ErrConflict)
	}
	if existing.Status == BrandStatusRejected {
		return Brand{}, fmt.Errorf("submit brand: %w", domainerr.ErrConflict)
	}
	return toBrand(existing), nil
}

// GetBrandByID returns a brand by its primary key.
func (s *Service) GetBrandByID(ctx context.Context, brandID int64) (Brand, error) {
	row, err := s.q.GetBrandByID(ctx, brandID)
	if err != nil {
		return Brand{}, fmt.Errorf("get brand by id: %w", domainerr.FromStorage(err))
	}
	return toBrand(row), nil
}

// ListBrands returns all brands ordered by name.
func (s *Service) ListBrands(ctx context.Context) ([]Brand, error) {
	rows, err := s.q.ListBrands(ctx)
	if err != nil {
		return nil, fmt.Errorf("list brands: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = toBrand(rows[i])
	}
	return out, nil
}

// ListBrandsVisible returns a page of brands visible to the given user:
// approved brands plus any pending brands they submitted.
func (s *Service) ListBrandsVisible(ctx context.Context, userID int64, limit, offset int32) ([]Brand, error) {
	rows, err := s.q.ListBrandsVisible(ctx, sqlc.ListBrandsVisibleParams{
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list brands visible: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = toBrand(rows[i])
	}
	return out, nil
}

// CountBrandsVisible returns the total number of brands visible to the user.
func (s *Service) CountBrandsVisible(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountBrandsVisible(ctx, pgtype.Int8{Int64: userID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("count brands visible: %w", err)
	}
	return n, nil
}

// SearchBrands returns brands visible to the given user whose normalized name
// contains the normalized search term, ordered by engagement tier.
func (s *Service) SearchBrands(ctx context.Context, term string, userID int64, rank RankParams, limit int32) ([]Brand, error) {
	rows, err := s.q.SearchBrands(ctx, sqlc.SearchBrandsParams{
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		RegexpReplace:     term,
		Limit:             limit,
		PersonalIds:       rank.PersonalIDs,
		HouseholdIds:      rank.HouseholdIDs,
		SearchTerms:       rank.SearchTerms,
		GlobalIds:         rank.GlobalIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("search brands: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = toBrand(rows[i])
	}
	return out, nil
}

// ListPendingBrands returns brands awaiting admin approval, oldest first.
func (s *Service) ListPendingBrands(ctx context.Context, limit, offset int32) ([]Brand, error) {
	rows, err := s.q.ListPendingBrands(ctx, sqlc.ListPendingBrandsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list pending brands: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = toBrand(rows[i])
	}
	return out, nil
}

// CountPendingBrands returns the number of brands awaiting approval.
func (s *Service) CountPendingBrands(ctx context.Context) (int64, error) {
	n, err := s.q.CountPendingBrands(ctx)
	if err != nil {
		return 0, fmt.Errorf("count pending brands: %w", err)
	}
	return n, nil
}

// SetBrandStatus marks a brand approved or rejected. Approving records the
// approving user and timestamp; rejecting clears them.
func (s *Service) SetBrandStatus(ctx context.Context, brandID int64, status string, approverUserID int64, by string) error {
	var approvedAt pgtype.Timestamptz
	var approver pgtype.Int8
	if status == BrandStatusApproved {
		approvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		approver = pgtype.Int8{Int64: approverUserID, Valid: true}
	}
	n, err := s.q.SetBrandStatus(ctx, sqlc.SetBrandStatusParams{
		BrandID:          brandID,
		Status:           status,
		ApprovedByUserID: approver,
		ApprovedAt:       approvedAt,
		UpdatedBy:        textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set brand status: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set brand status: %w", domainerr.ErrNotFound)
	}
	return nil
}

// GetBrandsByIDs returns a set of brands in a single query.
func (s *Service) GetBrandsByIDs(ctx context.Context, brandIDs []int64) ([]Brand, error) {
	rows, err := s.q.GetBrandsByIDs(ctx, brandIDs)
	if err != nil {
		return nil, fmt.Errorf("get brands by ids: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = toBrand(rows[i])
	}
	return out, nil
}

// UpdateBrand renames an existing brand.
func (s *Service) UpdateBrand(ctx context.Context, brandID int64, name string) (Brand, error) {
	row, err := s.q.UpdateBrand(ctx, sqlc.UpdateBrandParams{BrandID: brandID, Name: name})
	if err != nil {
		return Brand{}, fmt.Errorf("update brand: %w", domainerr.FromStorage(err))
	}
	return toBrand(row), nil
}

// DeleteBrand removes a brand from the catalog.
func (s *Service) DeleteBrand(ctx context.Context, brandID int64) error {
	return s.q.DeleteBrand(ctx, brandID)
}

// Category is a catalog category with optional description.
type Category struct {
	CategoryID  int64
	Name        string
	Description string
	IsActive    bool
	// IsProtein flags categories whose items count as protein for the
	// defrost-reminder sweep.
	IsProtein bool
}

// CreateCategory adds a new category.
func (s *Service) CreateCategory(ctx context.Context, name, description string, isProtein bool, by string) (Category, error) {
	row, err := s.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:        name,
		Description: textOrNull(description),
		IsActive:    true,
		IsProtein:   isProtein,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}
	return toCategory(row), nil
}

// GetCategoryByID returns a category by its primary key.
func (s *Service) GetCategoryByID(ctx context.Context, categoryID int64) (Category, error) {
	row, err := s.q.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return Category{}, fmt.Errorf("get category by id: %w", domainerr.FromStorage(err))
	}
	return toCategory(row), nil
}

// ListCategories returns all active categories.
func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	out := make([]Category, len(rows))
	for i := range rows {
		out[i] = toCategory(rows[i])
	}
	return out, nil
}

// GetCategoriesByIDs returns a set of categories in a single query.
func (s *Service) GetCategoriesByIDs(ctx context.Context, categoryIDs []int64) ([]Category, error) {
	rows, err := s.q.GetCategoriesByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, fmt.Errorf("get categories by ids: %w", err)
	}
	out := make([]Category, len(rows))
	for i := range rows {
		out[i] = toCategory(rows[i])
	}
	return out, nil
}

// UpdateCategory modifies an existing category.
func (s *Service) UpdateCategory(ctx context.Context, categoryID int64, name, description string, isActive, isProtein bool, by string) (Category, error) {
	row, err := s.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		CategoryID:  categoryID,
		Name:        name,
		Description: textOrNull(description),
		IsActive:    isActive,
		IsProtein:   isProtein,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Category{}, fmt.Errorf("update category: %w", domainerr.FromStorage(err))
	}
	return toCategory(row), nil
}

// DeleteCategory removes a category from the catalog.
func (s *Service) DeleteCategory(ctx context.Context, categoryID int64) error {
	return s.q.DeleteCategory(ctx, categoryID)
}

// Item approval statuses. User-submitted items start pending and stay
// visible only to their creator until an admin approves them; rejected
// items are hidden from everyone.
const (
	ItemStatusPending  = "pending"
	ItemStatusApproved = "approved"
	ItemStatusRejected = "rejected"
)

// Item is a catalog food item.
type Item struct {
	ItemID            int64
	Name              string
	BrandID           *int64
	Upc12             string
	Upc14             string
	CategoryID        int64
	UnitID            int64
	Status            string
	SubmittedByUserID *int64
	ApprovedByUserID  *int64
	ApprovedAt        *time.Time
	NetWeight         *float64
	IsMetric          bool
	// IngredientID is the catalog-level link to the generic ingredient this
	// branded product is. Household overrides layer on top at resolution
	// time — see ResolveItemIngredient.
	IngredientID *int64
}

// CreateItem adds a new item to the catalog. The item is immediately
// approved; this is the trusted (admin) create path.
func (s *Service) CreateItem(ctx context.Context, arg Item, by string) (Item, error) {
	netWeight, err := numericFromOptionalFloat64(arg.NetWeight)
	if err != nil {
		return Item{}, fmt.Errorf("create item: %w", err)
	}
	row, err := s.q.CreateItem(ctx, sqlc.CreateItemParams{
		Name:              arg.Name,
		BrandID:           optInt64(arg.BrandID),
		Upc12:             textOrNull(arg.Upc12),
		Upc14:             textOrNull(arg.Upc14),
		CategoryID:        arg.CategoryID,
		UnitID:            arg.UnitID,
		Status:            ItemStatusApproved,
		SubmittedByUserID: optInt64(arg.SubmittedByUserID),
		CreatedBy:         by,
		UpdatedBy:         textOrNull(by),
		NetWeight:         netWeight,
		IsMetric:          arg.IsMetric,
	})
	if err != nil {
		return Item{}, fmt.Errorf("create item: %w", err)
	}
	return toItem(row), nil
}

// SubmitItem adds a new item to the catalog in 'pending' status on behalf
// of the submitting user, who keeps visibility until an admin approves it.
func (s *Service) SubmitItem(ctx context.Context, arg Item, userID int64, by string) (Item, error) {
	netWeight, err := numericFromOptionalFloat64(arg.NetWeight)
	if err != nil {
		return Item{}, fmt.Errorf("submit item: %w", err)
	}
	row, err := s.q.CreateItem(ctx, sqlc.CreateItemParams{
		Name:              arg.Name,
		BrandID:           optInt64(arg.BrandID),
		Upc12:             textOrNull(arg.Upc12),
		Upc14:             textOrNull(arg.Upc14),
		CategoryID:        arg.CategoryID,
		UnitID:            arg.UnitID,
		Status:            ItemStatusPending,
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		CreatedBy:         by,
		UpdatedBy:         textOrNull(by),
		NetWeight:         netWeight,
		IsMetric:          arg.IsMetric,
	})
	if err != nil {
		return Item{}, fmt.Errorf("submit item: %w", err)
	}
	return toItem(row), nil
}

// GetItemByUpc returns the item whose upc12 or upc14 equals the normalized
// code, limited to items visible to the given user (approved or their own
// pending submissions). Returns domainerr.ErrNotFound when nothing matches.
func (s *Service) GetItemByUpc(ctx context.Context, code string, userID int64) (Item, error) {
	row, err := s.q.GetItemByUpc(ctx, sqlc.GetItemByUpcParams{
		Upc12:             textOrNull(code),
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
	})
	if err != nil {
		return Item{}, fmt.Errorf("get item by upc: %w", domainerr.FromStorage(err))
	}
	return toItem(row), nil
}

// ListPendingItems returns items awaiting admin approval, oldest first.
func (s *Service) ListPendingItems(ctx context.Context, limit, offset int32) ([]Item, error) {
	rows, err := s.q.ListPendingItems(ctx, sqlc.ListPendingItemsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list pending items: %w", err)
	}
	out := make([]Item, len(rows))
	for i := range rows {
		out[i] = toItem(rows[i])
	}
	return out, nil
}

// CountPendingItems returns the number of items awaiting approval.
func (s *Service) CountPendingItems(ctx context.Context) (int64, error) {
	n, err := s.q.CountPendingItems(ctx)
	if err != nil {
		return 0, fmt.Errorf("count pending items: %w", err)
	}
	return n, nil
}

// SetItemStatus marks an item approved or rejected. Approving records the
// approving user and timestamp; rejecting clears them.
func (s *Service) SetItemStatus(ctx context.Context, itemID int64, status string, approverUserID int64, by string) error {
	var approvedAt pgtype.Timestamptz
	var approver pgtype.Int8
	if status == ItemStatusApproved {
		approvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		approver = pgtype.Int8{Int64: approverUserID, Valid: true}
	}
	n, err := s.q.SetItemStatus(ctx, sqlc.SetItemStatusParams{
		ItemID:           itemID,
		Status:           status,
		ApprovedByUserID: approver,
		ApprovedAt:       approvedAt,
		UpdatedBy:        textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set item status: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set item status: %w", domainerr.ErrNotFound)
	}
	return nil
}

// GetItemByID returns an item by its primary key.
func (s *Service) GetItemByID(ctx context.Context, itemID int64) (Item, error) {
	row, err := s.q.GetItemByID(ctx, itemID)
	if err != nil {
		return Item{}, fmt.Errorf("get item by id: %w", domainerr.FromStorage(err))
	}
	return toItem(row), nil
}

// GetItemsByIDs returns a set of items in a single query.
func (s *Service) GetItemsByIDs(ctx context.Context, itemIDs []int64) ([]Item, error) {
	rows, err := s.q.GetItemsByIDs(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("get items by ids: %w", err)
	}
	out := make([]Item, len(rows))
	for i := range rows {
		out[i] = toItem(rows[i])
	}
	return out, nil
}

// ListItems returns a paginated list of items visible to the given user:
// approved items plus the user's own pending submissions, ordered by name.
func (s *Service) ListItems(ctx context.Context, userID int64, limit, offset int32) ([]Item, error) {
	rows, err := s.q.ListItems(ctx, sqlc.ListItemsParams{
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := make([]Item, len(rows))
	for i := range rows {
		out[i] = toItem(rows[i])
	}
	return out, nil
}

// CountItems returns the number of items visible to the given user.
func (s *Service) CountItems(ctx context.Context, userID int64) (int64, error) {
	n, err := s.q.CountItems(ctx, pgtype.Int8{Int64: userID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return n, nil
}

// RankParams carries BFF-computed engagement ranking inputs for catalog
// search/list queries. All slices are optional — nil/empty disables that
// tier — and must arrive pre-sorted by signal strength so position within
// each list doubles as the in-tier tiebreaker.
type RankParams struct {
	FavoriteIDs  []int64
	PersonalIDs  []int64
	HouseholdIDs []int64
	GlobalIDs    []int64
	SearchTerms  []string
}

// SearchItems returns one page of items visible to the given user, filtered
// by an optional name term and ordered by engagement tier before paging.
// Pass an empty term to rank the full catalog.
//
// The item catalog is large (~110k seeded rows), so ranking can't run as
// ORDER BY CASE over the whole table — every page would pay a full sort.
// Instead the engaged rows are fetched by ID (a few thousand at most) and
// tier-sorted here, while the remaining catalog is served in (name, id)
// index order behind a hashed NOT IN probe.
func (s *Service) SearchItems(ctx context.Context, userID int64, term string, brandID *int64, rank RankParams, limit, offset int32) ([]Item, error) {
	user := pgtype.Int8{Int64: userID, Valid: true}
	search := textOrNull(term)
	brand := pgtype.Int8{Valid: brandID != nil}
	if brandID != nil {
		brand.Int64 = *brandID
	}

	// Resolve the prior-search-term tier to IDs only when the user has
	// recorded terms — it costs a catalog scan otherwise.
	searchedIDs, err := s.searchedItemIDs(ctx, user, search, rank.SearchTerms)
	if err != nil {
		return nil, err
	}

	// Union of every engaged ID — the ranked fetch set and the remainder's
	// exclusion set.
	engagedIDs := dedupeIDs(rank.FavoriteIDs, rank.PersonalIDs, rank.HouseholdIDs, searchedIDs, rank.GlobalIDs)

	var ranked []sqlc.InventoryItem
	if len(engagedIDs) > 0 {
		rows, err := s.q.RankedItems(ctx, sqlc.RankedItemsParams{
			SubmittedByUserID: user,
			Search:            search,
			BrandID:           brand,
			EngagedIds:        engagedIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("ranked items: %w", err)
		}
		ranked = orderRankedItems(rows, rank, searchedIDs)
	}

	// Serve the front of the page from the ranked prefix.
	out := make([]Item, 0, limit)
	if int(offset) < len(ranked) {
		end := int(offset) + int(limit)
		if end > len(ranked) {
			end = len(ranked)
		}
		for _, row := range ranked[offset:end] {
			out = append(out, toItem(row))
		}
	}

	// Fill the rest of the page from the index-ordered remainder.
	if intToInt32(len(out)) < limit {
		rem, err := s.remainderItems(ctx, user, search, brand, engagedIDs, limit-intToInt32(len(out)), offset-intToInt32(len(ranked)))
		if err != nil {
			return nil, err
		}
		out = append(out, rem...)
	}
	return out, nil
}

// searchedItemIDs resolves the prior-search-term tier to item IDs. An
// empty term list skips the catalog scan entirely.
func (s *Service) searchedItemIDs(ctx context.Context, user pgtype.Int8, search pgtype.Text, terms []string) ([]int64, error) {
	if len(terms) == 0 {
		return nil, nil
	}
	ids, err := s.q.MatchItemIDsByTerms(ctx, sqlc.MatchItemIDsByTermsParams{
		SubmittedByUserID: user,
		Search:            search,
		SearchTerms:       terms,
	})
	if err != nil {
		return nil, fmt.Errorf("match searched item ids: %w", err)
	}
	return ids, nil
}

// remainderItems pages the index-ordered catalog behind the ranked prefix.
func (s *Service) remainderItems(ctx context.Context, user pgtype.Int8, search pgtype.Text, brand pgtype.Int8, engagedIDs []int64, limit, offset int32) ([]Item, error) {
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.SearchItemsRemainder(ctx, sqlc.SearchItemsRemainderParams{
		SubmittedByUserID: user,
		Search:            search,
		BrandID:           brand,
		EngagedIds:        engagedIDs,
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	out := make([]Item, 0, len(rows))
	for i := range rows {
		out = append(out, toItem(rows[i]))
	}
	return out, nil
}

// intToInt32 saturates an in-memory count at MaxInt32 — engagement sets are
// bounded well below that, so saturation is unreachable in practice.
func intToInt32(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	//nolint:gosec // n is saturated at MaxInt32 immediately above.
	return int32(n)
}

// dedupeIDs unions ID lists preserving first-seen order — later lists only
// contribute IDs not already claimed by a higher tier.
func dedupeIDs(lists ...[]int64) []int64 {
	seen := make(map[int64]struct{})
	var out []int64
	for _, ids := range lists {
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// orderRankedItems sorts the engaged rows by the engagement tiers SearchItems
// used to express in SQL: favorite, personal, household, searched, global —
// in-tier by position within the tier's own pre-sorted list (searched-tier
// rows are unordered, so they fall to the name tiebreak), then (name, id).
func orderRankedItems(rows []sqlc.InventoryItem, rank RankParams, searchedIDs []int64) []sqlc.InventoryItem {
	key := make(map[int64][2]int64, len(rows))
	put := func(ids []int64, tier int64) {
		for i, id := range ids {
			if _, ok := key[id]; !ok {
				key[id] = [2]int64{tier, int64(i)}
			}
		}
	}
	put(rank.FavoriteIDs, 0)
	put(rank.PersonalIDs, 1)
	put(rank.HouseholdIDs, 2)
	for _, id := range searchedIDs {
		if _, ok := key[id]; !ok {
			key[id] = [2]int64{3, 0}
		}
	}
	put(rank.GlobalIDs, 4)
	sort.Slice(rows, func(i, j int) bool {
		ki, kj := key[rows[i].ItemID], key[rows[j].ItemID]
		if ki != kj {
			return ki[0] < kj[0] || (ki[0] == kj[0] && ki[1] < kj[1])
		}
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].ItemID < rows[j].ItemID
	})
	return rows
}

// CountSearchItems returns the un-paged match count for the same visibility,
// term, and brand filters as SearchItems.
func (s *Service) CountSearchItems(ctx context.Context, userID int64, term string, brandID *int64) (int64, error) {
	brand := pgtype.Int8{Valid: brandID != nil}
	if brandID != nil {
		brand.Int64 = *brandID
	}
	n, err := s.q.CountSearchItems(ctx, sqlc.CountSearchItemsParams{
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		Search:            textOrNull(term),
		BrandID:           brand,
	})
	if err != nil {
		return 0, fmt.Errorf("count search items: %w", err)
	}
	return n, nil
}

// MatchItemIDs returns IDs of items visible to the given user whose name
// contains the term — used to scope household-level listings that cannot
// join this schema.
func (s *Service) MatchItemIDs(ctx context.Context, term string, userID int64) ([]int64, error) {
	ids, err := s.q.MatchItemIDs(ctx, sqlc.MatchItemIDsParams{
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
		Lower:             term,
	})
	if err != nil {
		return nil, fmt.Errorf("match item ids: %w", err)
	}
	return ids, nil
}

// UpdateItem modifies an existing item. All business logic about who can
// modify catalog data lives in Go, not in SQL triggers or procedures.
func (s *Service) UpdateItem(ctx context.Context, itemID int64, arg Item, by string) error {
	netWeight, err := numericFromOptionalFloat64(arg.NetWeight)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	n, err := s.q.UpdateItem(ctx, sqlc.UpdateItemParams{
		ItemID:     itemID,
		Name:       arg.Name,
		BrandID:    optInt64(arg.BrandID),
		Upc12:      textOrNull(arg.Upc12),
		Upc14:      textOrNull(arg.Upc14),
		CategoryID: arg.CategoryID,
		UnitID:     arg.UnitID,
		UpdatedBy:  textOrNull(by),
		NetWeight:  netWeight,
		IsMetric:   arg.IsMetric,
	})
	if err != nil {
		return fmt.Errorf("update item: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("update item: %w", domainerr.ErrNotFound)
	}
	return nil
}

// DeleteItem removes an item from the catalog. Dependent rows — pantry
// entries, nutrients, flavors — are removed by ON DELETE CASCADE.
func (s *Service) DeleteItem(ctx context.Context, itemID int64) error {
	return s.q.DeleteItem(ctx, itemID)
}

// FlavorProfile is a catalog food flavor profile.
type FlavorProfile struct {
	FlavorID  int64
	Name      string
	IsActive  bool
	CreatedAt time.Time
}

// CreateFlavorProfile adds a new flavor profile.
func (s *Service) CreateFlavorProfile(ctx context.Context, name, by string) (FlavorProfile, error) {
	row, err := s.q.CreateFlavorProfile(ctx, sqlc.CreateFlavorProfileParams{
		Name:      name,
		IsActive:  true,
		CreatedBy: by,
		UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return FlavorProfile{}, fmt.Errorf("create flavor profile: %w", err)
	}
	return toFlavorProfile(row), nil
}

// GetFlavorProfileByID returns a flavor profile by its primary key.
func (s *Service) GetFlavorProfileByID(ctx context.Context, flavorID int64) (FlavorProfile, error) {
	row, err := s.q.GetFlavorProfileByID(ctx, flavorID)
	if err != nil {
		return FlavorProfile{}, fmt.Errorf("get flavor profile by id: %w", domainerr.FromStorage(err))
	}
	return toFlavorProfile(row), nil
}

// ListFlavorProfiles returns all flavor profiles ordered by name.
func (s *Service) ListFlavorProfiles(ctx context.Context) ([]FlavorProfile, error) {
	rows, err := s.q.ListFlavorProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list flavor profiles: %w", err)
	}
	out := make([]FlavorProfile, len(rows))
	for i := range rows {
		out[i] = toFlavorProfile(rows[i])
	}
	return out, nil
}

// UpdateFlavorProfile modifies an existing flavor profile.
func (s *Service) UpdateFlavorProfile(ctx context.Context, flavorID int64, name string, isActive bool, by string) (FlavorProfile, error) {
	row, err := s.q.UpdateFlavorProfile(ctx, sqlc.UpdateFlavorProfileParams{
		FlavorID:  flavorID,
		Name:      name,
		IsActive:  isActive,
		UpdatedBy: textOrNull(by),
	})
	if err != nil {
		return FlavorProfile{}, fmt.Errorf("update flavor profile: %w", domainerr.FromStorage(err))
	}
	return toFlavorProfile(row), nil
}

// DeleteFlavorProfile removes a flavor profile from the catalog.
func (s *Service) DeleteFlavorProfile(ctx context.Context, flavorID int64) error {
	return s.q.DeleteFlavorProfile(ctx, flavorID)
}

// NutrientType is a catalog nutrient type.
type NutrientType struct {
	NutrientID int64
	Name       string
	Unit       string
	CreatedAt  time.Time
}

// CreateNutrientType adds a new nutrient type.
func (s *Service) CreateNutrientType(ctx context.Context, name, unit string) (NutrientType, error) {
	row, err := s.q.CreateNutrientType(ctx, sqlc.CreateNutrientTypeParams{
		Name: name,
		Unit: textOrNull(unit),
	})
	if err != nil {
		return NutrientType{}, fmt.Errorf("create nutrient type: %w", err)
	}
	return toNutrientType(row), nil
}

// GetNutrientTypeByID returns a nutrient type by its primary key.
func (s *Service) GetNutrientTypeByID(ctx context.Context, nutrientID int64) (NutrientType, error) {
	row, err := s.q.GetNutrientTypeByID(ctx, nutrientID)
	if err != nil {
		return NutrientType{}, fmt.Errorf("get nutrient type by id: %w", domainerr.FromStorage(err))
	}
	return toNutrientType(row), nil
}

// GetNutrientTypeByName returns a nutrient type matching the given name
// case-insensitively, or pgx.ErrNoRows if none exists.
func (s *Service) GetNutrientTypeByName(ctx context.Context, name string) (NutrientType, error) {
	row, err := s.q.GetNutrientTypeByName(ctx, name)
	if err != nil {
		return NutrientType{}, fmt.Errorf("get nutrient type by name: %w", domainerr.FromStorage(err))
	}
	return toNutrientType(row), nil
}

// ListNutrientTypes returns all nutrient types ordered by name.
func (s *Service) ListNutrientTypes(ctx context.Context) ([]NutrientType, error) {
	rows, err := s.q.ListNutrientTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nutrient types: %w", err)
	}
	out := make([]NutrientType, len(rows))
	for i := range rows {
		out[i] = toNutrientType(rows[i])
	}
	return out, nil
}

// UpdateNutrientType modifies an existing nutrient type.
func (s *Service) UpdateNutrientType(ctx context.Context, nutrientID int64, name, unit string) (NutrientType, error) {
	row, err := s.q.UpdateNutrientType(ctx, sqlc.UpdateNutrientTypeParams{
		NutrientID: nutrientID,
		Name:       name,
		Unit:       textOrNull(unit),
	})
	if err != nil {
		return NutrientType{}, fmt.Errorf("update nutrient type: %w", domainerr.FromStorage(err))
	}
	return toNutrientType(row), nil
}

// DeleteNutrientType removes a nutrient type from the catalog.
func (s *Service) DeleteNutrientType(ctx context.Context, nutrientID int64) error {
	return s.q.DeleteNutrientType(ctx, nutrientID)
}

// FoodNutrient is a nutrient value for a catalog item. The amount is
// expressed per BasisQuantity of BasisUnitID (default: per 100 grams).
type FoodNutrient struct {
	ItemID        int64
	NutrientID    int64
	Name          string
	Unit          string
	Amount        float64
	BasisQuantity float64
	BasisUnitID   int64
}

// ListFoodNutrientsByItem returns nutrient values for an item.
func (s *Service) ListFoodNutrientsByItem(ctx context.Context, itemID int64) ([]FoodNutrient, error) {
	rows, err := s.q.ListFoodNutrientsByItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list food nutrients by item: %w", err)
	}
	out := make([]FoodNutrient, len(rows))
	for i := range rows {
		fn, err := toFoodNutrient(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list food nutrients by item: %w", err)
		}
		out[i] = fn
	}
	return out, nil
}

// ListFoodNutrientsByItems returns nutrient values for a set of items in a
// single query; each result carries the item it belongs to.
func (s *Service) ListFoodNutrientsByItems(ctx context.Context, itemIDs []int64) ([]FoodNutrient, error) {
	rows, err := s.q.ListFoodNutrientsByItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("list food nutrients by items: %w", err)
	}
	out := make([]FoodNutrient, len(rows))
	for i := range rows {
		amount, err := numericToFloat64(rows[i].Amount)
		if err != nil {
			return nil, fmt.Errorf("list food nutrients by items: item %d nutrient %d: %w", rows[i].FoodID, rows[i].NutrientID, err)
		}
		basisQty, err := numericToFloat64(rows[i].BasisQuantity)
		if err != nil {
			return nil, fmt.Errorf("list food nutrients by items: item %d nutrient %d basis: %w", rows[i].FoodID, rows[i].NutrientID, err)
		}
		out[i] = FoodNutrient{
			ItemID:        rows[i].FoodID,
			NutrientID:    rows[i].NutrientID,
			Name:          rows[i].Name,
			Unit:          rows[i].Unit.String,
			Amount:        amount,
			BasisQuantity: basisQty,
			BasisUnitID:   rows[i].BasisUnitID,
		}
	}
	return out, nil
}

// CreateFoodNutrient adds a nutrient value to an item.
func (s *Service) CreateFoodNutrient(ctx context.Context, itemID, nutrientID int64, amount float64, by string) (FoodNutrient, error) {
	n, err := numericFromFloat64(amount)
	if err != nil {
		return FoodNutrient{}, fmt.Errorf("create food nutrient: %w", err)
	}
	row, err := s.q.CreateFoodNutrient(ctx, sqlc.CreateFoodNutrientParams{
		FoodID:     itemID,
		NutrientID: nutrientID,
		Amount:     n,
		CreatedBy:  by,
	})
	if err != nil {
		return FoodNutrient{}, fmt.Errorf("create food nutrient: %w", err)
	}
	result, err := numericToFloat64(row.Amount)
	if err != nil {
		return FoodNutrient{}, fmt.Errorf("create food nutrient: %w", err)
	}
	basisQty, err := numericToFloat64(row.BasisQuantity)
	if err != nil {
		return FoodNutrient{}, fmt.Errorf("create food nutrient: %w", err)
	}
	return FoodNutrient{
		ItemID:        row.FoodID,
		NutrientID:    row.NutrientID,
		Name:          row.Name,
		Unit:          row.Unit.String,
		Amount:        result,
		BasisQuantity: basisQty,
		BasisUnitID:   row.BasisUnitID,
	}, nil
}

// DeleteFoodNutrient removes a nutrient value from an item.
func (s *Service) DeleteFoodNutrient(ctx context.Context, itemID, nutrientID int64) error {
	return s.q.DeleteFoodNutrient(ctx, sqlc.DeleteFoodNutrientParams{
		FoodID:     itemID,
		NutrientID: nutrientID,
	})
}

// NutrientEntry is a single nutrient value in a SetItemNutrients call.
type NutrientEntry struct {
	NutrientID int64
	Amount     float64
	// BasisQuantity/BasisUnitID declare the quantity of food the amount
	// refers to; nil means the default basis of 100 grams.
	BasisQuantity *float64
	BasisUnitID   *int64
}

// SetItemNutrients replaces all nutrient values on an item atomically: the
// existing rows are deleted and the given entries inserted in one
// transaction. This is the single write path for nutrition data; both the
// manual mobile form and a future OCR label-scan pipeline feed it the same
// entry list.
func (s *Service) SetItemNutrients(ctx context.Context, itemID int64, entries []NutrientEntry, by string) error {
	return s.InTx(ctx, func(tx *Service) error {
		if err := tx.q.DeleteFoodNutrientsByItem(ctx, itemID); err != nil {
			return fmt.Errorf("clear food nutrients: %w", err)
		}
		for _, e := range entries {
			n, err := numericFromFloat64(e.Amount)
			if err != nil {
				return fmt.Errorf("set food nutrients: %w", err)
			}
			var basisQty any
			if e.BasisQuantity != nil {
				bq, err := numericFromFloat64(*e.BasisQuantity)
				if err != nil {
					return fmt.Errorf("set food nutrients: %w", err)
				}
				basisQty = bq
			}
			var basisUnit any
			if e.BasisUnitID != nil {
				basisUnit = *e.BasisUnitID
			}
			if _, err := tx.q.CreateFoodNutrient(ctx, sqlc.CreateFoodNutrientParams{
				FoodID:        itemID,
				NutrientID:    e.NutrientID,
				Amount:        n,
				CreatedBy:     by,
				BasisQuantity: basisQty,
				BasisUnitID:   basisUnit,
			}); err != nil {
				return fmt.Errorf("set food nutrients: %w", err)
			}
		}
		return nil
	})
}

// FoodFlavor is a flavor profile attached to a catalog item.
type FoodFlavor struct {
	ItemID    int64
	FlavorID  int64
	Name      string
	Intensity int16
}

// CreateFoodFlavor adds a flavor to an item.
func (s *Service) CreateFoodFlavor(ctx context.Context, itemID, flavorID int64, intensity int16, by string) (FoodFlavor, error) {
	row, err := s.q.CreateFoodFlavor(ctx, sqlc.CreateFoodFlavorParams{
		FoodID:    itemID,
		FlavorID:  flavorID,
		Intensity: intensity,
		CreatedBy: by,
	})
	if err != nil {
		return FoodFlavor{}, fmt.Errorf("create food flavor: %w", err)
	}
	return FoodFlavor{
		ItemID:    row.FoodID,
		FlavorID:  row.FlavorID,
		Name:      row.Name,
		Intensity: row.Intensity,
	}, nil
}

// ListFoodFlavorsByItem returns the flavor profiles for an item.
func (s *Service) ListFoodFlavorsByItem(ctx context.Context, itemID int64) ([]FoodFlavor, error) {
	rows, err := s.q.ListFoodFlavorsByItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list food flavors by item: %w", err)
	}
	out := make([]FoodFlavor, len(rows))
	for i := range rows {
		out[i] = toFoodFlavor(rows[i])
		out[i].ItemID = itemID
	}
	return out, nil
}

// ListFoodFlavorsByItems returns the flavor profiles for a set of items in a
// single query; each result carries the item it belongs to.
func (s *Service) ListFoodFlavorsByItems(ctx context.Context, itemIDs []int64) ([]FoodFlavor, error) {
	rows, err := s.q.ListFoodFlavorsByItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("list food flavors by items: %w", err)
	}
	out := make([]FoodFlavor, len(rows))
	for i := range rows {
		out[i] = FoodFlavor{
			ItemID:    rows[i].FoodID,
			FlavorID:  rows[i].FlavorID,
			Name:      rows[i].Name,
			Intensity: rows[i].Intensity,
		}
	}
	return out, nil
}

// DeleteFoodFlavor removes a flavor from an item.
func (s *Service) DeleteFoodFlavor(ctx context.Context, itemID, flavorID int64) error {
	return s.q.DeleteFoodFlavor(ctx, sqlc.DeleteFoodFlavorParams{
		FoodID:   itemID,
		FlavorID: flavorID,
	})
}

// Ingredient is a brand-agnostic generic ingredient (e.g. "all-purpose
// flour"). Branded items exist for barcode scanning; recipes, meal slots
// and grocery lists can reference an ingredient instead. Scaffolding only:
// nothing populates this data yet.
type Ingredient struct {
	IngredientID  int64
	Name          string
	CategoryID    *int64
	DefaultUnitID *int64
	IsActive      bool
}

// CreateIngredient adds a new generic ingredient.
func (s *Service) CreateIngredient(ctx context.Context, arg Ingredient, by string) (Ingredient, error) {
	row, err := s.q.CreateIngredient(ctx, sqlc.CreateIngredientParams{
		Name:          arg.Name,
		CategoryID:    optInt64(arg.CategoryID),
		DefaultUnitID: optInt64(arg.DefaultUnitID),
		IsActive:      arg.IsActive,
		CreatedBy:     by,
		UpdatedBy:     textOrNull(by),
	})
	if err != nil {
		return Ingredient{}, fmt.Errorf("create ingredient: %w", err)
	}
	return toIngredient(row), nil
}

// GetIngredientByID returns an ingredient by its primary key.
func (s *Service) GetIngredientByID(ctx context.Context, ingredientID int64) (Ingredient, error) {
	row, err := s.q.GetIngredientByID(ctx, ingredientID)
	if err != nil {
		return Ingredient{}, fmt.Errorf("get ingredient by id: %w", domainerr.FromStorage(err))
	}
	return toIngredient(row), nil
}

// GetIngredientsByIDs returns a set of ingredients in a single query.
func (s *Service) GetIngredientsByIDs(ctx context.Context, ingredientIDs []int64) ([]Ingredient, error) {
	rows, err := s.q.GetIngredientsByIDs(ctx, ingredientIDs)
	if err != nil {
		return nil, fmt.Errorf("get ingredients by ids: %w", err)
	}
	out := make([]Ingredient, len(rows))
	for i := range rows {
		out[i] = toIngredient(rows[i])
	}
	return out, nil
}

// ListIngredients returns a paginated list of ingredients ordered by name.
func (s *Service) ListIngredients(ctx context.Context, limit, offset int32) ([]Ingredient, error) {
	rows, err := s.q.ListIngredients(ctx, sqlc.ListIngredientsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list ingredients: %w", err)
	}
	out := make([]Ingredient, len(rows))
	for i := range rows {
		out[i] = toIngredient(rows[i])
	}
	return out, nil
}

// CountIngredients returns the total number of ingredients.
func (s *Service) CountIngredients(ctx context.Context) (int64, error) {
	n, err := s.q.CountIngredients(ctx)
	if err != nil {
		return 0, fmt.Errorf("count ingredients: %w", err)
	}
	return n, nil
}

// SearchIngredients returns one page of ingredients filtered by an optional
// name term and ordered by engagement tier before paging.
func (s *Service) SearchIngredients(ctx context.Context, term string, rank RankParams, limit, offset int32) ([]Ingredient, error) {
	rows, err := s.q.SearchIngredients(ctx, sqlc.SearchIngredientsParams{
		Search:       textOrNull(term),
		PersonalIds:  rank.PersonalIDs,
		HouseholdIds: rank.HouseholdIDs,
		SearchTerms:  rank.SearchTerms,
		GlobalIds:    rank.GlobalIDs,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		return nil, fmt.Errorf("search ingredients: %w", err)
	}
	out := make([]Ingredient, len(rows))
	for i := range rows {
		out[i] = toIngredient(rows[i])
	}
	return out, nil
}

// CountSearchIngredients returns the un-paged match count for the same term
// filter as SearchIngredients.
func (s *Service) CountSearchIngredients(ctx context.Context, term string) (int64, error) {
	n, err := s.q.CountSearchIngredients(ctx, textOrNull(term))
	if err != nil {
		return 0, fmt.Errorf("count search ingredients: %w", err)
	}
	return n, nil
}

// UpdateIngredient modifies an existing ingredient.
func (s *Service) UpdateIngredient(ctx context.Context, ingredientID int64, arg Ingredient, by string) (Ingredient, error) {
	row, err := s.q.UpdateIngredient(ctx, sqlc.UpdateIngredientParams{
		IngredientID:  ingredientID,
		Name:          arg.Name,
		CategoryID:    optInt64(arg.CategoryID),
		DefaultUnitID: optInt64(arg.DefaultUnitID),
		IsActive:      arg.IsActive,
		UpdatedBy:     textOrNull(by),
	})
	if err != nil {
		return Ingredient{}, fmt.Errorf("update ingredient: %w", domainerr.FromStorage(err))
	}
	return toIngredient(row), nil
}

// DeleteIngredient removes an ingredient from the catalog.
func (s *Service) DeleteIngredient(ctx context.Context, ingredientID int64) error {
	return s.q.DeleteIngredient(ctx, ingredientID)
}

// normalizeIngredientName produces the canonical spelling stored for an
// ingredient: whitespace-collapsed, trimmed, and lowercased — the same
// case-folded convention the seed catalog uses, so display and matching
// agree.
func normalizeIngredientName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// GetOrCreateIngredient returns the ingredient whose normalized name
// matches, creating it when none exists. The unique index on the
// normalized expression makes duplicates impossible; on a racing insert
// the unique violation is converted into a second lookup.
func (s *Service) GetOrCreateIngredient(ctx context.Context, name string, categoryID, defaultUnitID *int64, by string) (Ingredient, error) {
	name = normalizeIngredientName(name)
	if name == "" {
		return Ingredient{}, fmt.Errorf("get or create ingredient: %w", domainerr.ErrValidation)
	}
	if row, err := s.q.FindIngredientByNormalizedName(ctx, name); err == nil {
		return toIngredient(row), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Ingredient{}, fmt.Errorf("get or create ingredient: %w", domainerr.FromStorage(err))
	}
	row, err := s.q.CreateIngredient(ctx, sqlc.CreateIngredientParams{
		Name:          name,
		CategoryID:    optInt64(categoryID),
		DefaultUnitID: optInt64(defaultUnitID),
		IsActive:      true,
		CreatedBy:     by,
		UpdatedBy:     textOrNull(by),
	})
	if err == nil {
		return toIngredient(row), nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return Ingredient{}, fmt.Errorf("get or create ingredient: %w", domainerr.FromStorage(err))
	}
	// Lost the insert race — the winner's row is now visible.
	row, err = s.q.FindIngredientByNormalizedName(ctx, name)
	if err != nil {
		return Ingredient{}, fmt.Errorf("get or create ingredient: %w", domainerr.FromStorage(err))
	}
	return toIngredient(row), nil
}

// ResolveItemIngredient returns the effective ingredient for a branded
// item under a household: the household's override wins over the
// catalog-level link. A nil result means the item is unlinked.
func (s *Service) ResolveItemIngredient(ctx context.Context, householdID, itemID int64) (*int64, error) {
	row, err := s.q.GetItemIngredient(ctx, sqlc.GetItemIngredientParams{
		ItemID:      itemID,
		HouseholdID: householdID,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve item ingredient: %w", domainerr.FromStorage(err))
	}
	if row.OverrideIngredientID.Valid {
		return &row.OverrideIngredientID.Int64, nil
	}
	if !row.IngredientID.Valid {
		return nil, nil
	}
	return &row.IngredientID.Int64, nil
}

// ResolveIngredientItems lists every item that resolves to the ingredient
// under the same override-wins order, limited to items visible to the user.
func (s *Service) ResolveIngredientItems(ctx context.Context, householdID, ingredientID, userID int64) ([]Item, error) {
	rows, err := s.q.ListItemsForIngredient(ctx, sqlc.ListItemsForIngredientParams{
		IngredientID:      ingredientID,
		HouseholdID:       householdID,
		SubmittedByUserID: pgtype.Int8{Int64: userID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("resolve ingredient items: %w", err)
	}
	out := make([]Item, len(rows))
	for i := range rows {
		out[i] = toItem(rows[i])
	}
	return out, nil
}

// SetItemIngredient writes the catalog-level item -> ingredient link.
// Pass nil to clear it.
func (s *Service) SetItemIngredient(ctx context.Context, itemID int64, ingredientID *int64, by string) error {
	n, err := s.q.SetItemIngredient(ctx, sqlc.SetItemIngredientParams{
		ItemID:       itemID,
		IngredientID: optInt64(ingredientID),
		UpdatedBy:    textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set item ingredient: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set item ingredient: %w", domainerr.ErrNotFound)
	}
	return nil
}

// SetItemIngredientOverride records that a household treats this item as a
// different ingredient than the catalog link says.
func (s *Service) SetItemIngredientOverride(ctx context.Context, householdID, itemID, ingredientID int64, by string) error {
	if err := s.q.UpsertItemIngredientOverride(ctx, sqlc.UpsertItemIngredientOverrideParams{
		HouseholdID:  householdID,
		ItemID:       itemID,
		IngredientID: ingredientID,
		CreatedBy:    by,
	}); err != nil {
		return fmt.Errorf("set item ingredient override: %w", domainerr.FromStorage(err))
	}
	return nil
}

// ClearItemIngredientOverride removes the household remap, restoring the
// catalog-level resolution.
func (s *Service) ClearItemIngredientOverride(ctx context.Context, householdID, itemID int64) error {
	n, err := s.q.DeleteItemIngredientOverride(ctx, sqlc.DeleteItemIngredientOverrideParams{
		HouseholdID: householdID,
		ItemID:      itemID,
	})
	if err != nil {
		return fmt.Errorf("clear item ingredient override: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("clear item ingredient override: %w", domainerr.ErrNotFound)
	}
	return nil
}

// UsualItem is the brand a household habitually buys for an ingredient.
type UsualItem struct {
	ItemID     int64
	LastUsedAt time.Time
}

// GetUsualItemForIngredient returns the household's usual brand for an
// ingredient, or nil when none has been recorded.
func (s *Service) GetUsualItemForIngredient(ctx context.Context, householdID, ingredientID int64) (*UsualItem, error) {
	row, err := s.q.GetUsualItemForIngredient(ctx, sqlc.GetUsualItemForIngredientParams{
		HouseholdID:  householdID,
		IngredientID: ingredientID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get usual item: %w", domainerr.FromStorage(err))
	}
	return &UsualItem{ItemID: row.ItemID, LastUsedAt: row.LastUsedAt}, nil
}

// SetUsualItemForIngredient records the item a household just bought for
// the ingredient, refreshing the usual-brand record.
func (s *Service) SetUsualItemForIngredient(ctx context.Context, householdID, ingredientID, itemID int64, by string) error {
	if _, err := s.q.UpsertUsualItemForIngredient(ctx, sqlc.UpsertUsualItemForIngredientParams{
		HouseholdID:  householdID,
		IngredientID: ingredientID,
		ItemID:       itemID,
		CreatedBy:    by,
	}); err != nil {
		return fmt.Errorf("set usual item: %w", domainerr.FromStorage(err))
	}
	return nil
}

// GetUsualItemsForIngredients is the batch form of
// GetUsualItemForIngredient, used by grocery-line preloads.
func (s *Service) GetUsualItemsForIngredients(ctx context.Context, householdID int64, ingredientIDs []int64) (map[int64]UsualItem, error) {
	out := make(map[int64]UsualItem, len(ingredientIDs))
	if len(ingredientIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.ListUsualItemsForIngredients(ctx, sqlc.ListUsualItemsForIngredientsParams{
		HouseholdID:   householdID,
		IngredientIds: ingredientIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list usual items: %w", domainerr.FromStorage(err))
	}
	for _, r := range rows {
		out[r.IngredientID] = UsualItem{ItemID: r.ItemID, LastUsedAt: r.LastUsedAt}
	}
	return out, nil
}

// ResolveItemIngredients is the batch form of ResolveItemIngredient: every
// requested item maps to its resolved ingredient ID or nil when unlinked.
func (s *Service) ResolveItemIngredients(ctx context.Context, householdID int64, itemIDs []int64) (map[int64]*int64, error) {
	out := make(map[int64]*int64, len(itemIDs))
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.GetItemIngredientsForItems(ctx, sqlc.GetItemIngredientsForItemsParams{
		HouseholdID: householdID,
		ItemIds:     itemIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve item ingredients: %w", domainerr.FromStorage(err))
	}
	for _, r := range rows {
		switch {
		case r.OverrideIngredientID.Valid:
			v := r.OverrideIngredientID.Int64
			out[r.ItemID] = &v
		case r.IngredientID.Valid:
			v := r.IngredientID.Int64
			out[r.ItemID] = &v
		default:
			out[r.ItemID] = nil
		}
	}
	return out, nil
}

// RepresentativeItemForIngredient picks a branded item to stand in for the
// ingredient where an item is required (e.g. nutrition rollups). Order:
// household usual brand → first linked item for the household → nil.
func (s *Service) RepresentativeItemForIngredient(ctx context.Context, householdID, ingredientID, userID int64) (*Item, error) {
	if usual, err := s.GetUsualItemForIngredient(ctx, householdID, ingredientID); err != nil {
		return nil, err
	} else if usual != nil {
		it, err := s.GetItemByID(ctx, usual.ItemID)
		if err == nil {
			return &it, nil
		}
		if !errors.Is(err, domainerr.ErrNotFound) {
			return nil, err
		}
	}
	items, err := s.ResolveIngredientItems(ctx, householdID, ingredientID, userID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

// MergeIngredients folds source into target: every reference across
// recipes, meal slots, events, grocery lines, aisle routing, item links,
// and household prefs is repointed, then the source row is deleted. Rows
// that would collide with an existing target reference keep their item
// hint (demoted to item-only) or are dropped as true duplicates.
func (s *Service) MergeIngredients(ctx context.Context, sourceID, targetID int64) error {
	if sourceID == targetID {
		return fmt.Errorf("merge ingredients: %w", domainerr.ErrValidation)
	}
	return s.InTx(ctx, func(tx *Service) error {
		// Both rows must exist before any repointing happens.
		if _, err := tx.q.GetIngredientByID(ctx, sourceID); err != nil {
			return fmt.Errorf("merge ingredients source: %w", domainerr.FromStorage(err))
		}
		if _, err := tx.q.GetIngredientByID(ctx, targetID); err != nil {
			return fmt.Errorf("merge ingredients target: %w", domainerr.FromStorage(err))
		}
		// Columns declared nullable generate pgtype.Int8 params; the
		// NOT NULL userprefs keys take plain int64.
		nn := func(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }
		steps := []func(context.Context, int64, int64) error{
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsItem(ctx, sqlc.MergeIngredientRefsItemParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsOverride(ctx, sqlc.MergeIngredientRefsOverrideParams{IngredientID: src, IngredientID_2: tgt})
			},
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsUsual(ctx, sqlc.MergeIngredientRefsUsualParams{IngredientID: src, IngredientID_2: tgt})
			},
			func(ctx context.Context, src, _ int64) error { return tx.q.DeleteIngredientRefsUsual(ctx, src) },
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsRecipeItem(ctx, sqlc.MergeIngredientRefsRecipeItemParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, _ int64) error {
				return tx.q.DemoteIngredientRefsRecipeItem(ctx, nn(src))
			},
			func(ctx context.Context, src, _ int64) error {
				return tx.q.DeleteIngredientRefsRecipeItem(ctx, nn(src))
			},
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsMealSlot(ctx, sqlc.MergeIngredientRefsMealSlotParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, _ int64) error { return tx.q.DemoteIngredientRefsMealSlot(ctx, nn(src)) },
			func(ctx context.Context, src, _ int64) error { return tx.q.DeleteIngredientRefsMealSlot(ctx, nn(src)) },
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsEventRecipeItem(ctx, sqlc.MergeIngredientRefsEventRecipeItemParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, _ int64) error {
				return tx.q.DemoteIngredientRefsEventRecipeItem(ctx, nn(src))
			},
			func(ctx context.Context, src, _ int64) error {
				return tx.q.DeleteIngredientRefsEventRecipeItem(ctx, nn(src))
			},
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsGroceryLine(ctx, sqlc.MergeIngredientRefsGroceryLineParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, _ int64) error {
				return tx.q.DemoteIngredientRefsGroceryLine(ctx, nn(src))
			},
			func(ctx context.Context, src, _ int64) error {
				return tx.q.DeleteIngredientRefsGroceryLine(ctx, nn(src))
			},
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsAisle(ctx, sqlc.MergeIngredientRefsAisleParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, _ int64) error { return tx.q.DeleteIngredientRefsAisle(ctx, nn(src)) },
			func(ctx context.Context, src, tgt int64) error {
				return tx.q.MergeIngredientRefsRoute(ctx, sqlc.MergeIngredientRefsRouteParams{IngredientID: nn(src), IngredientID_2: nn(tgt)})
			},
			func(ctx context.Context, src, _ int64) error { return tx.q.DeleteIngredientRefsRoute(ctx, nn(src)) },
		}
		for _, step := range steps {
			if err := step(ctx, sourceID, targetID); err != nil {
				return fmt.Errorf("merge ingredients: %w", domainerr.FromStorage(err))
			}
		}
		return tx.q.DeleteIngredient(ctx, sourceID)
	})
}

func toIngredient(row sqlc.InventoryIngredient) Ingredient {
	in := Ingredient{
		IngredientID: row.IngredientID,
		Name:         row.Name,
		IsActive:     row.IsActive,
	}
	if row.CategoryID.Valid {
		in.CategoryID = &row.CategoryID.Int64
	}
	if row.DefaultUnitID.Valid {
		in.DefaultUnitID = &row.DefaultUnitID.Int64
	}
	return in
}

// Unit is a canonical unit of measure shared by every domain. Kind is one
// of "volume", "weight", or "count" and enables future unit conversion.
// ToBaseFactor is nil for count units and for units without a defined conversion.
type Unit struct {
	UnitID       int64
	Name         string
	Abbreviation string
	Kind         string
	IsActive     bool
	ToBaseFactor *float64
}

// CreateUnit adds a new unit of measure.
func (s *Service) CreateUnit(ctx context.Context, arg Unit, by string) (Unit, error) {
	toBase, err := numericFromOptionalFloat64(arg.ToBaseFactor)
	if err != nil {
		return Unit{}, fmt.Errorf("create unit: %w", err)
	}
	row, err := s.q.CreateUnit(ctx, sqlc.CreateUnitParams{
		Name:         arg.Name,
		Abbreviation: textOrNull(arg.Abbreviation),
		Kind:         arg.Kind,
		ToBaseFactor: toBase,
		IsActive:     arg.IsActive,
		CreatedBy:    by,
		UpdatedBy:    textOrNull(by),
	})
	if err != nil {
		return Unit{}, fmt.Errorf("create unit: %w", err)
	}
	return toUnit(row), nil
}

// GetUnitByID returns a unit by its primary key.
func (s *Service) GetUnitByID(ctx context.Context, unitID int64) (Unit, error) {
	row, err := s.q.GetUnitByID(ctx, unitID)
	if err != nil {
		return Unit{}, fmt.Errorf("get unit by id: %w", domainerr.FromStorage(err))
	}
	return toUnit(row), nil
}

// GetUnitByName returns a unit matching a name or abbreviation,
// case-insensitively. Callers should trim the input first.
func (s *Service) GetUnitByName(ctx context.Context, name string) (Unit, error) {
	row, err := s.q.GetUnitByName(ctx, name)
	if err != nil {
		return Unit{}, fmt.Errorf("get unit by name: %w", domainerr.FromStorage(err))
	}
	return toUnit(row), nil
}

// GetUnitsByIDs returns a set of units in a single query.
func (s *Service) GetUnitsByIDs(ctx context.Context, unitIDs []int64) ([]Unit, error) {
	rows, err := s.q.GetUnitsByIDs(ctx, unitIDs)
	if err != nil {
		return nil, fmt.Errorf("get units by ids: %w", err)
	}
	out := make([]Unit, len(rows))
	for i := range rows {
		out[i] = toUnit(rows[i])
	}
	return out, nil
}

// ListUnits returns all units grouped by kind then name.
func (s *Service) ListUnits(ctx context.Context) ([]Unit, error) {
	rows, err := s.q.ListUnits(ctx)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}
	out := make([]Unit, len(rows))
	for i := range rows {
		out[i] = toUnit(rows[i])
	}
	return out, nil
}

func toUnit(row sqlc.InventoryUnit) Unit {
	u := Unit{
		UnitID:       row.UnitID,
		Name:         row.Name,
		Abbreviation: row.Abbreviation.String,
		Kind:         row.Kind,
		IsActive:     row.IsActive,
	}
	if row.ToBaseFactor.Valid {
		v, err := numericToFloat64(row.ToBaseFactor)
		if err == nil {
			u.ToBaseFactor = &v
		}
	}
	return u
}

func toCategory(row sqlc.InventoryCategory) Category {
	return Category{
		CategoryID:  row.CategoryID,
		Name:        row.Name,
		Description: row.Description.String,
		IsActive:    row.IsActive,
		IsProtein:   row.IsProtein,
	}
}

func toBrand(row sqlc.InventoryBrand) Brand {
	b := Brand{
		BrandID:   row.BrandID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		Status:    row.Status,
	}
	if row.SubmittedByUserID.Valid {
		b.SubmittedByUserID = &row.SubmittedByUserID.Int64
	}
	if row.ApprovedByUserID.Valid {
		b.ApprovedByUserID = &row.ApprovedByUserID.Int64
	}
	if row.ApprovedAt.Valid {
		b.ApprovedAt = &row.ApprovedAt.Time
	}
	return b
}

func toItem(row sqlc.InventoryItem) Item {
	it := Item{
		ItemID:     row.ItemID,
		Name:       row.Name,
		Upc12:      row.Upc12.String,
		Upc14:      row.Upc14.String,
		CategoryID: row.CategoryID,
		UnitID:     row.UnitID,
		Status:     row.Status,
		IsMetric:   row.IsMetric,
	}
	if row.BrandID.Valid {
		it.BrandID = &row.BrandID.Int64
	}
	if row.SubmittedByUserID.Valid {
		it.SubmittedByUserID = &row.SubmittedByUserID.Int64
	}
	if row.ApprovedByUserID.Valid {
		it.ApprovedByUserID = &row.ApprovedByUserID.Int64
	}
	if row.ApprovedAt.Valid {
		it.ApprovedAt = &row.ApprovedAt.Time
	}
	if v, err := numericToOptionalFloat64(row.NetWeight); err == nil {
		it.NetWeight = v
	}
	if row.IngredientID.Valid {
		it.IngredientID = &row.IngredientID.Int64
	}
	return it
}

// Allergen flag kinds shared by ingredient- and item-level allergen rows.
// "contains" marks a known presence; "may_contain" records a
// cross-contamination advisory. Warnings prefer over-warning, so a union
// that mixes both keeps "contains" for that allergen.
const (
	AllergenFlagContains   = "contains"
	AllergenFlagMayContain = "may_contain"
)

// Allergen is one registry entry a member can be allergic to or restrict
// by diet (peanuts, gluten, pork, ...).
type Allergen struct {
	AllergenID  int64
	Name        string
	Description string
	IsActive    bool
}

// EntityAllergen is one flag on an ingredient or item: which allergen and
// whether it is a confirmed contains or a may-contain advisory.
type EntityAllergen struct {
	AllergenID int64
	Kind       string
}

func validAllergenFlagKind(kind string) bool {
	return kind == AllergenFlagContains || kind == AllergenFlagMayContain
}

// ListAllergens returns every registry entry, including inactive ones so
// admin tooling can reactivate them.
func (s *Service) ListAllergens(ctx context.Context) ([]Allergen, error) {
	rows, err := s.q.ListAllergens(ctx)
	if err != nil {
		return nil, fmt.Errorf("list allergens: %w", err)
	}
	out := make([]Allergen, len(rows))
	for i := range rows {
		out[i] = toAllergen(rows[i])
	}
	return out, nil
}

// GetAllergenByID returns one registry entry.
func (s *Service) GetAllergenByID(ctx context.Context, allergenID int64) (Allergen, error) {
	row, err := s.q.GetAllergenByID(ctx, allergenID)
	if err != nil {
		return Allergen{}, fmt.Errorf("get allergen by id: %w", domainerr.FromStorage(err))
	}
	return toAllergen(row), nil
}

// GetAllergensByIDs batch-loads registry entries keyed by ID — feeds the
// preload helpers so warnings never issue a query per allergen.
func (s *Service) GetAllergensByIDs(ctx context.Context, allergenIDs []int64) (map[int64]Allergen, error) {
	out := make(map[int64]Allergen, len(allergenIDs))
	if len(allergenIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.GetAllergensByIDs(ctx, allergenIDs)
	if err != nil {
		return nil, fmt.Errorf("get allergens by ids: %w", err)
	}
	for _, r := range rows {
		out[r.AllergenID] = toAllergen(r)
	}
	return out, nil
}

// CreateAllergen adds a registry entry.
func (s *Service) CreateAllergen(ctx context.Context, name, description, by string) (Allergen, error) {
	row, err := s.q.CreateAllergen(ctx, sqlc.CreateAllergenParams{
		Name:        name,
		Description: textOrNull(description),
		IsActive:    true,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Allergen{}, fmt.Errorf("create allergen: %w", domainerr.FromStorage(err))
	}
	return toAllergen(row), nil
}

// UpdateAllergen renames, redescribes, or activates/deactivates a registry
// entry.
func (s *Service) UpdateAllergen(ctx context.Context, allergenID int64, name, description string, isActive bool, by string) (Allergen, error) {
	row, err := s.q.UpdateAllergen(ctx, sqlc.UpdateAllergenParams{
		AllergenID:  allergenID,
		Name:        name,
		Description: textOrNull(description),
		IsActive:    isActive,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Allergen{}, fmt.Errorf("update allergen: %w", domainerr.FromStorage(err))
	}
	return toAllergen(row), nil
}

// ListIngredientAllergens returns the flags on one ingredient.
func (s *Service) ListIngredientAllergens(ctx context.Context, ingredientID int64) ([]EntityAllergen, error) {
	rows, err := s.q.ListIngredientAllergens(ctx, ingredientID)
	if err != nil {
		return nil, fmt.Errorf("list ingredient allergens: %w", err)
	}
	out := make([]EntityAllergen, len(rows))
	for i, r := range rows {
		out[i] = EntityAllergen{AllergenID: r.AllergenID, Kind: r.Kind}
	}
	return out, nil
}

// ListIngredientAllergensByIngredients batch-loads flags keyed by
// ingredient ID for the preload helpers.
func (s *Service) ListIngredientAllergensByIngredients(ctx context.Context, ingredientIDs []int64) (map[int64][]EntityAllergen, error) {
	out := make(map[int64][]EntityAllergen, len(ingredientIDs))
	if len(ingredientIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.ListIngredientAllergensByIngredients(ctx, ingredientIDs)
	if err != nil {
		return nil, fmt.Errorf("list ingredient allergens by ingredients: %w", err)
	}
	for _, r := range rows {
		out[r.IngredientID] = append(out[r.IngredientID], EntityAllergen{AllergenID: r.AllergenID, Kind: r.Kind})
	}
	return out, nil
}

// SetIngredientAllergen upserts one ingredient flag; kind must be
// "contains" or "may_contain".
func (s *Service) SetIngredientAllergen(ctx context.Context, ingredientID, allergenID int64, kind, by string) error {
	if !validAllergenFlagKind(kind) {
		return fmt.Errorf("set ingredient allergen: invalid kind %q", kind)
	}
	n, err := s.q.UpsertIngredientAllergen(ctx, sqlc.UpsertIngredientAllergenParams{
		IngredientID: ingredientID,
		AllergenID:   allergenID,
		Kind:         kind,
		CreatedBy:    by,
		UpdatedBy:    textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set ingredient allergen: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set ingredient allergen: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ClearIngredientAllergen removes one ingredient flag.
func (s *Service) ClearIngredientAllergen(ctx context.Context, ingredientID, allergenID int64) error {
	n, err := s.q.DeleteIngredientAllergen(ctx, sqlc.DeleteIngredientAllergenParams{IngredientID: ingredientID, AllergenID: allergenID})
	if err != nil {
		return fmt.Errorf("clear ingredient allergen: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("clear ingredient allergen: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ListItemAllergens returns the product-level flags on one item.
func (s *Service) ListItemAllergens(ctx context.Context, itemID int64) ([]EntityAllergen, error) {
	rows, err := s.q.ListItemAllergens(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list item allergens: %w", err)
	}
	out := make([]EntityAllergen, len(rows))
	for i, r := range rows {
		out[i] = EntityAllergen{AllergenID: r.AllergenID, Kind: r.Kind}
	}
	return out, nil
}

// ListItemAllergensByItems batch-loads product-level flags keyed by item
// ID for the preload helpers.
func (s *Service) ListItemAllergensByItems(ctx context.Context, itemIDs []int64) (map[int64][]EntityAllergen, error) {
	out := make(map[int64][]EntityAllergen, len(itemIDs))
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.ListItemAllergensByItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("list item allergens by items: %w", err)
	}
	for _, r := range rows {
		out[r.ItemID] = append(out[r.ItemID], EntityAllergen{AllergenID: r.AllergenID, Kind: r.Kind})
	}
	return out, nil
}

// SetItemAllergen upserts one product-level flag; kind must be
// "contains" or "may_contain".
func (s *Service) SetItemAllergen(ctx context.Context, itemID, allergenID int64, kind, by string) error {
	if !validAllergenFlagKind(kind) {
		return fmt.Errorf("set item allergen: invalid kind %q", kind)
	}
	n, err := s.q.UpsertItemAllergen(ctx, sqlc.UpsertItemAllergenParams{
		ItemID:     itemID,
		AllergenID: allergenID,
		Kind:       kind,
		CreatedBy:  by,
		UpdatedBy:  textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("set item allergen: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("set item allergen: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ClearItemAllergen removes one product-level flag.
func (s *Service) ClearItemAllergen(ctx context.Context, itemID, allergenID int64) error {
	n, err := s.q.DeleteItemAllergen(ctx, sqlc.DeleteItemAllergenParams{ItemID: itemID, AllergenID: allergenID})
	if err != nil {
		return fmt.Errorf("clear item allergen: %w", domainerr.FromStorage(err))
	}
	if n == 0 {
		return fmt.Errorf("clear item allergen: %w", domainerr.ErrNotFound)
	}
	return nil
}

// ---------- allergen flag suggestions (LEN-23 review queue) ----------

// AllergenSuggestion is one reviewable flag proposal — accepted rows write
// a real ingredient/item flag; dismissed rows keep the audit trail.
type AllergenSuggestion struct {
	AllergenSuggestionID int64
	RecipeID             *int64
	RecipeName           string
	TargetKind           string // "ingredient" | "item"
	IngredientID         *int64
	IngredientName       string
	ItemID               *int64
	ItemName             string
	AllergenID           int64
	AllergenName         string
	Kind                 string // "contains" | "may_contain"
	Rationale            string
	Status               string // "pending" | "accepted" | "dismissed"
	SuggestedByUserID    *int64
	ReviewedByUserID     *int64
	ReviewedAt           *time.Time
}

// NewAllergenSuggestion is one proposal to enqueue.
type NewAllergenSuggestion struct {
	RecipeID    *int64
	TargetKind  string
	TargetID    int64
	AllergenID  int64
	Kind        string
	Rationale   string
	SuggestedBy *int64
}

// CreateAllergenSuggestions enqueues proposals. Duplicates of a still-open
// suggestion for the same target+allergen are dropped by the partial
// unique index (ON CONFLICT DO NOTHING returns pgx.ErrNoRows) — callers
// get only the rows that were actually created.
func (s *Service) CreateAllergenSuggestions(ctx context.Context, proposals []NewAllergenSuggestion, by string) ([]AllergenSuggestion, error) {
	out := make([]AllergenSuggestion, 0, len(proposals))
	for _, p := range proposals {
		if !validAllergenFlagKind(p.Kind) {
			return nil, fmt.Errorf("create allergen suggestion: invalid kind %q", p.Kind)
		}
		arg := sqlc.CreateAllergenSuggestionParams{
			TargetKind: p.TargetKind,
			AllergenID: p.AllergenID,
			Kind:       p.Kind,
			Rationale:  textOrNull(p.Rationale),
			CreatedBy:  by,
		}
		if p.RecipeID != nil {
			arg.RecipeID = pgtype.Int8{Int64: *p.RecipeID, Valid: true}
		}
		switch p.TargetKind {
		case "ingredient":
			arg.IngredientID = pgtype.Int8{Int64: p.TargetID, Valid: true}
		case "item":
			arg.ItemID = pgtype.Int8{Int64: p.TargetID, Valid: true}
		default:
			return nil, fmt.Errorf("create allergen suggestion: invalid target %q", p.TargetKind)
		}
		if p.SuggestedBy != nil {
			arg.SuggestedByUserID = pgtype.Int8{Int64: *p.SuggestedBy, Valid: true}
		}
		row, err := s.q.CreateAllergenSuggestion(ctx, arg)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // pending duplicate — skipped by the partial index
		}
		if err != nil {
			return nil, fmt.Errorf("create allergen suggestion: %w", domainerr.FromStorage(err))
		}
		out = append(out, AllergenSuggestion{AllergenSuggestionID: row.AllergenSuggestionID})
	}
	return out, nil
}

// ListAllergenSuggestions returns queue rows newest-last; empty status
// returns every row, otherwise "pending", "accepted", or "dismissed".
func (s *Service) ListAllergenSuggestions(ctx context.Context, status string) ([]AllergenSuggestion, error) {
	var st pgtype.Text
	if status != "" {
		st = pgtype.Text{String: status, Valid: true}
	}
	rows, err := s.q.ListAllergenSuggestions(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("list allergen suggestions: %w", err)
	}
	out := make([]AllergenSuggestion, len(rows))
	for i, r := range rows {
		out[i] = toAllergenSuggestion(sqlc.GetAllergenSuggestionRow(r))
	}
	return out, nil
}

// GetAllergenSuggestion loads one queue row.
func (s *Service) GetAllergenSuggestion(ctx context.Context, id int64) (AllergenSuggestion, error) {
	r, err := s.q.GetAllergenSuggestion(ctx, id)
	if err != nil {
		return AllergenSuggestion{}, fmt.Errorf("get allergen suggestion: %w", domainerr.FromStorage(err))
	}
	return toAllergenSuggestion(r), nil
}

// AcceptAllergenSuggestion writes the proposed flag under the reviewer's
// attribution and marks the suggestion accepted — one transaction so a
// flag can never land without its audit row.
func (s *Service) AcceptAllergenSuggestion(ctx context.Context, id, reviewerUserID int64, by string) error {
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		txs := s.WithTx(tx)
		row, err := txs.q.SetAllergenSuggestionStatus(ctx, sqlc.SetAllergenSuggestionStatusParams{
			AllergenSuggestionID: id,
			Status:               "accepted",
			ReviewedByUserID:     pgtype.Int8{Int64: reviewerUserID, Valid: true},
			UpdatedBy:            textOrNull(by),
		})
		if err != nil {
			return fmt.Errorf("accept allergen suggestion: %w", domainerr.FromStorage(err))
		}
		switch row.TargetKind {
		case "ingredient":
			return txs.SetIngredientAllergen(ctx, row.IngredientID.Int64, row.AllergenID, row.Kind, by)
		default:
			return txs.SetItemAllergen(ctx, row.ItemID.Int64, row.AllergenID, row.Kind, by)
		}
	})
}

// DismissAllergenSuggestion marks a pending row dismissed; a row already
// reviewed fails the WHERE status check and reports not-found.
func (s *Service) DismissAllergenSuggestion(ctx context.Context, id, reviewerUserID int64, by string) error {
	_, err := s.q.SetAllergenSuggestionStatus(ctx, sqlc.SetAllergenSuggestionStatusParams{
		AllergenSuggestionID: id,
		Status:               "dismissed",
		ReviewedByUserID:     pgtype.Int8{Int64: reviewerUserID, Valid: true},
		UpdatedBy:            textOrNull(by),
	})
	if err != nil {
		return fmt.Errorf("dismiss allergen suggestion: %w", domainerr.FromStorage(err))
	}
	return nil
}

func toAllergenSuggestion(r sqlc.GetAllergenSuggestionRow) AllergenSuggestion {
	out := AllergenSuggestion{
		AllergenSuggestionID: r.AllergenSuggestionID,
		TargetKind:           r.TargetKind,
		AllergenID:           r.AllergenID,
		AllergenName:         r.AllergenName,
		Kind:                 r.Kind,
		Rationale:            r.Rationale.String,
		Status:               r.Status,
		IngredientName:       r.IngredientName.String,
		ItemName:             r.ItemName.String,
		RecipeName:           r.RecipeName.String,
	}
	if r.RecipeID.Valid {
		out.RecipeID = &r.RecipeID.Int64
	}
	if r.IngredientID.Valid {
		out.IngredientID = &r.IngredientID.Int64
	}
	if r.ItemID.Valid {
		out.ItemID = &r.ItemID.Int64
	}
	if r.SuggestedByUserID.Valid {
		out.SuggestedByUserID = &r.SuggestedByUserID.Int64
	}
	if r.ReviewedByUserID.Valid {
		out.ReviewedByUserID = &r.ReviewedByUserID.Int64
	}
	if r.ReviewedAt.Valid {
		out.ReviewedAt = &r.ReviewedAt.Time
	}
	return out
}

func toAllergen(row sqlc.InventoryAllergen) Allergen {
	return Allergen{
		AllergenID:  row.AllergenID,
		Name:        row.Name,
		Description: row.Description.String,
		IsActive:    row.IsActive,
	}
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func toFlavorProfile(row sqlc.InventoryFlavorProfile) FlavorProfile {
	return FlavorProfile{
		FlavorID:  row.FlavorID,
		Name:      row.Name,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt,
	}
}

func toNutrientType(row sqlc.InventoryNutrientType) NutrientType {
	return NutrientType{
		NutrientID: row.NutrientID,
		Name:       row.Name,
		Unit:       row.Unit.String,
		CreatedAt:  row.CreatedAt,
	}
}

func toFoodNutrient(row sqlc.ListFoodNutrientsByItemRow) (FoodNutrient, error) {
	amount, err := numericToFloat64(row.Amount)
	if err != nil {
		return FoodNutrient{}, err
	}
	basisQty, err := numericToFloat64(row.BasisQuantity)
	if err != nil {
		return FoodNutrient{}, err
	}
	return FoodNutrient{
		NutrientID:    row.NutrientID,
		Name:          row.Name,
		Unit:          row.Unit.String,
		Amount:        amount,
		BasisQuantity: basisQty,
		BasisUnitID:   row.BasisUnitID,
	}, nil
}

func toFoodFlavor(row sqlc.ListFoodFlavorsByItemRow) FoodFlavor {
	return FoodFlavor{
		FlavorID:  row.FlavorID,
		Name:      row.Name,
		Intensity: row.Intensity,
	}
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

func numericFromOptionalFloat64(f *float64) (pgtype.Numeric, error) {
	if f == nil {
		return pgtype.Numeric{}, nil
	}
	return numericFromFloat64(*f)
}

// numericToFloat64 converts a NOT NULL numeric column to float64. Callers
// reading nullable numerics must check .Valid themselves first — a NULL
// reaching this function is data corruption, not a real zero.
func numericToFloat64(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, fmt.Errorf("convert numeric to float64: value is NULL")
	}
	v, err := n.Float64Value()
	if err != nil {
		return 0, fmt.Errorf("convert numeric to float64: %w", err)
	}
	return v.Float64, nil
}

// numericToOptionalFloat64 converts a nullable numeric column to a *float64.
func numericToOptionalFloat64(n pgtype.Numeric) (*float64, error) {
	if !n.Valid {
		return nil, nil
	}
	v, err := n.Float64Value()
	if err != nil {
		return nil, fmt.Errorf("convert numeric to float64: %w", err)
	}
	f := v.Float64
	return &f, nil
}

func optInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
