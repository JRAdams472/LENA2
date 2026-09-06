package bff

import (
	"context"
	"strconv"

	"github.com/JRAdams472/LENA2/internal/wine"
	"github.com/graph-gophers/graphql-go"
)

// Bottle resolves a single wine bottle by ID.
func (r *Resolver) Bottle(ctx context.Context, args struct{ ID graphql.ID }) (*bottleResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	b, err := r.WineService.GetBottleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{wine: r.WineService, b: b}, nil
}

// Bottles resolves a paginated list of wine bottles.
func (r *Resolver) Bottles(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*bottlePageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	bottles, err := r.WineService.ListBottles(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.WineService.CountBottles(ctx)
	if err != nil {
		return nil, err
	}
	bc, err := loadBottleChildren(ctx, r.WineService, distinctIDs(bottles, func(b wine.Bottle) *int64 { return &b.BottleID }), false)
	if err != nil {
		return nil, err
	}
	return &bottlePageResolver{wine: r.WineService, bottles: bottles, bc: bc, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// Types resolves all wine types.
func (r *Resolver) Types(ctx context.Context) ([]*wineTypeResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	types, err := r.WineService.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*wineTypeResolver, len(types))
	for i := range types {
		out[i] = &wineTypeResolver{t: types[i]}
	}
	return out, nil
}

// Countries resolves all wine countries.
func (r *Resolver) Countries(ctx context.Context) ([]*countryResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	countries, err := r.WineService.ListCountries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*countryResolver, len(countries))
	for i := range countries {
		out[i] = &countryResolver{c: countries[i]}
	}
	return out, nil
}

// Regions resolves wine regions for a country.
func (r *Resolver) Regions(ctx context.Context, args struct{ CountryID graphql.ID }) ([]*regionResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	countryID, err := parseID(string(args.CountryID))
	if err != nil {
		return nil, err
	}
	regions, err := r.WineService.ListRegions(ctx, countryID)
	if err != nil {
		return nil, err
	}
	// All regions in this list share one country; fetch it once instead of
	// once per region. On lookup failure the resolver falls back to a lazy
	// per-region fetch.
	var country *wine.Country
	if len(regions) > 0 {
		if c, err := r.WineService.GetCountryByID(ctx, countryID); err == nil {
			country = &c
		}
	}
	out := make([]*regionResolver, len(regions))
	for i := range regions {
		out[i] = &regionResolver{wine: r.WineService, r: regions[i], country: country}
	}
	return out, nil
}

// Vintages resolves all wine vintages.
func (r *Resolver) Vintages(ctx context.Context) ([]*vintageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	vintages, err := r.WineService.ListVintages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*vintageResolver, len(vintages))
	for i := range vintages {
		out[i] = &vintageResolver{v: vintages[i]}
	}
	return out, nil
}

// GrapeVarieties resolves all grape varieties.
func (r *Resolver) GrapeVarieties(ctx context.Context) ([]*grapeVarietyResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	varieties, err := r.WineService.ListGrapeVarieties(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*grapeVarietyResolver, len(varieties))
	for i := range varieties {
		out[i] = &grapeVarietyResolver{g: varieties[i]}
	}
	return out, nil
}

// WineFlavorProfiles resolves all active wine flavor profiles.
func (r *Resolver) WineFlavorProfiles(ctx context.Context) ([]*wineFlavorProfileResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	flavors, err := r.WineService.ListWineFlavorProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*wineFlavorProfileResolver, len(flavors))
	for i := range flavors {
		out[i] = &wineFlavorProfileResolver{fp: flavors[i]}
	}
	return out, nil
}

