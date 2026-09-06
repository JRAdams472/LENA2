// Package wine owns the catalog of wine bottles and their reference data:
// countries, regions, types, vintages, grape varieties and flavor profiles.
package wine

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
	"github.com/JRAdams472/LENA2/internal/wine/sqlc"
)

// Service provides catalog operations for the wine domain.
type Service struct {
	q    sqlc.Querier
	pool dbtx.Pool
}

// NewService creates a wine Service using the given connection pool.
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

// UpdateCountry modifies an existing country.
func (s *Service) UpdateCountry(ctx context.Context, countryID int64, name, isoCode, description string, isActive bool, by string) (Country, error) {
	row, err := s.q.UpdateCountry(ctx, sqlc.UpdateCountryParams{
		CountryID:   countryID,
		Name:        name,
		IsoCode:     isoCode,
		Description: textOrNull(description),
		IsActive:    isActive,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Country{}, fmt.Errorf("update country: %w", err)
	}
	return toCountry(row), nil
}

// DeleteCountry removes a country from the catalog.
func (s *Service) DeleteCountry(ctx context.Context, countryID int64) error {
	return s.q.DeleteCountry(ctx, countryID)
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

// UpdateRegion modifies an existing region.
func (s *Service) UpdateRegion(ctx context.Context, regionID, countryID int64, name, description string, isActive bool, by string) (Region, error) {
	row, err := s.q.UpdateRegion(ctx, sqlc.UpdateRegionParams{
		RegionID:    regionID,
		CountryID:   countryID,
		Name:        name,
		Description: textOrNull(description),
		IsActive:    isActive,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Region{}, fmt.Errorf("update region: %w", err)
	}
	return toRegion(row), nil
}

// DeleteRegion removes a region from the catalog.
func (s *Service) DeleteRegion(ctx context.Context, regionID int64) error {
	return s.q.DeleteRegion(ctx, regionID)
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

// UpdateType modifies an existing wine type.
func (s *Service) UpdateType(ctx context.Context, typeID int64, name, description string, isActive bool, by string) (Type, error) {
	row, err := s.q.UpdateType(ctx, sqlc.UpdateTypeParams{
		TypeID:      typeID,
		Name:        name,
		Description: textOrNull(description),
		IsActive:    isActive,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Type{}, fmt.Errorf("update type: %w", err)
	}
	return toType(row), nil
}

// DeleteType removes a wine type from the catalog.
func (s *Service) DeleteType(ctx context.Context, typeID int64) error {
	return s.q.DeleteType(ctx, typeID)
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

// GetVintageByID returns a vintage by its primary key.
func (s *Service) GetVintageByID(ctx context.Context, vintageID int64) (Vintage, error) {
	row, err := s.q.GetVintageByID(ctx, vintageID)
	if err != nil {
		return Vintage{}, fmt.Errorf("get vintage by id: %w", err)
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

// UpdateVintage modifies an existing vintage.
func (s *Service) UpdateVintage(ctx context.Context, vintageID int64, year int32, description string, isActive bool, by string) (Vintage, error) {
	row, err := s.q.UpdateVintage(ctx, sqlc.UpdateVintageParams{
		VintageID:   vintageID,
		Year:        year,
		Description: textOrNull(description),
		IsActive:    isActive,
		UpdatedBy:   textOrNull(by),
	})
	if err != nil {
		return Vintage{}, fmt.Errorf("update vintage: %w", err)
	}
	return toVintage(row), nil
}

// DeleteVintage removes a vintage from the catalog.
func (s *Service) DeleteVintage(ctx context.Context, vintageID int64) error {
	return s.q.DeleteVintage(ctx, vintageID)
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

// GetGrapeVarietyByID returns a grape variety by its primary key.
func (s *Service) GetGrapeVarietyByID(ctx context.Context, grapeVarietyID int64) (GrapeVariety, error) {
	row, err := s.q.GetGrapeVarietyByID(ctx, grapeVarietyID)
	if err != nil {
		return GrapeVariety{}, fmt.Errorf("get grape variety by id: %w", err)
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

// UpdateGrapeVariety modifies an existing grape variety.
func (s *Service) UpdateGrapeVariety(ctx context.Context, grapeVarietyID int64, name, description string, isActive bool, by string) (GrapeVariety, error) {
	row, err := s.q.UpdateGrapeVariety(ctx, sqlc.UpdateGrapeVarietyParams{
		GrapeVarietyID: grapeVarietyID,
		Name:           name,
		Description:    textOrNull(description),
		IsActive:       isActive,
		UpdatedBy:      textOrNull(by),
	})
	if err != nil {
		return GrapeVariety{}, fmt.Errorf("update grape variety: %w", err)
	}
	return toGrapeVariety(row), nil
}

// DeleteGrapeVariety removes a grape variety from the catalog.
func (s *Service) DeleteGrapeVariety(ctx context.Context, grapeVarietyID int64) error {
	return s.q.DeleteGrapeVariety(ctx, grapeVarietyID)
}

// BottleGrapeVariety is a grape variety associated with a bottle.
type BottleGrapeVariety struct {
	BottleID       int64
	GrapeVarietyID int64
	Name           string
	Percentage     *int16
}

// AddBottleGrapeVariety links a grape variety to a bottle.
func (s *Service) AddBottleGrapeVariety(ctx context.Context, bottleID, grapeVarietyID int64, percentage *int16, by string) (BottleGrapeVariety, error) {
	row, err := s.q.CreateBottleGrapeVariety(ctx, sqlc.CreateBottleGrapeVarietyParams{
		BottleID:       bottleID,
		GrapeVarietyID: grapeVarietyID,
		Percentage:     optInt2(percentage),
		CreatedBy:      by,
	})
	if err != nil {
		return BottleGrapeVariety{}, fmt.Errorf("add bottle grape variety: %w", err)
	}
	return BottleGrapeVariety{
		GrapeVarietyID: row.GrapeVarietyID,
		Percentage:     int16Ptr(row.Percentage),
	}, nil
}

// ListBottleGrapeVarieties returns grape varieties for a bottle.
func (s *Service) ListBottleGrapeVarieties(ctx context.Context, bottleID int64) ([]BottleGrapeVariety, error) {
	rows, err := s.q.ListBottleGrapeVarieties(ctx, bottleID)
	if err != nil {
		return nil, fmt.Errorf("list bottle grape varieties: %w", err)
	}
	out := make([]BottleGrapeVariety, len(rows))
	for i := range rows {
		out[i] = toBottleGrapeVariety(rows[i])
		out[i].BottleID = bottleID
	}
	return out, nil
}

// ListBottleGrapeVarietiesByBottles returns grape varieties for a set of
// bottles in a single query; each result carries the bottle it belongs to.
func (s *Service) ListBottleGrapeVarietiesByBottles(ctx context.Context, bottleIDs []int64) ([]BottleGrapeVariety, error) {
	rows, err := s.q.ListBottleGrapeVarietiesByBottles(ctx, bottleIDs)
	if err != nil {
		return nil, fmt.Errorf("list bottle grape varieties by bottles: %w", err)
	}
	out := make([]BottleGrapeVariety, len(rows))
	for i := range rows {
		out[i] = BottleGrapeVariety{
			BottleID:       rows[i].BottleID,
			GrapeVarietyID: rows[i].GrapeVarietyID,
			Name:           rows[i].Name,
			Percentage:     int16Ptr(rows[i].Percentage),
		}
	}
	return out, nil
}

// RemoveBottleGrapeVariety removes a grape variety from a bottle.
func (s *Service) RemoveBottleGrapeVariety(ctx context.Context, bottleID, grapeVarietyID int64) error {
	return s.q.DeleteBottleGrapeVariety(ctx, sqlc.DeleteBottleGrapeVarietyParams{
		BottleID:       bottleID,
		GrapeVarietyID: grapeVarietyID,
	})
}

// WineFlavorProfile is a catalog of wine flavor profiles.
type WineFlavorProfile struct {
	FlavorProfileID int64
	Name            string
	Description     string
	IsActive        bool
}

// ListWineFlavorProfiles returns all wine flavor profiles.
func (s *Service) ListWineFlavorProfiles(ctx context.Context) ([]WineFlavorProfile, error) {
	rows, err := s.q.ListWineFlavorProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list wine flavor profiles: %w", err)
	}
	out := make([]WineFlavorProfile, len(rows))
	for i := range rows {
		out[i] = toWineFlavorProfile(rows[i])
	}
	return out, nil
}

// GetWineFlavorProfileByID returns a wine flavor profile by its primary key.
func (s *Service) GetWineFlavorProfileByID(ctx context.Context, flavorProfileID int64) (WineFlavorProfile, error) {
	row, err := s.q.GetWineFlavorProfileByID(ctx, flavorProfileID)
	if err != nil {
		return WineFlavorProfile{}, fmt.Errorf("get wine flavor profile by id: %w", err)
	}
	return toWineFlavorProfile(row), nil
}

// CreateWineFlavorProfile adds a new wine flavor profile.
func (s *Service) CreateWineFlavorProfile(ctx context.Context, name, description, by string) (WineFlavorProfile, error) {
	row, err := s.q.CreateWineFlavorProfile(ctx, sqlc.CreateWineFlavorProfileParams{
		Name:        name,
		Description: textOrNull(description),
		CreatedBy:   by,
	})
	if err != nil {
		return WineFlavorProfile{}, fmt.Errorf("create wine flavor profile: %w", err)
	}
	return toWineFlavorProfile(row), nil
}

// UpdateWineFlavorProfile modifies an existing wine flavor profile.
func (s *Service) UpdateWineFlavorProfile(ctx context.Context, flavorProfileID int64, name, description string, isActive bool, by string) (WineFlavorProfile, error) {
	row, err := s.q.UpdateWineFlavorProfile(ctx, sqlc.UpdateWineFlavorProfileParams{
		FlavorProfileID: flavorProfileID,
		Name:            name,
		Description:     textOrNull(description),
		IsActive:        isActive,
		UpdatedBy:       textOrNull(by),
	})
	if err != nil {
		return WineFlavorProfile{}, fmt.Errorf("update wine flavor profile: %w", err)
	}
	return toWineFlavorProfile(row), nil
}

// DeleteWineFlavorProfile removes a wine flavor profile from the catalog.
func (s *Service) DeleteWineFlavorProfile(ctx context.Context, flavorProfileID int64) error {
	return s.q.DeleteWineFlavorProfile(ctx, flavorProfileID)
}

// BottleFlavorProfile is a flavor profile attached to a bottle.
type BottleFlavorProfile struct {
	BottleID        int64
	FlavorProfileID int64
	Name            string
	Intensity       int16
}

// AddBottleFlavorProfile adds a flavor profile to a bottle.
func (s *Service) AddBottleFlavorProfile(ctx context.Context, bottleID, flavorProfileID int64, intensity int16, by string) (BottleFlavorProfile, error) {
	row, err := s.q.CreateBottleFlavorProfile(ctx, sqlc.CreateBottleFlavorProfileParams{
		BottleID:        bottleID,
		FlavorProfileID: flavorProfileID,
		Intensity:       intensity,
		CreatedBy:       by,
	})
	if err != nil {
		return BottleFlavorProfile{}, fmt.Errorf("add bottle flavor profile: %w", err)
	}
	return toBottleFlavorProfileFromRow(row), nil
}

// ListBottleFlavorProfiles returns flavor profiles for a bottle.
func (s *Service) ListBottleFlavorProfiles(ctx context.Context, bottleID int64) ([]BottleFlavorProfile, error) {
	rows, err := s.q.ListBottleFlavorProfiles(ctx, bottleID)
	if err != nil {
		return nil, fmt.Errorf("list bottle flavor profiles: %w", err)
	}
	out := make([]BottleFlavorProfile, len(rows))
	for i := range rows {
		out[i] = toBottleFlavorProfile(rows[i])
		out[i].BottleID = bottleID
	}
	return out, nil
}

// ListBottleFlavorProfilesByBottles returns flavor profiles for a set of
// bottles in a single query; each result carries the bottle it belongs to.
func (s *Service) ListBottleFlavorProfilesByBottles(ctx context.Context, bottleIDs []int64) ([]BottleFlavorProfile, error) {
	rows, err := s.q.ListBottleFlavorProfilesByBottles(ctx, bottleIDs)
	if err != nil {
		return nil, fmt.Errorf("list bottle flavor profiles by bottles: %w", err)
	}
	out := make([]BottleFlavorProfile, len(rows))
	for i := range rows {
		out[i] = BottleFlavorProfile{
			BottleID:        rows[i].BottleID,
			FlavorProfileID: rows[i].FlavorProfileID,
			Name:            rows[i].Name,
			Intensity:       rows[i].Intensity,
		}
	}
	return out, nil
}

// RemoveBottleFlavorProfile removes a flavor profile from a bottle.
func (s *Service) RemoveBottleFlavorProfile(ctx context.Context, bottleID, flavorProfileID int64) error {
	return s.q.DeleteBottleFlavorProfile(ctx, sqlc.DeleteBottleFlavorProfileParams{
		BottleID:        bottleID,
		FlavorProfileID: flavorProfileID,
	})
}

// Bottle is a catalog wine bottle definition. Nullable tasting-note fields
// are pointers so a real 0 is distinguishable from "not set".
type Bottle struct {
	BottleID       int64
	TypeID         int64
	CountryID      int64
	RegionID       int64
	VintageYear    int32
	Vineyard       string
	Abv            *float64
	Acidity        *int16
	TanninLevel    *int16
	Body           *int16
	Sweetness      *int16
	OakIntegration bool
	BottleSize     string
}

// CreateBottle adds a new bottle definition.
func (s *Service) CreateBottle(ctx context.Context, arg Bottle, by string) (Bottle, error) {
	abv, err := optNumeric(arg.Abv)
	if err != nil {
		return Bottle{}, fmt.Errorf("create bottle: %w", err)
	}
	row, err := s.q.CreateBottle(ctx, sqlc.CreateBottleParams{
		TypeID:         arg.TypeID,
		CountryID:      arg.CountryID,
		RegionID:       arg.RegionID,
		VintageYear:    arg.VintageYear,
		Vineyard:       textOrNull(arg.Vineyard),
		Abv:            abv,
		Acidity:        optInt2(arg.Acidity),
		TanninLevel:    optInt2(arg.TanninLevel),
		Body:           optInt2(arg.Body),
		Sweetness:      optInt2(arg.Sweetness),
		OakIntegration: boolOrNull(arg.OakIntegration),
		BottleSize:     arg.BottleSize,
		CreatedBy:      by,
		UpdatedBy:      textOrNull(by),
	})
	if err != nil {
		return Bottle{}, fmt.Errorf("create bottle: %w", err)
	}
	b, err := toBottle(row)
	if err != nil {
		return Bottle{}, fmt.Errorf("create bottle: %w", err)
	}
	return b, nil
}

// GetBottleByID returns a bottle by its primary key.
func (s *Service) GetBottleByID(ctx context.Context, bottleID int64) (Bottle, error) {
	row, err := s.q.GetBottleByID(ctx, bottleID)
	if err != nil {
		return Bottle{}, fmt.Errorf("get bottle by id: %w", err)
	}
	b, err := toBottle(row)
	if err != nil {
		return Bottle{}, fmt.Errorf("get bottle by id: %w", err)
	}
	return b, nil
}

// GetBottlesByIDs returns a set of bottles in a single query.
func (s *Service) GetBottlesByIDs(ctx context.Context, bottleIDs []int64) ([]Bottle, error) {
	rows, err := s.q.GetBottlesByIDs(ctx, bottleIDs)
	if err != nil {
		return nil, fmt.Errorf("get bottles by ids: %w", err)
	}
	out := make([]Bottle, len(rows))
	for i := range rows {
		b, err := toBottle(rows[i])
		if err != nil {
			return nil, fmt.Errorf("get bottles by ids: %w", err)
		}
		out[i] = b
	}
	return out, nil
}

// ListBottles returns a paginated list of bottles.
func (s *Service) ListBottles(ctx context.Context, limit, offset int32) ([]Bottle, error) {
	rows, err := s.q.ListBottles(ctx, sqlc.ListBottlesParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list bottles: %w", err)
	}
	out := make([]Bottle, len(rows))
	for i := range rows {
		b, err := toBottle(rows[i])
		if err != nil {
			return nil, fmt.Errorf("list bottles: %w", err)
		}
		out[i] = b
	}
	return out, nil
}

// CountBottles returns the total number of catalog bottles.
func (s *Service) CountBottles(ctx context.Context) (int64, error) {
	n, err := s.q.CountBottles(ctx)
	if err != nil {
		return 0, fmt.Errorf("count bottles: %w", err)
	}
	return n, nil
}

// UpdateBottle modifies an existing bottle.
func (s *Service) UpdateBottle(ctx context.Context, bottleID int64, arg Bottle, by string) error {
	abv, err := optNumeric(arg.Abv)
	if err != nil {
		return fmt.Errorf("update bottle: %w", err)
	}
	return s.q.UpdateBottle(ctx, sqlc.UpdateBottleParams{
		BottleID:       bottleID,
		TypeID:         arg.TypeID,
		CountryID:      arg.CountryID,
		RegionID:       arg.RegionID,
		VintageYear:    arg.VintageYear,
		Vineyard:       textOrNull(arg.Vineyard),
		Abv:            abv,
		Acidity:        optInt2(arg.Acidity),
		TanninLevel:    optInt2(arg.TanninLevel),
		Body:           optInt2(arg.Body),
		Sweetness:      optInt2(arg.Sweetness),
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

func toBottleGrapeVariety(row sqlc.ListBottleGrapeVarietiesRow) BottleGrapeVariety {
	return BottleGrapeVariety{
		GrapeVarietyID: row.GrapeVarietyID,
		Name:           row.Name,
		Percentage:     int16Ptr(row.Percentage),
	}
}

func toWineFlavorProfile(row sqlc.WineFlavorProfile) WineFlavorProfile {
	return WineFlavorProfile{
		FlavorProfileID: row.FlavorProfileID,
		Name:            row.Name,
		Description:     row.Description.String,
		IsActive:        row.IsActive,
	}
}

func toBottleFlavorProfile(row sqlc.ListBottleFlavorProfilesRow) BottleFlavorProfile {
	return BottleFlavorProfile{
		FlavorProfileID: row.FlavorProfileID,
		Name:            row.Name,
		Intensity:       row.Intensity,
	}
}

func toBottleFlavorProfileFromRow(row sqlc.WineBottleFlavorProfile) BottleFlavorProfile {
	return BottleFlavorProfile{
		FlavorProfileID: row.FlavorProfileID,
		Intensity:       row.Intensity,
	}
}

func int16Ptr(v pgtype.Int2) *int16 {
	if !v.Valid {
		return nil
	}
	return &v.Int16
}

func toBottle(row sqlc.WineBottle) (Bottle, error) {
	b := Bottle{
		BottleID:    row.BottleID,
		TypeID:      row.TypeID,
		CountryID:   row.CountryID,
		RegionID:    row.RegionID,
		VintageYear: row.VintageYear,
		Vineyard:    row.Vineyard.String,
		BottleSize:  row.BottleSize,
		Acidity:     int16Ptr(row.Acidity),
		TanninLevel: int16Ptr(row.TanninLevel),
		Body:        int16Ptr(row.Body),
		Sweetness:   int16Ptr(row.Sweetness),
	}
	if row.Abv.Valid {
		f8, err := row.Abv.Float64Value()
		if err != nil {
			return Bottle{}, fmt.Errorf("bottle %d abv: %w", row.BottleID, err)
		}
		v := f8.Float64
		b.Abv = &v
	}
	if row.OakIntegration.Valid {
		b.OakIntegration = row.OakIntegration.Bool
	}
	return b, nil
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

func optNumeric(f *float64) (pgtype.Numeric, error) {
	if f == nil {
		return pgtype.Numeric{}, nil
	}
	return numericFromFloat64(*f)
}

func optInt2(v *int16) pgtype.Int2 {
	if v == nil {
		return pgtype.Int2{}
	}
	return pgtype.Int2{Int16: *v, Valid: true}
}

func boolOrNull(v bool) pgtype.Bool {
	return pgtype.Bool{Bool: v, Valid: true}
}
