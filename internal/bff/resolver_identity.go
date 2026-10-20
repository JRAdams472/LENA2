package bff

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

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
	page, pageSize := pageArgs(args.Page, args.PageSize)
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
		out[i] = &userResolver{u: u, protected: r.IdentityService.IsProtected(u.Provider, u.Email), root: r}
	}
	return &userPageResolver{items: out, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// PendingUsers returns accounts awaiting admin approval, oldest first.
// Admin-only.
func (r *Resolver) PendingUsers(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*userPageResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	page, pageSize := pageArgs(args.Page, args.PageSize)
	users, err := r.IdentityService.ListPendingUsers(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.IdentityService.CountPendingUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*userResolver, len(users))
	for i, u := range users {
		out[i] = &userResolver{u: u, protected: r.IdentityService.IsProtected(u.Provider, u.Email), root: r}
	}
	return &userPageResolver{items: out, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// ApproveUser grants a pending (or denied) account member access.
// Admin-only; same guards as SetUserRole.
func (r *Resolver) ApproveUser(ctx context.Context, args struct {
	UserID graphql.ID
}) (*userResolver, error) {
	return r.setUserApproval(ctx, args.UserID, identity.StatusApproved, "approve_user")
}

// DenyUser records a durable denial — the account cannot self-retry into
// access. Admin-only; same guards as SetUserRole.
func (r *Resolver) DenyUser(ctx context.Context, args struct {
	UserID graphql.ID
}) (*userResolver, error) {
	return r.setUserApproval(ctx, args.UserID, identity.StatusDenied, "deny_user")
}

// setUserApproval is the shared approve/deny mutation path.
func (r *Resolver) setUserApproval(ctx context.Context, userID graphql.ID, status, action string) (*userResolver, error) {
	actor, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	targetID, err := parseID(string(userID))
	if err != nil {
		return nil, err
	}
	if err := r.IdentityService.AdminSetApproval(ctx, actor.UserID, targetID, status, actor.Email); err != nil {
		return nil, mapAdminGuardError(err)
	}
	// Evict the cached identity so the decision applies on the target's
	// next request instead of up to userCacheTTL later.
	r.invalidateUserID(ctx, targetID)
	slog.Default().Info("audit",
		"action", action,
		"actor", actor.Email,
		"target_id", targetID,
	)
	return r.userByID(ctx, targetID)
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
	// Evict the cached identity so a demotion applies on the target's next
	// request instead of up to userCacheTTL later.
	r.invalidateUserID(ctx, targetID)
	slog.Default().Info("audit",
		"action", "set_user_role",
		"actor", actor.Email,
		"target_id", targetID,
		"role", args.Role,
	)
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
	// Evict the cached identity so a ban applies on the target's next
	// request instead of up to userCacheTTL later.
	r.invalidateUserID(ctx, targetID)
	slog.Default().Info("audit",
		"action", "set_user_active",
		"actor", actor.Email,
		"target_id", targetID,
		"is_active", args.IsActive,
	)
	return r.userByID(ctx, targetID)
}

// updateProfileInput mirrors the UpdateProfileInput GraphQL input type.
// A nil field leaves the stored value unchanged; an empty string clears it.
type updateProfileInput struct {
	FirstName    *string
	LastName     *string
	BackupEmail  *string
	Birthdate    *string
	IsSearchable *bool
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
	first, err := profileNameField(args.Input.FirstName, current.FirstName, "firstName")
	if err != nil {
		return nil, err
	}
	last, err := profileNameField(args.Input.LastName, current.LastName, "lastName")
	if err != nil {
		return nil, err
	}
	backup, err := profileBackupEmail(args.Input.BackupEmail, current.BackupEmail)
	if err != nil {
		return nil, err
	}
	birthdate, err := profileBirthdate(args.Input.Birthdate, current.Birthdate)
	if err != nil {
		return nil, err
	}
	if err := r.IdentityService.UpdateProfile(ctx, actor.UserID, first, last, backup, birthdate, actor.Email); err != nil {
		return nil, err
	}
	// Searchability lives on the identity row but is cached in the
	// authenticator's user resolution — invalidate on change so the next
	// request picks it up.
	if args.Input.IsSearchable != nil && *args.Input.IsSearchable != current.IsSearchable {
		if err := r.IdentityService.SetUserSearchable(ctx, actor.UserID, *args.Input.IsSearchable, actor.Email); err != nil {
			return nil, err
		}
		r.invalidateUser(ctx, actor)
	}
	return r.userByID(ctx, actor.UserID)
}

// profileNameField normalizes an optional name override; a nil input keeps
// the stored value.
func profileNameField(input *string, current, field string) (string, error) {
	if input == nil {
		return current, nil
	}
	v := strings.TrimSpace(*input)
	if len(v) > 100 {
		return "", badInputf("%s must be at most 100 characters", field)
	}
	return v, nil
}

// profileBackupEmail normalizes an optional backup-email override; a nil
// input keeps the stored value and an empty string clears it.
func profileBackupEmail(input *string, current string) (string, error) {
	if input == nil {
		return current, nil
	}
	v := strings.TrimSpace(*input)
	if v == "" {
		return v, nil
	}
	if len(v) > 320 {
		return "", badInputf("backupEmail must be at most 320 characters")
	}
	if _, err := mail.ParseAddress(v); err != nil {
		return "", badInputf("backupEmail is not a valid email address")
	}
	return v, nil
}

// profileBirthdate parses an optional birthdate override; a nil input
// keeps the stored value and an empty string clears it.
func profileBirthdate(input *string, current *time.Time) (*time.Time, error) {
	if input == nil {
		return current, nil
	}
	raw := strings.TrimSpace(*input)
	if raw == "" {
		return nil, nil
	}
	bd, err := time.Parse(time.DateOnly, raw)
	if err != nil || bd.Format(time.DateOnly) != raw {
		return nil, badInputf("birthdate must be YYYY-MM-DD")
	}
	if bd.After(time.Now()) || bd.Year() < 1900 {
		return nil, badInputf("birthdate is out of range")
	}
	return &bd, nil
}

// DeleteMyAccount permanently deletes the caller's account and every
// row they personally own, in one transaction (LEN-146). The identity row
// cascades sessions, linked logins, device/push tokens, notifications,
// ratings, preferences, memberships, and invites; households the caller
// alone belongs to are dissolved with their pantry, cellar, plans, lists,
// and events. Shared-household content and catalog content (approved
// recipes, submitted items/brands) survive — attribution columns SET
// NULL. Ownership of a shared household passes to the earliest remaining
// member, matching leaveHousehold. The last-active-admin invariant blocks
// a sole administrator.
func (r *Resolver) DeleteMyAccount(ctx context.Context) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	var promoted []int64
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		// Classify memberships before the identity row (and its member
		// rows) disappears: sole-member households are dissolved below.
		memberships, err := r.HouseholdService.ListMembershipsByUser(ctx, u.UserID)
		if err != nil {
			return err
		}
		sole := make([]int64, 0, len(memberships))
		for _, m := range memberships {
			// Lock first — a concurrent invite-accept must not turn a
			// household multi-member after we counted it as sole.
			if _, err := r.HouseholdService.LockHousehold(ctx, m.HouseholdID); err != nil {
				return err
			}
			members, err := r.HouseholdService.ListMembersByHousehold(ctx, m.HouseholdID)
			if err != nil {
				return err
			}
			if len(members) == 1 {
				sole = append(sole, m.HouseholdID)
				continue
			}
			// A shared household whose owner deletes their account would
			// otherwise be left with nobody able to manage members — hand
			// ownership to the earliest admin/member, like leaveHousehold.
			id, err := r.promoteSuccessor(ctx, u, m.Role, members, m.HouseholdID)
			if err != nil {
				return err
			}
			if id != 0 {
				promoted = append(promoted, id)
			}
		}
		if err := r.IdentityService.DeleteUser(ctx, u.UserID); err != nil {
			return err
		}
		for _, hid := range sole {
			if err := r.UserPrefsService.DeleteHouseholdStock(ctx, hid); err != nil {
				return err
			}
			if err := r.MealPlanService.DeleteHouseholdPlans(ctx, hid); err != nil {
				return err
			}
			if err := r.GroceryService.DeleteHouseholdLists(ctx, hid); err != nil {
				return err
			}
			if err := r.HouseholdService.DeleteHousehold(ctx, hid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, mapAdminGuardError(err)
	}
	// Evict the cached identity so outstanding access tokens stop
	// resolving immediately instead of living out the cache TTL.
	r.invalidateUser(ctx, u)
	for _, id := range promoted {
		r.invalidateUserID(ctx, id)
	}
	slog.Default().Info("audit",
		"action", "delete_my_account",
		"actor", u.Email,
		"user_id", u.UserID,
	)
	return true, nil
}

// AcceptDisclaimer records the caller's acceptance of the medical/allergen
// disclaimer (LEN-31). Idempotent; allergy records can't be written until
// this has run once.
func (r *Resolver) AcceptDisclaimer(ctx context.Context) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	if err := r.IdentityService.AcceptDisclaimer(ctx, u.UserID, u.Email); err != nil {
		return false, err
	}
	r.invalidateUser(ctx, u)
	slog.Default().Info("audit",
		"action", "accept_disclaimer",
		"actor", u.Email,
		"user_id", u.UserID,
	)
	return true, nil
}

// userByID reloads a user and wraps it in a resolver, marking protected
// status from the identity service's configured list.
func (r *Resolver) userByID(ctx context.Context, userID int64) (*userResolver, error) {
	u, err := r.IdentityService.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &userResolver{u: u, protected: r.IdentityService.IsProtected(u.Provider, u.Email), root: r}, nil
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