// CreateWineFlavorProfile adds a new wine flavor profile.
func (r *Resolver) CreateWineFlavorProfile(ctx context.Context, args struct{ Input createWineFlavorProfileInput }) (*wineFlavorProfileResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	fp, err := r.WineService.CreateWineFlavorProfile(ctx, args.Input.Name, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &wineFlavorProfileResolver{fp: fp}, nil
}

// UpdateWineFlavorProfile modifies an existing wine flavor profile.
func (r *Resolver) UpdateWineFlavorProfile(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateWineFlavorProfileInput
}) (*wineFlavorProfileResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetWineFlavorProfileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	fp, err := r.WineService.UpdateWineFlavorProfile(ctx, id, name, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &wineFlavorProfileResolver{fp: fp}, nil
}

// DeleteWineFlavorProfile removes a wine flavor profile.
func (r *Resolver) DeleteWineFlavorProfile(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteWineFlavorProfile(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// CreateVintage adds a new vintage.
func (r *Resolver) CreateVintage(ctx context.Context, args struct{ Input createVintageInput }) (*vintageResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	v, err := r.WineService.CreateVintage(ctx, args.Input.Year, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &vintageResolver{v: v}, nil
}

// CreateGrapeVariety adds a new grape variety.
func (r *Resolver) CreateGrapeVariety(ctx context.Context, args struct{ Input createGrapeVarietyInput }) (*grapeVarietyResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	g, err := r.WineService.CreateGrapeVariety(ctx, args.Input.Name, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &grapeVarietyResolver{g: g}, nil
}

// CreateBottle adds a new bottle definition.
func (r *Resolver) CreateBottle(ctx context.Context, args struct{ Input createBottleInput }) (*bottleResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	typeID, err := parseID(string(args.Input.TypeID))
	if err != nil {
		return nil, err
	}
	countryID, err := parseID(string(args.Input.CountryID))
	if err != nil {
		return nil, err
	}
	regionID, err := parseID(string(args.Input.RegionID))
	if err != nil {
		return nil, err
	}
	var vineyard string
	if args.Input.Vineyard != nil {
		vineyard = *args.Input.Vineyard
	}
	b, err := r.WineService.CreateBottle(ctx, wine.Bottle{
		TypeID:         typeID,
		CountryID:      countryID,
		RegionID:       regionID,
		VintageYear:    args.Input.VintageYear,
		Vineyard:       vineyard,
		Abv:            args.Input.Abv,
		Acidity:        int16Ptr(args.Input.Acidity),
		TanninLevel:    int16Ptr(args.Input.TanninLevel),
		Body:           int16Ptr(args.Input.Body),
		Sweetness:      int16Ptr(args.Input.Sweetness),
		OakIntegration: boolValue(args.Input.OakIntegration),
		BottleSize:     args.Input.BottleSize,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{wine: r.WineService, b: b}, nil
}

// UpdateBottle modifies an existing bottle definition.
func (r *Resolver) UpdateBottle(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateBottleInput
}) (*bottleResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetBottleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	typeID := existing.TypeID
	if args.Input.TypeID != nil {
		tid, err := parseID(string(*args.Input.TypeID))
		if err != nil {
			return nil, err
		}
		typeID = tid
	}
	countryID := existing.CountryID
	if args.Input.CountryID != nil {
		cid, err := parseID(string(*args.Input.CountryID))
		if err != nil {
			return nil, err
		}
		countryID = cid
	}
	regionID := existing.RegionID
	if args.Input.RegionID != nil {
		rid, err := parseID(string(*args.Input.RegionID))
		if err != nil {
			return nil, err
		}
		regionID = rid
	}
	vintage := existing.VintageYear
	if args.Input.VintageYear != nil {
		vintage = *args.Input.VintageYear
	}
	vineyard := existing.Vineyard
	if args.Input.Vineyard != nil {
		vineyard = *args.Input.Vineyard
	}
	abv := existing.Abv
	if args.Input.Abv != nil {
		abv = args.Input.Abv
	}
	acidity := existing.Acidity
	if args.Input.Acidity != nil {
		acidity = int16Ptr(args.Input.Acidity)
	}
	tanninLevel := existing.TanninLevel
	if args.Input.TanninLevel != nil {
		tanninLevel = int16Ptr(args.Input.TanninLevel)
	}
	body := existing.Body
	if args.Input.Body != nil {
		body = int16Ptr(args.Input.Body)
	}
	sweetness := existing.Sweetness
	if args.Input.Sweetness != nil {
		sweetness = int16Ptr(args.Input.Sweetness)
	}
	oakIntegration := existing.OakIntegration
	if args.Input.OakIntegration != nil {
		oakIntegration = *args.Input.OakIntegration
	}
	bottleSize := existing.BottleSize
	if args.Input.BottleSize != nil {
		bottleSize = *args.Input.BottleSize
	}
	if err := r.WineService.UpdateBottle(ctx, id, wine.Bottle{
		TypeID:         typeID,
		CountryID:      countryID,
		RegionID:       regionID,
		VintageYear:    vintage,
		Vineyard:       vineyard,
		Abv:            abv,
		Acidity:        acidity,
		TanninLevel:    tanninLevel,
		Body:           body,
		Sweetness:      sweetness,
		OakIntegration: oakIntegration,
		BottleSize:     bottleSize,
	}, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.WineService.GetBottleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &bottleResolver{wine: r.WineService, b: updated}, nil
}

// AddBottleGrapeVariety links a grape variety to a bottle.
func (r *Resolver) AddBottleGrapeVariety(ctx context.Context, args struct{ Input addBottleGrapeVarietyInput }) (*bottleGrapeVarietyResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	bottleID, err := parseID(string(args.Input.BottleID))
	if err != nil {
		return nil, err
	}
	grapeVarietyID, err := parseID(string(args.Input.GrapeVarietyID))
	if err != nil {
		return nil, err
	}
	v, err := r.WineService.AddBottleGrapeVariety(ctx, bottleID, grapeVarietyID, int16Ptr(args.Input.Percentage), u.Email)
	if err != nil {
		return nil, err
	}
	return &bottleGrapeVarietyResolver{wine: r.WineService, variety: v}, nil
}

// RemoveBottleGrapeVariety unlinks a grape variety from a bottle.
func (r *Resolver) RemoveBottleGrapeVariety(ctx context.Context, args struct {
	BottleID       graphql.ID
	GrapeVarietyID graphql.ID
}) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	bottleID, err := parseID(string(args.BottleID))
	if err != nil {
		return false, err
	}
	grapeVarietyID, err := parseID(string(args.GrapeVarietyID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.RemoveBottleGrapeVariety(ctx, bottleID, grapeVarietyID); err != nil {
		return false, err
	}
	return true, nil
}

// AddBottleFlavorProfile links a flavor profile to a bottle.
func (r *Resolver) AddBottleFlavorProfile(ctx context.Context, args struct{ Input addBottleFlavorProfileInput }) (*bottleFlavorProfileResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	bottleID, err := parseID(string(args.Input.BottleID))
	if err != nil {
		return nil, err
	}
	flavorProfileID, err := parseID(string(args.Input.FlavorProfileID))
	if err != nil {
		return nil, err
	}
	f, err := r.WineService.AddBottleFlavorProfile(ctx, bottleID, flavorProfileID, int16(args.Input.Intensity), u.Email)
	if err != nil {
		return nil, err
	}
	return &bottleFlavorProfileResolver{wine: r.WineService, flavor: f}, nil
}

// RemoveBottleFlavorProfile unlinks a flavor profile from a bottle.
func (r *Resolver) RemoveBottleFlavorProfile(ctx context.Context, args struct {
	BottleID        graphql.ID
	FlavorProfileID graphql.ID
}) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	bottleID, err := parseID(string(args.BottleID))
	if err != nil {
		return false, err
	}
	flavorProfileID, err := parseID(string(args.FlavorProfileID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.RemoveBottleFlavorProfile(ctx, bottleID, flavorProfileID); err != nil {
		return false, err
	}
	return true, nil
}

// bottleResolver resolves Bottle fields. When bc is non-nil its
// batch-loaded maps are used instead of per-bottle service calls.
type bottleResolver struct {
	wine WineService
	b    wine.Bottle
	bc   *bottleChildren
}

func (r *bottleResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.b.BottleID, 10)) }

func (r *bottleResolver) TypeID() graphql.ID { return graphql.ID(strconv.FormatInt(r.b.TypeID, 10)) }

func (r *bottleResolver) CountryID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.b.CountryID, 10))
}

func (r *bottleResolver) RegionID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.b.RegionID, 10))
}

func (r *bottleResolver) Vineyard() *string { return nilIfEmpty(r.b.Vineyard) }

func (r *bottleResolver) VintageYear() int32 { return r.b.VintageYear }

func (r *bottleResolver) Abv() *float64 { return r.b.Abv }

func (r *bottleResolver) Acidity() *int32 { return int16ToInt32Ptr(r.b.Acidity) }

func (r *bottleResolver) TanninLevel() *int32 { return int16ToInt32Ptr(r.b.TanninLevel) }

func (r *bottleResolver) Body() *int32 { return int16ToInt32Ptr(r.b.Body) }

func (r *bottleResolver) Sweetness() *int32 { return int16ToInt32Ptr(r.b.Sweetness) }

func (r *bottleResolver) OakIntegration() *bool { return &r.b.OakIntegration }

func (r *bottleResolver) BottleSize() string { return r.b.BottleSize }

func (r *bottleResolver) GrapeVarieties(ctx context.Context) ([]*bottleGrapeVarietyResolver, error) {
	var varieties []wine.BottleGrapeVariety
	if r.bc != nil {
		varieties = r.bc.grapesBy[r.b.BottleID]
	} else {
		var err error
		varieties, err = r.wine.ListBottleGrapeVarieties(ctx, r.b.BottleID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*bottleGrapeVarietyResolver, len(varieties))
	for i := range varieties {
		out[i] = &bottleGrapeVarietyResolver{wine: r.wine, variety: varieties[i]}
	}
	return out, nil
}

type bottlePageResolver struct {
	wine     WineService
	bottles  []wine.Bottle
	bc       *bottleChildren
	page     int32
	pageSize int32
	total    int32
}

func (r *bottlePageResolver) Items() []*bottleResolver {
	out := make([]*bottleResolver, len(r.bottles))
	for i := range r.bottles {
		out[i] = &bottleResolver{wine: r.wine, b: r.bottles[i], bc: r.bc}
	}
	return out
}

func (r *bottlePageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// wineTypeResolver resolves a wine type.
type wineTypeResolver struct{ t wine.Type }

func (r *wineTypeResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.t.TypeID, 10)) }

func (r *wineTypeResolver) Name() string { return r.t.Name }

func (r *wineTypeResolver) Description() *string { return nilIfEmpty(r.t.Description) }

// countryResolver resolves a wine country.
type countryResolver struct{ c wine.Country }

func (r *countryResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.c.CountryID, 10)) }

