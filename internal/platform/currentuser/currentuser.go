// Package currentuser carries the authenticated user through request
// context so downstream services and resolvers never accept a user id
// directly from the client.
package currentuser

import "context"

// User is the authenticated caller of the current request.
type User struct {
	UserID          int64
	Provider        string
	ExternalSubject string
	Email           string
	DisplayName     string
	// IsAdmin is set when the user's persisted identity.users role is
	// 'admin'. Admin-only mutations check this flag, not just authn.
	IsAdmin bool
	// HouseholdID scopes shared operational data (pantry, cellar, meal
	// plans, grocery lists). Zero means the user has no household yet; the
	// authenticator ensures one on the first cache miss after sign-in.
	HouseholdID int64
	// IsSearchable controls whether the user appears in household-invite
	// search results.
	IsSearchable bool
}

type contextKey struct{}

// WithUser returns a new context carrying the given user.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

// FromContext returns the authenticated user stored in ctx, if any.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(contextKey{}).(User)
	return u, ok
}
