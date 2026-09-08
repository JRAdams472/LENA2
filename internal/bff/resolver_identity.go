package bff

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/identity"
)

// Users returns a paged list of all registered users. Admin-only.
func (r *Resolver) Users(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userPageResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	users, err := r.IdentityService.ListUsers(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.IdentityService.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*userResolver, len(users))
	for i, u := range users {
		out[i] = &userResolver{u: u, protected: r.IdentityService.IsProtected(u.Email)}
	}
	return &userPageResolver{items: out, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// SetUserRole changes another user's role. Admin-only; rejects
// self-modification, protected admins, and removing the last active admin.
func (r *Resolver) SetUserRole(ctx context.Context, args struct {
	UserID graphql.ID
	Role   string
}) (*userResolver, error) {
	actor, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	targetID, err := parseID(string(args.UserID))
	if err != nil {
		return nil, err
	}
	if args.Role != identity.RoleMember && args.Role != identity.RoleAdmin {
		return nil, badInputf("invalid role %q: must be %q or %q", args.Role, identity.RoleMember, identity.RoleAdmin)
	}
	if err := r.IdentityService.AdminSetRole(ctx, actor.UserID, targetID, args.Role); err != nil {
		return nil, mapAdminGuardError(err)
	}
	return r.userByID(ctx, targetID)
}

// SetUserActive bans (isActive=false) or unbans another user. Admin-only
// with the same guards as SetUserRole.
func (r *Resolver) SetUserActive(ctx context.Context, args struct {
	UserID   graphql.ID
	IsActive bool
}) (*userResolver, error) {
	actor, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	targetID, err := parseID(string(args.UserID))
	if err != nil {
		return nil, err
	}
	if err := r.IdentityService.AdminSetActive(ctx, actor.UserID, targetID, args.IsActive, actor.Email); err != nil {
		return nil, mapAdminGuardError(err)
	}
	return r.userByID(ctx, targetID)
}

// updateProfileInput mirrors the UpdateProfileInput GraphQL input type.
// A nil field leaves the stored value unchanged; an empty string clears it.
type updateProfileInput struct {
	FirstName   *string
	LastName    *string
	BackupEmail *string
}

// UpdateMyProfile updates the caller's own profile fields.
func (r *Resolver) UpdateMyProfile(ctx context.Context, args struct {
	Input updateProfileInput
}) (*userResolver, error) {
	actor, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	current, err := r.IdentityService.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	first, last, backup := current.FirstName, current.LastName, current.BackupEmail
	if args.Input.FirstName != nil {
		first = strings.TrimSpace(*args.Input.FirstName)
		if len(first) > 100 {
			return nil, badInputf("firstName must be at most 100 characters")
		}
	}
	if args.Input.LastName != nil {
		last = strings.TrimSpace(*args.Input.LastName)
		if len(last) > 100 {
			return nil, badInputf("lastName must be at most 100 characters")
		}
	}
	if args.Input.BackupEmail != nil {
		backup = strings.TrimSpace(*args.Input.BackupEmail)
		if backup != "" {
			if len(backup) > 320 {
				return nil, badInputf("backupEmail must be at most 320 characters")
			}
			if _, err := mail.ParseAddress(backup); err != nil {
				return nil, badInputf("backupEmail is not a valid email address")
			}
		}
	}
	if err := r.IdentityService.UpdateProfile(ctx, actor.UserID, first, last, backup, actor.Email); err != nil {
		return nil, err
	}
	return r.userByID(ctx, actor.UserID)
}

// userByID reloads a user and wraps it in a resolver, marking protected
// status from the identity service's configured list.
func (r *Resolver) userByID(ctx context.Context, userID int64) (*userResolver, error) {
	u, err := r.IdentityService.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &userResolver{u: u, protected: r.IdentityService.IsProtected(u.Email)}, nil
}

// mapAdminGuardError translates identity guard failures into client-safe
// GraphQL errors; anything else is an internal error.
func mapAdminGuardError(err error) error {
	switch {
	case errors.Is(err, identity.ErrSelfModification),
		errors.Is(err, identity.ErrProtectedUser),
		errors.Is(err, identity.ErrLastAdmin):
		return &clientError{msg: "forbidden: " + err.Error(), code: codeForbidden}
	default:
		return err
	}
}

type userPageResolver struct {
	items    []*userResolver
	page     int32
	pageSize int32
	total    int32
}

func (r *userPageResolver) Items() []*userResolver { return r.items }

func (r *userPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}
