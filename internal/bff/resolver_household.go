package bff

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

// maxHouseholdMembers caps household size; the invite path pre-checks it
// and accept re-checks under the household row lock so a race can never
// push past it.
const maxHouseholdMembers = 10

// MyHousehold resolves the caller's household with its members; nil when
// the caller has none (should not occur once the authenticator ensures a
// default household, but the query must not error on it).
func (r *Resolver) MyHousehold(ctx context.Context) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.HouseholdID == 0 {
		return nil, nil
	}
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID)
}

// householdWithMembers loads a household and its members in two queries.
// viewerID marks the caller in the member list and resolves myRole from
// the DB rows rather than the (possibly stale) auth cache.
func (r *Resolver) householdWithMembers(ctx context.Context, householdID, viewerID int64) (*householdResolver, error) {
	hh, err := r.HouseholdService.GetHouseholdByID(ctx, householdID)
	if err != nil {
		return nil, err
	}
	members, err := r.IdentityService.ListUsersByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	out := &householdResolver{hh: hh}
	for _, m := range members {
		if m.UserID == viewerID {
			out.myRole = m.HouseholdRole
		}
		out.members = append(out.members, &householdMemberResolver{
			u:    m,
			isMe: m.UserID == viewerID,
		})
	}
	return out, nil
}

// HouseholdInvites returns pending invites addressed to the caller plus
// pending invites the caller sent (so they can be cancelled).
func (r *Resolver) HouseholdInvites(ctx context.Context) ([]*householdInviteResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	incoming, err := r.HouseholdService.ListPendingInvitesForUser(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	sent, err := r.HouseholdService.ListSentInvitesForUser(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	return r.hydrateInvites(ctx, append(incoming, sent...))
}

// hydrateInvites batch-loads both parties of every invite in one
// ListUsersByIDs call — no per-row queries.
func (r *Resolver) hydrateInvites(ctx context.Context, invites []household.Invite) ([]*householdInviteResolver, error) {
	if len(invites) == 0 {
		return nil, nil
	}
	idSet := make(map[int64]struct{}, len(invites)*2)
	for _, inv := range invites {
		idSet[inv.FromUserID] = struct{}{}
		idSet[inv.ToUserID] = struct{}{}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	users, err := r.IdentityService.ListUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]identity.User, len(users))
	for _, u := range users {
		byID[u.UserID] = u
	}
	out := make([]*householdInviteResolver, 0, len(invites))
	for _, inv := range invites {
		out = append(out, &householdInviteResolver{
			inv:  inv,
			from: &householdUserResolver{u: byID[inv.FromUserID]},
			to:   &householdUserResolver{u: byID[inv.ToUserID]},
		})
	}
	return out, nil
}

// SearchHouseholdUsers finds opted-in, active users matching a name or
// email term. The caller and current household members are excluded;
// results are the restricted HouseholdUser projection.
func (r *Resolver) SearchHouseholdUsers(ctx context.Context, args struct {
	Term  string
	Limit int32
}) ([]*householdUserResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	term := strings.TrimSpace(args.Term)
	if len(term) < 2 {
		return nil, badInputf("term must be at least 2 characters")
	}
	limit := clamp(args.Limit, 1, 50)
	users, err := r.IdentityService.SearchUsers(ctx, term, u.UserID, u.HouseholdID, limit)
	if err != nil {
		return nil, err
	}
	return householdUserResolvers(users), nil
}

// InviteHouseholdMember creates a pending invite from the caller to the
// target user. Requires owner or admin; self-invites, existing household
// mates, and full households are rejected; a duplicate pending invite
// surfaces as CONFLICT.
func (r *Resolver) InviteHouseholdMember(ctx context.Context, args struct {
	UserID graphql.ID
}) (*householdInviteResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireHouseholdRole(u, identity.HouseholdRoleOwner, identity.HouseholdRoleAdmin); err != nil {
		return nil, err
	}
	targetID, err := parseID(string(args.UserID))
	if err != nil {
		return nil, err
	}
	if targetID == u.UserID {
		return nil, badInputf("cannot invite yourself")
	}
	if u.HouseholdID == 0 {
		return nil, badInputf("no household to invite into")
	}
	target, err := r.IdentityService.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.HouseholdID != nil && *target.HouseholdID == u.HouseholdID {
		return nil, badInputf("user is already in your household")
	}
	if n, err := r.IdentityService.CountUsersByHousehold(ctx, u.HouseholdID); err != nil {
		return nil, err
	} else if n >= maxHouseholdMembers {
		return nil, badInputf("household is full")
	}
	var inv household.Invite
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		inv, err = r.HouseholdService.CreateInvite(ctx, u.UserID, targetID, u.HouseholdID, u.Email)
		if err != nil {
			return err
		}
		return r.notify(ctx, []int64{targetID}, household.KindInviteReceived, u.HouseholdID, u.UserID, &inv.InviteID)
	})
	if err != nil {
		return nil, err
	}
	out, err := r.hydrateInvites(ctx, []household.Invite{inv})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// AcceptHouseholdInvite moves the caller into the inviter's household and
