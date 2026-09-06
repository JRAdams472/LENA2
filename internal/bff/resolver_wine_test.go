package bff

import (
	"context"
	"errors"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/wine"
)

var errWineBoom = errors.New("wine boom")

const (
	wineUserID = int64(11)
	wineEmail  = "wine@example.com"
)

func wineCtx() context.Context {
	return testenv.WithAdmin(context.Background(), wineUserID, wineEmail)
}

func wineUserCtx() context.Context {
	return testenv.WithUser(context.Background(), wineUserID, wineEmail)
}

func newWineTestResolver(m *mock.MockWineService) *Resolver {
	return &Resolver{WineService: m}
}

func winePtrInt32(v int32) *int32       { return &v }
func winePtrInt16(v int16) *int16       { return &v }
func winePtrFloat64(v float64) *float64 { return &v }
func winePtrString(v string) *string    { return &v }
func winePtrBool(v bool) *bool          { return &v }

func TestResolver_Wine_Types_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListTypes(gomock.Any()).Return([]wine.Type{
		{TypeID: 1, Name: "Red", Description: "", IsActive: true},
		{TypeID: 2, Name: "White", Description: "Crisp", IsActive: true},
	}, nil)

	res, err := r.Types(wineCtx())
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, graphql.ID("1"), res[0].ID())
	assert.Equal(t, "Red", res[0].Name())
	assert.Nil(t, res[0].Description())
	assert.Equal(t, graphql.ID("2"), res[1].ID())
	assert.Equal(t, "White", res[1].Name())
	assert.Equal(t, "Crisp", *res[1].Description())
}

func TestResolver_Wine_Countries_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListCountries(gomock.Any()).Return([]wine.Country{
		{CountryID: 1, Name: "France", IsoCode: "FR", Description: "", IsActive: true},
		{CountryID: 2, Name: "Italy", IsoCode: "", Description: "Wine country", IsActive: true},
	}, nil)

	res, err := r.Countries(wineCtx())
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, graphql.ID("1"), res[0].ID())
	assert.Equal(t, "France", res[0].Name())
	assert.Equal(t, "FR", *res[0].IsoCode())
	assert.Nil(t, res[1].IsoCode())
	assert.Equal(t, "Wine country", *res[1].Description())
}

func TestResolver_Wine_Regions_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListRegions(gomock.Any(), int64(7)).Return([]wine.Region{
		{RegionID: 5, CountryID: 7, Name: "Bordeaux", Description: "Left bank", IsActive: true},
	}, nil)
	m.EXPECT().GetCountryByID(gomock.Any(), int64(7)).Return(wine.Country{
		CountryID: 7, Name: "France", IsoCode: "FR", Description: "", IsActive: true,
	}, nil)

	res, err := r.Regions(wineCtx(), struct{ CountryID graphql.ID }{CountryID: "7"})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, graphql.ID("5"), res[0].ID())
	assert.Equal(t, "Bordeaux", res[0].Name())
	assert.Equal(t, "Left bank", *res[0].Description())

	country, err := res[0].Country(wineCtx())
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("7"), country.ID())
	assert.Equal(t, "France", country.Name())
	assert.Equal(t, "FR", *country.IsoCode())
}

func TestResolver_Wine_Vintages_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListVintages(gomock.Any()).Return([]wine.Vintage{
		{VintageID: 10, Year: 2020, Description: "", IsActive: true},
		{VintageID: 11, Year: 2021, Description: "Great", IsActive: false},
	}, nil)

	res, err := r.Vintages(wineCtx())
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, graphql.ID("10"), res[0].ID())
	assert.Equal(t, int32(2020), res[0].Year())
	assert.Nil(t, res[0].Description())
	assert.True(t, res[0].IsActive())
	assert.Equal(t, int32(2021), res[1].Year())
	assert.Equal(t, "Great", *res[1].Description())
	assert.False(t, res[1].IsActive())
}

func TestResolver_Wine_GrapeVarieties_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListGrapeVarieties(gomock.Any()).Return([]wine.GrapeVariety{
		{GrapeVarietyID: 20, Name: "Cabernet", Description: "", IsActive: true},
		{GrapeVarietyID: 21, Name: "Merlot", Description: "Soft", IsActive: true},
	}, nil)

	res, err := r.GrapeVarieties(wineCtx())
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, graphql.ID("20"), res[0].ID())
	assert.Equal(t, "Cabernet", res[0].Name())
	assert.Nil(t, res[0].Description())
	assert.Equal(t, "Soft", *res[1].Description())
}

