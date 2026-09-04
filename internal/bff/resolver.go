package bff

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"strconv"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

//go:embed schema.graphqls
var schema string

// Resolver is the root GraphQL resolver. It is the only package that is
// allowed to orchestrate across domain modules.
type Resolver struct{}

// NewResolver returns a new BFF resolver.
func NewResolver() *Resolver {
	return &Resolver{}
}

// userFromContext returns the authenticated user or an error.
func userFromContext(ctx context.Context) (currentuser.User, error) {
	u, ok := currentuser.FromContext(ctx)
	if !ok {
		return currentuser.User{}, errors.New("unauthorized")
	}
	return u, nil
}

// Me resolves the current authenticated user.
func (r *Resolver) Me(ctx context.Context) (*userResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &userResolver{u: u}, nil
}

// userResolver resolves User fields.
type userResolver struct {
	u currentuser.User
}

func (r *userResolver) ID() graphql.ID       { return graphql.ID(strconv.FormatInt(r.u.UserID, 10)) }
func (r *userResolver) Email() string        { return r.u.Email }
func (r *userResolver) DisplayName() *string { return &r.u.DisplayName }

// NewGraphQLHandler returns an Echo handler that executes GraphQL requests.
func NewGraphQLHandler(r *Resolver) echo.HandlerFunc {
	parsed, err := graphql.ParseSchema(schema, r)
	if err != nil {
		panic(err)
	}
	return func(c echo.Context) error {
		var req struct {
			Query         string                 `json:"query"`
			Variables     map[string]interface{} `json:"variables"`
			OperationName string                 `json:"operationName"`
		}
		if err := c.Bind(&req); err != nil {
			return err
		}
		resp := parsed.Exec(c.Request().Context(), req.Query, req.OperationName, req.Variables)
		return c.JSONBlob(http.StatusOK, resp.Data)
	}
}
