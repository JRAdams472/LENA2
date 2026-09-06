// Package inventory owns the catalog of food items: brands, categories,
// items, flavor profiles and nutrient types. It contains no per-user state.
package inventory

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/inventory/sqlc"
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
)

// Service provides catalog operations for the inventory domain.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
}

// NewService creates an inventory Service using the given connection pool.
func NewService(pool dbtx.Pool) *Service {
	return &Service{q: sqlc.New(pool), pool: pool}
}

// WithTx returns a copy of the service whose queries run on tx. Callers that
// hold a transaction can bind a service to it and compose multiple service
// operations into one atomic unit of work.
func (s *Service) WithTx(tx pgx.Tx) *Service {
	return &Service{q: sqlc.New(tx), pool: s.pool}
}

// InTx runs fn inside a single transaction; the *Service passed to fn is
// bound to that transaction. The transaction commits when fn returns nil and
// rolls back otherwise.
func (s *Service) InTx(ctx context.Context, fn func(*Service) error) error {
	return dbtx.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.WithTx(tx)) })
}

// Brand is a catalog brand.
type Brand struct {
	BrandID   int64
	Name      string
	CreatedAt time.Time
}

// CreateBrand adds a new brand.
func (s *Service) CreateBrand(ctx context.Context, name string) (Brand, error) {
	row, err := s.q.CreateBrand(ctx, name)
	if err != nil {
		return Brand{}, fmt.Errorf("create brand: %w", err)
	}
	return Brand{BrandID: row.BrandID, Name: row.Name, CreatedAt: row.CreatedAt}, nil
}

// GetBrandByID returns a brand by its primary key.
func (s *Service) GetBrandByID(ctx context.Context, brandID int64) (Brand, error) {
	row, err := s.q.GetBrandByID(ctx, brandID)
	if err != nil {
		return Brand{}, fmt.Errorf("get brand by id: %w", err)
	}
	return Brand{BrandID: row.BrandID, Name: row.Name, CreatedAt: row.CreatedAt}, nil
}