func TestResolver_Wine_WineFlavorProfiles_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListWineFlavorProfiles(gomock.Any()).Return([]wine.WineFlavorProfile{
		{FlavorProfileID: 30, Name: "Fruity", Description: "", IsActive: true},
		{FlavorProfileID: 31, Name: "Oaky", Description: "Wood", IsActive: true},
	}, nil)

	res, err := r.WineFlavorProfiles(wineCtx())
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, graphql.ID("30"), res[0].ID())
	assert.Equal(t, "Fruity", res[0].Name())
	assert.Nil(t, res[0].Description())
	assert.Equal(t, "Wood", *res[1].Description())
}

func TestResolver_Wine_Bottle_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	b := wine.Bottle{
		BottleID: 101, TypeID: 1, CountryID: 2, RegionID: 3, VintageYear: 2020,
		Vineyard: "Chateau", Abv: winePtrFloat64(13.5), Acidity: winePtrInt16(3), TanninLevel: winePtrInt16(4), Body: winePtrInt16(5),
		Sweetness: winePtrInt16(2), OakIntegration: true, BottleSize: "750ml",
	}
	m.EXPECT().GetBottleByID(gomock.Any(), int64(101)).Return(b, nil)
	m.EXPECT().ListBottleGrapeVarieties(gomock.Any(), int64(101)).Return([]wine.BottleGrapeVariety{
		{GrapeVarietyID: 20, Name: "Cabernet", Percentage: winePtrInt16(80)},
		{GrapeVarietyID: 21, Name: "Merlot", Percentage: winePtrInt16(20)},
	}, nil)
	m.EXPECT().ListBottleFlavorProfiles(gomock.Any(), int64(101)).Return([]wine.BottleFlavorProfile{
		{FlavorProfileID: 30, Name: "Fruity", Intensity: 7},
	}, nil)

	res, err := r.Bottle(wineCtx(), struct{ ID graphql.ID }{ID: "101"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, graphql.ID("101"), res.ID())
	assert.Equal(t, graphql.ID("1"), res.TypeID())
	assert.Equal(t, graphql.ID("2"), res.CountryID())
	assert.Equal(t, graphql.ID("3"), res.RegionID())
	assert.Equal(t, int32(2020), res.VintageYear())
	assert.Equal(t, "Chateau", *res.Vineyard())
	assert.Equal(t, 13.5, *res.Abv())
	assert.Equal(t, int32(3), *res.Acidity())
	assert.Equal(t, int32(4), *res.TanninLevel())
	assert.Equal(t, int32(5), *res.Body())
	assert.Equal(t, int32(2), *res.Sweetness())
	assert.True(t, *res.OakIntegration())
	assert.Equal(t, "750ml", res.BottleSize())

	gvs, err := res.GrapeVarieties(wineCtx())
	require.NoError(t, err)
	require.Len(t, gvs, 2)
	gv, err := gvs[0].GrapeVariety(wineCtx())
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("20"), gv.ID())
	assert.Equal(t, "Cabernet", gv.Name())
	assert.Equal(t, int32(80), *gvs[0].Percentage())

	fps, err := res.FlavorProfiles(wineCtx())
	require.NoError(t, err)
	require.Len(t, fps, 1)
	fp, err := fps[0].FlavorProfile(wineCtx())
	require.NoError(t, err)
	assert.Equal(t, graphql.ID("30"), fp.ID())
	assert.Equal(t, "Fruity", fp.Name())
	assert.Equal(t, int32(7), fps[0].Intensity())
}