// merges the caller's default-household data (pantry, cellar, meal plans,
// grocery lists) in a single transaction.
func (r *Resolver) AcceptHouseholdInvite(ctx context.Context, args struct {
	InviteID graphql.ID
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inviteID, err := parseID(string(args.InviteID))
	if err != nil {
		return nil, err
	}
	inv, err := r.HouseholdService.GetInviteByID(ctx, inviteID)
	if err != nil {
		return nil, err
	}
	if inv.ToUserID != u.UserID {
		// Non-party access must not leak that the invite exists.
		return nil, domainerr.ErrNotFound
	}
	if err := r.acceptInvite(ctx, u, inv); err != nil {
		return nil, err
	}
	r.invalidateUser(ctx, u)
	r.invalidateUserID(ctx, inv.FromUserID)
	return r.householdWithMembers(ctx, inv.HouseholdID, u.UserID)
}

// acceptInvite runs the invite-accept orchestration in one unit of work:
// lock the household row (serializing concurrent accepts/leaves/removals
// and the member cap), transition the invite, merge the caller's
// household data into the inviter's household, move the caller as a
// member with an expected-value guard, then write notifications.
func (r *Resolver) acceptInvite(ctx context.Context, u currentuser.User, inv household.Invite) error {
	return r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.HouseholdService.LockHousehold(ctx, inv.HouseholdID); err != nil {
			return err
		}
		if n, err := r.IdentityService.CountUsersByHousehold(ctx, inv.HouseholdID); err != nil {
			return err
		} else if n >= maxHouseholdMembers {
			return badInputf("household is full")
		}
		inviter, err := r.IdentityService.GetByID(ctx, inv.FromUserID)
		if err != nil {
			return err
		}
		if inviter.HouseholdID == nil || *inviter.HouseholdID != inv.HouseholdID {
			return domainerr.ErrConflict
		}
		target, err := r.IdentityService.GetByID(ctx, u.UserID)
		if err != nil {
			return err
		}
		if _, err := r.HouseholdService.TransitionInvite(ctx, inv.InviteID, household.StatusAccepted, u.Email); err != nil {
			return err
		}
		if target.HouseholdID != nil && *target.HouseholdID != inv.HouseholdID {
			if err := r.UserPrefsService.MergeHouseholdStock(ctx, *target.HouseholdID, inv.HouseholdID, u.Email); err != nil {
				return err
			}
			if err := r.MealPlanService.ReassignHousehold(ctx, *target.HouseholdID, inv.HouseholdID, u.Email); err != nil {
				return err
			}
			if err := r.GroceryService.ReassignHousehold(ctx, *target.HouseholdID, inv.HouseholdID, u.Email); err != nil {
				return err
			}
		}
		if err := r.IdentityService.SetUserHousehold(ctx, u.UserID, inv.HouseholdID, identity.HouseholdRoleMember, target.HouseholdID); err != nil {
			return err
		}
		if err := r.notify(ctx, []int64{inv.FromUserID}, household.KindInviteAccepted, inv.HouseholdID, u.UserID, &inv.InviteID); err != nil {
			return err
		}
		// Other members learn that someone joined; the inviter's accept
		// notification above already covers them.
		members, err := r.IdentityService.ListUsersByHousehold(ctx, inv.HouseholdID)
		if err != nil {
			return err
		}
		var others []int64
		for _, m := range members {
			if m.UserID != u.UserID && m.UserID != inv.FromUserID {
				others = append(others, m.UserID)
			}
		}
		return r.notify(ctx, others, household.KindMemberJoined, inv.HouseholdID, u.UserID, nil)
	})
}