func (r *countryResolver) Name() string { return r.c.Name }

func (r *countryResolver) IsoCode() *string { return nilIfEmpty(r.c.IsoCode) }

func (r *countryResolver) Description() *string { return nilIfEmpty(r.c.Description) }

// regionResolver resolves a wine region. When country is non-nil it is
// used instead of a per-region lookup.
type regionResolver struct {
	wine    WineService
	r       wine.Region
	country *wine.Country
}

func (r *regionResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.r.RegionID, 10)) }

func (r *regionResolver) Name() string { return r.r.Name }

func (r *regionResolver) Description() *string { return nilIfEmpty(r.r.Description) }

func (r *regionResolver) Country(ctx context.Context) (*countryResolver, error) {
	if r.country != nil {
		return &countryResolver{c: *r.country}, nil
	}
	c, err := r.wine.GetCountryByID(ctx, r.r.CountryID)
	if err != nil {
		return nil, err
	}
	return &countryResolver{c: c}, nil
}

// vintageResolver resolves a wine vintage.
type vintageResolver struct{ v wine.Vintage }

func (r *vintageResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.v.VintageID, 10)) }

func (r *vintageResolver) Year() int32 { return r.v.Year }

func (r *vintageResolver) Description() *string { return nilIfEmpty(r.v.Description) }

