package bff

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/JRAdams472/LENA2/internal/bff/mock"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/testutil"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/app/recipeimport"
)

func recipeImportAdminCtx() context.Context {
	return currentuser.WithUser(context.Background(), currentuser.User{
		UserID: 1, Email: "admin@example.com", IsAdmin: true,
	})
}

func TestResolver_RecipeImportMutations_AdminOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockRecipeImportService(ctrl)
	r := &Resolver{RecipeImportService: svc}
	member := testutil.WithUser(context.Background(), 2, "m@example.com")

	// Every admin mutation must reject unauthenticated callers.
	for name, call := range map[string]func() error{
		"RecipeImport": func() error {
			_, err := r.RecipeImport(context.Background(), struct{ ID graphql.ID }{ID: "1"})
			return err
		},
		"UpdateRecipeImport": func() error {
			_, err := r.UpdateRecipeImport(context.Background(), struct {
				ID    graphql.ID
				Input recipeImportReviewInput
			}{ID: "1"})
			return err
		},
		"ApproveRecipeImport": func() error {
			_, err := r.ApproveRecipeImport(context.Background(), struct{ ID graphql.ID }{ID: "1"})
			return err
		},
		"RejectRecipeImport": func() error {
			_, err := r.RejectRecipeImport(context.Background(), struct{ ID graphql.ID }{ID: "1"})
			return err
		},
		"RetryRecipeImport": func() error {
			_, err := r.RetryRecipeImport(context.Background(), struct{ ID graphql.ID }{ID: "1"})
			return err
		},
	} {
		assert.EqualError(t, call(), "unauthorized", name)
	}

	// And non-admin members.
	for name, call := range map[string]func() error{
		"ApproveRecipeImport": func() error {
			_, err := r.ApproveRecipeImport(member, struct{ ID graphql.ID }{ID: "1"})
			return err
		},
		"RejectRecipeImport": func() error {
			_, err := r.RejectRecipeImport(member, struct{ ID graphql.ID }{ID: "1"})
			return err
		},
	} {
		assert.EqualError(t, call(), "forbidden: admin role required", name)
	}
}

func TestResolver_ApproveRecipeImport(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockRecipeImportService(ctrl)
	r := &Resolver{RecipeImportService: svc}

	rcp := recipe.Recipe{RecipeID: 42, Name: "IT Pancakes"}
	ri := &recipeimport.RecipeImport{ID: 5, Status: recipeimport.StatusPersisted}
	svc.EXPECT().
		Approve(gomock.Any(), int64(5), gomock.Any()).
		Return(&rcp, ri, nil)

	res, err := r.ApproveRecipeImport(recipeImportAdminCtx(), struct{ ID graphql.ID }{ID: "5"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "IT Pancakes", res.recipe.Name)
}

func TestResolver_RejectAndRetryRecipeImport(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockRecipeImportService(ctrl)
	r := &Resolver{RecipeImportService: svc}
	ctx := recipeImportAdminCtx()

	rejected := &recipeimport.RecipeImport{ID: 5, Status: recipeimport.StatusRejected}
	svc.EXPECT().Reject(gomock.Any(), int64(5)).Return(nil)
	svc.EXPECT().Get(gomock.Any(), int64(5)).Return(rejected, nil)

	res, err := r.RejectRecipeImport(ctx, struct{ ID graphql.ID }{ID: "5"})
	require.NoError(t, err)
	assert.Equal(t, "rejected", res.Status())

	pending := &recipeimport.RecipeImport{ID: 5, Status: recipeimport.StatusPending}
	svc.EXPECT().Retry(gomock.Any(), int64(5)).Return(nil)
	svc.EXPECT().Get(gomock.Any(), int64(5)).Return(pending, nil)

	res, err = r.RetryRecipeImport(ctx, struct{ ID graphql.ID }{ID: "5"})
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Status())
}

func TestResolver_PendingRecipeImports_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mock.NewMockRecipeImportService(ctrl)
	r := &Resolver{RecipeImportService: svc}

	items := []recipeimport.RecipeImport{
		{ID: 1, Status: recipeimport.StatusPending},
		{ID: 2, Status: recipeimport.StatusReady},
	}
	// pageSize=5000 must clamp to 100 and the clamped values echoed back.
	svc.EXPECT().ListPending(gomock.Any(), int32(1), int32(100)).Return(items, nil)
	svc.EXPECT().CountPending(gomock.Any()).Return(int64(7), nil)

	page, err := r.PendingRecipeImports(recipeImportAdminCtx(), struct {
		Page     int32
		PageSize int32
	}{Page: 1, PageSize: 5000})
	require.NoError(t, err)
	assert.Len(t, page.Items(), 2)
	info := page.PageInfo()
	assert.Equal(t, int32(1), info.PageNumber())
	assert.Equal(t, int32(100), info.PageSize())
	assert.Equal(t, int32(7), info.TotalCount(), "total must come from the DB count, not len(items)")
}
