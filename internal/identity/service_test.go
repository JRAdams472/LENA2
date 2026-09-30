package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/identity/sqlc"
	"github.com/JRAdams472/LENA2/internal/identity/sqlc/mock"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

var errDB = errors.New("db error")

// newService returns a service whose plain and tx-bound queries both land
// on the mock; the stub pool supplies a no-op transaction for InTx.
func newService(t *testing.T) (*Service, *mock.MockQuerier) {
	t.Helper()
	mq := mock.NewMockQuerier(gomock.NewController(t))
	s := &Service{q: mq, pool: &stubPool{tx: &stubTx{}}}
	s.newQ = func(pgx.Tx) sqlc.Querier { return mq }
	return s, mq
}

// missLogin stubs the login-table miss that precedes user creation.
func missLogin(mq *mock.MockQuerier) {
	mq.EXPECT().GetUserByLogin(gomock.Any(), gomock.Any()).
		Return(sqlc.IdentityUser{}, pgx.ErrNoRows)
}

// expectLogin stub the login row write that follows a user upsert.
func expectLogin(mq *mock.MockQuerier) {
	mq.EXPECT().UpsertLogin(gomock.Any(), gomock.Any()).
		Return(sqlc.IdentityUserLogin{}, nil)
}

func TestUpsertUser(t *testing.T) {
	ctx := context.Background()

	t.Run("new identity creates user and login rows", func(t *testing.T) {
		svc, mq := newService(t)
		missLogin(mq)
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
		mq.EXPECT().UpsertLogin(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertLoginParams) (sqlc.IdentityUserLogin, error) {
				assert.Equal(t, int64(42), arg.UserID)
				assert.Equal(t, "entra", arg.Provider)
				assert.Equal(t, "sub-123", arg.ExternalSubject)
				return sqlc.IdentityUserLogin{UserID: arg.UserID}, nil
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

	t.Run("existing login resolves to its user", func(t *testing.T) {
		svc, mq := newService(t)
		// A linked (non-primary) login still lands on the owning user;
		// the user row keeps its primary provider claims.
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).Return(sqlc.IdentityUser{
			UserID:          42,
			Provider:        "google",
			ExternalSubject: "g-sub",
			Email:           "a@b.com",
			DisplayName:     pgtype.Text{String: "Alice", Valid: true},
			IsActive:        true,
			Role:            RoleMember,
		}, nil)
		expectLogin(mq)
		mq.EXPECT().TouchUserLogin(ctx, int64(42)).Return(int64(1), nil)

		got, err := svc.UpsertUser(ctx, "discord", "d-sub", "x@y.com", "Al")
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.UserID)
		assert.Equal(t, "google", got.Provider)
	})

	t.Run("primary login refreshes the user row", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).Return(sqlc.IdentityUser{
			UserID:          42,
			Provider:        "entra",
			ExternalSubject: "sub-123",
			Role:            RoleMember,
		}, nil)
		expectLogin(mq)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).Return(sqlc.IdentityUser{UserID: 42}, nil)
		mq.EXPECT().GetUserByID(ctx, int64(42)).Return(sqlc.IdentityUser{
			UserID: 42, Provider: "entra", ExternalSubject: "sub-123", Role: RoleMember,
		}, nil)

		got, err := svc.UpsertUser(ctx, "entra", "sub-123", "a@b.com", "Alice")
		require.NoError(t, err)
		assert.Equal(t, int64(42), got.UserID)
	})

	t.Run("empty display name becomes null", func(t *testing.T) {
		svc, mq := newService(t)
		missLogin(mq)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpsertUserParams) (sqlc.IdentityUser, error) {
				assert.False(t, arg.DisplayName.Valid)
				return sqlc.IdentityUser{UserID: 43, Provider: arg.Provider, Email: arg.Email, Role: RoleMember}, nil
			})
		expectLogin(mq)

		got, err := svc.UpsertUser(ctx, "entra", "sub-124", "b@c.com", "")
		require.NoError(t, err)
		assert.Equal(t, int64(43), got.UserID)
		assert.Equal(t, "", got.DisplayName)
		assert.Equal(t, RoleMember, got.Role)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		missLogin(mq)
		mq.EXPECT().UpsertUser(ctx, gomock.Any()).Return(sqlc.IdentityUser{}, errDB)

		_, err := svc.UpsertUser(ctx, "entra", "sub-123", "a@b.com", "Alice")
		require.Error(t, err)
		assert.ErrorContains(t, err, "upsert user")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestLinkLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("inserts the login", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).
			Return(sqlc.IdentityUser{}, pgx.ErrNoRows)
		mq.EXPECT().InsertLogin(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.InsertLoginParams) (sqlc.IdentityUserLogin, error) {
				assert.Equal(t, int64(42), arg.UserID)
				assert.Equal(t, "discord", arg.Provider)
				return sqlc.IdentityUserLogin{}, nil
			})

		require.NoError(t, svc.LinkLogin(ctx, 42, "discord", "d-sub", "x@y.com", "Al"))
	})

	t.Run("re-linking own login is a no-op", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).Return(sqlc.IdentityUser{
			UserID: 42,
		}, nil)
		require.NoError(t, svc.LinkLogin(ctx, 42, "discord", "d-sub", "x@y.com", "Al"))
	})

	t.Run("login bound to another user is ErrLoginTaken", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).
			Return(sqlc.IdentityUser{UserID: 99}, nil)

		err := svc.LinkLogin(ctx, 42, "discord", "d-sub", "x@y.com", "Al")
		assert.ErrorIs(t, err, ErrLoginTaken)
	})

	t.Run("insert race still reports ErrLoginTaken", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).
			Return(sqlc.IdentityUser{}, pgx.ErrNoRows)
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "user_login_provider_key"}
		mq.EXPECT().InsertLogin(ctx, gomock.Any()).Return(sqlc.IdentityUserLogin{}, pgErr)

		err := svc.LinkLogin(ctx, 42, "discord", "d-sub", "x@y.com", "Al")
		assert.ErrorIs(t, err, ErrLoginTaken)
	})

	t.Run("db error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByLogin(ctx, gomock.Any()).
			Return(sqlc.IdentityUser{}, pgx.ErrNoRows)
		mq.EXPECT().InsertLogin(ctx, gomock.Any()).Return(sqlc.IdentityUserLogin{}, errDB)

		err := svc.LinkLogin(ctx, 42, "discord", "d-sub", "x@y.com", "Al")
		assert.ErrorContains(t, err, "link login")
	})
}

func TestUnlinkLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("removes the provider login", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(42)).Return(sqlc.IdentityUser{
			UserID: 42, Provider: "google",
		}, nil)
		mq.EXPECT().CountLoginsByUser(ctx, int64(42)).Return(int64(2), nil)
		mq.EXPECT().DeleteLoginsByProvider(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.DeleteLoginsByProviderParams) (int64, error) {
				assert.Equal(t, "discord", arg.Provider)
				return int64(1), nil
			})

		require.NoError(t, svc.UnlinkLogin(ctx, 42, "discord"))
	})

	t.Run("last login cannot be removed", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(42)).Return(sqlc.IdentityUser{
			UserID: 42, Provider: "google",
		}, nil)
		mq.EXPECT().CountLoginsByUser(ctx, int64(42)).Return(int64(1), nil)

		assert.ErrorIs(t, svc.UnlinkLogin(ctx, 42, "discord"), ErrLastLogin)
	})

	t.Run("primary login cannot be removed", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(42)).Return(sqlc.IdentityUser{
			UserID: 42, Provider: "google",
		}, nil)

		assert.ErrorIs(t, svc.UnlinkLogin(ctx, 42, "google"), ErrPrimaryLogin)
	})

	t.Run("unknown provider is not found", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(42)).Return(sqlc.IdentityUser{
			UserID: 42, Provider: "google",
		}, nil)
		mq.EXPECT().CountLoginsByUser(ctx, int64(42)).Return(int64(2), nil)
		mq.EXPECT().DeleteLoginsByProvider(ctx, gomock.Any()).Return(int64(0), nil)

		assert.ErrorIs(t, svc.UnlinkLogin(ctx, 42, "discord"), domainerr.ErrNotFound)
	})
}

func TestListLogins(t *testing.T) {
	ctx := context.Background()

	t.Run("maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().ListLoginsByUser(ctx, int64(42)).Return([]sqlc.IdentityUserLogin{
			{Provider: "google", Email: "a@b.com", DisplayName: pgtype.Text{String: "Alice", Valid: true}},
			{Provider: "discord", Email: "x@y.com"},
		}, nil)

		got, err := svc.ListLogins(ctx, 42)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "google", got[0].Provider)
		assert.Equal(t, "Alice", got[0].DisplayName)
	})
}