func (r *vintageResolver) IsActive() bool { return r.v.IsActive }

// grapeVarietyResolver resolves a grape variety.
type grapeVarietyResolver struct{ g wine.GrapeVariety }

func (r *grapeVarietyResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.g.GrapeVarietyID, 10))
}

func (r *grapeVarietyResolver) Name() string { return r.g.Name }

func (r *grapeVarietyResolver) Description() *string { return nilIfEmpty(r.g.Description) }

func (r *grapeVarietyResolver) IsActive() bool { return r.g.IsActive }

// bottleGrapeVarietyResolver resolves a grape variety on a bottle.
type bottleGrapeVarietyResolver struct {
	wine    WineService
	variety wine.BottleGrapeVariety
}

func (r *bottleGrapeVarietyResolver) GrapeVariety(ctx context.Context) (*grapeVarietyResolver, error) {
	return &grapeVarietyResolver{g: wine.GrapeVariety{
		GrapeVarietyID: r.variety.GrapeVarietyID,
		Name:           r.variety.Name,
	}}, nil
}

func (r *bottleGrapeVarietyResolver) Percentage() *int32 {
	return int16ToInt32Ptr(r.variety.Percentage)
}

func (r *bottleResolver) FlavorProfiles(ctx context.Context) ([]*bottleFlavorProfileResolver, error) {
	var flavors []wine.BottleFlavorProfile
	if r.bc != nil {
		flavors = r.bc.favorsBy[r.b.BottleID]
	} else {
		var err error
		flavors, err = r.wine.ListBottleFlavorProfiles(ctx, r.b.BottleID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*bottleFlavorProfileResolver, len(flavors))
	for i := range flavors {
		out[i] = &bottleFlavorProfileResolver{wine: r.wine, flavor: flavors[i]}
	}
	return out, nil
}

type wineFlavorProfileResolver struct {
	fp wine.WineFlavorProfile
}

func (r *wineFlavorProfileResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.fp.FlavorProfileID, 10))
}

