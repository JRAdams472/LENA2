// Package wine owns the catalog of wine bottles and their reference data:
// countries, regions, types, vintages, grape varieties and flavor profiles.
package wine

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JRAdams472/LENA2/internal/wine/sqlc"
)

// Service provides catalog operations for the wine domain.
type Service struct {
	q *sqlc.Queries
}

// NewService creates a wine Service using the given connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

// Country is a catalog wine country.
type Country struct {
	CountryID   int64
	Name        string
	IsoCode     string
	Description string
	IsActive    bool
}

// CreateCountry adds a new country.
func (s *Service) CreateCountry(ctx context.Context, name, isoCode, description, by string) (Country, error) {
	row, err := s.q.CreateCountry(ctx, sqlc.CreateCountryParams{
		Name:        name,
		IsoCode:     isoCode,
		Description: textOrNull(description),
		IsActive:    true,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Country{}, fmt.Errorf("create country: %w", err)
	}
	return toCountry(row), nil
}

// GetCountryByID returns a country by its primary key.
func (s *Service) GetCountryByID(ctx context.Context, countryID int64) (Country, error) {
	row, err := s.q.GetCountryByID(ctx, countryID)
	if err != nil {
		return Country{}, fmt.Errorf("get country by id: %w", err)
	}
	return toCountry(row), nil
}

// ListCountries returns all countries ordered by name.
func (s *Service) ListCountries(ctx context.Context) ([]Country, error) {
	rows, err := s.q.ListCountries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list countries: %w", err)
	}
	out := make([]Country, len(rows))
	for i := range rows {
		out[i] = toCountry(rows[i])
	}
	return out, nil
}

// Region is a catalog wine region within a country.
type Region struct {
	RegionID    int64
	CountryID   int64
	Name        string
	Description string
	IsActive    bool
}