// ListBrands returns all brands ordered by name.
func (s *Service) ListBrands(ctx context.Context) ([]Brand, error) {
	rows, err := s.q.ListBrands(ctx)
	if err != nil {
		return nil, fmt.Errorf("list brands: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = Brand{BrandID: rows[i].BrandID, Name: rows[i].Name, CreatedAt: rows[i].CreatedAt}
	}
	return out, nil
}

// GetBrandsByIDs returns a set of brands in a single query.
func (s *Service) GetBrandsByIDs(ctx context.Context, brandIDs []int64) ([]Brand, error) {
	rows, err := s.q.GetBrandsByIDs(ctx, brandIDs)
	if err != nil {
		return nil, fmt.Errorf("get brands by ids: %w", err)
	}
	out := make([]Brand, len(rows))
	for i := range rows {
		out[i] = Brand{BrandID: rows[i].BrandID, Name: rows[i].Name, CreatedAt: rows[i].CreatedAt}
	}
	return out, nil
}

// UpdateBrand renames an existing brand.
func (s *Service) UpdateBrand(ctx context.Context, brandID int64, name string) (Brand, error) {
	row, err := s.q.UpdateBrand(ctx, sqlc.UpdateBrandParams{BrandID: brandID, Name: name})
	if err != nil {
		return Brand{}, fmt.Errorf("update brand: %w", err)
	}
	return Brand{BrandID: row.BrandID, Name: row.Name, CreatedAt: row.CreatedAt}, nil
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
}

// CreateCategory adds a new category.
func (s *Service) CreateCategory(ctx context.Context, name, description, by string) (Category, error) {
	row, err := s.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:        name,
		Description: textOrNull(description),
		IsActive:    true,
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
		return Category{}, fmt.Errorf("get category by id: %w", err)
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
func (s *Service) UpdateCategory(ctx context.Context, categoryID int64, name, description string, isActive bool, by string) (Category, error) {
	row, err := s.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		CategoryID:  categoryID,
		Name:        name,
		Description: textOrNull(description),
		IsActive:    isActive,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Category{}, fmt.Errorf("update category: %w", err)
	}
	return toCategory(row), nil
}

// DeleteCategory removes a category from the catalog.
func (s *Service) DeleteCategory(ctx context.Context, categoryID int64) error {
	return s.q.DeleteCategory(ctx, categoryID)
}

// Item is a catalog food item.
type Item struct {
	ItemID     int64
	Name       string
	BrandID    *int64
	Upc12      string
	Upc14      string
	CategoryID int64
	Unit       string
}

// CreateItem adds a new item to the catalog.
func (s *Service) CreateItem(ctx context.Context, arg Item, by string) (Item, error) {
	row, err := s.q.CreateItem(ctx, sqlc.CreateItemParams{
		Name:       arg.Name,
		BrandID:    optInt64(arg.BrandID),
		Upc12:      textOrNull(arg.Upc12),
		Upc14:      textOrNull(arg.Upc14),
		CategoryID: arg.CategoryID,
		Unit:       arg.Unit,
		CreatedBy:  by,
		UpdatedBy:  textOrNull(by),
	})
	if err != nil {
		return Item{}, fmt.Errorf("create item: %w", err)
	}
	return toItem(row), nil
}

// GetItemByID returns an item by its primary key.
func (s *Service) GetItemByID(ctx context.Context, itemID int64) (Item, error) {
	row, err := s.q.GetItemByID(ctx, itemID)
	if err != nil {
		return Item{}, fmt.Errorf("get item by id: %w", err)
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

// ListItems returns a paginated list of items ordered by name.
func (s *Service) ListItems(ctx context.Context, limit, offset int32) ([]Item, error) {
	rows, err := s.q.ListItems(ctx, sqlc.ListItemsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := make([]Item, len(rows))
	for i := range rows {
		out[i] = toItem(rows[i])
	}
	return out, nil
}

// CountItems returns the total number of catalog items.
func (s *Service) CountItems(ctx context.Context) (int64, error) {
	n, err := s.q.CountItems(ctx)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return n, nil
}

// UpdateItem modifies an existing item. All business logic about who can
// modify catalog data lives in Go, not in SQL triggers or procedures.
func (s *Service) UpdateItem(ctx context.Context, itemID int64, arg Item, by string) error {
	return s.q.UpdateItem(ctx, sqlc.UpdateItemParams{
		ItemID:     itemID,
		Name:       arg.Name,
		BrandID:    optInt64(arg.BrandID),
		Upc12:      textOrNull(arg.Upc12),
		Upc14:      textOrNull(arg.Upc14),
		CategoryID: arg.CategoryID,
		Unit:       arg.Unit,
		UpdatedBy:  textOrNull(by),
	})
}

// DeleteItem removes an item from the catalog.
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
		return FlavorProfile{}, fmt.Errorf("get flavor profile by id: %w", err)
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
		return FlavorProfile{}, fmt.Errorf("update flavor profile: %w", err)
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
		return NutrientType{}, fmt.Errorf("get nutrient type by id: %w", err)
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
		return NutrientType{}, fmt.Errorf("update nutrient type: %w", err)
	}
	return toNutrientType(row), nil
}

// DeleteNutrientType removes a nutrient type from the catalog.
func (s *Service) DeleteNutrientType(ctx context.Context, nutrientID int64) error {
	return s.q.DeleteNutrientType(ctx, nutrientID)
}

// FoodNutrient is a nutrient value for a catalog item.
type FoodNutrient struct {
	ItemID     int64
	NutrientID int64
	Name       string
	Unit       string
	Amount     float64
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
		out[i] = FoodNutrient{
			ItemID:     rows[i].FoodID,
			NutrientID: rows[i].NutrientID,
			Name:       rows[i].Name,
			Unit:       rows[i].Unit.String,
			Amount:     amount,
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
	return FoodNutrient{
		NutrientID: row.NutrientID,
		Amount:     result,
	}, nil
}

// DeleteFoodNutrient removes a nutrient value from an item.
func (s *Service) DeleteFoodNutrient(ctx context.Context, itemID, nutrientID int64) error {
	return s.q.DeleteFoodNutrient(ctx, sqlc.DeleteFoodNutrientParams{
		FoodID:     itemID,
		NutrientID: nutrientID,
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
		FlavorID:  row.FlavorID,
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

func toCategory(row sqlc.InventoryCategory) Category {
	return Category{
		CategoryID:  row.CategoryID,
		Name:        row.Name,
		Description: row.Description.String,
		IsActive:    row.IsActive,
	}
}

func toItem(row sqlc.InventoryItem) Item {
	it := Item{
		ItemID:     row.ItemID,
		Name:       row.Name,
		Upc12:      row.Upc12.String,
		Upc14:      row.Upc14.String,
		CategoryID: row.CategoryID,
		Unit:       row.Unit,
	}
	if row.BrandID.Valid {
		it.BrandID = &row.BrandID.Int64
	}
	return it
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
	return FoodNutrient{
		NutrientID: row.NutrientID,
		Name:       row.Name,
		Unit:       row.Unit.String,
		Amount:     amount,
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

func numericToFloat64(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, nil
	}
	v, err := n.Float64Value()
	if err != nil {
		return 0, fmt.Errorf("convert numeric to float64: %w", err)
	}
	return v.Float64, nil
}

func optInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
