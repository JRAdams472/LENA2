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
					Role:            RoleMember,
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
			Role:            RoleMember,
		}, got)
		assert.False(t, got.IsAdmin())
	})

	t.Run("empty display name becomes null", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserParams) (sqlc.IdentityUser, error) {
				assert.False(t, arg.DisplayName.Valid)
				return sqlc.IdentityUser{UserID: 43, Provider: arg.Provider, Email: arg.Email, Role: RoleMember}, nil
			})

		got, err := svc.UpsertUser(ctx, "entra", "sub-124", "b@c.com", "")
		require.NoError(t, err)
		assert.Equal(t, int64(43), got.UserID)
		assert.Equal(t, "", got.DisplayName)
		assert.Equal(t, RoleMember, got.Role)
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
			Role:            RoleAdmin,
		}, nil)

		got, err := svc.GetByID(ctx, 42)
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.UserID)
		assert.Equal(t, "entra", got.Provider)
		assert.Equal(t, "sub-123", got.ExternalSubject)
		assert.Equal(t, "a@b.com", got.Email)
		assert.Equal(t, "Alice", got.DisplayName)
		assert.True(t, got.IsActive)
		assert.Equal(t, RoleAdmin, got.Role)
		assert.True(t, got.IsAdmin())
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

func TestSetUserRole(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SetUserRole(ctx, sqlc.SetUserRoleParams{UserID: 42, Role: RoleAdmin}).Return(nil)

		err := svc.SetUserRole(ctx, 42, RoleAdmin)
		require.NoError(t, err)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SetUserRole(ctx, gomock.Any()).Return(errDB)

		err := svc.SetUserRole(ctx, 42, RoleAdmin)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
	})
}

func adminRow(userID int64, email string) sqlc.IdentityUser {
	return sqlc.IdentityUser{UserID: userID, Email: email, Role: RoleAdmin, IsActive: true}
}

func memberRow(userID int64, email string) sqlc.IdentityUser {
	return sqlc.IdentityUser{UserID: userID, Email: email, Role: RoleMember, IsActive: true}
}

func TestAdminSetRole(t *testing.T) {
	ctx := context.Background()

	t.Run("promotes a member", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(memberRow(2, "m@b.com"), nil)
		mq.EXPECT().SetUserRole(ctx, sqlc.SetUserRoleParams{UserID: 2, Role: RoleAdmin}).Return(nil)

		require.NoError(t, svc.AdminSetRole(ctx, 1, 2, RoleAdmin))
	})

	t.Run("demoting an admin requires another active admin", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(adminRow(2, "a2@b.com"), nil)
		mq.EXPECT().CountActiveAdmins(ctx).Return(int64(3), nil)
		mq.EXPECT().SetUserRole(ctx, sqlc.SetUserRoleParams{UserID: 2, Role: RoleMember}).Return(nil)

		require.NoError(t, svc.AdminSetRole(ctx, 1, 2, RoleMember))
	})

	t.Run("cannot demote self", func(t *testing.T) {
		svc, _ := newService(t)
		err := svc.AdminSetRole(ctx, 2, 2, RoleMember)
		assert.ErrorIs(t, err, ErrSelfModification)
	})

	t.Run("cannot demote the last active admin", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(adminRow(2, "a2@b.com"), nil)
		mq.EXPECT().CountActiveAdmins(ctx).Return(int64(1), nil)

		err := svc.AdminSetRole(ctx, 1, 2, RoleMember)
		assert.ErrorIs(t, err, ErrLastAdmin)
	})

	t.Run("cannot demote a protected admin", func(t *testing.T) {
		svc, mq := newService(t)
		svc = svc.WithProtectedEmails([]string{"Boss@Example.com"})
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(adminRow(2, "boss@example.com"), nil)

		err := svc.AdminSetRole(ctx, 1, 2, RoleMember)
		assert.ErrorIs(t, err, ErrProtectedUser)
	})
}