func TestResolver_Wine_Bottles_Happy(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	m.EXPECT().ListBottles(gomock.Any(), int32(10), int32(0)).Return([]wine.Bottle{
		{BottleID: 1, TypeID: 1, CountryID: 2, RegionID: 3, VintageYear: 2020, BottleSize: "750ml"},
	}, nil)
	m.EXPECT().CountBottles(gomock.Any()).Return(int64(5), nil)

	res, err := r.Bottles(wineCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.NotNil(t, res)
	items := res.Items()
	require.Len(t, items, 1)
	assert.Equal(t, graphql.ID("1"), items[0].ID())
	pi := res.PageInfo()
	assert.Equal(t, int32(1), pi.PageNumber())
	assert.Equal(t, int32(10), pi.PageSize())
	assert.Equal(t, int32(5), pi.TotalCount())
}

func TestResolver_Wine_Type_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateType(gomock.Any(), "Red", "Wine", wineEmail).Return(wine.Type{
			TypeID: 1, Name: "Red", Description: "Wine", IsActive: true,
		}, nil)

		res, err := r.CreateType(wineCtx(), struct{ Input createTypeInput }{
			Input: createTypeInput{Name: "Red", Description: winePtrString("Wine")},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("1"), res.ID())
		assert.Equal(t, "Red", res.Name())
		assert.Equal(t, "Wine", *res.Description())
	})

	t.Run("Update", func(t *testing.T) {
		gomock.InOrder(
			m.EXPECT().GetTypeByID(gomock.Any(), int64(1)).Return(wine.Type{
				TypeID: 1, Name: "Old", Description: "", IsActive: true,
			}, nil),
			m.EXPECT().UpdateType(gomock.Any(), int64(1), "New", "desc", false, wineEmail).Return(wine.Type{
				TypeID: 1, Name: "New", Description: "desc", IsActive: false,
			}, nil),
		)

		name := "New"
		desc := "desc"
		isActive := false
		res, err := r.UpdateType(wineCtx(), struct {
			ID    graphql.ID
			Input updateTypeInput
		}{
			ID:    "1",
			Input: updateTypeInput{Name: &name, Description: &desc, IsActive: &isActive},
		})
		require.NoError(t, err)
		assert.Equal(t, "New", res.Name())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteType(gomock.Any(), int64(1)).Return(nil)
		ok, err := r.DeleteType(wineCtx(), struct{ ID graphql.ID }{ID: "1"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_Country_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateCountry(gomock.Any(), "France", "FR", "EU", wineEmail).Return(wine.Country{
			CountryID: 1, Name: "France", IsoCode: "FR", Description: "EU", IsActive: true,
		}, nil)

		res, err := r.CreateCountry(wineCtx(), struct{ Input createCountryInput }{
			Input: createCountryInput{Name: "France", IsoCode: "FR", Description: winePtrString("EU")},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("1"), res.ID())
		assert.Equal(t, "FR", *res.IsoCode())
	})

	t.Run("Update", func(t *testing.T) {
		gomock.InOrder(
			m.EXPECT().GetCountryByID(gomock.Any(), int64(1)).Return(wine.Country{
				CountryID: 1, Name: "Old", IsoCode: "OLD", Description: "", IsActive: true,
			}, nil),
			m.EXPECT().UpdateCountry(gomock.Any(), int64(1), "France", "FR", "EU", false, wineEmail).Return(wine.Country{
				CountryID: 1, Name: "France", IsoCode: "FR", Description: "EU", IsActive: false,
			}, nil),
		)

		name := "France"
		iso := "FR"
		desc := "EU"
		isActive := false
		res, err := r.UpdateCountry(wineCtx(), struct {
			ID    graphql.ID
			Input updateCountryInput
		}{
			ID:    "1",
			Input: updateCountryInput{Name: &name, IsoCode: &iso, Description: &desc, IsActive: &isActive},
		})
		require.NoError(t, err)
		assert.Equal(t, "France", res.Name())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteCountry(gomock.Any(), int64(1)).Return(nil)
		ok, err := r.DeleteCountry(wineCtx(), struct{ ID graphql.ID }{ID: "1"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_Region_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateRegion(gomock.Any(), gomock.Eq(wine.Region{CountryID: 1, Name: "Bordeaux"}), wineEmail).Return(wine.Region{
			RegionID: 5, CountryID: 1, Name: "Bordeaux", Description: "", IsActive: true,
		}, nil)
		m.EXPECT().GetCountryByID(gomock.Any(), int64(1)).Return(wine.Country{
			CountryID: 1, Name: "France", IsoCode: "FR", Description: "", IsActive: true,
		}, nil)

		res, err := r.CreateRegion(wineCtx(), struct{ Input createRegionInput }{
			Input: createRegionInput{CountryID: "1", Name: "Bordeaux"},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("5"), res.ID())

		country, err := res.Country(wineCtx())
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("1"), country.ID())
	})

	t.Run("Update", func(t *testing.T) {
		gomock.InOrder(
			m.EXPECT().GetRegionByID(gomock.Any(), int64(5)).Return(wine.Region{
				RegionID: 5, CountryID: 1, Name: "Old", Description: "old", IsActive: true,
			}, nil),
			m.EXPECT().UpdateRegion(gomock.Any(), int64(5), int64(2), "New", "new", false, wineEmail).Return(wine.Region{
				RegionID: 5, CountryID: 2, Name: "New", Description: "new", IsActive: false,
			}, nil),
		)

		cid := graphql.ID("2")
		name := "New"
		desc := "new"
		isActive := false
		res, err := r.UpdateRegion(wineCtx(), struct {
			ID    graphql.ID
			Input updateRegionInput
		}{
			ID:    "5",
			Input: updateRegionInput{CountryID: &cid, Name: &name, Description: &desc, IsActive: &isActive},
		})
		require.NoError(t, err)
		assert.Equal(t, "New", res.Name())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteRegion(gomock.Any(), int64(5)).Return(nil)
		ok, err := r.DeleteRegion(wineCtx(), struct{ ID graphql.ID }{ID: "5"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_Vintage_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateVintage(gomock.Any(), int32(2020), "great", wineEmail).Return(wine.Vintage{
			VintageID: 1, Year: 2020, Description: "great", IsActive: true,
		}, nil)

		res, err := r.CreateVintage(wineCtx(), struct{ Input createVintageInput }{
			Input: createVintageInput{Year: 2020, Description: winePtrString("great")},
		})
		require.NoError(t, err)
		assert.Equal(t, int32(2020), res.Year())
	})

	t.Run("Update", func(t *testing.T) {
		gomock.InOrder(
			m.EXPECT().GetVintageByID(gomock.Any(), int64(1)).Return(wine.Vintage{
				VintageID: 1, Year: 2019, Description: "old", IsActive: true,
			}, nil),
			m.EXPECT().UpdateVintage(gomock.Any(), int64(1), int32(2020), "great", false, wineEmail).Return(wine.Vintage{
				VintageID: 1, Year: 2020, Description: "great", IsActive: false,
			}, nil),
		)

		year := int32(2020)
		desc := "great"
		isActive := false
		res, err := r.UpdateVintage(wineCtx(), struct {
			ID    graphql.ID
			Input updateVintageInput
		}{
			ID:    "1",
			Input: updateVintageInput{Year: &year, Description: &desc, IsActive: &isActive},
		})
		require.NoError(t, err)
		assert.Equal(t, int32(2020), res.Year())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteVintage(gomock.Any(), int64(1)).Return(nil)
		ok, err := r.DeleteVintage(wineCtx(), struct{ ID graphql.ID }{ID: "1"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_GrapeVariety_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateGrapeVariety(gomock.Any(), "Cabernet", "Red grape", wineEmail).Return(wine.GrapeVariety{
			GrapeVarietyID: 20, Name: "Cabernet", Description: "Red grape", IsActive: true,
		}, nil)

		res, err := r.CreateGrapeVariety(wineCtx(), struct{ Input createGrapeVarietyInput }{
			Input: createGrapeVarietyInput{Name: "Cabernet", Description: winePtrString("Red grape")},
		})
		require.NoError(t, err)
		assert.Equal(t, "Cabernet", res.Name())
	})

	t.Run("Update", func(t *testing.T) {
		gomock.InOrder(
			m.EXPECT().GetGrapeVarietyByID(gomock.Any(), int64(20)).Return(wine.GrapeVariety{
				GrapeVarietyID: 20, Name: "Old", Description: "", IsActive: true,
			}, nil),
			m.EXPECT().UpdateGrapeVariety(gomock.Any(), int64(20), "Cabernet", "Red", false, wineEmail).Return(wine.GrapeVariety{
				GrapeVarietyID: 20, Name: "Cabernet", Description: "Red", IsActive: false,
			}, nil),
		)

		name := "Cabernet"
		desc := "Red"
		isActive := false
		res, err := r.UpdateGrapeVariety(wineCtx(), struct {
			ID    graphql.ID
			Input updateGrapeVarietyInput
		}{
			ID:    "20",
			Input: updateGrapeVarietyInput{Name: &name, Description: &desc, IsActive: &isActive},
		})
		require.NoError(t, err)
		assert.Equal(t, "Red", *res.Description())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteGrapeVariety(gomock.Any(), int64(20)).Return(nil)
		ok, err := r.DeleteGrapeVariety(wineCtx(), struct{ ID graphql.ID }{ID: "20"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_WineFlavorProfile_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateWineFlavorProfile(gomock.Any(), "Fruity", "Berries", wineEmail).Return(wine.WineFlavorProfile{
			FlavorProfileID: 30, Name: "Fruity", Description: "Berries", IsActive: true,
		}, nil)

		res, err := r.CreateWineFlavorProfile(wineCtx(), struct{ Input createWineFlavorProfileInput }{
			Input: createWineFlavorProfileInput{Name: "Fruity", Description: winePtrString("Berries")},
		})
		require.NoError(t, err)
		assert.Equal(t, "Fruity", res.Name())
	})

	t.Run("Update", func(t *testing.T) {
		gomock.InOrder(
			m.EXPECT().GetWineFlavorProfileByID(gomock.Any(), int64(30)).Return(wine.WineFlavorProfile{
				FlavorProfileID: 30, Name: "Old", Description: "", IsActive: true,
			}, nil),
			m.EXPECT().UpdateWineFlavorProfile(gomock.Any(), int64(30), "Fruity", "Berries", false, wineEmail).Return(wine.WineFlavorProfile{
				FlavorProfileID: 30, Name: "Fruity", Description: "Berries", IsActive: false,
			}, nil),
		)

		name := "Fruity"
		desc := "Berries"
		isActive := false
		res, err := r.UpdateWineFlavorProfile(wineCtx(), struct {
			ID    graphql.ID
			Input updateWineFlavorProfileInput
		}{
			ID:    "30",
			Input: updateWineFlavorProfileInput{Name: &name, Description: &desc, IsActive: &isActive},
		})
		require.NoError(t, err)
		assert.Equal(t, "Berries", *res.Description())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteWineFlavorProfile(gomock.Any(), int64(30)).Return(nil)
		ok, err := r.DeleteWineFlavorProfile(wineCtx(), struct{ ID graphql.ID }{ID: "30"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_Bottle_Mutations(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	want := wine.Bottle{
		TypeID: 1, CountryID: 2, RegionID: 3, VintageYear: 2020,
		Vineyard: "Chateau", Abv: winePtrFloat64(13.5), Acidity: winePtrInt16(3), TanninLevel: winePtrInt16(4), Body: winePtrInt16(5),
		Sweetness: winePtrInt16(2), OakIntegration: true, BottleSize: "750ml",
	}
	created := want
	created.BottleID = 101

	t.Run("Create", func(t *testing.T) {
		m.EXPECT().CreateBottle(gomock.Any(), gomock.Eq(want), wineEmail).Return(created, nil)

		res, err := r.CreateBottle(wineCtx(), struct{ Input createBottleInput }{
			Input: createBottleInput{
				TypeID: "1", CountryID: "2", RegionID: "3", VintageYear: 2020,
				Vineyard: winePtrString("Chateau"), Abv: winePtrFloat64(13.5),
				Acidity: winePtrInt32(3), TanninLevel: winePtrInt32(4), Body: winePtrInt32(5),
				Sweetness: winePtrInt32(2), OakIntegration: winePtrBool(true), BottleSize: "750ml",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("101"), res.ID())
	})

	t.Run("Update", func(t *testing.T) {
		existing := wine.Bottle{
			BottleID: 101, TypeID: 9, CountryID: 8, RegionID: 7, VintageYear: 2019,
			Vineyard: "Old", Abv: winePtrFloat64(12), Acidity: winePtrInt16(2), TanninLevel: winePtrInt16(3), Body: winePtrInt16(4),
			Sweetness: winePtrInt16(1), OakIntegration: false, BottleSize: "750ml",
		}
		updated := want
		updated.BottleID = 101

		typeID := graphql.ID("1")
		countryID := graphql.ID("2")
		regionID := graphql.ID("3")
		vintage := int32(2020)
		vineyard := "Chateau"
		abv := 13.5
		acidity := int32(3)
		tannin := int32(4)
		body := int32(5)
		sweetness := int32(2)
		oak := true
		size := "750ml"

		gomock.InOrder(
			m.EXPECT().GetBottleByID(gomock.Any(), int64(101)).Return(existing, nil),
			m.EXPECT().UpdateBottle(gomock.Any(), int64(101), gomock.Eq(want), wineEmail).Return(nil),
			m.EXPECT().GetBottleByID(gomock.Any(), int64(101)).Return(updated, nil),
		)

		res, err := r.UpdateBottle(wineCtx(), struct {
			ID    graphql.ID
			Input updateBottleInput
		}{
			ID: "101",
			Input: updateBottleInput{
				TypeID: &typeID, CountryID: &countryID, RegionID: &regionID,
				VintageYear: &vintage, Vineyard: &vineyard, Abv: &abv,
				Acidity: &acidity, TanninLevel: &tannin, Body: &body,
				Sweetness: &sweetness, OakIntegration: &oak, BottleSize: &size,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("101"), res.ID())
		assert.Equal(t, int32(2020), res.VintageYear())
	})

	t.Run("Delete", func(t *testing.T) {
		m.EXPECT().DeleteBottle(gomock.Any(), int64(101)).Return(nil)
		ok, err := r.DeleteBottle(wineCtx(), struct{ ID graphql.ID }{ID: "101"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Delete forbidden for non-admin", func(t *testing.T) {
		ok, err := r.DeleteBottle(wineUserCtx(), struct{ ID graphql.ID }{ID: "101"})
		assert.False(t, ok)
		require.ErrorContains(t, err, "forbidden")
	})
}

func TestResolver_Wine_BottleJunctions(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockWineService(ctrl)
	r := newWineTestResolver(m)

	t.Run("AddBottleGrapeVariety", func(t *testing.T) {
		m.EXPECT().AddBottleGrapeVariety(gomock.Any(), int64(101), int64(201), winePtrInt16(80), wineEmail).Return(wine.BottleGrapeVariety{
			GrapeVarietyID: 201, Name: "Cabernet", Percentage: winePtrInt16(80),
		}, nil)

		res, err := r.AddBottleGrapeVariety(wineCtx(), struct{ Input addBottleGrapeVarietyInput }{
			Input: addBottleGrapeVarietyInput{
				BottleID:       "101",
				GrapeVarietyID: "201",
				Percentage:     winePtrInt32(80),
			},
		})
		require.NoError(t, err)
		gv, err := res.GrapeVariety(wineCtx())
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("201"), gv.ID())
		assert.Equal(t, int32(80), *res.Percentage())
	})

	t.Run("RemoveBottleGrapeVariety", func(t *testing.T) {
		m.EXPECT().RemoveBottleGrapeVariety(gomock.Any(), int64(101), int64(201)).Return(nil)
		ok, err := r.RemoveBottleGrapeVariety(wineCtx(), struct {
			BottleID       graphql.ID
			GrapeVarietyID graphql.ID
		}{BottleID: "101", GrapeVarietyID: "201"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("AddBottleFlavorProfile", func(t *testing.T) {
		m.EXPECT().AddBottleFlavorProfile(gomock.Any(), int64(101), int64(301), int16(7), wineEmail).Return(wine.BottleFlavorProfile{
			FlavorProfileID: 301, Name: "Fruity", Intensity: 7,
		}, nil)

		res, err := r.AddBottleFlavorProfile(wineCtx(), struct{ Input addBottleFlavorProfileInput }{
			Input: addBottleFlavorProfileInput{
				BottleID:        "101",
				FlavorProfileID: "301",
				Intensity:       7,
			},
		})
		require.NoError(t, err)
		fp, err := res.FlavorProfile(wineCtx())
		require.NoError(t, err)
		assert.Equal(t, graphql.ID("301"), fp.ID())
		assert.Equal(t, int32(7), res.Intensity())
	})

	t.Run("RemoveBottleFlavorProfile", func(t *testing.T) {
		m.EXPECT().RemoveBottleFlavorProfile(gomock.Any(), int64(101), int64(301)).Return(nil)
		ok, err := r.RemoveBottleFlavorProfile(wineCtx(), struct {
			BottleID        graphql.ID
			FlavorProfileID graphql.ID
		}{BottleID: "101", FlavorProfileID: "301"})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestResolver_Wine_Unauthorized(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context) (any, error)
	}{
		{"Types", func(ctx context.Context) (any, error) { return (&Resolver{}).Types(ctx) }},
		{"Countries", func(ctx context.Context) (any, error) { return (&Resolver{}).Countries(ctx) }},
		{"Regions", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Regions(ctx, struct{ CountryID graphql.ID }{CountryID: "7"})
		}},
		{"Vintages", func(ctx context.Context) (any, error) { return (&Resolver{}).Vintages(ctx) }},
		{"GrapeVarieties", func(ctx context.Context) (any, error) { return (&Resolver{}).GrapeVarieties(ctx) }},
		{"WineFlavorProfiles", func(ctx context.Context) (any, error) { return (&Resolver{}).WineFlavorProfiles(ctx) }},
		{"Bottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Bottle(ctx, struct{ ID graphql.ID }{ID: "101"})
		}},
		{"Bottles", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Bottles(ctx, struct{ Page, PageSize int32 }{Page: 1, PageSize: 10})
		}},
		{"CreateType", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateType(ctx, struct{ Input createTypeInput }{Input: createTypeInput{Name: "Red"}})
		}},
		{"UpdateType", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateType(ctx, struct {
				ID    graphql.ID
				Input updateTypeInput
			}{ID: "1", Input: updateTypeInput{Name: winePtrString("Red")}})
		}},
		{"DeleteType", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteType(ctx, struct{ ID graphql.ID }{ID: "1"})
		}},
		{"CreateCountry", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateCountry(ctx, struct{ Input createCountryInput }{
				Input: createCountryInput{Name: "France", IsoCode: "FR"},
			})
		}},
		{"UpdateCountry", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateCountry(ctx, struct {
				ID    graphql.ID
				Input updateCountryInput
			}{ID: "1", Input: updateCountryInput{Name: winePtrString("France")}})
		}},
		{"DeleteCountry", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteCountry(ctx, struct{ ID graphql.ID }{ID: "1"})
		}},
		{"CreateRegion", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateRegion(ctx, struct{ Input createRegionInput }{
				Input: createRegionInput{CountryID: "1", Name: "Bordeaux"},
			})
		}},
		{"UpdateRegion", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateRegion(ctx, struct {
				ID    graphql.ID
				Input updateRegionInput
			}{ID: "5", Input: updateRegionInput{Name: winePtrString("Bordeaux")}})
		}},
		{"DeleteRegion", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteRegion(ctx, struct{ ID graphql.ID }{ID: "5"})
		}},
		{"CreateVintage", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateVintage(ctx, struct{ Input createVintageInput }{
				Input: createVintageInput{Year: 2020},
			})
		}},
		{"UpdateVintage", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateVintage(ctx, struct {
				ID    graphql.ID
				Input updateVintageInput
			}{ID: "1", Input: updateVintageInput{Year: winePtrInt32(2020)}})
		}},
		{"DeleteVintage", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteVintage(ctx, struct{ ID graphql.ID }{ID: "1"})
		}},
		{"CreateGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateGrapeVariety(ctx, struct{ Input createGrapeVarietyInput }{
				Input: createGrapeVarietyInput{Name: "Cab"},
			})
		}},
		{"UpdateGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateGrapeVariety(ctx, struct {
				ID    graphql.ID
				Input updateGrapeVarietyInput
			}{ID: "20", Input: updateGrapeVarietyInput{Name: winePtrString("Cab")}})
		}},
		{"DeleteGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteGrapeVariety(ctx, struct{ ID graphql.ID }{ID: "20"})
		}},
		{"CreateWineFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateWineFlavorProfile(ctx, struct{ Input createWineFlavorProfileInput }{
				Input: createWineFlavorProfileInput{Name: "Fruity"},
			})
		}},
		{"UpdateWineFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateWineFlavorProfile(ctx, struct {
				ID    graphql.ID
				Input updateWineFlavorProfileInput
			}{ID: "30", Input: updateWineFlavorProfileInput{Name: winePtrString("Fruity")}})
		}},
		{"DeleteWineFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteWineFlavorProfile(ctx, struct{ ID graphql.ID }{ID: "30"})
		}},
		{"CreateBottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateBottle(ctx, struct{ Input createBottleInput }{
				Input: createBottleInput{TypeID: "1", CountryID: "2", RegionID: "3", VintageYear: 2020, BottleSize: "750ml"},
			})
		}},
		{"UpdateBottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateBottle(ctx, struct {
				ID    graphql.ID
				Input updateBottleInput
			}{ID: "101", Input: updateBottleInput{BottleSize: winePtrString("750ml")}})
		}},
		{"DeleteBottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteBottle(ctx, struct{ ID graphql.ID }{ID: "101"})
		}},
		{"AddBottleGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddBottleGrapeVariety(ctx, struct{ Input addBottleGrapeVarietyInput }{
				Input: addBottleGrapeVarietyInput{BottleID: "101", GrapeVarietyID: "201"},
			})
		}},
		{"RemoveBottleGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveBottleGrapeVariety(ctx, struct {
				BottleID       graphql.ID
				GrapeVarietyID graphql.ID
			}{BottleID: "101", GrapeVarietyID: "201"})
		}},
		{"AddBottleFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddBottleFlavorProfile(ctx, struct{ Input addBottleFlavorProfileInput }{
				Input: addBottleFlavorProfileInput{BottleID: "101", FlavorProfileID: "301", Intensity: 7},
			})
		}},
		{"RemoveBottleFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveBottleFlavorProfile(ctx, struct {
				BottleID        graphql.ID
				FlavorProfileID graphql.ID
			}{BottleID: "101", FlavorProfileID: "301"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.call(context.Background())
			if b, ok := res.(bool); ok {
				assert.False(t, b)
			} else {
				assert.Nil(t, res)
			}
			assert.EqualError(t, err, "unauthorized")
		})
	}
}