// CreateRegion adds a new region.
func (s *Service) CreateRegion(ctx context.Context, arg Region, by string) (Region, error) {
	row, err := s.q.CreateRegion(ctx, sqlc.CreateRegionParams{
		CountryID:   arg.CountryID,
		Name:        arg.Name,
		Description: textOrNull(arg.Description),
		IsActive:    true,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Region{}, fmt.Errorf("create region: %w", err)
	}
	return toRegion(row), nil
}

// GetRegionByID returns a region by its primary key.
func (s *Service) GetRegionByID(ctx context.Context, regionID int64) (Region, error) {
	row, err := s.q.GetRegionByID(ctx, regionID)
	if err != nil {
		return Region{}, fmt.Errorf("get region by id: %w", err)
	}
	return toRegion(row), nil
}

// ListRegions returns all regions within a country.
func (s *Service) ListRegions(ctx context.Context, countryID int64) ([]Region, error) {
	rows, err := s.q.ListRegions(ctx, countryID)
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	out := make([]Region, len(rows))
	for i := range rows {
		out[i] = toRegion(rows[i])
	}
	return out, nil
}

// Type is a catalog wine type (e.g. red, white, sparkling).
type Type struct {
	TypeID      int64
	Name        string
	Description string
	IsActive    bool
}

// CreateType adds a new wine type.
func (s *Service) CreateType(ctx context.Context, name, description, by string) (Type, error) {
	row, err := s.q.CreateType(ctx, sqlc.CreateTypeParams{
		Name:        name,
		Description: textOrNull(description),
		IsActive:    true,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Type{}, fmt.Errorf("create type: %w", err)
	}
	return toType(row), nil
}

// GetTypeByID returns a wine type by its primary key.
func (s *Service) GetTypeByID(ctx context.Context, typeID int64) (Type, error) {
	row, err := s.q.GetTypeByID(ctx, typeID)
	if err != nil {
		return Type{}, fmt.Errorf("get type by id: %w", err)
	}
	return toType(row), nil
}

// ListTypes returns all wine types ordered by name.
func (s *Service) ListTypes(ctx context.Context) ([]Type, error) {
	rows, err := s.q.ListTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list types: %w", err)
	}
	out := make([]Type, len(rows))
	for i := range rows {
		out[i] = toType(rows[i])
	}
	return out, nil
}

// Vintage is a wine vintage year.
type Vintage struct {
	VintageID   int64
	Year        int32
	Description string
	IsActive    bool
}

// CreateVintage adds a new vintage.
func (s *Service) CreateVintage(ctx context.Context, year int32, description, by string) (Vintage, error) {
	row, err := s.q.CreateVintage(ctx, sqlc.CreateVintageParams{
		Year:        year,
		Description: textOrNull(description),
		IsActive:    true,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Vintage{}, fmt.Errorf("create vintage: %w", err)
	}
	return toVintage(row), nil
}

// ListVintages returns all vintages ordered by year descending.
func (s *Service) ListVintages(ctx context.Context) ([]Vintage, error) {
	rows, err := s.q.ListVintages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vintages: %w", err)
	}
	out := make([]Vintage, len(rows))
	for i := range rows {
		out[i] = toVintage(rows[i])
	}
	return out, nil
}

// GrapeVariety is a catalog grape variety.
type GrapeVariety struct {
	GrapeVarietyID int64
	Name           string
	Description    string
	IsActive       bool
}

// CreateGrapeVariety adds a new grape variety.
func (s *Service) CreateGrapeVariety(ctx context.Context, name, description, by string) (GrapeVariety, error) {
	row, err := s.q.CreateGrapeVariety(ctx, sqlc.CreateGrapeVarietyParams{
		Name:        name,
		Description: textOrNull(description),
		IsActive:    true,
		CreatedBy:   by,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return GrapeVariety{}, fmt.Errorf("create grape variety: %w", err)
	}
	return toGrapeVariety(row), nil
}

// ListGrapeVarieties returns all grape varieties ordered by name.
func (s *Service) ListGrapeVarieties(ctx context.Context) ([]GrapeVariety, error) {
	rows, err := s.q.ListGrapeVarieties(ctx)
	if err != nil {
		return nil, fmt.Errorf("list grape varieties: %w", err)
	}
	out := make([]GrapeVariety, len(rows))
	for i := range rows {
		out[i] = toGrapeVariety(rows[i])
	}
	return out, nil
}

// Bottle is a catalog wine bottle definition.
type Bottle struct {
	BottleID       int64
	TypeID         int64
	CountryID      int64
	RegionID       int64
	VintageYear    int32
	Vineyard       string
	Abv            float64
	Acidity        int16
	TanninLevel    int16
	Body           int16
	Sweetness      int16
	OakIntegration bool
	BottleSize     string
}

// CreateBottle adds a new bottle definition.
func (s *Service) CreateBottle(ctx context.Context, arg Bottle, by string) (Bottle, error) {
	row, err := s.q.CreateBottle(ctx, sqlc.CreateBottleParams{
		TypeID:         arg.TypeID,
		CountryID:      arg.CountryID,
		RegionID:       arg.RegionID,
		VintageYear:    arg.VintageYear,
		Vineyard:       textOrNull(arg.Vineyard),
		Abv:            numericOrNull(arg.Abv),
		Acidity:        int2OrNull(arg.Acidity),
		TanninLevel:    int2OrNull(arg.TanninLevel),
		Body:           int2OrNull(arg.Body),
		Sweetness:      int2OrNull(arg.Sweetness),
		OakIntegration: boolOrNull(arg.OakIntegration),
		BottleSize:     arg.BottleSize,
		CreatedBy:      by,
		UpdatedBy:      textOrNull(by),
	})
	if err != nil {
		return Bottle{}, fmt.Errorf("create bottle: %w", err)
	}
	return toBottle(row), nil
}

// GetBottleByID returns a bottle by its primary key.
func (s *Service) GetBottleByID(ctx context.Context, bottleID int64) (Bottle, error) {
	row, err := s.q.GetBottleByID(ctx, bottleID)
	if err != nil {
		return Bottle{}, fmt.Errorf("get bottle by id: %w", err)
	}
	return toBottle(row), nil
}

// ListBottles returns a paginated list of bottles.
func (s *Service) ListBottles(ctx context.Context, limit, offset int32) ([]Bottle, error) {
	rows, err := s.q.ListBottles(ctx, sqlc.ListBottlesParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list bottles: %w", err)
	}
	out := make([]Bottle, len(rows))
	for i := range rows {
		out[i] = toBottle(rows[i])
	}
	return out, nil
}

// UpdateBottle modifies an existing bottle.
func (s *Service) UpdateBottle(ctx context.Context, bottleID int64, arg Bottle, by string) error {
	return s.q.UpdateBottle(ctx, sqlc.UpdateBottleParams{
		BottleID:       bottleID,
		TypeID:         arg.TypeID,
		CountryID:      arg.CountryID,
		RegionID:       arg.RegionID,
		VintageYear:    arg.VintageYear,
		Vineyard:       textOrNull(arg.Vineyard),
		Abv:            numericOrNull(arg.Abv),
		Acidity:        int2OrNull(arg.Acidity),
		TanninLevel:    int2OrNull(arg.TanninLevel),
		Body:           int2OrNull(arg.Body),
		Sweetness:      int2OrNull(arg.Sweetness),
		OakIntegration: boolOrNull(arg.OakIntegration),
		BottleSize:     arg.BottleSize,
		UpdatedBy:      textOrNull(by),
	})
}

// DeleteBottle removes a bottle from the catalog.
func (s *Service) DeleteBottle(ctx context.Context, bottleID int64) error {
	return s.q.DeleteBottle(ctx, bottleID)
}

func toCountry(row sqlc.WineCountry) Country {
	return Country{
		CountryID:   row.CountryID,
		Name:        row.Name,
		IsoCode:     row.IsoCode,
		Description: row.Description.String,
		IsActive:    row.IsActive,
	}
}

func toRegion(row sqlc.WineRegion) Region {
	return Region{
		RegionID:    row.RegionID,
		CountryID:   row.CountryID,
		Name:        row.Name,
		Description: row.Description.String,
		IsActive:    row.IsActive,
	}
}

func toType(row sqlc.WineType) Type {
	return Type{
		TypeID:      row.TypeID,
		Name:        row.Name,
		Description: row.Description.String,
		IsActive:    row.IsActive,
	}
}

func toVintage(row sqlc.WineVintage) Vintage {
	return Vintage{
		VintageID:   row.VintageID,
		Year:        row.Year,
		Description: row.Description.String,
		IsActive:    row.IsActive,
	}
}

func toGrapeVariety(row sqlc.WineGrapeVariety) GrapeVariety {
	return GrapeVariety{
		GrapeVarietyID: row.GrapeVarietyID,
		Name:           row.Name,
		Description:    row.Description.String,
		IsActive:       row.IsActive,
	}
}

func toBottle(row sqlc.WineBottle) Bottle {
	b := Bottle{
		BottleID:    row.BottleID,
		TypeID:      row.TypeID,
		CountryID:   row.CountryID,
		RegionID:    row.RegionID,
		VintageYear: row.VintageYear,
		Vineyard:    row.Vineyard.String,
		BottleSize:  row.BottleSize,
	}
	if row.Abv.Valid {
		f8, _ := row.Abv.Float64Value()
		b.Abv = f8.Float64
	}
	if row.Acidity.Valid {
		b.Acidity = row.Acidity.Int16
	}
	if row.TanninLevel.Valid {
		b.TanninLevel = row.TanninLevel.Int16
	}
	if row.Body.Valid {
		b.Body = row.Body.Int16
	}
	if row.Sweetness.Valid {
		b.Sweetness = row.Sweetness.Int16
	}
	if row.OakIntegration.Valid {
		b.OakIntegration = row.OakIntegration.Bool
	}
	return b
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func numericOrNull(f float64) pgtype.Numeric {
	if f == 0 {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	n.Valid = true
	return n
}

func int2OrNull(v int16) pgtype.Int2 {
	if v == 0 {
		return pgtype.Int2{}
	}
	return pgtype.Int2{Int16: v, Valid: true}
}

func boolOrNull(v bool) pgtype.Bool {
	return pgtype.Bool{Bool: v, Valid: true}
}