// --- test doubles (mirror internal/session/service_test.go) ---

type stubTx struct{ closed bool }

func (f *stubTx) Begin(context.Context) (pgx.Tx, error) { return nil, errors.New("nested tx") }
func (f *stubTx) Commit(context.Context) error          { f.closed = true; return nil }
func (f *stubTx) Rollback(context.Context) error        { f.closed = true; return nil }
func (f *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (f *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (f *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (f *stubTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
func (f *stubTx) Conn() *pgx.Conn                                  { return nil }

type stubPool struct{ tx *stubTx }

func (p *stubPool) Begin(context.Context) (pgx.Tx, error) { return p.tx, nil }
func (p *stubPool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not implemented")
}
func (p *stubPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (p *stubPool) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

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
		mq.EXPECT().SetUserRole(ctx, sqlc.SetUserRoleParams{UserID: 42, Role: RoleAdmin}).Return(int64(1), nil)

		err := svc.SetUserRole(ctx, 42, RoleAdmin)
		require.NoError(t, err)
	})

	t.Run("error is wrapped", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SetUserRole(ctx, gomock.Any()).Return(int64(0), errDB)

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
		mq.EXPECT().ConditionalSetUserRole(ctx, sqlc.ConditionalSetUserRoleParams{UserID: 2, Role: RoleAdmin}).Return(int64(1), nil)

		require.NoError(t, svc.AdminSetRole(ctx, 1, 2, RoleAdmin))
	})

	t.Run("demoting an admin requires another active admin", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(adminRow(2, "a2@b.com"), nil)
		mq.EXPECT().ConditionalSetUserRole(ctx, sqlc.ConditionalSetUserRoleParams{UserID: 2, Role: RoleMember}).Return(int64(1), nil)

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
		mq.EXPECT().ConditionalSetUserRole(ctx, sqlc.ConditionalSetUserRoleParams{UserID: 2, Role: RoleMember}).Return(int64(0), nil)

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
		mq.EXPECT().ConditionalSetUserActive(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.ConditionalSetUserActiveParams) (int64, error) {
				assert.Equal(t, int64(2), arg.UserID)
				assert.False(t, arg.IsActive)
				assert.Equal(t, pgtype.Text{String: "admin@b.com", Valid: true}, arg.UpdatedBy)
				return 1, nil
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
		mq.EXPECT().ConditionalSetUserActive(ctx, gomock.Any()).Return(int64(0), nil)

		err := svc.AdminSetActive(ctx, 1, 2, false, "a@b.com")
		assert.ErrorIs(t, err, ErrLastAdmin)
	})

	t.Run("unban has no admin guard", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().GetUserByID(ctx, int64(2)).Return(memberRow(2, "m@b.com"), nil)
		mq.EXPECT().ConditionalSetUserActive(ctx, gomock.Any()).Return(int64(1), nil)

		require.NoError(t, svc.AdminSetActive(ctx, 1, 2, true, "admin@b.com"))
	})
}

func TestUpdateProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("passes profile fields", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateUserProfile(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpdateUserProfileParams) (int64, error) {
				assert.Equal(t, int64(7), arg.UserID)
				assert.Equal(t, pgtype.Text{String: "Ada", Valid: true}, arg.FirstName)
				assert.Equal(t, pgtype.Text{String: "Lovelace", Valid: true}, arg.LastName)
				assert.Equal(t, pgtype.Text{String: "alt@b.com", Valid: true}, arg.BackupEmail)
				assert.Equal(t, pgtype.Text{String: "a@b.com", Valid: true}, arg.UpdatedBy)
				return 1, nil
			})

		require.NoError(t, svc.UpdateProfile(ctx, 7, "Ada", "Lovelace", "alt@b.com", nil, "a@b.com"))
	})

	t.Run("birthdate is stored", func(t *testing.T) {
		svc, mq := newService(t)
		bd := time.Date(1990, 5, 4, 0, 0, 0, 0, time.UTC)
		mq.EXPECT().UpdateUserProfile(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpdateUserProfileParams) (int64, error) {
				assert.True(t, arg.Birthdate.Valid)
				assert.Equal(t, bd, arg.Birthdate.Time)
				return 1, nil
			})

		require.NoError(t, svc.UpdateProfile(ctx, 7, "Ada", "Lovelace", "", &bd, "a@b.com"))
	})

	t.Run("empty fields become null", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().UpdateUserProfile(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.UpdateUserProfileParams) (int64, error) {
				assert.False(t, arg.FirstName.Valid)
				assert.False(t, arg.BackupEmail.Valid)
				assert.False(t, arg.Birthdate.Valid)
				return 1, nil
			})

		require.NoError(t, svc.UpdateProfile(ctx, 7, "", "Lovelace", "", nil, "a@b.com"))
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
	assert.False(t, svc.IsProtected("https://issuer.example.com", "a@b.com"))

	svc = svc.WithProtectedEmails([]string{"https://issuer.example.com:Boss@Example.com", "", "https://issuer.example.com:second@b.com"})
	assert.True(t, svc.IsProtected("https://issuer.example.com", "boss@example.com"))
	assert.True(t, svc.IsProtected("https://issuer.example.com", "BOSS@EXAMPLE.COM"))
	assert.True(t, svc.IsProtected("https://issuer.example.com", "second@b.com"))
	assert.False(t, svc.IsProtected("https://other.example.com", "boss@example.com"))
	assert.False(t, svc.IsProtected("https://issuer.example.com", "other@b.com"))
}

func TestHouseholdQueries(t *testing.T) {
	ctx := context.Background()

	t.Run("set household success", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SetUserHousehold(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.SetUserHouseholdParams) (int64, error) {
				assert.Equal(t, int64(2), arg.UserID)
				assert.Equal(t, pgtype.Int8{Int64: 10, Valid: true}, arg.HouseholdID)
				assert.Equal(t, pgtype.Int8{Int64: 2, Valid: true}, arg.ExpectedHouseholdID)
				assert.Equal(t, HouseholdRoleMember, arg.HouseholdRole)
				return 1, nil
			})
		exp := int64(2)
		require.NoError(t, svc.SetUserHousehold(ctx, 2, 10, HouseholdRoleMember, &exp))
	})

	t.Run("set household stale expectation is a conflict", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SetUserHousehold(ctx, gomock.Any()).Return(int64(0), nil)
		err := svc.SetUserHousehold(ctx, 2, 10, HouseholdRoleMember, nil)
		assert.ErrorIs(t, err, domainerr.ErrConflict)
	})

	t.Run("set searchable missing user is not found", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SetUserSearchable(ctx, gomock.Any()).Return(int64(0), nil)
		err := svc.SetUserSearchable(ctx, 99, false, "a@b.com")
		assert.ErrorIs(t, err, domainerr.ErrNotFound)
	})

	t.Run("search users escapes wildcards and maps rows", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SearchUsers(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.SearchUsersParams) ([]sqlc.IdentityUser, error) {
				assert.Equal(t, int64(1), arg.ExcludeUserID)
				assert.Equal(t, pgtype.Int8{Int64: 5, Valid: true}, arg.ExcludeHouseholdID)
				assert.Equal(t, pgtype.Text{String: `%a\%b\_c%`, Valid: true}, arg.Pattern)
				assert.Equal(t, int32(20), arg.RowLimit)
				return []sqlc.IdentityUser{{UserID: 3, Email: "c@d.com"}}, nil
			})
		users, err := svc.SearchUsers(ctx, `a%b_c`, 1, 5, 20)
		require.NoError(t, err)
		require.Len(t, users, 1)
		assert.Equal(t, "c@d.com", users[0].Email)
	})

	t.Run("search users no household exclusion", func(t *testing.T) {
		svc, mq := newService(t)
		mq.EXPECT().SearchUsers(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, arg sqlc.SearchUsersParams) ([]sqlc.IdentityUser, error) {
				assert.False(t, arg.ExcludeHouseholdID.Valid)
				return nil, nil
			})
		_, err := svc.SearchUsers(ctx, "x", 1, 0, 20)
		require.NoError(t, err)
	})

	t.Run("list by ids empty short-circuits", func(t *testing.T) {
		svc, _ := newService(t)
		users, err := svc.ListUsersByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, users)
	})
}