// DeclineHouseholdInvite marks a received invite declined. Only the
// invitee may decline; non-party access returns NOT_FOUND.
func (r *Resolver) DeclineHouseholdInvite(ctx context.Context, args struct {
	InviteID graphql.ID
}) (*householdInviteResolver, error) {
	return r.transitionInvite(ctx, args.InviteID, household.StatusDeclined, func(u currentuser.User, inv household.Invite) bool {
		return inv.ToUserID == u.UserID
	}, func(ctx context.Context, u currentuser.User, inv household.Invite) error {
		return r.notify(ctx, []int64{inv.FromUserID}, household.KindInviteDeclined, inv.HouseholdID, u.UserID, &inv.InviteID)
	})
}

// CancelHouseholdInvite marks a sent invite cancelled. Only the inviter
// may cancel; non-party access returns NOT_FOUND.
func (r *Resolver) CancelHouseholdInvite(ctx context.Context, args struct {
	InviteID graphql.ID
}) (*householdInviteResolver, error) {
	return r.transitionInvite(ctx, args.InviteID, household.StatusCancelled, func(u currentuser.User, inv household.Invite) bool {
		return inv.FromUserID == u.UserID
	}, func(ctx context.Context, u currentuser.User, inv household.Invite) error {
		return r.notify(ctx, []int64{inv.ToUserID}, household.KindInviteCancelled, inv.HouseholdID, u.UserID, &inv.InviteID)
	})
}

// transitionInvite implements decline/cancel: fetch, party check, guarded
// transition (conflict when the invite was already resolved), side-effect
// notification, hydrate.
func (r *Resolver) transitionInvite(ctx context.Context, rawID graphql.ID, to household.Status, isParty func(currentuser.User, household.Invite) bool, effect func(context.Context, currentuser.User, household.Invite) error) (*householdInviteResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inviteID, err := parseID(string(rawID))
	if err != nil {
		return nil, err
	}
	var inv household.Invite
	var out household.Invite
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		inv, err = r.HouseholdService.GetInviteByID(ctx, inviteID)
		if err != nil {
			return err
		}
		if !isParty(u, inv) {
			return domainerr.ErrNotFound
		}
		out, err = r.HouseholdService.TransitionInvite(ctx, inviteID, to, u.Email)
		if err != nil {
			return err
		}
		return effect(ctx, u, inv)
	})
	if err != nil {
		return nil, err
	}
	resolved, err := r.hydrateInvites(ctx, []household.Invite{out})
	if err != nil {
		return nil, err
	}
	return resolved[0], nil
}