func (r *wineFlavorProfileResolver) Name() string { return r.fp.Name }

func (r *wineFlavorProfileResolver) Description() *string { return nilIfEmpty(r.fp.Description) }

func (r *wineFlavorProfileResolver) IsActive() bool { return r.fp.IsActive }

// bottleFlavorProfileResolver resolves a flavor profile on a bottle.
type bottleFlavorProfileResolver struct {
	wine   WineService
	flavor wine.BottleFlavorProfile
}

func (r *bottleFlavorProfileResolver) FlavorProfile(ctx context.Context) (*wineFlavorProfileResolver, error) {
	return &wineFlavorProfileResolver{fp: wine.WineFlavorProfile{
		FlavorProfileID: r.flavor.FlavorProfileID,
		Name:            r.flavor.Name,
	}}, nil
}

func (r *bottleFlavorProfileResolver) Intensity() int32 { return int32(r.flavor.Intensity) }

type createVintageInput struct {
	Year        int32
	Description *string
}

type createGrapeVarietyInput struct {
	Name        string
	Description *string
}

type addBottleGrapeVarietyInput struct {
	BottleID       graphql.ID
	GrapeVarietyID graphql.ID
	Percentage     *int32
}

type addBottleFlavorProfileInput struct {
	BottleID        graphql.ID
	FlavorProfileID graphql.ID
	Intensity       int32
}

type createBottleInput struct {
	TypeID         graphql.ID
	CountryID      graphql.ID
	RegionID       graphql.ID
	VintageYear    int32
	Vineyard       *string
	Abv            *float64
	Acidity        *int32
	TanninLevel    *int32
	Body           *int32
	Sweetness      *int32
	OakIntegration *bool
	BottleSize     string
}

type updateBottleInput struct {
	TypeID         *graphql.ID
	CountryID      *graphql.ID
	RegionID       *graphql.ID
	VintageYear    *int32
	Vineyard       *string
	Abv            *float64
	Acidity        *int32
	TanninLevel    *int32
	Body           *int32
	Sweetness      *int32
	OakIntegration *bool
	BottleSize     *string
}

