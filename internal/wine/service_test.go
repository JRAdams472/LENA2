package wine

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/wine/sqlc"
	"github.com/JRAdams472/LENA2/internal/wine/sqlc/mock"
)

var errDB = errors.New(`db error`)

func f64(v float64) *float64 { return &v }
func i16(v int16) *int16     { return &v }

func mustNum(v float64) pgtype.Numeric {
	n, err := numericFromFloat64(v)
	if err != nil {
		panic(err)
	}
	return n
}

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mq := mock.NewMockQuerier(ctrl)
	return &Service{q: mq}, mq
}

func TestCountry(t *testing.T) {
	ctx := context.Background()

	t.Run(`CreateCountry success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateCountry(ctx, gomock.Eq(sqlc.CreateCountryParams{
			Name:        `France`,
			IsoCode:     `FR`,
			Description: textOrNull(`Wine country`),
			IsActive:    true,
			CreatedBy:   `user`,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineCountry{
			CountryID:   1,
			Name:        `France`,
			IsoCode:     `FR`,
			Description: textOrNull(`Wine country`),
			IsActive:    true,
		}, nil)

		got, err := svc.CreateCountry(ctx, `France`, `FR`, `Wine country`, `user`)
		require.NoError(t, err)
		assert.Equal(t, Country{CountryID: 1, Name: `France`, IsoCode: `FR`, Description: `Wine country`, IsActive: true}, got)
	})

	t.Run(`CreateCountry error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateCountry(ctx, gomock.Any()).Return(sqlc.WineCountry{}, errDB)

		_, err := svc.CreateCountry(ctx, `France`, `FR`, `Wine country`, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create country`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetCountryByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetCountryByID(ctx, int64(1)).Return(sqlc.WineCountry{
			CountryID:   1,
			Name:        `France`,
			IsoCode:     `FR`,
			Description: textOrNull(``),
			IsActive:    true,
		}, nil)

		got, err := svc.GetCountryByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, Country{CountryID: 1, Name: `France`, IsoCode: `FR`, Description: ``, IsActive: true}, got)
	})

	t.Run(`GetCountryByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetCountryByID(ctx, int64(1)).Return(sqlc.WineCountry{}, errDB)

		_, err := svc.GetCountryByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get country by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListCountries success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListCountries(ctx).Return([]sqlc.WineCountry{
			{CountryID: 1, Name: `France`, IsoCode: `FR`, Description: textOrNull(`A`), IsActive: true},
			{CountryID: 2, Name: `Italy`, IsoCode: `IT`, Description: textOrNull(`B`), IsActive: true},
		}, nil)

		got, err := svc.ListCountries(ctx)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, Country{CountryID: 1, Name: `France`, IsoCode: `FR`, Description: `A`, IsActive: true}, got[0])
		assert.Equal(t, Country{CountryID: 2, Name: `Italy`, IsoCode: `IT`, Description: `B`, IsActive: true}, got[1])
	})

	t.Run(`ListCountries error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListCountries(ctx).Return(nil, errDB)

		_, err := svc.ListCountries(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list countries`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateCountry success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateCountry(ctx, gomock.Eq(sqlc.UpdateCountryParams{
			CountryID:   1,
			Name:        `France`,
			IsoCode:     `FR`,
			Description: textOrNull(`Updated`),
			IsActive:    false,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineCountry{
			CountryID:   1,
			Name:        `France`,
			IsoCode:     `FR`,
			Description: textOrNull(`Updated`),
			IsActive:    false,
		}, nil)

		got, err := svc.UpdateCountry(ctx, 1, `France`, `FR`, `Updated`, false, `user`)
		require.NoError(t, err)
		assert.Equal(t, Country{CountryID: 1, Name: `France`, IsoCode: `FR`, Description: `Updated`, IsActive: false}, got)
	})

	t.Run(`UpdateCountry error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateCountry(ctx, gomock.Any()).Return(sqlc.WineCountry{}, errDB)

		_, err := svc.UpdateCountry(ctx, 1, `France`, `FR`, `Updated`, false, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `update country`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteCountry success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteCountry(ctx, int64(1)).Return(nil)

		err := svc.DeleteCountry(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteCountry error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteCountry(ctx, int64(1)).Return(errDB)

		err := svc.DeleteCountry(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}
func TestRegion(t *testing.T) {
	ctx := context.Background()

	t.Run(`CreateRegion success`, func(t *testing.T) {
		svc, mq := newService(t)
		arg := Region{CountryID: 1, Name: `Bordeaux`, Description: `Red wine`}
		mq.EXPECT().CreateRegion(ctx, gomock.Eq(sqlc.CreateRegionParams{
			CountryID:   1,
			Name:        `Bordeaux`,
			Description: textOrNull(`Red wine`),
			IsActive:    true,
			CreatedBy:   `user`,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineRegion{
			RegionID:    1,
			CountryID:   1,
			Name:        `Bordeaux`,
			Description: textOrNull(`Red wine`),
			IsActive:    true,
		}, nil)

		got, err := svc.CreateRegion(ctx, arg, `user`)
		require.NoError(t, err)
		assert.Equal(t, Region{RegionID: 1, CountryID: 1, Name: `Bordeaux`, Description: `Red wine`, IsActive: true}, got)
	})

	t.Run(`CreateRegion error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateRegion(ctx, gomock.Any()).Return(sqlc.WineRegion{}, errDB)

		_, err := svc.CreateRegion(ctx, Region{CountryID: 1, Name: `Bordeaux`}, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create region`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetRegionByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRegionByID(ctx, int64(1)).Return(sqlc.WineRegion{
			RegionID:    1,
			CountryID:   1,
			Name:        `Bordeaux`,
			Description: textOrNull(``),
			IsActive:    true,
		}, nil)

		got, err := svc.GetRegionByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, Region{RegionID: 1, CountryID: 1, Name: `Bordeaux`, Description: ``, IsActive: true}, got)
	})

	t.Run(`GetRegionByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetRegionByID(ctx, int64(1)).Return(sqlc.WineRegion{}, errDB)

		_, err := svc.GetRegionByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get region by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListRegions success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRegions(ctx, int64(1)).Return([]sqlc.WineRegion{
			{RegionID: 1, CountryID: 1, Name: `Bordeaux`, Description: textOrNull(`A`), IsActive: true},
		}, nil)

		got, err := svc.ListRegions(ctx, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, Region{RegionID: 1, CountryID: 1, Name: `Bordeaux`, Description: `A`, IsActive: true}, got[0])
	})

	t.Run(`ListRegions error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListRegions(ctx, int64(1)).Return(nil, errDB)

		_, err := svc.ListRegions(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list regions`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateRegion success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateRegion(ctx, gomock.Eq(sqlc.UpdateRegionParams{
			RegionID:    1,
			CountryID:   1,
			Name:        `Bordeaux`,
			Description: textOrNull(`Red`),
			IsActive:    false,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineRegion{
			RegionID:    1,
			CountryID:   1,
			Name:        `Bordeaux`,
			Description: textOrNull(`Red`),
			IsActive:    false,
		}, nil)

		got, err := svc.UpdateRegion(ctx, 1, 1, `Bordeaux`, `Red`, false, `user`)
		require.NoError(t, err)
		assert.Equal(t, Region{RegionID: 1, CountryID: 1, Name: `Bordeaux`, Description: `Red`, IsActive: false}, got)
	})

	t.Run(`UpdateRegion error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateRegion(ctx, gomock.Any()).Return(sqlc.WineRegion{}, errDB)

		_, err := svc.UpdateRegion(ctx, 1, 1, `Bordeaux`, `Red`, false, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `update region`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteRegion success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRegion(ctx, int64(1)).Return(nil)

		err := svc.DeleteRegion(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteRegion error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteRegion(ctx, int64(1)).Return(errDB)

		err := svc.DeleteRegion(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestType(t *testing.T) {
	ctx := context.Background()

	t.Run(`CreateType success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateType(ctx, gomock.Eq(sqlc.CreateTypeParams{
			Name:        `Red`,
			Description: textOrNull(`Red wine`),
			IsActive:    true,
			CreatedBy:   `user`,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineType{
			TypeID:      1,
			Name:        `Red`,
			Description: textOrNull(`Red wine`),
			IsActive:    true,
		}, nil)

		got, err := svc.CreateType(ctx, `Red`, `Red wine`, `user`)
		require.NoError(t, err)
		assert.Equal(t, Type{TypeID: 1, Name: `Red`, Description: `Red wine`, IsActive: true}, got)
	})

	t.Run(`CreateType error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateType(ctx, gomock.Any()).Return(sqlc.WineType{}, errDB)

		_, err := svc.CreateType(ctx, `Red`, `Red wine`, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create type`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetTypeByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetTypeByID(ctx, int64(1)).Return(sqlc.WineType{
			TypeID:      1,
			Name:        `Red`,
			Description: textOrNull(``),
			IsActive:    true,
		}, nil)

		got, err := svc.GetTypeByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, Type{TypeID: 1, Name: `Red`, Description: ``, IsActive: true}, got)
	})

	t.Run(`GetTypeByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetTypeByID(ctx, int64(1)).Return(sqlc.WineType{}, errDB)

		_, err := svc.GetTypeByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get type by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListTypes success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListTypes(ctx).Return([]sqlc.WineType{
			{TypeID: 1, Name: `Red`, Description: textOrNull(`A`), IsActive: true},
		}, nil)

		got, err := svc.ListTypes(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, Type{TypeID: 1, Name: `Red`, Description: `A`, IsActive: true}, got[0])
	})

	t.Run(`ListTypes error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListTypes(ctx).Return(nil, errDB)

		_, err := svc.ListTypes(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list types`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateType success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateType(ctx, gomock.Eq(sqlc.UpdateTypeParams{
			TypeID:      1,
			Name:        `Red`,
			Description: textOrNull(`Red wine`),
			IsActive:    false,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineType{
			TypeID:      1,
			Name:        `Red`,
			Description: textOrNull(`Red wine`),
			IsActive:    false,
		}, nil)

		got, err := svc.UpdateType(ctx, 1, `Red`, `Red wine`, false, `user`)
		require.NoError(t, err)
		assert.Equal(t, Type{TypeID: 1, Name: `Red`, Description: `Red wine`, IsActive: false}, got)
	})

	t.Run(`UpdateType error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateType(ctx, gomock.Any()).Return(sqlc.WineType{}, errDB)

		_, err := svc.UpdateType(ctx, 1, `Red`, `Red wine`, false, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `update type`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteType success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteType(ctx, int64(1)).Return(nil)

		err := svc.DeleteType(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteType error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteType(ctx, int64(1)).Return(errDB)

		err := svc.DeleteType(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestVintage(t *testing.T) {
	ctx := context.Background()

	t.Run(`CreateVintage success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateVintage(ctx, gomock.Eq(sqlc.CreateVintageParams{
			Year:        2020,
			Description: textOrNull(`Great year`),
			IsActive:    true,
			CreatedBy:   `user`,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineVintage{
			VintageID:   1,
			Year:        2020,
			Description: textOrNull(`Great year`),
			IsActive:    true,
		}, nil)

		got, err := svc.CreateVintage(ctx, 2020, `Great year`, `user`)
		require.NoError(t, err)
		assert.Equal(t, Vintage{VintageID: 1, Year: 2020, Description: `Great year`, IsActive: true}, got)
	})

	t.Run(`CreateVintage error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateVintage(ctx, gomock.Any()).Return(sqlc.WineVintage{}, errDB)

		_, err := svc.CreateVintage(ctx, 2020, `Great year`, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create vintage`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetVintageByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetVintageByID(ctx, int64(1)).Return(sqlc.WineVintage{
			VintageID:   1,
			Year:        2020,
			Description: textOrNull(``),
			IsActive:    true,
		}, nil)

		got, err := svc.GetVintageByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, Vintage{VintageID: 1, Year: 2020, Description: ``, IsActive: true}, got)
	})

	t.Run(`GetVintageByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetVintageByID(ctx, int64(1)).Return(sqlc.WineVintage{}, errDB)

		_, err := svc.GetVintageByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get vintage by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListVintages success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListVintages(ctx).Return([]sqlc.WineVintage{
			{VintageID: 1, Year: 2020, Description: textOrNull(`A`), IsActive: true},
		}, nil)

		got, err := svc.ListVintages(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, Vintage{VintageID: 1, Year: 2020, Description: `A`, IsActive: true}, got[0])
	})

	t.Run(`ListVintages error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListVintages(ctx).Return(nil, errDB)

		_, err := svc.ListVintages(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list vintages`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateVintage success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateVintage(ctx, gomock.Eq(sqlc.UpdateVintageParams{
			VintageID:   1,
			Year:        2020,
			Description: textOrNull(`Great year`),
			IsActive:    false,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineVintage{
			VintageID:   1,
			Year:        2020,
			Description: textOrNull(`Great year`),
			IsActive:    false,
		}, nil)

		got, err := svc.UpdateVintage(ctx, 1, 2020, `Great year`, false, `user`)
		require.NoError(t, err)
		assert.Equal(t, Vintage{VintageID: 1, Year: 2020, Description: `Great year`, IsActive: false}, got)
	})

	t.Run(`UpdateVintage error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateVintage(ctx, gomock.Any()).Return(sqlc.WineVintage{}, errDB)

		_, err := svc.UpdateVintage(ctx, 1, 2020, `Great year`, false, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `update vintage`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteVintage success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteVintage(ctx, int64(1)).Return(nil)

		err := svc.DeleteVintage(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteVintage error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteVintage(ctx, int64(1)).Return(errDB)

		err := svc.DeleteVintage(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGrapeVariety(t *testing.T) {
	ctx := context.Background()

	t.Run(`CreateGrapeVariety success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateGrapeVariety(ctx, gomock.Eq(sqlc.CreateGrapeVarietyParams{
			Name:        `Merlot`,
			Description: textOrNull(`Dark fruit`),
			IsActive:    true,
			CreatedBy:   `user`,
			UpdatedBy:   textOrNull(`user`),
		})).Return(sqlc.WineGrapeVariety{
			GrapeVarietyID: 1,
			Name:           `Merlot`,
			Description:    textOrNull(`Dark fruit`),
			IsActive:       true,
		}, nil)

		got, err := svc.CreateGrapeVariety(ctx, `Merlot`, `Dark fruit`, `user`)
		require.NoError(t, err)
		assert.Equal(t, GrapeVariety{GrapeVarietyID: 1, Name: `Merlot`, Description: `Dark fruit`, IsActive: true}, got)
	})

	t.Run(`CreateGrapeVariety error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateGrapeVariety(ctx, gomock.Any()).Return(sqlc.WineGrapeVariety{}, errDB)

		_, err := svc.CreateGrapeVariety(ctx, `Merlot`, `Dark fruit`, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create grape variety`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetGrapeVarietyByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetGrapeVarietyByID(ctx, int64(1)).Return(sqlc.WineGrapeVariety{
			GrapeVarietyID: 1,
			Name:           `Merlot`,
			Description:    textOrNull(``),
			IsActive:       true,
		}, nil)

		got, err := svc.GetGrapeVarietyByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, GrapeVariety{GrapeVarietyID: 1, Name: `Merlot`, Description: ``, IsActive: true}, got)
	})

	t.Run(`GetGrapeVarietyByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetGrapeVarietyByID(ctx, int64(1)).Return(sqlc.WineGrapeVariety{}, errDB)

		_, err := svc.GetGrapeVarietyByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get grape variety by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListGrapeVarieties success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListGrapeVarieties(ctx).Return([]sqlc.WineGrapeVariety{
			{GrapeVarietyID: 1, Name: `Merlot`, Description: textOrNull(`A`), IsActive: true},
		}, nil)

		got, err := svc.ListGrapeVarieties(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, GrapeVariety{GrapeVarietyID: 1, Name: `Merlot`, Description: `A`, IsActive: true}, got[0])
	})

	t.Run(`ListGrapeVarieties error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListGrapeVarieties(ctx).Return(nil, errDB)

		_, err := svc.ListGrapeVarieties(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list grape varieties`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateGrapeVariety success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateGrapeVariety(ctx, gomock.Eq(sqlc.UpdateGrapeVarietyParams{
			GrapeVarietyID: 1,
			Name:           `Merlot`,
			Description:    textOrNull(`Dark fruit`),
			IsActive:       false,
			UpdatedBy:      textOrNull(`user`),
		})).Return(sqlc.WineGrapeVariety{
			GrapeVarietyID: 1,
			Name:           `Merlot`,
			Description:    textOrNull(`Dark fruit`),
			IsActive:       false,
		}, nil)

		got, err := svc.UpdateGrapeVariety(ctx, 1, `Merlot`, `Dark fruit`, false, `user`)
		require.NoError(t, err)
		assert.Equal(t, GrapeVariety{GrapeVarietyID: 1, Name: `Merlot`, Description: `Dark fruit`, IsActive: false}, got)
	})

	t.Run(`UpdateGrapeVariety error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateGrapeVariety(ctx, gomock.Any()).Return(sqlc.WineGrapeVariety{}, errDB)

		_, err := svc.UpdateGrapeVariety(ctx, 1, `Merlot`, `Dark fruit`, false, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `update grape variety`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteGrapeVariety success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteGrapeVariety(ctx, int64(1)).Return(nil)

		err := svc.DeleteGrapeVariety(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteGrapeVariety error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteGrapeVariety(ctx, int64(1)).Return(errDB)

		err := svc.DeleteGrapeVariety(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestWineFlavorProfile(t *testing.T) {
	ctx := context.Background()

	t.Run(`ListWineFlavorProfiles success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListWineFlavorProfiles(ctx).Return([]sqlc.WineFlavorProfile{
			{FlavorProfileID: 1, Name: `Oak`, Description: textOrNull(`Woody`), IsActive: true},
		}, nil)

		got, err := svc.ListWineFlavorProfiles(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, WineFlavorProfile{FlavorProfileID: 1, Name: `Oak`, Description: `Woody`, IsActive: true}, got[0])
	})

	t.Run(`ListWineFlavorProfiles error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListWineFlavorProfiles(ctx).Return(nil, errDB)

		_, err := svc.ListWineFlavorProfiles(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list wine flavor profiles`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetWineFlavorProfileByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetWineFlavorProfileByID(ctx, int64(1)).Return(sqlc.WineFlavorProfile{
			FlavorProfileID: 1,
			Name:            `Oak`,
			Description:     textOrNull(``),
			IsActive:        true,
		}, nil)

		got, err := svc.GetWineFlavorProfileByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, WineFlavorProfile{FlavorProfileID: 1, Name: `Oak`, Description: ``, IsActive: true}, got)
	})

	t.Run(`GetWineFlavorProfileByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetWineFlavorProfileByID(ctx, int64(1)).Return(sqlc.WineFlavorProfile{}, errDB)

		_, err := svc.GetWineFlavorProfileByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get wine flavor profile by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`CreateWineFlavorProfile success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateWineFlavorProfile(ctx, gomock.Eq(sqlc.CreateWineFlavorProfileParams{
			Name:        `Oak`,
			Description: textOrNull(`Woody`),
			CreatedBy:   `user`,
		})).Return(sqlc.WineFlavorProfile{
			FlavorProfileID: 1,
			Name:            `Oak`,
			Description:     textOrNull(`Woody`),
			IsActive:        true,
		}, nil)

		got, err := svc.CreateWineFlavorProfile(ctx, `Oak`, `Woody`, `user`)
		require.NoError(t, err)
		assert.Equal(t, WineFlavorProfile{FlavorProfileID: 1, Name: `Oak`, Description: `Woody`, IsActive: true}, got)
	})

	t.Run(`CreateWineFlavorProfile error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateWineFlavorProfile(ctx, gomock.Any()).Return(sqlc.WineFlavorProfile{}, errDB)

		_, err := svc.CreateWineFlavorProfile(ctx, `Oak`, `Woody`, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create wine flavor profile`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateWineFlavorProfile success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateWineFlavorProfile(ctx, gomock.Eq(sqlc.UpdateWineFlavorProfileParams{
			FlavorProfileID: 1,
			Name:            `Oak`,
			Description:     textOrNull(`Woody`),
			IsActive:        false,
			UpdatedBy:       textOrNull(`user`),
		})).Return(sqlc.WineFlavorProfile{
			FlavorProfileID: 1,
			Name:            `Oak`,
			Description:     textOrNull(`Woody`),
			IsActive:        false,
		}, nil)

		got, err := svc.UpdateWineFlavorProfile(ctx, 1, `Oak`, `Woody`, false, `user`)
		require.NoError(t, err)
		assert.Equal(t, WineFlavorProfile{FlavorProfileID: 1, Name: `Oak`, Description: `Woody`, IsActive: false}, got)
	})

	t.Run(`UpdateWineFlavorProfile error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateWineFlavorProfile(ctx, gomock.Any()).Return(sqlc.WineFlavorProfile{}, errDB)

		_, err := svc.UpdateWineFlavorProfile(ctx, 1, `Oak`, `Woody`, false, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `update wine flavor profile`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteWineFlavorProfile success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteWineFlavorProfile(ctx, int64(1)).Return(nil)

		err := svc.DeleteWineFlavorProfile(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteWineFlavorProfile error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteWineFlavorProfile(ctx, int64(1)).Return(errDB)

		err := svc.DeleteWineFlavorProfile(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestBottle(t *testing.T) {
	ctx := context.Background()

	bottleArg := Bottle{
		TypeID:         1,
		CountryID:      2,
		RegionID:       3,
		VintageYear:    2020,
		Vineyard:       `Chateau`,
		Abv:            f64(13.5),
		Acidity:        i16(5),
		TanninLevel:    i16(4),
		Body:           i16(3),
		Sweetness:      i16(2),
		OakIntegration: true,
		BottleSize:     `750ml`,
	}

	bottleRow := sqlc.WineBottle{
		BottleID:       1,
		TypeID:         1,
		CountryID:      2,
		RegionID:       3,
		VintageYear:    2020,
		Vineyard:       textOrNull(`Chateau`),
		Abv:            mustNum(13.5),
		Acidity:        optInt2(i16(5)),
		TanninLevel:    optInt2(i16(4)),
		Body:           optInt2(i16(3)),
		Sweetness:      optInt2(i16(2)),
		OakIntegration: boolOrNull(true),
		BottleSize:     `750ml`,
	}

	expectedBottle := Bottle{
		BottleID:       1,
		TypeID:         1,
		CountryID:      2,
		RegionID:       3,
		VintageYear:    2020,
		Vineyard:       `Chateau`,
		Abv:            f64(13.5),
		Acidity:        i16(5),
		TanninLevel:    i16(4),
		Body:           i16(3),
		Sweetness:      i16(2),
		OakIntegration: true,
		BottleSize:     `750ml`,
	}

	t.Run(`CreateBottle success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateBottle(ctx, gomock.Eq(sqlc.CreateBottleParams{
			TypeID:         1,
			CountryID:      2,
			RegionID:       3,
			VintageYear:    2020,
			Vineyard:       textOrNull(`Chateau`),
			Abv:            mustNum(13.5),
			Acidity:        optInt2(i16(5)),
			TanninLevel:    optInt2(i16(4)),
			Body:           optInt2(i16(3)),
			Sweetness:      optInt2(i16(2)),
			OakIntegration: boolOrNull(true),
			BottleSize:     `750ml`,
			CreatedBy:      `user`,
			UpdatedBy:      textOrNull(`user`),
		})).Return(bottleRow, nil)

		got, err := svc.CreateBottle(ctx, bottleArg, `user`)
		require.NoError(t, err)
		assert.Equal(t, expectedBottle, got)
	})

	t.Run(`CreateBottle error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateBottle(ctx, gomock.Any()).Return(sqlc.WineBottle{}, errDB)

		_, err := svc.CreateBottle(ctx, bottleArg, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `create bottle`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`GetBottleByID success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetBottleByID(ctx, int64(1)).Return(bottleRow, nil)

		got, err := svc.GetBottleByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, expectedBottle, got)
	})

	t.Run(`GetBottleByID error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetBottleByID(ctx, int64(1)).Return(sqlc.WineBottle{}, errDB)

		_, err := svc.GetBottleByID(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `get bottle by id`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListBottles success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListBottles(ctx, gomock.Eq(sqlc.ListBottlesParams{
			Limit:  10,
			Offset: 5,
		})).Return([]sqlc.WineBottle{bottleRow}, nil)

		got, err := svc.ListBottles(ctx, 10, 5)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, expectedBottle, got[0])
	})

	t.Run(`ListBottles error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListBottles(ctx, gomock.Any()).Return(nil, errDB)

		_, err := svc.ListBottles(ctx, 10, 5)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list bottles`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`UpdateBottle success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateBottle(ctx, gomock.Eq(sqlc.UpdateBottleParams{
			BottleID:       1,
			TypeID:         1,
			CountryID:      2,
			RegionID:       3,
			VintageYear:    2020,
			Vineyard:       textOrNull(`Chateau`),
			Abv:            mustNum(13.5),
			Acidity:        optInt2(i16(5)),
			TanninLevel:    optInt2(i16(4)),
			Body:           optInt2(i16(3)),
			Sweetness:      optInt2(i16(2)),
			OakIntegration: boolOrNull(true),
			BottleSize:     `750ml`,
			UpdatedBy:      textOrNull(`user`),
		})).Return(nil)

		err := svc.UpdateBottle(ctx, 1, bottleArg, `user`)
		require.NoError(t, err)
	})

	t.Run(`UpdateBottle error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateBottle(ctx, gomock.Any()).Return(errDB)

		err := svc.UpdateBottle(ctx, 1, bottleArg, `user`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`DeleteBottle success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteBottle(ctx, int64(1)).Return(nil)

		err := svc.DeleteBottle(ctx, 1)
		require.NoError(t, err)
	})

	t.Run(`DeleteBottle error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteBottle(ctx, int64(1)).Return(errDB)

		err := svc.DeleteBottle(ctx, 1)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestJunction(t *testing.T) {
	ctx := context.Background()

	t.Run(`AddBottleGrapeVariety success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateBottleGrapeVariety(ctx, gomock.Eq(sqlc.CreateBottleGrapeVarietyParams{
			BottleID:       1,
			GrapeVarietyID: 2,
			Percentage:     optInt2(i16(45)),
			CreatedBy:      `user`,
		})).Return(sqlc.WineBottleGrapeVariety{
			BottleID:       1,
			GrapeVarietyID: 2,
			Percentage:     optInt2(i16(45)),
			CreatedBy:      `user`,
		}, nil)

		got, err := svc.AddBottleGrapeVariety(ctx, 1, 2, i16(45), `user`)
		require.NoError(t, err)
		assert.Equal(t, BottleGrapeVariety{GrapeVarietyID: 2, Percentage: i16(45)}, got)
	})

	t.Run(`AddBottleGrapeVariety error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateBottleGrapeVariety(ctx, gomock.Any()).Return(sqlc.WineBottleGrapeVariety{}, errDB)

		_, err := svc.AddBottleGrapeVariety(ctx, 1, 2, i16(45), `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `add bottle grape variety`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListBottleGrapeVarieties success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListBottleGrapeVarieties(ctx, int64(1)).Return([]sqlc.ListBottleGrapeVarietiesRow{
			{GrapeVarietyID: 2, Name: `Merlot`, Percentage: optInt2(i16(45))},
		}, nil)

		got, err := svc.ListBottleGrapeVarieties(ctx, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, BottleGrapeVariety{BottleID: 1, GrapeVarietyID: 2, Name: `Merlot`, Percentage: i16(45)}, got[0])
	})

	t.Run(`ListBottleGrapeVarieties error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListBottleGrapeVarieties(ctx, int64(1)).Return(nil, errDB)

		_, err := svc.ListBottleGrapeVarieties(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list bottle grape varieties`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`RemoveBottleGrapeVariety success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteBottleGrapeVariety(ctx, gomock.Eq(sqlc.DeleteBottleGrapeVarietyParams{
			BottleID:       1,
			GrapeVarietyID: 2,
		})).Return(nil)

		err := svc.RemoveBottleGrapeVariety(ctx, 1, 2)
		require.NoError(t, err)
	})

	t.Run(`RemoveBottleGrapeVariety error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteBottleGrapeVariety(ctx, gomock.Any()).Return(errDB)

		err := svc.RemoveBottleGrapeVariety(ctx, 1, 2)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`AddBottleFlavorProfile success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateBottleFlavorProfile(ctx, gomock.Eq(sqlc.CreateBottleFlavorProfileParams{
			BottleID:        1,
			FlavorProfileID: 2,
			Intensity:       7,
			CreatedBy:       `user`,
		})).Return(sqlc.WineBottleFlavorProfile{
			BottleID:        1,
			FlavorProfileID: 2,
			Intensity:       7,
			CreatedBy:       `user`,
		}, nil)

		got, err := svc.AddBottleFlavorProfile(ctx, 1, 2, 7, `user`)
		require.NoError(t, err)
		assert.Equal(t, BottleFlavorProfile{FlavorProfileID: 2, Intensity: 7}, got)
	})

	t.Run(`AddBottleFlavorProfile error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CreateBottleFlavorProfile(ctx, gomock.Any()).Return(sqlc.WineBottleFlavorProfile{}, errDB)

		_, err := svc.AddBottleFlavorProfile(ctx, 1, 2, 7, `user`)
		require.Error(t, err)
		assert.ErrorContains(t, err, `add bottle flavor profile`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`ListBottleFlavorProfiles success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListBottleFlavorProfiles(ctx, int64(1)).Return([]sqlc.ListBottleFlavorProfilesRow{
			{FlavorProfileID: 2, Name: `Oak`, Intensity: 7},
		}, nil)

		got, err := svc.ListBottleFlavorProfiles(ctx, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, BottleFlavorProfile{BottleID: 1, FlavorProfileID: 2, Name: `Oak`, Intensity: 7}, got[0])
	})

	t.Run(`ListBottleFlavorProfiles error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListBottleFlavorProfiles(ctx, int64(1)).Return(nil, errDB)

		_, err := svc.ListBottleFlavorProfiles(ctx, 1)
		require.Error(t, err)
		assert.ErrorContains(t, err, `list bottle flavor profiles`)
		assert.ErrorIs(t, err, errDB)
	})

	t.Run(`RemoveBottleFlavorProfile success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteBottleFlavorProfile(ctx, gomock.Eq(sqlc.DeleteBottleFlavorProfileParams{
			BottleID:        1,
			FlavorProfileID: 2,
		})).Return(nil)

		err := svc.RemoveBottleFlavorProfile(ctx, 1, 2)
		require.NoError(t, err)
	})

	t.Run(`RemoveBottleFlavorProfile error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().DeleteBottleFlavorProfile(ctx, gomock.Any()).Return(errDB)

		err := svc.RemoveBottleFlavorProfile(ctx, 1, 2)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestCountBottles(t *testing.T) {
	ctx := context.Background()

	t.Run(`success`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountBottles(ctx).Return(int64(42), nil)

		got, err := svc.CountBottles(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(42), got)
	})

	t.Run(`error`, func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountBottles(ctx).Return(int64(0), errDB)

		_, err := svc.CountBottles(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, `count bottles`)
		assert.ErrorIs(t, err, errDB)
	})
}