// LeaveHousehold moves the caller into a fresh single-person household as
// its owner. Their data stays with the household they left. When the
// caller was the owner and other members remain, ownership transfers to
// the earliest admin, else the earliest member.
func (r *Resolver) LeaveHousehold(ctx context.Context) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	var promoted int64
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		fresh, err := r.IdentityService.GetByID(ctx, u.UserID)
		if err != nil {
			return err
		}
		if fresh.HouseholdID == nil {
			return badInputf("no household to leave")
		}
		// The DB row is authoritative — the cached household id may lag a
		// recent move, so lock and operate on fresh.HouseholdID.
		oldHouseholdID := *fresh.HouseholdID
		if _, err := r.HouseholdService.LockHousehold(ctx, oldHouseholdID); err != nil {
			return err
		}
		members, err := r.IdentityService.ListUsersByHousehold(ctx, oldHouseholdID)
		if err != nil {
			return err
		}
		if fresh.HouseholdRole == identity.HouseholdRoleOwner && len(members) > 1 {
			successor := firstMemberWithRole(members, u.UserID, identity.HouseholdRoleAdmin)
			if successor == 0 {
				successor = firstMemberWithRole(members, u.UserID, identity.HouseholdRoleMember)
			}
			if successor != 0 {
				if err := r.IdentityService.SetUserHouseholdRole(ctx, successor, oldHouseholdID, identity.HouseholdRoleOwner, u.Email); err != nil {
					return err
				}
				promoted = successor
				if err := r.notify(ctx, []int64{successor}, household.KindRoleChanged, oldHouseholdID, u.UserID, nil); err != nil {
					return err
				}
			}
		}
		// Pending invites the caller sent for this household can no longer
		// be accepted against it — cancel and tell the recipients.
		cancelled, err := r.HouseholdService.CancelPendingInvitesFrom(ctx, u.UserID, oldHouseholdID, u.Email)
		if err != nil {
			return err
		}
		for _, c := range cancelled {
			if err := r.notify(ctx, []int64{c.ToUserID}, household.KindInviteCancelled, oldHouseholdID, u.UserID, &c.InviteID); err != nil {
				return err
			}
		}
		hh, err := r.HouseholdService.CreateHousehold(ctx, u.Email)
		if err != nil {
			return err
		}
		if err := r.IdentityService.SetUserHousehold(ctx, u.UserID, hh.HouseholdID, identity.HouseholdRoleOwner, fresh.HouseholdID); err != nil {
			return err
		}
		var remaining []int64
		for _, m := range members {
			if m.UserID != u.UserID {
				remaining = append(remaining, m.UserID)
			}
		}
		return r.notify(ctx, remaining, household.KindMemberLeft, oldHouseholdID, u.UserID, nil)
	})
	if err != nil {
		return false, err
	}
	r.invalidateUser(ctx, u)
	if promoted != 0 {
		r.invalidateUserID(ctx, promoted)
	}
	return true, nil
}

// RenameHousehold sets or clears the caller's household display name.
// Owner-only; other members are notified.
func (r *Resolver) RenameHousehold(ctx context.Context, args struct {
	Name string
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireHouseholdRole(u, identity.HouseholdRoleOwner); err != nil {
		return nil, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.HouseholdService.RenameHousehold(ctx, u.HouseholdID, args.Name, u.Email); err != nil {
			return err
		}
		members, err := r.IdentityService.ListUsersByHousehold(ctx, u.HouseholdID)
		if err != nil {
			return err
		}
		var others []int64
		for _, m := range members {
			if m.UserID != u.UserID {
				others = append(others, m.UserID)
			}
		}
		return r.notify(ctx, others, household.KindHouseholdRenamed, u.HouseholdID, u.UserID, nil)
	})
	if err != nil {
		return nil, err
	}
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID)
}

