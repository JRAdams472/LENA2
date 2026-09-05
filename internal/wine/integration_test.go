package wine

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/platform/testenv"
)

const itBy = "integration-test"

func newIntegrationService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return NewService(pool), pool
}

func TestIntegrationCountryCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	c, err := svc.CreateCountry(ctx, "IT Country Alpha", "IT1", "test country", itBy)
	require.NoError(t, err)
	require.NotZero(t, c.CountryID)
	assert.Equal(t, "IT Country Alpha", c.Name)
	assert.Equal(t, "IT1", c.IsoCode)
	assert.True(t, c.IsActive)

	got, err := svc.GetCountryByID(ctx, c.CountryID)
	require.NoError(t, err)
	assert.Equal(t, "IT Country Alpha", got.Name)

	cs, err := svc.ListCountries(ctx)
	require.NoError(t, err)
	var found bool
	for _, x := range cs {
		if x.CountryID == c.CountryID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateCountry(ctx, c.CountryID, "IT Country Beta", "IT1", "updated", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Country Beta", updated.Name)
	assert.False(t, updated.IsActive)

	// Unique constraint on iso_code.
	_, err = svc.CreateCountry(ctx, "IT Country DupIso", "IT1", "", itBy)
	assert.Error(t, err, "duplicate iso_code should violate unique constraint")

	require.NoError(t, svc.DeleteCountry(ctx, c.CountryID))
	_, err = svc.GetCountryByID(ctx, c.CountryID)
	assert.Error(t, err)
}

func TestIntegrationRegionCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	country, err := svc.CreateCountry(ctx, "IT Region Country", "IT2", "", itBy)
	require.NoError(t, err)

	r, err := svc.CreateRegion(ctx, Region{
		CountryID:   country.CountryID,
		Name:        "IT Region Alpha",
		Description: "test region",
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, r.RegionID)
	assert.Equal(t, country.CountryID, r.CountryID)
	assert.Equal(t, "IT Region Alpha", r.Name)

	got, err := svc.GetRegionByID(ctx, r.RegionID)
	require.NoError(t, err)
	assert.Equal(t, r.RegionID, got.RegionID)

	rs, err := svc.ListRegions(ctx, country.CountryID)
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, r.RegionID, rs[0].RegionID)

	updated, err := svc.UpdateRegion(ctx, r.RegionID, country.CountryID, "IT Region Beta", "updated", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Region Beta", updated.Name)
	assert.False(t, updated.IsActive)

	// FK violation: region referencing a non-existent country.
	_, err = svc.CreateRegion(ctx, Region{
		CountryID: 99999999,
		Name:      "IT Region BadCountry",
	}, itBy)
	assert.Error(t, err, "region with non-existent country_id should fail")

	require.NoError(t, svc.DeleteRegion(ctx, r.RegionID))
	_, err = svc.GetRegionByID(ctx, r.RegionID)
	assert.Error(t, err)
}

func TestIntegrationTypeCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	wt, err := svc.CreateType(ctx, "IT Type Alpha", "test type", itBy)
	require.NoError(t, err)
	require.NotZero(t, wt.TypeID)
	assert.True(t, wt.IsActive)

	got, err := svc.GetTypeByID(ctx, wt.TypeID)
	require.NoError(t, err)
	assert.Equal(t, "IT Type Alpha", got.Name)

	ts, err := svc.ListTypes(ctx)
	require.NoError(t, err)
	var found bool
	for _, x := range ts {
		if x.TypeID == wt.TypeID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateType(ctx, wt.TypeID, "IT Type Beta", "updated", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Type Beta", updated.Name)
	assert.False(t, updated.IsActive)

	require.NoError(t, svc.DeleteType(ctx, wt.TypeID))
	_, err = svc.GetTypeByID(ctx, wt.TypeID)
	assert.Error(t, err)
}

func TestIntegrationVintageCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	v, err := svc.CreateVintage(ctx, 1998, "test vintage", itBy)
	require.NoError(t, err)
	require.NotZero(t, v.VintageID)
	assert.Equal(t, int32(1998), v.Year)

	got, err := svc.GetVintageByID(ctx, v.VintageID)
	require.NoError(t, err)
	assert.Equal(t, int32(1998), got.Year)

	vs, err := svc.ListVintages(ctx)
	require.NoError(t, err)
	var found bool
	for _, x := range vs {
		if x.VintageID == v.VintageID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateVintage(ctx, v.VintageID, 1999, "updated", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, int32(1999), updated.Year)
	assert.False(t, updated.IsActive)

	// Unique constraint on year.
	_, err = svc.CreateVintage(ctx, 1999, "dup", itBy)
	assert.Error(t, err, "duplicate vintage year should violate unique constraint")

	require.NoError(t, svc.DeleteVintage(ctx, v.VintageID))
	_, err = svc.GetVintageByID(ctx, v.VintageID)
	assert.Error(t, err)
}

func TestIntegrationGrapeVarietyCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	g, err := svc.CreateGrapeVariety(ctx, "IT Grape Alpha", "test grape", itBy)
	require.NoError(t, err)
	require.NotZero(t, g.GrapeVarietyID)

	got, err := svc.GetGrapeVarietyByID(ctx, g.GrapeVarietyID)
	require.NoError(t, err)
	assert.Equal(t, "IT Grape Alpha", got.Name)

	gs, err := svc.ListGrapeVarieties(ctx)
	require.NoError(t, err)
	var found bool
	for _, x := range gs {
		if x.GrapeVarietyID == g.GrapeVarietyID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateGrapeVariety(ctx, g.GrapeVarietyID, "IT Grape Beta", "updated", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Grape Beta", updated.Name)
	assert.False(t, updated.IsActive)

	require.NoError(t, svc.DeleteGrapeVariety(ctx, g.GrapeVarietyID))
	_, err = svc.GetGrapeVarietyByID(ctx, g.GrapeVarietyID)
	assert.Error(t, err)
}

func TestIntegrationWineFlavorProfileCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	fp, err := svc.CreateWineFlavorProfile(ctx, "IT Wine Flavor Alpha", "test flavor", itBy)
	require.NoError(t, err)
	require.NotZero(t, fp.FlavorProfileID)

	got, err := svc.GetWineFlavorProfileByID(ctx, fp.FlavorProfileID)
	require.NoError(t, err)
	assert.Equal(t, "IT Wine Flavor Alpha", got.Name)

	fps, err := svc.ListWineFlavorProfiles(ctx)
	require.NoError(t, err)
	var found bool
	for _, x := range fps {
		if x.FlavorProfileID == fp.FlavorProfileID {
			found = true
		}
	}
	assert.True(t, found)

	updated, err := svc.UpdateWineFlavorProfile(ctx, fp.FlavorProfileID, "IT Wine Flavor Beta", "updated", false, itBy)
	require.NoError(t, err)
	assert.Equal(t, "IT Wine Flavor Beta", updated.Name)
	assert.False(t, updated.IsActive)

	require.NoError(t, svc.DeleteWineFlavorProfile(ctx, fp.FlavorProfileID))
	_, err = svc.GetWineFlavorProfileByID(ctx, fp.FlavorProfileID)
	assert.Error(t, err)
}

func TestIntegrationBottleCRUDAndJunctions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	svc, _ := newIntegrationService(t, ctx)

	country, err := svc.CreateCountry(ctx, "IT Bottle Country", "IT3", "", itBy)
	require.NoError(t, err)
	region, err := svc.CreateRegion(ctx, Region{CountryID: country.CountryID, Name: "IT Bottle Region"}, itBy)
	require.NoError(t, err)
	wtype, err := svc.CreateType(ctx, "IT Bottle Type", "", itBy)
	require.NoError(t, err)
	grape, err := svc.CreateGrapeVariety(ctx, "IT Bottle Grape", "", itBy)
	require.NoError(t, err)
	flavor, err := svc.CreateWineFlavorProfile(ctx, "IT Bottle Flavor", "", itBy)
	require.NoError(t, err)

	bottle, err := svc.CreateBottle(ctx, Bottle{
		TypeID:         wtype.TypeID,
		CountryID:      country.CountryID,
		RegionID:       region.RegionID,
		VintageYear:    2015,
		Vineyard:       "IT Vineyard",
		Abv:            13.5,
		Acidity:        3,
		TanninLevel:    4,
		Body:           4,
		Sweetness:      1,
		OakIntegration: true,
		BottleSize:     "750ml",
	}, itBy)
	require.NoError(t, err)
	require.NotZero(t, bottle.BottleID)
	assert.Equal(t, int32(2015), bottle.VintageYear)
	assert.Equal(t, "IT Vineyard", bottle.Vineyard)
	assert.InDelta(t, 13.5, bottle.Abv, 0.001)

	got, err := svc.GetBottleByID(ctx, bottle.BottleID)
	require.NoError(t, err)
	assert.Equal(t, bottle.BottleID, got.BottleID)

	bottles, err := svc.ListBottles(ctx, 100, 0)
	require.NoError(t, err)
	var found bool
	for _, b := range bottles {
		if b.BottleID == bottle.BottleID {
			found = true
		}
	}
	assert.True(t, found)

	require.NoError(t, svc.UpdateBottle(ctx, bottle.BottleID, Bottle{
		TypeID:      wtype.TypeID,
		CountryID:   country.CountryID,
		RegionID:    region.RegionID,
		VintageYear: 2016,
		Vineyard:    "IT Vineyard Updated",
		Abv:         14.0,
		BottleSize:  "1.5L",
	}, itBy))
	got, err = svc.GetBottleByID(ctx, bottle.BottleID)
	require.NoError(t, err)
	assert.Equal(t, int32(2016), got.VintageYear)
	assert.Equal(t, "IT Vineyard Updated", got.Vineyard)
	assert.Equal(t, "1.5L", got.BottleSize)

	// Junction: bottle grape variety.
	bgv, err := svc.AddBottleGrapeVariety(ctx, bottle.BottleID, grape.GrapeVarietyID, 85, itBy)
	require.NoError(t, err)
	assert.Equal(t, grape.GrapeVarietyID, bgv.GrapeVarietyID)
	assert.Equal(t, int16(85), bgv.Percentage)

	gvs, err := svc.ListBottleGrapeVarieties(ctx, bottle.BottleID)
	require.NoError(t, err)
	require.Len(t, gvs, 1)
	assert.Equal(t, "IT Bottle Grape", gvs[0].Name)

	require.NoError(t, svc.RemoveBottleGrapeVariety(ctx, bottle.BottleID, grape.GrapeVarietyID))
	gvs, err = svc.ListBottleGrapeVarieties(ctx, bottle.BottleID)
	require.NoError(t, err)
	assert.Empty(t, gvs)

	// Junction: bottle flavor profile.
	bfp, err := svc.AddBottleFlavorProfile(ctx, bottle.BottleID, flavor.FlavorProfileID, 3, itBy)
	require.NoError(t, err)
	assert.Equal(t, flavor.FlavorProfileID, bfp.FlavorProfileID)

	fps, err := svc.ListBottleFlavorProfiles(ctx, bottle.BottleID)
	require.NoError(t, err)
	require.Len(t, fps, 1)
	assert.Equal(t, "IT Bottle Flavor", fps[0].Name)
	assert.Equal(t, int16(3), fps[0].Intensity)

	require.NoError(t, svc.RemoveBottleFlavorProfile(ctx, bottle.BottleID, flavor.FlavorProfileID))
	fps, err = svc.ListBottleFlavorProfiles(ctx, bottle.BottleID)
	require.NoError(t, err)
	assert.Empty(t, fps)

	// FK violation: bottle referencing a non-existent region.
	_, err = svc.CreateBottle(ctx, Bottle{
		TypeID:      wtype.TypeID,
		CountryID:   country.CountryID,
		RegionID:    99999999,
		VintageYear: 2015,
		BottleSize:  "750ml",
	}, itBy)
	assert.Error(t, err, "bottle with non-existent region_id should fail")

	require.NoError(t, svc.DeleteBottle(ctx, bottle.BottleID))
	_, err = svc.GetBottleByID(ctx, bottle.BottleID)
	assert.Error(t, err)
}