func TestAdminSetActive(t *testing.T) {
	ctx := context.Background()

	t.Run("bans a member", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(memberRow(2, "m@b.com"), nil)
		mq.EXPECT().SetUserActive(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.SetUserActiveParams) error {
				assert.Equal(t, int64(2), arg.UserID)
				assert.False(t, arg.IsActive)
				assert.Equal(t, pgtype.Text{String: "admin@b.com", Valid: true}, arg.UpdatedBy)
				return nil
			})

		require.NoError(t, svc.AdminSetActive(ctx, 1, 2, false, "admin@b.com"))
	})

	t.Run("cannot ban self", func(t *testing.T) {
		svc, _ := newService(t)
		err := svc.AdminSetActive(ctx, 2, 2, false, "a@b.com")
		assert.ErrorIs(t, err, ErrSelfModification)
	})

	t.Run("cannot ban a protected admin", func(t *testing.T) {
		svc, mq := newService(t)
		svc = svc.WithProtectedEmails([]string{"boss@example.com"})
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(adminRow(2, "BOSS@example.com"), nil)

		err := svc.AdminSetActive(ctx, 1, 2, false, "a@b.com")
		assert.ErrorIs(t, err, ErrProtectedUser)
	})

	t.Run("cannot ban the last active admin", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(adminRow(2, "a2@b.com"), nil)
		mq.EXPECT().CountActiveAdmins(ctx).Return(int64(1), nil)

		err := svc.AdminSetActive(ctx, 1, 2, false, "a@b.com")
		assert.ErrorIs(t, err, ErrLastAdmin)
	})

	t.Run("unban has no admin guard", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(memberRow(2, "m@b.com"), nil)
		mq.EXPECT().SetUserActive(ctx, gomock.Any()).Return(nil)

		require.NoError(t, svc.AdminSetActive(ctx, 1, 2, true, "admin@b.com"))
	})
}

func TestUpdateProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("passes profile fields", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateUserProfile(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpdateUserProfileParams) error {
				assert.Equal(t, int64(7), arg.UserID)
				assert.Equal(t, pgtype.Text{String: "Ada", Valid: true}, arg.FirstName)
				assert.Equal(t, pgtype.Text{String: "Lovelace", Valid: true}, arg.LastName)
				assert.Equal(t, pgtype.Text{String: "alt@b.com", Valid: true}, arg.BackupEmail)
				assert.Equal(t, pgtype.Text{String: "a@b.com", Valid: true}, arg.UpdatedBy)
				return nil
			})

		require.NoError(t, svc.UpdateProfile(ctx, 7, "Ada", "Lovelace", "alt@b.com", "a@b.com"))
	})

	t.Run("empty fields become null", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateUserProfile(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpdateUserProfileParams) error {
				assert.False(t, arg.FirstName.Valid)
				assert.False(t, arg.BackupEmail.Valid)
				return nil
			})

		require.NoError(t, svc.UpdateProfile(ctx, 7, "", "Lovelace", "", "a@b.com"))
	})
}

func TestListAndCountUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("list maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListUsers(ctx, sqlc.ListUsersParams{Limit: 25, Offset: 50}).Return([]sqlc.IdentityUser{
			adminRow(1, "a@b.com"),
			memberRow(2, "m@b.com"),
		}, nil)

		users, err := svc.ListUsers(ctx, 25, 50)
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "a@b.com", users[0].Email)
		assert.True(t, users[0].IsAdmin())
	})

	t.Run("count", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().CountUsers(ctx).Return(int64(17), nil)

		n, err := svc.CountUsers(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(17), n)
	})
}

func TestIsProtected(t *testing.T) {
	svc, _ := newService(t)
	assert.False(t, svc.IsProtected("a@b.com"))

	svc = svc.WithProtectedEmails([]string{" Boss@Example.com ", "", "second@b.com"})
	assert.True(t, svc.IsProtected("boss@example.com"))
	assert.True(t, svc.IsProtected("BOSS@EXAMPLE.COM"))
	assert.True(t, svc.IsProtected("second@b.com"))
	assert.False(t, svc.IsProtected("other@b.com"))
}
