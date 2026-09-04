// Package inventory owns the catalog of food items: brands, categories,
// items, flavor profiles and nutrient types. It contains no per-user state.
package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JRAdams472/LENA2/internal/inventory/sqlc"
)

// Service provides catalog operations for the inventory domain.
type Service struct {
	q *sqlc.Queries
}

// NewService creates an inventory Service using the given connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
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

func optInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