// SetHouseholdRole promotes a member to admin or demotes an admin to
// member. Owner-only; ownership itself moves via transferHouseholdOwnership.
func (r *Resolver) SetHouseholdRole(ctx context.Context, args struct {
	UserID graphql.ID
	Role   string
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireHouseholdRole(u, identity.HouseholdRoleOwner); err != nil {
		return nil, err
	}
	role := strings.ToLower(args.Role)
	if role != identity.HouseholdRoleAdmin && role != identity.HouseholdRoleMember {
		return nil, badInputf("role must be ADMIN or MEMBER; use transferHouseholdOwnership for OWNER")
	}
	targetID, err := parseID(string(args.UserID))
	if err != nil {
		return nil, err
	}
	if targetID == u.UserID {
		return nil, badInputf("cannot change your own role")
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.HouseholdService.LockHousehold(ctx, u.HouseholdID); err != nil {
			return err
		}
		target, err := r.householdMember(ctx, u.HouseholdID, targetID)
		if err != nil {
			return err
		}
		if target.HouseholdRole == identity.HouseholdRoleOwner {
			return badInputf("cannot change the owner's role")
		}
		if err := r.IdentityService.SetUserHouseholdRole(ctx, targetID, u.HouseholdID, role, u.Email); err != nil {
			return err
		}
		return r.notify(ctx, []int64{targetID}, household.KindRoleChanged, u.HouseholdID, u.UserID, nil)
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUserID(ctx, targetID)
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID)
}

// TransferHouseholdOwnership hands ownership to another member; the
// caller becomes a regular member. Owner-only.
func (r *Resolver) TransferHouseholdOwnership(ctx context.Context, args struct {
	UserID graphql.ID
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireHouseholdRole(u, identity.HouseholdRoleOwner); err != nil {
		return nil, err
	}
	targetID, err := parseID(string(args.UserID))
	if err != nil {
		return nil, err
	}
	if targetID == u.UserID {
		return nil, badInputf("cannot transfer ownership to yourself")
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.HouseholdService.LockHousehold(ctx, u.HouseholdID); err != nil {
			return err
		}
		if _, err := r.householdMember(ctx, u.HouseholdID, targetID); err != nil {
			return err
		}
		if err := r.IdentityService.SetUserHouseholdRole(ctx, targetID, u.HouseholdID, identity.HouseholdRoleOwner, u.Email); err != nil {
			return err
		}
		if err := r.IdentityService.SetUserHouseholdRole(ctx, u.UserID, u.HouseholdID, identity.HouseholdRoleMember, u.Email); err != nil {
			return err
		}
		return r.notify(ctx, []int64{targetID}, household.KindRoleChanged, u.HouseholdID, u.UserID, nil)
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUser(ctx, u)
	r.invalidateUserID(ctx, targetID)
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID)
}

// RemoveHouseholdMember moves a member into a fresh single-person
// household as its owner; their shared data stays behind. Owners remove
// admins and members; admins remove members only. The owner is never
// removable — they leave via leaveHousehold or transfer ownership first.
func (r *Resolver) RemoveHouseholdMember(ctx context.Context, args struct {
	UserID graphql.ID
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireHouseholdRole(u, identity.HouseholdRoleOwner, identity.HouseholdRoleAdmin); err != nil {
		return nil, err
	}
	targetID, err := parseID(string(args.UserID))
	if err != nil {
		return nil, err
	}
	if targetID == u.UserID {
		return nil, badInputf("use leaveHousehold to remove yourself")
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.HouseholdService.LockHousehold(ctx, u.HouseholdID); err != nil {
			return err
		}
		target, err := r.householdMember(ctx, u.HouseholdID, targetID)
		if err != nil {
			return err
		}
		if target.HouseholdRole == identity.HouseholdRoleOwner {
			return errForbidden()
		}
		if u.HouseholdRole == identity.HouseholdRoleAdmin && target.HouseholdRole != identity.HouseholdRoleMember {
			return errForbidden()
		}
		cancelled, err := r.HouseholdService.CancelPendingInvitesFrom(ctx, targetID, u.HouseholdID, u.Email)
		if err != nil {
			return err
		}
		for _, c := range cancelled {
			if err := r.notify(ctx, []int64{c.ToUserID}, household.KindInviteCancelled, u.HouseholdID, u.UserID, &c.InviteID); err != nil {
				return err
			}
		}
		fresh, err := r.HouseholdService.CreateHousehold(ctx, u.Email)
		if err != nil {
			return err
		}
		if err := r.IdentityService.SetUserHousehold(ctx, targetID, fresh.HouseholdID, identity.HouseholdRoleOwner, target.HouseholdID); err != nil {
			return err
		}
		if err := r.notify(ctx, []int64{targetID}, household.KindMemberRemoved, u.HouseholdID, u.UserID, nil); err != nil {
			return err
		}
		var remaining []int64
		members, err := r.IdentityService.ListUsersByHousehold(ctx, u.HouseholdID)
		if err != nil {
			return err
		}
		for _, m := range members {
			if m.UserID != u.UserID {
				remaining = append(remaining, m.UserID)
			}
		}
		// To the rest of the household a removal reads the same as a leave.
		return r.notify(ctx, remaining, household.KindMemberLeft, u.HouseholdID, targetID, nil)
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUserID(ctx, targetID)
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID)
}

// MyNotifications returns the caller's household notifications,
// newest first.
func (r *Resolver) MyNotifications(ctx context.Context, args struct {
	Limit int32
}) ([]*householdNotificationResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	notifs, err := r.HouseholdService.ListNotificationsForUser(ctx, u.UserID, clamp(args.Limit, 1, 100))
	if err != nil {
		return nil, err
	}
	return r.hydrateNotifications(ctx, notifs)
}

// UnreadNotificationCount backs the nav badge; clients poll it.
func (r *Resolver) UnreadNotificationCount(ctx context.Context) (int32, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return 0, err
	}
	n, err := r.HouseholdService.CountUnreadNotifications(ctx, u.UserID)
	if err != nil {
		return 0, err
	}
	if n > math.MaxInt32 {
		n = math.MaxInt32
	}
	//nolint:gosec // n is clamped to math.MaxInt32 immediately above.
	return int32(n), nil
}

// MarkAllNotificationsRead clears the caller's unread notifications.
func (r *Resolver) MarkAllNotificationsRead(ctx context.Context) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	if err := r.HouseholdService.MarkAllNotificationsRead(ctx, u.UserID); err != nil {
		return false, err
	}
	return true, nil
}

// hydrateNotifications batch-loads actors for a notification page in one
// ListUsersByIDs call.
func (r *Resolver) hydrateNotifications(ctx context.Context, notifs []household.Notification) ([]*householdNotificationResolver, error) {
	if len(notifs) == 0 {
		return nil, nil
	}
	idSet := make(map[int64]struct{}, len(notifs))
	for _, n := range notifs {
		if n.ActorUserID != nil {
			idSet[*n.ActorUserID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	users, err := r.IdentityService.ListUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]identity.User, len(users))
	for _, u := range users {
		byID[u.UserID] = u
	}
	out := make([]*householdNotificationResolver, 0, len(notifs))
	for _, n := range notifs {
		nr := &householdNotificationResolver{n: n}
		if n.ActorUserID != nil {
			if actor, ok := byID[*n.ActorUserID]; ok {
				nr.actor = &householdUserResolver{u: actor}
			}
		}
		out = append(out, nr)
	}
	return out, nil
}

// notify writes a notification row for each target inside the caller's
// transaction. A nil or empty target list is a no-op.
func (r *Resolver) notify(ctx context.Context, userIDs []int64, kind household.NotificationKind, householdID int64, actorID int64, inviteID *int64) error {
	for _, uid := range userIDs {
		hh := householdID
		var actor *int64
		if actorID != 0 {
			a := actorID
			actor = &a
		}
		if err := r.HouseholdService.CreateNotification(ctx, uid, kind, &hh, actor, inviteID); err != nil {
			return err
		}
	}
	return nil
}

// requireHouseholdRole gates mutations on the caller's household role.
// A member without the required role gets FORBIDDEN; the role is taken
// from the cached request identity and stale roles lose at the write
// guards inside the transaction.
func requireHouseholdRole(u currentuser.User, roles ...string) error {
	for _, role := range roles {
		if u.HouseholdRole == role {
			return nil
		}
	}
	return errForbidden()
}

// householdMember fetches a user that must currently belong to
// householdID; strangers get NOT_FOUND so membership is never leaked.
func (r *Resolver) householdMember(ctx context.Context, householdID, userID int64) (identity.User, error) {
	target, err := r.IdentityService.GetByID(ctx, userID)
	if err != nil {
		return identity.User{}, err
	}
	if target.HouseholdID == nil || *target.HouseholdID != householdID {
		return identity.User{}, domainerr.ErrNotFound
	}
	return target, nil
}

// firstMemberWithRole returns the earliest-listed member carrying role,
// excluding excludeUserID. Callers pass the ListUsersByHousehold result,
// which is ordered by created_at.
func firstMemberWithRole(members []identity.User, excludeUserID int64, role string) int64 {
	for _, m := range members {
		if m.UserID != excludeUserID && m.HouseholdRole == role {
			return m.UserID
		}
	}
	return 0
}

// invalidateUser evicts the caller's cached identity resolution so the
// next request re-reads household membership and searchability.
func (r *Resolver) invalidateUser(_ context.Context, u currentuser.User) {
	if r.AuthInvalidator != nil {
		r.AuthInvalidator.InvalidateUser(u.Provider, u.ExternalSubject)
	}
}

// invalidateUserID evicts a user's cached resolution by ID — used for the
// counterparty whose provider/subject the resolver does not hold.
func (r *Resolver) invalidateUserID(ctx context.Context, userID int64) {
	if r.AuthInvalidator != nil {
		r.AuthInvalidator.InvalidateUserID(ctx, userID)
	}
}

// householdUserResolvers projects identity users to the restricted
// HouseholdUser shape.
func householdUserResolvers(users []identity.User) []*householdUserResolver {
	out := make([]*householdUserResolver, len(users))
	for i, u := range users {
		out[i] = &householdUserResolver{u: u}
	}
	return out
}

type householdResolver struct {
	hh      household.Household
	members []*householdMemberResolver
	myRole  string
}

func (r *householdResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.hh.HouseholdID, 10))
}