// CreateCountry adds a new wine country.
func (r *Resolver) CreateCountry(ctx context.Context, args struct{ Input createCountryInput }) (*countryResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	c, err := r.WineService.CreateCountry(ctx, args.Input.Name, args.Input.IsoCode, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &countryResolver{c: c}, nil
}

// UpdateCountry modifies an existing wine country.
func (r *Resolver) UpdateCountry(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateCountryInput
}) (*countryResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetCountryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	isoCode := existing.IsoCode
	if args.Input.IsoCode != nil {
		isoCode = *args.Input.IsoCode
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	c, err := r.WineService.UpdateCountry(ctx, id, name, isoCode, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &countryResolver{c: c}, nil
}

// DeleteCountry removes a wine country.
func (r *Resolver) DeleteCountry(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteCountry(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// CreateRegion adds a new wine region.
func (r *Resolver) CreateRegion(ctx context.Context, args struct{ Input createRegionInput }) (*regionResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	countryID, err := parseID(string(args.Input.CountryID))
	if err != nil {
		return nil, err
	}
	region, err := r.WineService.CreateRegion(ctx, wine.Region{
		CountryID: countryID,
		Name:      args.Input.Name,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &regionResolver{wine: r.WineService, r: region}, nil
}

// UpdateRegion modifies an existing wine region.
func (r *Resolver) UpdateRegion(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateRegionInput
}) (*regionResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetRegionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	countryID := existing.CountryID
	if args.Input.CountryID != nil {
		cid, err := parseID(string(*args.Input.CountryID))
		if err != nil {
			return nil, err
		}
		countryID = cid
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	region, err := r.WineService.UpdateRegion(ctx, id, countryID, name, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &regionResolver{wine: r.WineService, r: region}, nil
}

// DeleteRegion removes a wine region.
func (r *Resolver) DeleteRegion(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteRegion(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// CreateType adds a new wine type.
func (r *Resolver) CreateType(ctx context.Context, args struct{ Input createTypeInput }) (*wineTypeResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	t, err := r.WineService.CreateType(ctx, args.Input.Name, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &wineTypeResolver{t: t}, nil
}

// UpdateType modifies an existing wine type.
func (r *Resolver) UpdateType(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateTypeInput
}) (*wineTypeResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetTypeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	t, err := r.WineService.UpdateType(ctx, id, name, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &wineTypeResolver{t: t}, nil
}

// DeleteType removes a wine type.
func (r *Resolver) DeleteType(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteType(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateVintage modifies an existing vintage.
func (r *Resolver) UpdateVintage(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateVintageInput
}) (*vintageResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetVintageByID(ctx, id)
	if err != nil {
		return nil, err
	}
	year := existing.Year
	if args.Input.Year != nil {
		year = *args.Input.Year
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	v, err := r.WineService.UpdateVintage(ctx, id, year, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &vintageResolver{v: v}, nil
}

// DeleteVintage removes a vintage.
func (r *Resolver) DeleteVintage(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteVintage(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateGrapeVariety modifies an existing grape variety.
func (r *Resolver) UpdateGrapeVariety(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateGrapeVarietyInput
}) (*grapeVarietyResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.WineService.GetGrapeVarietyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	g, err := r.WineService.UpdateGrapeVariety(ctx, id, name, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &grapeVarietyResolver{g: g}, nil
}

// DeleteGrapeVariety removes a grape variety.
func (r *Resolver) DeleteGrapeVariety(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteGrapeVariety(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteBottle removes a bottle definition.
func (r *Resolver) DeleteBottle(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.WineService.DeleteBottle(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

type createCountryInput struct {
	Name        string
	IsoCode     string
	Description *string
}

type updateCountryInput struct {
	Name        *string
	IsoCode     *string
	Description *string
	IsActive    *bool
}

type createRegionInput struct {
	CountryID   graphql.ID
	Name        string
	Description *string
}

type updateRegionInput struct {
	CountryID   *graphql.ID
	Name        *string
	Description *string
	IsActive    *bool
}

type createTypeInput struct {
	Name        string
	Description *string
}

type updateTypeInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}

type updateVintageInput struct {
	Year        *int32
	Description *string
	IsActive    *bool
}

type updateGrapeVarietyInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}

type createWineFlavorProfileInput struct {
	Name        string
	Description *string
}

type updateWineFlavorProfileInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}
