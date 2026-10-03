package bff

import (
	"context"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// AllowIntrospectionForAdmins returns the graphql.RestrictIntrospection
// filter used by the production schema. Returning true enables
// introspection for that request: admins may introspect, members cannot
// enumerate the schema (which includes admin-only mutations), and a nil
// filter return blocks everyone when disableAll is set.
func AllowIntrospectionForAdmins(disableAll bool) func(context.Context) bool {
	if disableAll {
		return func(context.Context) bool { return false }
	}
	return func(ctx context.Context) bool {
		u, ok := currentuser.FromContext(ctx)
		return ok && u.IsAdmin
	}
}