func (r *householdResolver) Name() *string { return r.hh.Name }

func (r *householdResolver) Members() []*householdMemberResolver { return r.members }

func (r *householdResolver) MyRole() string { return strings.ToUpper(r.myRole) }

func (r *householdResolver) CreatedAt() graphql.Time { return graphql.Time{Time: r.hh.CreatedAt} }

// householdMemberResolver resolves HouseholdMember — the restricted user
// projection paired with per-household role metadata.
type householdMemberResolver struct {
	u    identity.User
	isMe bool
}

func (r *householdMemberResolver) User() *householdUserResolver {
	return &householdUserResolver{u: r.u}
}

func (r *householdMemberResolver) Role() string { return strings.ToUpper(r.u.HouseholdRole) }

func (r *householdMemberResolver) IsMe() bool { return r.isMe }

// householdUserResolver resolves HouseholdUser — the restricted
// projection without email, role, or activity fields.
type householdUserResolver struct {
	u identity.User
}

func (r *householdUserResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.u.UserID, 10))
}

func (r *householdUserResolver) DisplayName() *string { return nilIfEmpty(r.u.DisplayName) }

func (r *householdUserResolver) FirstName() *string { return nilIfEmpty(r.u.FirstName) }

func (r *householdUserResolver) LastName() *string { return nilIfEmpty(r.u.LastName) }

type householdInviteResolver struct {
	inv  household.Invite
	from *householdUserResolver
	to   *householdUserResolver
}

func (r *householdInviteResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.inv.InviteID, 10))
}

func (r *householdInviteResolver) FromUser() *householdUserResolver { return r.from }

func (r *householdInviteResolver) ToUser() *householdUserResolver { return r.to }

// Status returns the GraphQL enum name for the invite status.
func (r *householdInviteResolver) Status() string {
	return strings.ToUpper(string(r.inv.Status))
}

func (r *householdInviteResolver) CreatedAt() graphql.Time {
	return graphql.Time{Time: r.inv.CreatedAt}
}

type householdNotificationResolver struct {
	n     household.Notification
	actor *householdUserResolver
}

func (r *householdNotificationResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.n.NotificationID, 10))
}

func (r *householdNotificationResolver) Kind() string {
	return strings.ToUpper(string(r.n.Kind))
}

func (r *householdNotificationResolver) Actor() *householdUserResolver { return r.actor }

func (r *householdNotificationResolver) CreatedAt() graphql.Time {
	return graphql.Time{Time: r.n.CreatedAt}
}