func TestResolver_Wine_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context) (any, error)
	}{
		{"Bottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Bottle(ctx, struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"Regions", func(ctx context.Context) (any, error) {
			return (&Resolver{}).Regions(ctx, struct{ CountryID graphql.ID }{CountryID: "abc"})
		}},
		{"UpdateType", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateType(wineCtx(), struct {
				ID    graphql.ID
				Input updateTypeInput
			}{ID: "abc", Input: updateTypeInput{Name: winePtrString("Red")}})
		}},
		{"DeleteType", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteType(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"UpdateCountry", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateCountry(wineCtx(), struct {
				ID    graphql.ID
				Input updateCountryInput
			}{ID: "abc", Input: updateCountryInput{Name: winePtrString("France")}})
		}},
		{"DeleteCountry", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteCountry(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"CreateRegion", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateRegion(wineCtx(), struct{ Input createRegionInput }{
				Input: createRegionInput{CountryID: "abc", Name: "Bordeaux"},
			})
		}},
		{"UpdateRegion", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateRegion(wineCtx(), struct {
				ID    graphql.ID
				Input updateRegionInput
			}{ID: "abc", Input: updateRegionInput{Name: winePtrString("Bordeaux")}})
		}},
		{"DeleteRegion", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteRegion(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"UpdateVintage", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateVintage(wineCtx(), struct {
				ID    graphql.ID
				Input updateVintageInput
			}{ID: "abc", Input: updateVintageInput{Year: winePtrInt32(2020)}})
		}},
		{"DeleteVintage", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteVintage(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"UpdateGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateGrapeVariety(wineCtx(), struct {
				ID    graphql.ID
				Input updateGrapeVarietyInput
			}{ID: "abc", Input: updateGrapeVarietyInput{Name: winePtrString("Cab")}})
		}},
		{"DeleteGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteGrapeVariety(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"UpdateWineFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateWineFlavorProfile(wineCtx(), struct {
				ID    graphql.ID
				Input updateWineFlavorProfileInput
			}{ID: "abc", Input: updateWineFlavorProfileInput{Name: winePtrString("Fruity")}})
		}},
		{"DeleteWineFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteWineFlavorProfile(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"CreateBottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).CreateBottle(wineCtx(), struct{ Input createBottleInput }{
				Input: createBottleInput{TypeID: "abc", CountryID: "2", RegionID: "3", VintageYear: 2020, BottleSize: "750ml"},
			})
		}},
		{"UpdateBottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).UpdateBottle(wineCtx(), struct {
				ID    graphql.ID
				Input updateBottleInput
			}{ID: "abc", Input: updateBottleInput{BottleSize: winePtrString("750ml")}})
		}},
		{"DeleteBottle", func(ctx context.Context) (any, error) {
			return (&Resolver{}).DeleteBottle(wineCtx(), struct{ ID graphql.ID }{ID: "abc"})
		}},
		{"AddBottleGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddBottleGrapeVariety(wineCtx(), struct{ Input addBottleGrapeVarietyInput }{
				Input: addBottleGrapeVarietyInput{BottleID: "abc", GrapeVarietyID: "201"},
			})
		}},
		{"RemoveBottleGrapeVariety", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveBottleGrapeVariety(wineCtx(), struct {
				BottleID       graphql.ID
				GrapeVarietyID graphql.ID
			}{BottleID: "abc", GrapeVarietyID: "201"})
		}},
		{"AddBottleFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).AddBottleFlavorProfile(wineCtx(), struct{ Input addBottleFlavorProfileInput }{
				Input: addBottleFlavorProfileInput{BottleID: "abc", FlavorProfileID: "301", Intensity: 7},
			})
		}},
		{"RemoveBottleFlavorProfile", func(ctx context.Context) (any, error) {
			return (&Resolver{}).RemoveBottleFlavorProfile(wineCtx(), struct {
				BottleID        graphql.ID
				FlavorProfileID graphql.ID
			}{BottleID: "abc", FlavorProfileID: "301"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call(wineCtx())
			assert.Error(t, err)
		})
	}
}

