package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/identity/sqlc"
	"github.com/JRAdams472/LENA2/internal/identity/sqlc/mock"
)

var errDB = errors.New("db error")

func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	return &Service{q: mq}, mq
}

func TestUpsertUser(t *testing.T) {
	ctx := context.Background()

	t.Run("success passes params and maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserParams) (sqlc.IdentityUser, error) {
				assert.Equal(t, "entra", arg.Provider)
				assert.Equal(t, "sub-123", arg.ExternalSubject)
				assert.Equal(t, "a@b.com", arg.Email)
				assert.Equal(t, pgtype.Text{String: "Alice", Valid: true}, arg.DisplayName)
				assert.Equal(t, "a@b.com", arg.CreatedBy)
				assert.Equal(t, pgtype.Text{String: "a@b.com", Valid: true}, arg.UpdatedBy)
				return sqlc.IdentityUser{
					UserID:          42,
					Provider:        arg.Provider,
					ExternalSubject: arg.ExternalSubject,
					Email:           arg.Email,
					DisplayName:     arg.DisplayName,
					IsActive:        true,
				}, nil
			})

		got, err := svc.UpsertUser(ctx, "entra", "sub-123", "a@b.com", "Alice")
		require.NoError(t, err)
		assert.Equal(t, User{
			UserID:          42,
			Provider:        "entra",
			ExternalSubject: "sub-123",
			Email:           "a@b.com",
			DisplayName:     "Alice",
			IsActive:        true,
		}, got)
	})

	t.Run("empty display name becomes null", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserParams) (sqlc.IdentityUser, error) {
				assert.False(t, arg.DisplayName.Valid)
				return sqlc.IdentityUser{UserID: 43, Provider: arg.Provider, Email: arg.Email}, nil
			})

		got, err := svc.UpsertUser(ctx, "entra", "sub-124", "b@c.com", "")
		require.NoError(t, err)
		assert.Equal(t, int64(43), got.UserID)
		assert.Equal(t, "", got.DisplayName)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).Return(sqlc.IdentityUser{}, errDB)

		_, err := svc.UpsertUser(ctx, "entra", "sub-123", "a@b.com", "Alice")
		require.Error(t, err)
		assert.ErrorContains(t, err, "upsert user")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestGetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success maps row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(42)).Return(sqlc.IdentityUser{
			UserID:          42,
			Provider:        "entra",
			ExternalSubject: "sub-123",
			Email:           "a@b.com",
			DisplayName:     pgtype.Text{String: "Alice", Valid: true},
			IsActive:        true,
		}, nil)

		got, err := svc.GetByID(ctx, 42)
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.UserID)
		assert.Equal(t, "entra", got.Provider)
		assert.Equal(t, "sub-123", got.ExternalSubject)
		assert.Equal(t, "a@b.com", got.Email)
		assert.Equal(t, "Alice", got.DisplayName)
		assert.True(t, got.IsActive)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(99)).Return(sqlc.IdentityUser{}, errDB)

		_, err := svc.GetByID(ctx, 99)
		require.Error(t, err)
		assert.ErrorContains(t, err, "get user by id")
		assert.ErrorIs(t, err, errDB)
	})
}
