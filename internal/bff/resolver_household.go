package bff

import (
	"context"
	"strconv"
	"strings"

	"github.com/graph-gophers/graphql-go"

	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/platform/domainerr"
)

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
	return r.householdWithMembers(ctx, u.HouseholdID)
}

// householdWithMembers loads a household and its members in two queries.
func (r *Resolver) householdWithMembers(ctx context.Context, householdID int64) (*householdResolver, error) {
	hh, err := r.HouseholdService.GetHouseholdByID(ctx, householdID)
	if err != nil {
		return nil, err
	}
	members, err := r.IdentityService.ListUsersByHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	return &householdResolver{hh: hh, members: householdUserResolvers(members)}, nil
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
// target user. Self-invites and existing household mates are rejected;
// a duplicate pending invite surfaces as CONFLICT.
func (r *Resolver) InviteHouseholdMember(ctx context.Context, args struct {
	UserID graphql.ID
}) (*householdInviteResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
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
	inv, err := r.HouseholdService.CreateInvite(ctx, u.UserID, targetID, u.HouseholdID, u.Email)
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
	return r.householdWithMembers(ctx, inv.HouseholdID)
}

// acceptInvite runs the invite-accept orchestration in one unit of work:
// transition the invite, merge the caller's household data into the
// inviter's household, then move the caller with an expected-value guard.
func (r *Resolver) acceptInvite(ctx context.Context, u currentuser.User, inv household.Invite) error {
	return r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
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
		return r.IdentityService.SetUserHousehold(ctx, u.UserID, inv.HouseholdID, target.HouseholdID)
	})
}

// DeclineHouseholdInvite marks a received invite declined. Only the
// invitee may decline; non-party access returns NOT_FOUND.
func (r *Resolver) DeclineHouseholdInvite(ctx context.Context, args struct {
	InviteID graphql.ID
}) (*householdInviteResolver, error) {
	return r.transitionInvite(ctx, args.InviteID, household.StatusDeclined, func(u currentuser.User, inv household.Invite) bool {
		return inv.ToUserID == u.UserID
	})
}

// CancelHouseholdInvite marks a sent invite cancelled. Only the inviter
// may cancel; non-party access returns NOT_FOUND.
func (r *Resolver) CancelHouseholdInvite(ctx context.Context, args struct {
	InviteID graphql.ID
}) (*householdInviteResolver, error) {
	return r.transitionInvite(ctx, args.InviteID, household.StatusCancelled, func(u currentuser.User, inv household.Invite) bool {
		return inv.FromUserID == u.UserID
	})
}

// transitionInvite implements decline/cancel: fetch, party check, guarded
// transition (conflict when the invite was already resolved), hydrate.
func (r *Resolver) transitionInvite(ctx context.Context, rawID graphql.ID, to household.Status, isParty func(currentuser.User, household.Invite) bool) (*householdInviteResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inviteID, err := parseID(string(rawID))
	if err != nil {
		return nil, err
	}
	inv, err := r.HouseholdService.GetInviteByID(ctx, inviteID)
	if err != nil {
		return nil, err
	}
	if !isParty(u, inv) {
		return nil, domainerr.ErrNotFound
	}
	out, err := r.HouseholdService.TransitionInvite(ctx, inviteID, to, u.Email)
	if err != nil {
		return nil, err
	}
	resolved, err := r.hydrateInvites(ctx, []household.Invite{out})
	if err != nil {
		return nil, err
	}
	return resolved[0], nil
}

// LeaveHousehold moves the caller into a fresh single-person household.
// Their data stays with the household they left.
func (r *Resolver) LeaveHousehold(ctx context.Context) (bool, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return false, err
	}
	err = r.unitOfWork().InTx(ctx, func(ctx context.Context) error {
		fresh, err := r.IdentityService.GetByID(ctx, u.UserID)
		if err != nil {
			return err
		}
		hh, err := r.HouseholdService.CreateHousehold(ctx, u.Email)
		if err != nil {
			return err
		}
		return r.IdentityService.SetUserHousehold(ctx, u.UserID, hh.HouseholdID, fresh.HouseholdID)
	})
	if err != nil {
		return false, err
	}
	r.invalidateUser(ctx, u)
	return true, nil
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
	members []*householdUserResolver
}

func (r *householdResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.hh.HouseholdID, 10))
}

func (r *householdResolver) Members() []*householdUserResolver { return r.members }

func (r *householdResolver) CreatedAt() graphql.Time { return graphql.Time{Time: r.hh.CreatedAt} }

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