func TestResolver_Wine_ServiceError(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*mock.MockWineService)
		call     func(*Resolver, context.Context) (any, error)
		wantBool bool
	}{
		{
			name: "Types",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().ListTypes(gomock.Any()).Return(nil, errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) { return r.Types(ctx) },
		},
		{
			name: "Bottle",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().GetBottleByID(gomock.Any(), int64(101)).Return(wine.Bottle{}, errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.Bottle(ctx, struct{ ID graphql.ID }{ID: "101"})
			},
		},
		{
			name: "CreateBottle",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().CreateBottle(gomock.Any(), gomock.Any(), wineEmail).Return(wine.Bottle{}, errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.CreateBottle(ctx, struct{ Input createBottleInput }{
					Input: createBottleInput{TypeID: "1", CountryID: "2", RegionID: "3", VintageYear: 2020, BottleSize: "750ml"},
				})
			},
		},
		{
			name: "UpdateType",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().GetTypeByID(gomock.Any(), int64(1)).Return(wine.Type{}, errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				name := "Red"
				return r.UpdateType(ctx, struct {
					ID    graphql.ID
					Input updateTypeInput
				}{ID: "1", Input: updateTypeInput{Name: &name}})
			},
		},
		{
			name: "DeleteBottle",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().DeleteBottle(gomock.Any(), int64(101)).Return(errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.DeleteBottle(ctx, struct{ ID graphql.ID }{ID: "101"})
			},
			wantBool: true,
		},
		{
			name: "AddBottleGrapeVariety",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().AddBottleGrapeVariety(gomock.Any(), int64(101), int64(201), gomock.Any(), wineEmail).Return(wine.BottleGrapeVariety{}, errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.AddBottleGrapeVariety(ctx, struct{ Input addBottleGrapeVarietyInput }{
					Input: addBottleGrapeVarietyInput{BottleID: "101", GrapeVarietyID: "201"},
				})
			},
		},
		{
			name: "RemoveBottleFlavorProfile",
			setup: func(m *mock.MockWineService) {
				m.EXPECT().RemoveBottleFlavorProfile(gomock.Any(), int64(101), int64(301)).Return(errWineBoom)
			},
			call: func(r *Resolver, ctx context.Context) (any, error) {
				return r.RemoveBottleFlavorProfile(ctx, struct {
					BottleID        graphql.ID
					FlavorProfileID graphql.ID
				}{BottleID: "101", FlavorProfileID: "301"})
			},
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockWineService(ctrl)
			r := newWineTestResolver(m)
			tt.setup(m)

			res, err := tt.call(r, wineCtx())
			if tt.wantBool {
				assert.False(t, res.(bool))
			} else {
				assert.Nil(t, res)
			}
			assert.ErrorIs(t, err, errWineBoom)
		})
	}
}
