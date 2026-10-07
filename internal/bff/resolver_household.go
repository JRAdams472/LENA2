package bff

import (
	"context"
	"errors"
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

// msgCannotInvite is the single rejection InviteHouseholdMember returns
// for unknown, inactive, unsearchable, already-a-mate, or
// duplicate-pending targets — one generic message keeps the endpoint from
// enumerating user IDs.
const msgCannotInvite = "cannot invite this user"

// MyHousehold resolves the caller's active household with its members;
// nil when the caller has none (should not occur once the authenticator
// ensures a default household, but the query must not error on it).
func (r *Resolver) MyHousehold(ctx context.Context) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if u.HouseholdID == 0 {
		return nil, nil
	}
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID, u.HouseholdID)
}

// MyHouseholds lists every household the caller belongs to — the data
// behind the active-household switcher.
func (r *Resolver) MyHouseholds(ctx context.Context) ([]*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	mine, err := r.HouseholdService.ListMyHouseholds(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]*householdResolver, 0, len(mine))
	for _, m := range mine {
		hr, err := r.householdWithMembers(ctx, m.Household.HouseholdID, u.UserID, u.HouseholdID)
		if err != nil {
			return nil, err
		}
		out = append(out, hr)
	}
	return out, nil
}

// householdWithMembers loads a household and its members. Roles come from
// the household_member rows (membership order, oldest first) — a user's
// active-pointer role would report the wrong value once they belong to
// several households. activeHouseholdID marks the caller's active
// household; passing it keeps the flag honest when the caller's cached
// pointer is stale (just-switched mutations).
func (r *Resolver) householdWithMembers(ctx context.Context, householdID, viewerID, activeHouseholdID int64) (*householdResolver, error) {
	hh, err := r.HouseholdService.GetHouseholdByID(ctx, householdID)
	if err != nil {
		return nil, err
	}
	mems, err := r.HouseholdService.ListMembersByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(mems))
	for _, m := range mems {
		ids = append(ids, m.UserID)
	}
	users, err := r.IdentityService.ListUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]identity.User, len(users))
	for _, usr := range users {
		byID[usr.UserID] = usr
	}
	out := &householdResolver{hh: hh, isActive: householdID == activeHouseholdID}
	for _, m := range mems {
		usr, ok := byID[m.UserID]
		if !ok {
			continue // membership outlived the user row only mid-cascade
		}
		if m.UserID == viewerID {
			out.myRole = m.Role
		}
		out.members = append(out.members, &householdMemberResolver{
			u:    usr,
			role: m.Role,
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
	sent, err := r.HouseholdService.ListSentInvitesForUser(ctx, u.UserID, u.HouseholdID)
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
// target user. Requires owner or admin; self-invites and full households
// are rejected with distinct errors, but every target-side rejection —
// unknown, inactive, non-searchable, already a household mate, duplicate
// pending invite — returns the same generic error so the endpoint cannot
// enumerate accounts or discoverability state (LEN-29 finding 3).
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
	if !r.inviteLimiter().allow(u.UserID) {
		return nil, &clientError{msg: "too many invites; try again later", code: codeBusy}
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
		if errors.Is(err, domainerr.ErrNotFound) {
			return nil, badInputf(msgCannotInvite)
		}
		return nil, err
	}
	if !target.IsActive || !target.IsSearchable {
		return nil, badInputf(msgCannotInvite)
	}
	// Already a mate is membership, not pointer equality — the target's
	// active household may be a different one they also belong to.
	if _, err := r.HouseholdService.GetMembership(ctx, u.HouseholdID, targetID); err == nil {
		return nil, badInputf(msgCannotInvite)
	} else if !errors.Is(err, domainerr.ErrNotFound) {
		return nil, err
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
		// A duplicate pending invite is target-side state — fold it into
		// the generic rejection rather than leaking CONFLICT.
		if errors.Is(err, domainerr.ErrConflict) {
			return nil, badInputf(msgCannotInvite)
		}
		return nil, err
	}
	out, err := r.hydrateInvites(ctx, []household.Invite{inv})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// AcceptHouseholdInvite joins the caller to the inviter's household and
// activates it — prior memberships are kept (LEN-26 multi-membership).
// mergeFromHouseholdId optionally names another household the caller
// solely owns; its pantry/cellar/plans/lists/events are merged in and the
// source household is dissolved.
func (r *Resolver) AcceptHouseholdInvite(ctx context.Context, args struct {
	InviteID             graphql.ID
	MergeFromHouseholdID *graphql.ID
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inviteID, err := parseID(string(args.InviteID))
	if err != nil {
		return nil, err
	}
	var mergeFrom *int64
	if args.MergeFromHouseholdID != nil {
		id, err := parseID(string(*args.MergeFromHouseholdID))
		if err != nil {
			return nil, err
		}
		mergeFrom = &id
	}
	inv, err := r.HouseholdService.GetInviteByID(ctx, inviteID)
	if err != nil {
		return nil, err
	}
	if inv.ToUserID != u.UserID {
		// Non-party access must not leak that the invite exists.
		return nil, domainerr.ErrNotFound
	}
	if err := r.acceptInvite(ctx, u, inv, mergeFrom); err != nil {
		return nil, err
	}
	r.invalidateUser(ctx, u)
	r.invalidateUserID(ctx, inv.FromUserID)
	return r.householdWithMembers(ctx, inv.HouseholdID, u.UserID, inv.HouseholdID)
}

// acceptInvite runs the invite-accept orchestration in one unit of work:
// lock the household row (serializing concurrent accepts/leaves/removals
// and the member cap), transition the invite, add the caller's
// membership, optionally merge a named sole-owned household, activate the
// joined household, dissolve the merge source, then notify.
func (r *Resolver) acceptInvite(ctx context.Context, u currentuser.User, inv household.Invite, mergeFrom *int64) error {
	return r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		if _, err := r.HouseholdService.LockHousehold(ctx, inv.HouseholdID); err != nil {
			return err
		}
		if n, err := r.HouseholdService.CountMembers(ctx, inv.HouseholdID); err != nil {
			return err
		} else if n >= maxHouseholdMembers {
			return badInputf("household is full")
		}
		// The invite is dead if the inviter left the household — membership
		// check, not pointer equality, since members may be active elsewhere.
		if _, err := r.HouseholdService.GetMembership(ctx, inv.HouseholdID, inv.FromUserID); err != nil {
			if errors.Is(err, domainerr.ErrNotFound) {
				return domainerr.ErrConflict
			}
			return err
		}
		if _, err := r.HouseholdService.TransitionInvite(ctx, inv.InviteID, household.StatusAccepted, u.Email); err != nil {
			return err
		}
		if _, err := r.HouseholdService.GetMembership(ctx, inv.HouseholdID, u.UserID); errors.Is(err, domainerr.ErrNotFound) {
			if _, err := r.HouseholdService.JoinHousehold(ctx, inv.HouseholdID, u.UserID, identity.HouseholdRoleMember, u.Email); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if mergeFrom != nil {
			if err := r.validateMergeSource(ctx, *mergeFrom, inv.HouseholdID, u.UserID); err != nil {
				return err
			}
			if err := r.mergeIntoHousehold(ctx, mergeFrom, inv.HouseholdID, u.Email); err != nil {
				return err
			}
		}
		if err := r.IdentityService.SetActiveHousehold(ctx, u.UserID, inv.HouseholdID, u.Email); err != nil {
			return err
		}
		if mergeFrom != nil {
			// Pointer is already off the source (activated above); the
			// caller was its sole member so no users row can reference it.
			if err := r.HouseholdService.DeleteHousehold(ctx, *mergeFrom); err != nil {
				return err
			}
		}
		return r.notifyJoin(ctx, u, inv)
	})
}

// validateMergeSource gates the dissolve: only a household the caller
// solely owns may merge — shared households are always rejected so their
// other members keep their data.
func (r *Resolver) validateMergeSource(ctx context.Context, sourceID, targetID, userID int64) error {
	if sourceID == targetID {
		return badInputf("cannot merge a household into itself")
	}
	mem, err := r.HouseholdService.GetMembership(ctx, sourceID, userID)
	if err != nil {
		return err // strangers get NOT_FOUND — don't leak the household
	}
	if mem.Role != identity.HouseholdRoleOwner {
		return badInputf("only a household you own can be merged")
	}
	if n, err := r.HouseholdService.CountMembers(ctx, sourceID); err != nil {
		return err
	} else if n != 1 {
		return badInputf("only a household you alone belong to can be merged")
	}
	return nil
}

// mergeIntoHousehold moves a joining member's previous-household stock,
// meal plan, grocery and event data into the household they're accepting
// into. No-op when the member had no household or already belongs to the
// target (re-accept of an own-household invite).
func (r *Resolver) mergeIntoHousehold(ctx context.Context, fromHouseholdID *int64, toHouseholdID int64, email string) error {
	if fromHouseholdID == nil || *fromHouseholdID == toHouseholdID {
		return nil
	}
	from := *fromHouseholdID
	if err := r.UserPrefsService.MergeHouseholdStock(ctx, from, toHouseholdID, email); err != nil {
		return err
	}
	if err := r.MealPlanService.ReassignHousehold(ctx, from, toHouseholdID, email); err != nil {
		return err
	}
	if err := r.GroceryService.ReassignHousehold(ctx, from, toHouseholdID, email); err != nil {
		return err
	}
	if r.EventService != nil {
		if err := r.EventService.ReassignHousehold(ctx, from, toHouseholdID, email); err != nil {
			return err
		}
	}
	return nil
}

// notifyJoin tells the inviter their invite was accepted, then every other
// member that someone joined — the inviter's accept notification already
// covers them.
func (r *Resolver) notifyJoin(ctx context.Context, u currentuser.User, inv household.Invite) error {
	if err := r.notify(ctx, []int64{inv.FromUserID}, household.KindInviteAccepted, inv.HouseholdID, u.UserID, &inv.InviteID); err != nil {
		return err
	}
	members, err := r.HouseholdService.ListMembersByHousehold(ctx, inv.HouseholdID)
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

// LeaveHousehold removes the caller's membership from the given
// household (default: their active one). Data stays behind. An owner
// leaving a multi-member household promotes the earliest admin, else the
// earliest member. Leaving the active household activates the earliest
// remaining membership — or, when none remain, recreates the legacy
// fresh single-person household.
func (r *Resolver) LeaveHousehold(ctx context.Context, args struct {
	HouseholdID *graphql.ID
}) (bool, error) {
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
		// The DB row is authoritative — the cached active pointer may lag
		// a recent switch, so default the target from fresh.HouseholdID.
		var leaveID int64
		switch {
		case args.HouseholdID != nil:
			if leaveID, err = parseID(string(*args.HouseholdID)); err != nil {
				return err
			}
		case fresh.HouseholdID != nil:
			leaveID = *fresh.HouseholdID
		default:
			return badInputf("no household to leave")
		}
		if _, err := r.HouseholdService.LockHousehold(ctx, leaveID); err != nil {
			return err
		}
		caller, err := r.HouseholdService.GetMembership(ctx, leaveID, u.UserID)
		if err != nil {
			return err
		}
		members, err := r.HouseholdService.ListMembersByHousehold(ctx, leaveID)
		if err != nil {
			return err
		}
		promoted, err = r.promoteSuccessor(ctx, u, caller.Role, members, leaveID)
		if err != nil {
			return err
		}
		if err := r.cancelPendingInvitesFromUser(ctx, u, leaveID); err != nil {
			return err
		}
		if err := r.HouseholdService.RemoveMembership(ctx, leaveID, u.UserID); err != nil {
			return err
		}
		if fresh.HouseholdID != nil && *fresh.HouseholdID == leaveID {
			if err := r.activateFallbackHousehold(ctx, u.UserID, u.Email); err != nil {
				return err
			}
		}
		var remaining []int64
		for _, m := range members {
			if m.UserID != u.UserID {
				remaining = append(remaining, m.UserID)
			}
		}
		return r.notify(ctx, remaining, household.KindMemberLeft, leaveID, u.UserID, nil)
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

// activateFallbackHousehold points a user who just left/was removed from
// their active household at their earliest remaining membership — or, when
// none remain, creates the fresh single-person household that preserves
// legacy behavior. Call after the old membership row is deleted.
func (r *Resolver) activateFallbackHousehold(ctx context.Context, userID int64, by string) error {
	mems, err := r.HouseholdService.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(mems) > 0 {
		return r.IdentityService.SetActiveHousehold(ctx, userID, mems[0].HouseholdID, by)
	}
	hh, err := r.HouseholdService.CreateHousehold(ctx, by)
	if err != nil {
		return err
	}
	if _, err := r.HouseholdService.JoinHousehold(ctx, hh.HouseholdID, userID, identity.HouseholdRoleOwner, by); err != nil {
		return err
	}
	return r.IdentityService.SetActiveHousehold(ctx, userID, hh.HouseholdID, by)
}

// promoteSuccessor hands ownership to the earliest admin (else the
// earliest member) when the caller leaving was the owner of a multi-member
// household. Membership order defines "earliest". Returns the promoted
// member's ID, or 0 when no promotion was needed/possible.
func (r *Resolver) promoteSuccessor(ctx context.Context, u currentuser.User, callerRole string, members []household.Membership, householdID int64) (int64, error) {
	if callerRole != identity.HouseholdRoleOwner || len(members) <= 1 {
		return 0, nil
	}
	successor := firstMemberWithRole(members, u.UserID, identity.HouseholdRoleAdmin)
	if successor == 0 {
		successor = firstMemberWithRole(members, u.UserID, identity.HouseholdRoleMember)
	}
	if successor == 0 {
		return 0, nil
	}
	if _, err := r.HouseholdService.JoinHousehold(ctx, householdID, successor, identity.HouseholdRoleOwner, u.Email); err != nil {
		return 0, err
	}
	// Sync the pointer's cached role only when this household is the
	// successor's active one — a member active elsewhere keeps their own
	// pointer role untouched.
	if succ, err := r.IdentityService.GetByID(ctx, successor); err != nil {
		return 0, err
	} else if succ.HouseholdID != nil && *succ.HouseholdID == householdID {
		if err := r.IdentityService.SetUserHouseholdRole(ctx, successor, householdID, identity.HouseholdRoleOwner, u.Email); err != nil {
			return 0, err
		}
	}
	if err := r.notify(ctx, []int64{successor}, household.KindRoleChanged, householdID, u.UserID, nil); err != nil {
		return 0, err
	}
	return successor, nil
}

// cancelPendingInvitesFromUser cancels the caller's pending invites for the
// household they're leaving — they can no longer be accepted against it —
// and notifies each recipient.
func (r *Resolver) cancelPendingInvitesFromUser(ctx context.Context, u currentuser.User, householdID int64) error {
	cancelled, err := r.HouseholdService.CancelPendingInvitesFrom(ctx, u.UserID, householdID, u.Email)
	if err != nil {
		return err
	}
	for _, c := range cancelled {
		if err := r.notify(ctx, []int64{c.ToUserID}, household.KindInviteCancelled, householdID, u.UserID, &c.InviteID); err != nil {
			return err
		}
	}
	return nil
}

// CreateHousehold makes a new household, joins the caller as owner, and
// activates it — the "add a household" entry point beside invite accept.
func (r *Resolver) CreateHousehold(ctx context.Context, args struct {
	Name *string
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var hh household.Household
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		var err error
		hh, err = r.HouseholdService.CreateHousehold(ctx, u.Email)
		if err != nil {
			return err
		}
		if _, err := r.HouseholdService.JoinHousehold(ctx, hh.HouseholdID, u.UserID, identity.HouseholdRoleOwner, u.Email); err != nil {
			return err
		}
		if err := r.IdentityService.SetActiveHousehold(ctx, u.UserID, hh.HouseholdID, u.Email); err != nil {
			return err
		}
		if args.Name != nil {
			if _, err := r.HouseholdService.RenameHousehold(ctx, hh.HouseholdID, strings.TrimSpace(*args.Name), u.Email); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUser(ctx, u)
	return r.householdWithMembers(ctx, hh.HouseholdID, u.UserID, hh.HouseholdID)
}

// SetActiveHousehold moves the caller's active-household pointer to a
// household they belong to. All household-scoped reads and writes follow
// it; non-members get NOT_FOUND so household IDs are not enumerable.
func (r *Resolver) SetActiveHousehold(ctx context.Context, args struct {
	HouseholdID graphql.ID
}) (*householdResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	hhID, err := parseID(string(args.HouseholdID))
	if err != nil {
		return nil, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		err := r.IdentityService.SetActiveHousehold(ctx, u.UserID, hhID, u.Email)
		if errors.Is(err, domainerr.ErrConflict) {
			return domainerr.ErrNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUser(ctx, u)
	return r.householdWithMembers(ctx, hhID, u.UserID, hhID)
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
		members, err := r.HouseholdService.ListMembersByHousehold(ctx, u.HouseholdID)
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
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID, u.HouseholdID)
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
		target, mem, err := r.householdMember(ctx, u.HouseholdID, targetID)
		if err != nil {
			return err
		}
		if mem.Role == identity.HouseholdRoleOwner {
			return badInputf("cannot change the owner's role")
		}
		if _, err := r.HouseholdService.JoinHousehold(ctx, u.HouseholdID, targetID, role, u.Email); err != nil {
			return err
		}
		// The membership role is authoritative; mirror it onto the user's
		// active-pointer role only when this household is their active one.
		if target.HouseholdID != nil && *target.HouseholdID == u.HouseholdID {
			if err := r.IdentityService.SetUserHouseholdRole(ctx, targetID, u.HouseholdID, role, u.Email); err != nil {
				return err
			}
		}
		return r.notify(ctx, []int64{targetID}, household.KindRoleChanged, u.HouseholdID, u.UserID, nil)
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUserID(ctx, targetID)
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID, u.HouseholdID)
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
		target, _, err := r.householdMember(ctx, u.HouseholdID, targetID)
		if err != nil {
			return err
		}
		if _, err := r.HouseholdService.JoinHousehold(ctx, u.HouseholdID, targetID, identity.HouseholdRoleOwner, u.Email); err != nil {
			return err
		}
		if _, err := r.HouseholdService.JoinHousehold(ctx, u.HouseholdID, u.UserID, identity.HouseholdRoleMember, u.Email); err != nil {
			return err
		}
		if target.HouseholdID != nil && *target.HouseholdID == u.HouseholdID {
			if err := r.IdentityService.SetUserHouseholdRole(ctx, targetID, u.HouseholdID, identity.HouseholdRoleOwner, u.Email); err != nil {
				return err
			}
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
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID, u.HouseholdID)
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
		target, mem, err := r.householdMember(ctx, u.HouseholdID, targetID)
		if err != nil {
			return err
		}
		if mem.Role == identity.HouseholdRoleOwner {
			return errForbidden()
		}
		if u.HouseholdRole == identity.HouseholdRoleAdmin && mem.Role != identity.HouseholdRoleMember {
			return errForbidden()
		}
		if err := r.HouseholdService.RemoveMembership(ctx, u.HouseholdID, targetID); err != nil {
			return err
		}
		// The target keeps every other membership; only when this was their
		// active household does their pointer move — to the earliest
		// remaining membership or a fresh solo household.
		if target.HouseholdID != nil && *target.HouseholdID == u.HouseholdID {
			if err := r.activateFallbackHousehold(ctx, targetID, u.Email); err != nil {
				return err
			}
		}
		return r.notifyHouseholdMemberRemoval(ctx, u, targetID)
	})
	if err != nil {
		return nil, err
	}
	r.invalidateUserID(ctx, targetID)
	return r.householdWithMembers(ctx, u.HouseholdID, u.UserID, u.HouseholdID)
}

// notifyHouseholdMemberRemoval cancels the target's pending invites from
// this household, tells them they were removed, and tells the remaining
// members — to them a removal reads the same as a leave. Runs inside the
// caller's ambient transaction.
func (r *Resolver) notifyHouseholdMemberRemoval(ctx context.Context, u currentuser.User, targetID int64) error {
	cancelled, err := r.HouseholdService.CancelPendingInvitesFrom(ctx, targetID, u.HouseholdID, u.Email)
	if err != nil {
		return err
	}
	for _, c := range cancelled {
		if err := r.notify(ctx, []int64{c.ToUserID}, household.KindInviteCancelled, u.HouseholdID, u.UserID, &c.InviteID); err != nil {
			return err
		}
	}
	if err := r.notify(ctx, []int64{targetID}, household.KindMemberRemoved, u.HouseholdID, u.UserID, nil); err != nil {
		return err
	}
	mems, err := r.HouseholdService.ListMembersByHousehold(ctx, u.HouseholdID)
	if err != nil {
		return err
	}
	var remaining []int64
	for _, m := range mems {
		if m.UserID != u.UserID {
			remaining = append(remaining, m.UserID)
		}
	}
	return r.notify(ctx, remaining, household.KindMemberLeft, u.HouseholdID, targetID, nil)
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
	notifs, err := r.HouseholdService.ListNotificationsForUser(ctx, u.UserID, u.HouseholdID, clamp(args.Limit, 1, 100))
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
	n, err := r.HouseholdService.CountUnreadNotifications(ctx, u.UserID, u.HouseholdID)
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
	if err := r.HouseholdService.MarkAllNotificationsRead(ctx, u.UserID, u.HouseholdID); err != nil {
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
	return r.notifyEvent(ctx, userIDs, kind, householdID, actorID, inviteID, nil)
}

// notifyEvent is notify with a food-event deep-link target.
func (r *Resolver) notifyEvent(ctx context.Context, userIDs []int64, kind household.NotificationKind, householdID int64, actorID int64, inviteID, foodEventID *int64) error {
	for _, uid := range userIDs {
		// User-directed notices (received/cancelled invites, removal) go to
		// non-members — scope them user-global or feed filtering hides them.
		var hh *int64
		if !kind.UserDirected() {
			hh = &householdID
		}
		var actor *int64
		if actorID != 0 {
			a := actorID
			actor = &a
		}
		if err := r.HouseholdService.CreateNotification(ctx, uid, kind, hh, actor, inviteID, foodEventID); err != nil {
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

// householdMember fetches a user that must belong to householdID —
// membership is authoritative (household_member), so a member whose
// active pointer sits in a different household still resolves; strangers
// get NOT_FOUND so membership is never leaked.
func (r *Resolver) householdMember(ctx context.Context, householdID, userID int64) (identity.User, household.Membership, error) {
	mem, err := r.HouseholdService.GetMembership(ctx, householdID, userID)
	if err != nil {
		return identity.User{}, household.Membership{}, err
	}
	target, err := r.IdentityService.GetByID(ctx, userID)
	if err != nil {
		return identity.User{}, household.Membership{}, err
	}
	return target, mem, nil
}

// firstMemberWithRole returns the earliest-joined member carrying role,
// excluding excludeUserID. Callers pass the ListMembersByHousehold
// result, which is ordered by membership created_at.
func firstMemberWithRole(members []household.Membership, excludeUserID int64, role string) int64 {
	for _, m := range members {
		if m.UserID != excludeUserID && m.Role == role {
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
	hh       household.Household
	members  []*householdMemberResolver
	myRole   string
	isActive bool
}

func (r *householdResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.hh.HouseholdID, 10))
}

func (r *householdResolver) Name() *string { return r.hh.Name }

func (r *householdResolver) Members() []*householdMemberResolver { return r.members }

func (r *householdResolver) MyRole() string { return strings.ToUpper(r.myRole) }

// IsActive reports whether this household is the caller's active one —
// set at build time against the caller's (post-mutation) pointer.
func (r *householdResolver) IsActive() bool { return r.isActive }

func (r *householdResolver) CreatedAt() graphql.Time { return graphqlTime(r.hh.CreatedAt) }

// householdMemberResolver resolves HouseholdMember — the restricted user
// projection paired with the role the member holds in THIS household
// (from household_member; the user's pointer role may be for a different
// household).
type householdMemberResolver struct {
	u    identity.User
	role string
	isMe bool
}

func (r *householdMemberResolver) User() *householdUserResolver {
	return &householdUserResolver{u: r.u}
}

func (r *householdMemberResolver) Role() string { return strings.ToUpper(r.role) }

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
	return graphqlTime(r.inv.CreatedAt)
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

func (r *householdNotificationResolver) FoodEventID() *graphql.ID {
	if r.n.FoodEventID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.n.FoodEventID, 10))
	return &id
}

func (r *householdNotificationResolver) CreatedAt() graphql.Time {
	return graphqlTime(r.n.CreatedAt)
}

// Title is the server-rendered heading for scheduled reminders; null on
// event-driven rows.
func (r *householdNotificationResolver) Title() *string { return r.n.Title }

// Body is the server-rendered detail text for scheduled reminders.
func (r *householdNotificationResolver) Body() *string { return r.n.Body }

// RecipeID deep-links meal-reminder notifications to the recipe page.
func (r *householdNotificationResolver) RecipeID() *graphql.ID {
	if r.n.RecipeID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.n.RecipeID, 10))
	return &id
}

// ItemID deep-links expiry notifications to the pantry item and keys the
// "add to grocery list" action.
func (r *householdNotificationResolver) ItemID() *graphql.ID {
	if r.n.ItemID == nil {
		return nil
	}
	id := graphql.ID(strconv.FormatInt(*r.n.ItemID, 10))
	return &id
}
