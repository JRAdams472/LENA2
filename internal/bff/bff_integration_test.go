package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/testutil"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func TestBFF_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	defer cleanup()

	issuer := testutil.NewTestIssuer(t)

	identitySvc := identity.NewService(pool)
	// The authenticator writes household.households, so it needs the real
	// household service — the fake returns IDs the FK would reject.
	authenticator := mustNewAuthenticatorWithHouseholds(t, AuthConfig{
		Issuers:     []string{issuer.URL},
		Audiences:   []string{issuer.Audience},
		AdminEmails: []string{"auth@example.com", "user-a@example.com"},
	}, identitySvc, household.NewService(pool))

	resolver := NewResolver(pool, Services{
		Analytics: analytics.NewService(pool),
		Event:     event.NewService(pool),
		Grocery:   grocery.NewService(pool),
		Inventory: inventory.NewService(pool),
		MealPlan:  mealplan.NewService(pool),
		Recipe:    recipe.NewService(pool),
		UserPrefs: userprefs.NewService(pool),
		Wine:      wine.NewService(pool),
		Identity:  identitySvc,
		Household: household.NewService(pool),
		Auth:      authenticator,
	}, Options{})

	e := echo.New()
	e.HideBanner = true
	handler, err := NewGraphQLHandler(resolver, 10*time.Second, 0)
	require.NoError(t, err)
	e.POST("/graphql", handler, authenticator.Middleware())
	srv := httptest.NewServer(e)
	defer srv.Close()

	t.Run("auth", func(t *testing.T) {
		runAuthTests(t, srv, issuer, authenticator)
	})
	t.Run("authorization", func(t *testing.T) {
		runAuthorizationTests(t, srv, issuer)
	})
	t.Run("end to end", func(t *testing.T) {
		runEndToEndTests(t, srv, issuer)
	})
	t.Run("household", func(t *testing.T) {
		runHouseholdTests(t, srv, issuer)
	})
	t.Run("household management", func(t *testing.T) {
		runHouseholdManagementTests(t, srv, issuer)
	})
	t.Run("events", func(t *testing.T) {
		runEventTests(t, srv, issuer)
	})
}

// runAuthorizationTests verifies that a plain member (a valid token, no
// admin role) is rejected with FORBIDDEN — not UNAUTHENTICATED — on
// admin-only mutations across every domain.
func runAuthorizationTests(t *testing.T, srv *httptest.Server, issuer *testutil.TestIssuer) {
	member := issuer.Token(t, "member-user", "member@example.com", "Member User")

	// Provision the member's identity row so rejection comes from the role
	// check, not user creation.
	status, _ := doGraphQL(t, srv, member, `{ me { id } }`, nil)
	require.Equal(t, http.StatusOK, status)

	adminOps := []struct {
		domain string
		query  string
	}{
		{"inventory/approveItem", `mutation { approveItem(id: "1") { id } }`},
		{"inventory/rejectBrand", `mutation { rejectBrand(id: "1") { id } }`},
		{"inventory/createNutrientType", `mutation { createNutrientType(input: { name: "x", unit: "g" }) { id } }`},
		{"inventory/createCategory", `mutation { createCategory(input: { name: "x" }) { id } }`},
		{"wine/createBottle", `mutation { createBottle(input: { typeId: "1", countryId: "1", regionId: "1", vintageYear: 2020, bottleSize: "750ml" }) { id } }`},
		{"recipe/createRecipe", `mutation { createRecipe(input: { name: "x", items: [], steps: [] }) { id } }`},
		{"recipe-import/approveRecipeImport", `mutation { approveRecipeImport(id: "1") { id } }`},
		{"recipe-import/submitRecipeScan", `mutation { submitRecipeScan(fileBase64: "aGk=") { id } }`},
		{"identity/setUserRole", `mutation { setUserRole(userId: "1", role: admin) { id } }`},
		{"identity/setUserActive", `mutation { setUserActive(userId: "1", isActive: false) { id } }`},
		{"identity/users", `query { users { items { id } pageInfo { totalCount } } }`},
		{"recipe-import/pendingRecipeImports", `query { pendingRecipeImports { items { id } pageInfo { totalCount } } }`},
	}
	for _, op := range adminOps {
		t.Run("member "+op.domain+" is forbidden", func(t *testing.T) {
			status, gr := doGraphQLExpectErrors(t, srv, member, op.query, nil)
			require.Equal(t, http.StatusOK, status)
			require.NotEmpty(t, gr.Errors, "expected a graphql error")
			code, _ := gr.Errors[0].Extensions["code"].(string)
			assert.Equal(t, codeForbidden, code,
				"admin-only operation must reject members with FORBIDDEN, not %s", code)
			assert.NotEqual(t, codeUnauthenticated, code,
				"a valid member token must not be reported as unauthenticated")
		})
	}
}

func runAuthTests(t *testing.T, srv *httptest.Server, issuer *testutil.TestIssuer, authenticator *Authenticator) {
	t.Run("no token returns 401", func(t *testing.T) {
		status, _ := doGraphQL(t, srv, "", `{ me { email } }`, nil)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("garbage token returns 401", func(t *testing.T) {
		status, _ := doGraphQL(t, srv, "not.a.jwt", `{ me { email } }`, nil)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("untrusted issuer token returns 401", func(t *testing.T) {
		otherIssuer := testutil.NewTestIssuer(t)
		tok := otherIssuer.Token(t, "other", "other@example.com", "Other")
		status, _ := doGraphQL(t, srv, tok, `{ me { email } }`, nil)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("valid token returns current user", func(t *testing.T) {
		tok := issuer.Token(t, "auth-user", "auth@example.com", "Auth User")
		status, gr := doGraphQL(t, srv, tok, `{ me { id email } }`, nil)
		require.Equal(t, http.StatusOK, status)

		var data struct {
			Me struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			} `json:"me"`
		}
		decodeData(t, gr.Data, &data)
		require.NotEmpty(t, data.Me.ID)
		assert.Equal(t, "auth@example.com", data.Me.Email)
	})

	t.Run("admin email promotes user to admin", func(t *testing.T) {
		tok := issuer.Token(t, "admin-user", "auth@example.com", "Admin User")
		u, err := authenticator.authenticate(context.Background(), tok)
		require.NoError(t, err)
		assert.True(t, u.IsAdmin)
	})

	t.Run("jwks rotation is handled with forced refetch", func(t *testing.T) {
		tok1 := issuer.Token(t, "rot-user", "user-a@example.com", "Rot User")
		_, err := authenticator.authenticate(context.Background(), tok1)
		require.NoError(t, err)

		issuer.RotateKey(t)

		// The forced refresh is rate-limited; age the cached entry past the
		// minimum interval so the refetch is allowed to hit the issuer.
		authenticator.mu.Lock()
		authenticator.jwks[issuer.URL].fetchedAt = time.Now().Add(-jwksMinRefreshInterval)
		authenticator.mu.Unlock()

		tok2 := issuer.Token(t, "rot-user", "user-a@example.com", "Rot User")
		_, err = authenticator.authenticate(context.Background(), tok2)
		require.NoError(t, err)
	})
}

func runEndToEndTests(t *testing.T, srv *httptest.Server, issuer *testutil.TestIssuer) {
	tokA := issuer.Token(t, "user-a", "user-a@example.com", "User A")
	tokB := issuer.Token(t, "user-b", "user-b@example.com", "User B")

	// createBrand + brands query
	status, gr := doGraphQL(t, srv, tokA, `mutation { createBrand(input: { name: "Integration Brand" }) { id name } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var brandRes struct {
		CreateBrand struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"createBrand"`
	}
	decodeData(t, gr.Data, &brandRes)
	require.NotEmpty(t, brandRes.CreateBrand.ID)
	assert.Equal(t, "Integration Brand", brandRes.CreateBrand.Name)

	// brands is paginated (pageSize caps at 100) and the seed catalog is
	// large, so walk pages until the created brand appears.
	foundBrand := false
	for page := 1; !foundBrand; page++ {
		status, gr = doGraphQL(t, srv, tokA, `query Brands($p: Int!) { brands(page: $p, pageSize: 100) { items { id name } } }`, map[string]any{"p": page})
		require.Equal(t, http.StatusOK, status)
		var brandsRes struct {
			Brands struct {
				Items []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"items"`
			} `json:"brands"`
		}
		decodeData(t, gr.Data, &brandsRes)
		foundBrand = containsBrand(brandsRes.Brands.Items, brandRes.CreateBrand.ID)
		if len(brandsRes.Brands.Items) < 100 {
			break
		}
	}
	require.True(t, foundBrand, "created brand should appear in paged brands query")

	// createCategory + category query
	status, gr = doGraphQL(t, srv, tokA, `mutation { createCategory(input: { name: "Integration Category", description: "Milk products" }) { id name description } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var catRes struct {
		CreateCategory struct {
			ID          string  `json:"id"`
			Name        string  `json:"name"`
			Description *string `json:"description"`
		} `json:"createCategory"`
	}
	decodeData(t, gr.Data, &catRes)
	require.NotEmpty(t, catRes.CreateCategory.ID)
	assert.Equal(t, "Integration Category", catRes.CreateCategory.Name)

	status, gr = doGraphQL(t, srv, tokA, `query Category($id: ID!) { category(id: $id) { id name } }`, map[string]any{
		"id": catRes.CreateCategory.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var catQuery struct {
		Category *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"category"`
	}
	decodeData(t, gr.Data, &catQuery)
	require.NotNil(t, catQuery.Category)
	assert.Equal(t, catRes.CreateCategory.ID, catQuery.Category.ID)

	// createItem
	status, gr = doGraphQL(t, srv, tokA, `mutation CreateItem($input: CreateItemInput!) { createItem(input: $input) { id name category { id name } unit netWeight isMetric } }`, map[string]any{
		"input": map[string]any{
			"name":       "Integration Milk",
			"categoryId": catRes.CreateCategory.ID,
			"unit":       "gallon",
			"netWeight":  128.0,
			"isMetric":   false,
		},
	})
	require.Equal(t, http.StatusOK, status)
	var itemRes struct {
		CreateItem struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"category"`
			Unit string `json:"unit"`
		} `json:"createItem"`
	}
	decodeData(t, gr.Data, &itemRes)
	require.NotEmpty(t, itemRes.CreateItem.ID)
	assert.Equal(t, "Integration Milk", itemRes.CreateItem.Name)
	assert.Equal(t, catRes.CreateCategory.ID, itemRes.CreateItem.Category.ID)

	// updateItem
	status, gr = doGraphQL(t, srv, tokA, `mutation UpdateItem($id: ID!, $input: UpdateItemInput!) { updateItem(id: $id, input: $input) { id name unit } }`, map[string]any{
		"id": itemRes.CreateItem.ID,
		"input": map[string]any{
			"name": "Updated Milk",
		},
	})
	require.Equal(t, http.StatusOK, status)
	var updateItemRes struct {
		UpdateItem struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Unit string `json:"unit"`
		} `json:"updateItem"`
	}
	decodeData(t, gr.Data, &updateItemRes)
	assert.Equal(t, "Updated Milk", updateItemRes.UpdateItem.Name)
	assert.Equal(t, itemRes.CreateItem.Unit, updateItemRes.UpdateItem.Unit)

	// wine reference data + bottle
	status, gr = doGraphQL(t, srv, tokA, `mutation { createType(input: { name: "Integration Type", description: "Red wine" }) { id name } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var typeRes struct {
		CreateType struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"createType"`
	}
	decodeData(t, gr.Data, &typeRes)
	require.NotEmpty(t, typeRes.CreateType.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation { createCountry(input: { name: "Integrationland", isoCode: "INT", description: "USA" }) { id name isoCode } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var countryRes struct {
		CreateCountry struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			IsoCode string `json:"isoCode"`
		} `json:"createCountry"`
	}
	decodeData(t, gr.Data, &countryRes)
	require.NotEmpty(t, countryRes.CreateCountry.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation CreateRegion($input: CreateRegionInput!) { createRegion(input: $input) { id name country { id name } } }`, map[string]any{
		"input": map[string]any{
			"countryId": countryRes.CreateCountry.ID,
			"name":      "Integration Valley",
		},
	})
	require.Equal(t, http.StatusOK, status)
	var regionRes struct {
		CreateRegion struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Country struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"country"`
		} `json:"createRegion"`
	}
	decodeData(t, gr.Data, &regionRes)
	require.NotEmpty(t, regionRes.CreateRegion.ID)
	assert.Equal(t, countryRes.CreateCountry.ID, regionRes.CreateRegion.Country.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation { createVintage(input: { year: 1999, description: "Great year" }) { id year } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var vintageRes struct {
		CreateVintage struct {
			ID   string `json:"id"`
			Year int    `json:"year"`
		} `json:"createVintage"`
	}
	decodeData(t, gr.Data, &vintageRes)
	require.NotEmpty(t, vintageRes.CreateVintage.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation { createGrapeVariety(input: { name: "Integration Grape" }) { id name } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var grapeRes struct {
		CreateGrapeVariety struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"createGrapeVariety"`
	}
	decodeData(t, gr.Data, &grapeRes)
	require.NotEmpty(t, grapeRes.CreateGrapeVariety.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation { createWineFlavorProfile(input: { name: "Integration Oaky", description: "Oaky profile" }) { id name } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var wineFlavRes struct {
		CreateWineFlavorProfile struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"createWineFlavorProfile"`
	}
	decodeData(t, gr.Data, &wineFlavRes)
	require.NotEmpty(t, wineFlavRes.CreateWineFlavorProfile.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation CreateBottle($input: CreateBottleInput!) { createBottle(input: $input) { id typeId countryId regionId vintageYear bottleSize } }`, map[string]any{
		"input": map[string]any{
			"typeId":      typeRes.CreateType.ID,
			"countryId":   countryRes.CreateCountry.ID,
			"regionId":    regionRes.CreateRegion.ID,
			"vintageYear": 1999,
			"bottleSize":  "750ml",
		},
	})
	require.Equal(t, http.StatusOK, status)
	var bottleRes struct {
		CreateBottle struct {
			ID          string `json:"id"`
			TypeID      string `json:"typeId"`
			CountryID   string `json:"countryId"`
			RegionID    string `json:"regionId"`
			VintageYear int    `json:"vintageYear"`
			BottleSize  string `json:"bottleSize"`
		} `json:"createBottle"`
	}
	decodeData(t, gr.Data, &bottleRes)
	require.NotEmpty(t, bottleRes.CreateBottle.ID)
	assert.Equal(t, typeRes.CreateType.ID, bottleRes.CreateBottle.TypeID)

	status, gr = doGraphQL(t, srv, tokA, `query Bottle($id: ID!) { bottle(id: $id) { id typeId countryId regionId vintageYear bottleSize } }`, map[string]any{
		"id": bottleRes.CreateBottle.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var bottleQuery struct {
		Bottle *struct {
			ID          string `json:"id"`
			TypeID      string `json:"typeId"`
			CountryID   string `json:"countryId"`
			RegionID    string `json:"regionId"`
			VintageYear int    `json:"vintageYear"`
			BottleSize  string `json:"bottleSize"`
		} `json:"bottle"`
	}
	decodeData(t, gr.Data, &bottleQuery)
	require.NotNil(t, bottleQuery.Bottle)
	assert.Equal(t, bottleRes.CreateBottle.ID, bottleQuery.Bottle.ID)

	// createRecipe with items and steps
	status, gr = doGraphQL(t, srv, tokA, `mutation CreateRecipe($input: CreateRecipeInput!) { createRecipe(input: $input) { id name items { item { id name } quantity unit } steps { stepNumber instruction } } }`, map[string]any{
		"input": map[string]any{
			"name":            "Integration Pancakes",
			"description":     "Fluffy pancakes",
			"servings":        4,
			"prepTimeMinutes": 10,
			"cookTimeMinutes": 15,
			"items": []map[string]any{
				{
					"itemId":     itemRes.CreateItem.ID,
					"quantity":   2.0,
					"unit":       "cup",
					"notes":      "sifted",
					"isOptional": false,
				},
			},
			"steps": []map[string]any{
				{"stepNumber": 1, "instruction": "Mix dry ingredients"},
			},
		},
	})
	require.Equal(t, http.StatusOK, status)
	var recipeRes struct {
		CreateRecipe struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Items []struct {
				Item struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"item"`
				Quantity float64 `json:"quantity"`
				Unit     string  `json:"unit"`
			} `json:"items"`
			Steps []struct {
				StepNumber  int32  `json:"stepNumber"`
				Instruction string `json:"instruction"`
			} `json:"steps"`
		} `json:"createRecipe"`
	}
	decodeData(t, gr.Data, &recipeRes)
	require.NotEmpty(t, recipeRes.CreateRecipe.ID)
	assert.Equal(t, "Integration Pancakes", recipeRes.CreateRecipe.Name)
	assert.Len(t, recipeRes.CreateRecipe.Items, 1)
	assert.Equal(t, itemRes.CreateItem.ID, recipeRes.CreateRecipe.Items[0].Item.ID)
	assert.Len(t, recipeRes.CreateRecipe.Steps, 1)

	// setRecipeFavorite + query isFavorite
	status, gr = doGraphQL(t, srv, tokA, `mutation SetRecipeFavorite($recipeId: ID!) { setRecipeFavorite(recipeId: $recipeId, isFavorite: true) }`, map[string]any{
		"recipeId": recipeRes.CreateRecipe.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var favRes struct {
		SetRecipeFavorite bool `json:"setRecipeFavorite"`
	}
	decodeData(t, gr.Data, &favRes)
	require.True(t, favRes.SetRecipeFavorite)

	status, gr = doGraphQL(t, srv, tokA, `query Recipe($id: ID!) { recipe(id: $id) { id name isFavorite } }`, map[string]any{
		"id": recipeRes.CreateRecipe.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var recipeQuery struct {
		Recipe *struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			IsFavorite bool   `json:"isFavorite"`
		} `json:"recipe"`
	}
	decodeData(t, gr.Data, &recipeQuery)
	require.NotNil(t, recipeQuery.Recipe)
	assert.True(t, recipeQuery.Recipe.IsFavorite)

	// createMealPlan, addMealSlot, addMealSlotItem
	status, gr = doGraphQL(t, srv, tokA, `mutation CreateMealPlan($input: CreateMealPlanInput!) { createMealPlan(input: $input) { id name weekStartDate isActive } }`, map[string]any{
		"input": map[string]any{
			"name":               "Integration Plan",
			"weekStartDate":      "2025-01-06",
			"weekStartDayOfWeek": 1,
		},
	})
	require.Equal(t, http.StatusOK, status)
	var mealPlanRes struct {
		CreateMealPlan struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			WeekStartDate string `json:"weekStartDate"`
			IsActive      bool   `json:"isActive"`
		} `json:"createMealPlan"`
	}
	decodeData(t, gr.Data, &mealPlanRes)
	require.NotEmpty(t, mealPlanRes.CreateMealPlan.ID)
	assert.Equal(t, "Integration Plan", mealPlanRes.CreateMealPlan.Name)

	status, gr = doGraphQL(t, srv, tokA, `mutation AddMealSlot($input: AddMealSlotInput!) { addMealSlot(input: $input) { id dayOfWeek mealType recipe { id name } } }`, map[string]any{
		"input": map[string]any{
			"mealPlanId": mealPlanRes.CreateMealPlan.ID,
			"dayOfWeek":  1,
			"mealType":   "Dinner",
			"recipeId":   recipeRes.CreateRecipe.ID,
			"servings":   4,
		},
	})
	require.Equal(t, http.StatusOK, status)
	var slotRes struct {
		AddMealSlot struct {
			ID        string `json:"id"`
			DayOfWeek int    `json:"dayOfWeek"`
			MealType  string `json:"mealType"`
			Recipe    *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"recipe"`
		} `json:"addMealSlot"`
	}
	decodeData(t, gr.Data, &slotRes)
	require.NotEmpty(t, slotRes.AddMealSlot.ID)
	require.NotNil(t, slotRes.AddMealSlot.Recipe)
	assert.Equal(t, recipeRes.CreateRecipe.ID, slotRes.AddMealSlot.Recipe.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation AddMealSlotItem($input: AddMealSlotItemInput!) { addMealSlotItem(input: $input) { id item { id name } quantity unit } }`, map[string]any{
		"input": map[string]any{
			"slotId":       slotRes.AddMealSlot.ID,
			"itemId":       itemRes.CreateItem.ID,
			"quantity":     1.0,
			"unit":         "cup",
			"isFromRecipe": false,
		},
	})
	require.Equal(t, http.StatusOK, status)
	var slotItemRes struct {
		AddMealSlotItem struct {
			ID       string  `json:"id"`
			Quantity float64 `json:"quantity"`
			Unit     string  `json:"unit"`
			Item     *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"item"`
		} `json:"addMealSlotItem"`
	}
	decodeData(t, gr.Data, &slotItemRes)
	require.NotEmpty(t, slotItemRes.AddMealSlotItem.ID)
	assert.Equal(t, itemRes.CreateItem.ID, slotItemRes.AddMealSlotItem.Item.ID)

	status, gr = doGraphQL(t, srv, tokA, `query MealPlan($id: ID!) { mealPlan(id: $id) { id name slots { id dayOfWeek mealType items { id quantity unit } } } }`, map[string]any{
		"id": mealPlanRes.CreateMealPlan.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var mealPlanQuery struct {
		MealPlan *struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Slots []struct {
				ID        string `json:"id"`
				DayOfWeek int    `json:"dayOfWeek"`
				MealType  string `json:"mealType"`
				Items     []struct {
					ID       string  `json:"id"`
					Quantity float64 `json:"quantity"`
					Unit     string  `json:"unit"`
				} `json:"items"`
			} `json:"slots"`
		} `json:"mealPlan"`
	}
	decodeData(t, gr.Data, &mealPlanQuery)
	require.NotNil(t, mealPlanQuery.MealPlan)
	assert.Equal(t, mealPlanRes.CreateMealPlan.ID, mealPlanQuery.MealPlan.ID)
	require.Len(t, mealPlanQuery.MealPlan.Slots, 1)
	require.Len(t, mealPlanQuery.MealPlan.Slots[0].Items, 1)

	// generate grocery list, add manual item, toggle checked
	status, gr = doGraphQL(t, srv, tokA, `mutation GenerateGroceryList($mealPlanId: ID!) { generateGroceryList(mealPlanId: $mealPlanId) { id items { id source isChecked } } }`, map[string]any{
		"mealPlanId": mealPlanRes.CreateMealPlan.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var groceryListRes struct {
		GenerateGroceryList struct {
			ID    string `json:"id"`
			Items []struct {
				ID        string `json:"id"`
				Source    string `json:"source"`
				IsChecked bool   `json:"isChecked"`
			} `json:"items"`
		} `json:"generateGroceryList"`
	}
	decodeData(t, gr.Data, &groceryListRes)
	require.NotEmpty(t, groceryListRes.GenerateGroceryList.ID)

	status, gr = doGraphQL(t, srv, tokA, `mutation AddGroceryItem($input: AddGroceryItemInput!) { addGroceryItem(input: $input) { id manualItemName quantityNeeded unitOfMeasure isChecked source } }`, map[string]any{
		"input": map[string]any{
			"groceryListId":  groceryListRes.GenerateGroceryList.ID,
			"manualItemName": "Bananas",
			"quantity":       2.0,
			"unit":           "bunch",
		},
	})
	require.Equal(t, http.StatusOK, status)
	var groceryItemRes struct {
		AddGroceryItem struct {
			ID             string  `json:"id"`
			ManualItemName string  `json:"manualItemName"`
			QuantityNeeded float64 `json:"quantityNeeded"`
			UnitOfMeasure  string  `json:"unitOfMeasure"`
			IsChecked      bool    `json:"isChecked"`
			Source         string  `json:"source"`
		} `json:"addGroceryItem"`
	}
	decodeData(t, gr.Data, &groceryItemRes)
	require.NotEmpty(t, groceryItemRes.AddGroceryItem.ID)
	assert.False(t, groceryItemRes.AddGroceryItem.IsChecked)

	status, gr = doGraphQL(t, srv, tokA, `mutation ToggleGroceryItem($groceryListItemId: ID!) { toggleGroceryItemChecked(groceryListItemId: $groceryListItemId) { id isChecked } }`, map[string]any{
		"groceryListItemId": groceryItemRes.AddGroceryItem.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var toggleRes struct {
		ToggleGroceryItemChecked struct {
			ID        string `json:"id"`
			IsChecked bool   `json:"isChecked"`
		} `json:"toggleGroceryItemChecked"`
	}
	decodeData(t, gr.Data, &toggleRes)
	assert.True(t, toggleRes.ToggleGroceryItemChecked.IsChecked)

	status, gr = doGraphQL(t, srv, tokA, `query GroceryList($id: ID!) { groceryList(id: $id) { id items { id manualItemName quantityNeeded unitOfMeasure isChecked } } }`, map[string]any{
		"id": groceryListRes.GenerateGroceryList.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var groceryListQuery struct {
		GroceryList *struct {
			ID    string `json:"id"`
			Items []struct {
				ID             string  `json:"id"`
				ManualItemName string  `json:"manualItemName"`
				QuantityNeeded float64 `json:"quantityNeeded"`
				UnitOfMeasure  string  `json:"unitOfMeasure"`
				IsChecked      bool    `json:"isChecked"`
			} `json:"items"`
		} `json:"groceryList"`
	}
	decodeData(t, gr.Data, &groceryListQuery)
	require.NotNil(t, groceryListQuery.GroceryList)
	assert.True(t, findGroceryItemChecked(groceryListQuery.GroceryList.Items, groceryItemRes.AddGroceryItem.ID))

	// adjustUserItem + userItems
	status, gr = doGraphQL(t, srv, tokA, `mutation AdjustUserItem($itemId: ID!) { adjustUserItem(itemId: $itemId, quantity: 5.0) { id currentQty item { id name } } }`, map[string]any{
		"itemId": itemRes.CreateItem.ID,
	})
	require.Equal(t, http.StatusOK, status)
	var userItemRes struct {
		AdjustUserItem struct {
			ID         string  `json:"id"`
			CurrentQty float64 `json:"currentQty"`
			Item       struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"item"`
		} `json:"adjustUserItem"`
	}
	decodeData(t, gr.Data, &userItemRes)
	require.NotEmpty(t, userItemRes.AdjustUserItem.ID)
	assert.InDelta(t, 5.0, userItemRes.AdjustUserItem.CurrentQty, 0.001)

	status, gr = doGraphQL(t, srv, tokA, `{ userItems { items { id currentQty item { id name } } pageInfo { totalCount } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var userItemsRes struct {
		UserItems struct {
			Items []struct {
				ID         string  `json:"id"`
				CurrentQty float64 `json:"currentQty"`
				Item       struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"item"`
			} `json:"items"`
			PageInfo struct {
				TotalCount int `json:"totalCount"`
			} `json:"pageInfo"`
		} `json:"userItems"`
	}
	decodeData(t, gr.Data, &userItemsRes)
	require.True(t, containsUserItemID(userItemsRes.UserItems.Items, userItemRes.AdjustUserItem.ID))

	// cross-user isolation: user B cannot read user A's meal plan —
	// not-found is indistinguishable from foreign-owned, so the wire
	// answer is a NOT_FOUND error with null data.
	status, gr = doGraphQLExpectErrors(t, srv, tokB, `query MealPlan($id: ID!) { mealPlan(id: $id) { id name } }`, map[string]any{
		"id": mealPlanRes.CreateMealPlan.ID,
	})
	require.Equal(t, http.StatusOK, status)
	require.Len(t, gr.Errors, 1)
	assert.Equal(t, codeNotFound, gr.Errors[0].Extensions["code"])
	var crossUser struct {
		MealPlan *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"mealPlan"`
	}
	decodeData(t, gr.Data, &crossUser)
	assert.Nil(t, crossUser.MealPlan)

	status, gr = doGraphQL(t, srv, tokB, `{ mealPlans { items { id } pageInfo { totalCount } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var bMealPlans struct {
		MealPlans struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
			PageInfo struct {
				TotalCount int `json:"totalCount"`
			} `json:"pageInfo"`
		} `json:"mealPlans"`
	}
	decodeData(t, gr.Data, &bMealPlans)
	assert.False(t, containsID(bMealPlans.MealPlans.Items, mealPlanRes.CreateMealPlan.ID))
}

// runHouseholdTests exercises the household lifecycle over GraphQL:
// search → invite → accept → shared pantry visibility → leave. It uses
// fresh users so joining a household cannot disturb the cross-user
// isolation assertions in runEndToEndTests.
func runHouseholdTests(t *testing.T, srv *httptest.Server, issuer *testutil.TestIssuer) {
	tokC := issuer.Token(t, "hh-c", "hh-c@example.com", "HH C")
	tokD := issuer.Token(t, "hh-d", "hh-d@example.com", "HH D")

	meID := func(token string) string {
		status, gr := doGraphQL(t, srv, token, `{ me { id isSearchable household { id myRole members { user { id displayName } role isMe } } } }`, nil)
		require.Equal(t, http.StatusOK, status)
		var res struct {
			Me struct {
				ID           string `json:"id"`
				IsSearchable bool   `json:"isSearchable"`
				Household    *struct {
					ID      string `json:"id"`
					MyRole  string `json:"myRole"`
					Members []struct {
						User struct {
							ID          string `json:"id"`
							DisplayName string `json:"displayName"`
						} `json:"user"`
						Role string `json:"role"`
						IsMe bool   `json:"isMe"`
					} `json:"members"`
				} `json:"household"`
			} `json:"me"`
		}
		decodeData(t, gr.Data, &res)
		return res.Me.ID
	}
	idC := meID(tokC)
	idD := meID(tokD)

	// D is searchable by default; C finds them by term.
	status, gr := doGraphQL(t, srv, tokC, `query Search($term: String!) {
		searchHouseholdUsers(term: $term) { id displayName }
	}`, map[string]any{"term": "hh-d"})
	require.Equal(t, http.StatusOK, status)
	var searchRes struct {
		Search []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"searchHouseholdUsers"`
	}
	decodeData(t, gr.Data, &searchRes)
	require.Len(t, searchRes.Search, 1)
	assert.Equal(t, idD, searchRes.Search[0].ID)

	// C invites D.
	status, gr = doGraphQL(t, srv, tokC, `mutation Invite($userId: ID!) {
		inviteHouseholdMember(userId: $userId) { id status fromUser { id } toUser { id } }
	}`, map[string]any{"userId": idD})
	require.Equal(t, http.StatusOK, status)
	var inviteRes struct {
		Invite struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			FromUser struct {
				ID string `json:"id"`
			} `json:"fromUser"`
			ToUser struct {
				ID string `json:"id"`
			} `json:"toUser"`
		} `json:"inviteHouseholdMember"`
	}
	decodeData(t, gr.Data, &inviteRes)
	assert.Equal(t, "PENDING", inviteRes.Invite.Status)
	assert.Equal(t, idC, inviteRes.Invite.FromUser.ID)
	assert.Equal(t, idD, inviteRes.Invite.ToUser.ID)

	// D sees the pending invite and accepts it.
	status, gr = doGraphQL(t, srv, tokD, `{ householdInvites { id status } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var invitesRes struct {
		Invites []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"householdInvites"`
	}
	decodeData(t, gr.Data, &invitesRes)
	require.Len(t, invitesRes.Invites, 1)

	status, gr = doGraphQL(t, srv, tokD, `mutation Accept($inviteId: ID!) {
		acceptHouseholdInvite(inviteId: $inviteId) { id myRole members { user { id } role isMe } }
	}`, map[string]any{"inviteId": inviteRes.Invite.ID})
	require.Equal(t, http.StatusOK, status)
	var acceptRes struct {
		Accept struct {
			ID      string `json:"id"`
			MyRole  string `json:"myRole"`
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
				Role string `json:"role"`
				IsMe bool   `json:"isMe"`
			} `json:"members"`
		} `json:"acceptHouseholdInvite"`
	}
	decodeData(t, gr.Data, &acceptRes)
	require.Len(t, acceptRes.Accept.Members, 2)
	// D joins as a member; C remains the owner.
	assert.Equal(t, "MEMBER", acceptRes.Accept.MyRole)
	roles := map[string]string{}
	for _, m := range acceptRes.Accept.Members {
		roles[m.User.ID] = m.Role
	}
	assert.Equal(t, "OWNER", roles[idC])
	assert.Equal(t, "MEMBER", roles[idD])

	// The invite fan-out produced an unread notification for C
	// (invite_accepted); D's feed holds invite_received.
	status, gr = doGraphQL(t, srv, tokC, `{ unreadNotificationCount }`, nil)
	require.Equal(t, http.StatusOK, status)
	var unreadC struct {
		Count int `json:"unreadNotificationCount"`
	}
	decodeData(t, gr.Data, &unreadC)
	assert.Equal(t, 1, unreadC.Count)
	status, gr = doGraphQL(t, srv, tokC, `mutation { markAllNotificationsRead }`, nil)
	require.Equal(t, http.StatusOK, status)

	// C puts an item in the shared pantry; D sees it.
	status, gr = doGraphQL(t, srv, tokC, `{ items(page: 1, pageSize: 1) { items { id } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var itemsRes struct {
		Items struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"items"`
	}
	decodeData(t, gr.Data, &itemsRes)
	require.NotEmpty(t, itemsRes.Items.Items)
	itemID := itemsRes.Items.Items[0].ID

	status, gr = doGraphQL(t, srv, tokC, `mutation Adjust($itemId: ID!) {
		adjustUserItem(itemId: $itemId, quantity: 3.0) { id currentQty }
	}`, map[string]any{"itemId": itemID})
	require.Equal(t, http.StatusOK, status)

	status, gr = doGraphQL(t, srv, tokD, `{ userItems { items { currentQty } pageInfo { totalCount } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var pantryRes struct {
		UserItems struct {
			Items []struct {
				CurrentQty float64 `json:"currentQty"`
			} `json:"items"`
			PageInfo struct {
				TotalCount int `json:"totalCount"`
			} `json:"pageInfo"`
		} `json:"userItems"`
	}
	decodeData(t, gr.Data, &pantryRes)
	require.Equal(t, 1, pantryRes.UserItems.PageInfo.TotalCount, "household member should see shared pantry stock")
	assert.InDelta(t, 3.0, pantryRes.UserItems.Items[0].CurrentQty, 0.001)

	// D leaves: fresh household of one, shared stock stays behind.
	status, gr = doGraphQL(t, srv, tokD, `mutation { leaveHousehold }`, nil)
	require.Equal(t, http.StatusOK, status)

	status, gr = doGraphQL(t, srv, tokD, `{ myHousehold { id myRole members { user { id } role } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var myHH struct {
		MyHousehold *struct {
			ID      string `json:"id"`
			MyRole  string `json:"myRole"`
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
				Role string `json:"role"`
			} `json:"members"`
		} `json:"myHousehold"`
	}
	decodeData(t, gr.Data, &myHH)
	require.NotNil(t, myHH.MyHousehold)
	require.Len(t, myHH.MyHousehold.Members, 1)
	assert.Equal(t, "OWNER", myHH.MyHousehold.MyRole)
	assert.NotEqual(t, acceptRes.Accept.ID, myHH.MyHousehold.ID)

	// C was notified that D left.
	status, gr = doGraphQL(t, srv, tokC, `{ unreadNotificationCount }`, nil)
	require.Equal(t, http.StatusOK, status)
	decodeData(t, gr.Data, &unreadC)
	assert.Equal(t, 1, unreadC.Count)

	status, gr = doGraphQL(t, srv, tokD, `{ userItems { pageInfo { totalCount } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var afterLeave struct {
		UserItems struct {
			PageInfo struct {
				TotalCount int `json:"totalCount"`
			} `json:"pageInfo"`
		} `json:"userItems"`
	}
	decodeData(t, gr.Data, &afterLeave)
	assert.Equal(t, 0, afterLeave.UserItems.PageInfo.TotalCount)
}

// runHouseholdManagementTests exercises the multi-member and role surface:
// a three-member household, owner rename/setRole/transfer, the admin
// boundary, member removal, and the notification trail each step leaves.
func runHouseholdManagementTests(t *testing.T, srv *httptest.Server, issuer *testutil.TestIssuer) {
	tokE := issuer.Token(t, "hh-e", "hh-e@example.com", "HH E")
	tokF := issuer.Token(t, "hh-f", "hh-f@example.com", "HH F")
	tokG := issuer.Token(t, "hh-g", "hh-g@example.com", "HH G")

	meID := func(token string) string {
		status, gr := doGraphQL(t, srv, token, `{ me { id } }`, nil)
		require.Equal(t, http.StatusOK, status)
		var res struct {
			Me struct {
				ID string `json:"id"`
			} `json:"me"`
		}
		decodeData(t, gr.Data, &res)
		return res.Me.ID
	}
	idE, idF, idG := meID(tokE), meID(tokF), meID(tokG)

	inviteAndAccept := func(ownerTok, memberTok, memberID string) {
		status, gr := doGraphQL(t, srv, ownerTok, `mutation Invite($userId: ID!) {
			inviteHouseholdMember(userId: $userId) { id }
		}`, map[string]any{"userId": memberID})
		require.Equal(t, http.StatusOK, status)
		var inv struct {
			Invite struct {
				ID string `json:"id"`
			} `json:"inviteHouseholdMember"`
		}
		decodeData(t, gr.Data, &inv)
		status, gr = doGraphQL(t, srv, memberTok, `mutation Accept($inviteId: ID!) {
			acceptHouseholdInvite(inviteId: $inviteId) { id }
		}`, map[string]any{"inviteId": inv.Invite.ID})
		require.Equal(t, http.StatusOK, status)
	}
	inviteAndAccept(tokE, tokF, idF)
	inviteAndAccept(tokE, tokG, idG)

	// Three-member household: E owner, F and G members.
	status, gr := doGraphQL(t, srv, tokE, `{ myHousehold { name myRole members { user { id } role } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var hhRes struct {
		Household struct {
			Name    *string `json:"name"`
			MyRole  string  `json:"myRole"`
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
				Role string `json:"role"`
			} `json:"members"`
		} `json:"myHousehold"`
	}
	decodeData(t, gr.Data, &hhRes)
	require.Len(t, hhRes.Household.Members, 3)
	assert.Equal(t, "OWNER", hhRes.Household.MyRole)
	assert.Nil(t, hhRes.Household.Name)

	// E renames; F's feed records the event.
	status, gr = doGraphQL(t, srv, tokE, `mutation { renameHousehold(name: "Casa E") { name } }`, nil)
	require.Equal(t, http.StatusOK, status)
	status, gr = doGraphQL(t, srv, tokF, `{ myNotifications { kind actor { id } } unreadNotificationCount }`, nil)
	require.Equal(t, http.StatusOK, status)
	var notifRes struct {
		Notifs []struct {
			Kind  string `json:"kind"`
			Actor *struct {
				ID string `json:"id"`
			} `json:"actor"`
		} `json:"myNotifications"`
		Unread int `json:"unreadNotificationCount"`
	}
	decodeData(t, gr.Data, &notifRes)
	kinds := map[string]bool{}
	for _, n := range notifRes.Notifs {
		kinds[n.Kind] = true
	}
	assert.True(t, kinds["HOUSEHOLD_RENAMED"], "rename must fan out to members")
	assert.True(t, kinds["MEMBER_JOINED"], "accepts must notify existing members")
	assert.Greater(t, notifRes.Unread, 0)

	// E promotes G to admin.
	status, gr = doGraphQL(t, srv, tokE, `mutation SetRole($userId: ID!) {
		setHouseholdRole(userId: $userId, role: ADMIN) { members { user { id } role } }
	}`, map[string]any{"userId": idG})
	require.Equal(t, http.StatusOK, status)
	var roleRes struct {
		HH struct {
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
				Role string `json:"role"`
			} `json:"members"`
		} `json:"setHouseholdRole"`
	}
	decodeData(t, gr.Data, &roleRes)
	roles := map[string]string{}
	for _, m := range roleRes.HH.Members {
		roles[m.User.ID] = m.Role
	}
	assert.Equal(t, "ADMIN", roles[idG])
	assert.Equal(t, "OWNER", roles[idE])

	// Admins cannot transfer ownership or set roles — owner-only.
	_, grErr := doGraphQLExpectErrors(t, srv, tokG, `mutation Transfer($userId: ID!) {
		transferHouseholdOwnership(userId: $userId) { id }
	}`, map[string]any{"userId": idF})
	require.NotEmpty(t, grErr.Errors)
	assert.Equal(t, codeForbidden, grErr.Errors[0].Extensions["code"])

	// Admins can remove a member: G removes F.
	status, gr = doGraphQL(t, srv, tokG, `mutation Remove($userId: ID!) {
		removeHouseholdMember(userId: $userId) { members { user { id } } }
	}`, map[string]any{"userId": idF})
	require.Equal(t, http.StatusOK, status)
	var removeRes struct {
		HH struct {
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			} `json:"members"`
		} `json:"removeHouseholdMember"`
	}
	decodeData(t, gr.Data, &removeRes)
	require.Len(t, removeRes.HH.Members, 2)

	// F lands in a fresh single-member household with a removal notice.
	status, gr = doGraphQL(t, srv, tokF, `{ myHousehold { myRole } myNotifications { kind } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var fRes struct {
		HH *struct {
			MyRole string `json:"myRole"`
		} `json:"myHousehold"`
		Notifs []struct {
			Kind string `json:"kind"`
		} `json:"myNotifications"`
	}
	decodeData(t, gr.Data, &fRes)
	require.NotNil(t, fRes.HH)
	assert.Equal(t, "OWNER", fRes.HH.MyRole)
	fKinds := map[string]bool{}
	for _, n := range fRes.Notifs {
		fKinds[n.Kind] = true
	}
	assert.True(t, fKinds["MEMBER_REMOVED"], "removed member must be notified")

	// E transfers ownership to G, then leaves as a member.
	status, gr = doGraphQL(t, srv, tokE, `mutation Transfer($userId: ID!) {
		transferHouseholdOwnership(userId: $userId) { myRole members { user { id } role } }
	}`, map[string]any{"userId": idG})
	require.Equal(t, http.StatusOK, status)
	var transferRes struct {
		HH struct {
			MyRole  string `json:"myRole"`
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
				Role string `json:"role"`
			} `json:"members"`
		} `json:"transferHouseholdOwnership"`
	}
	decodeData(t, gr.Data, &transferRes)
	assert.Equal(t, "MEMBER", transferRes.HH.MyRole)
	roles = map[string]string{}
	for _, m := range transferRes.HH.Members {
		roles[m.User.ID] = m.Role
	}
	assert.Equal(t, "OWNER", roles[idG])

	status, gr = doGraphQL(t, srv, tokE, `mutation { leaveHousehold }`, nil)
	require.Equal(t, http.StatusOK, status)

	// G is sole owner of the remaining household; the rename persisted.
	status, gr = doGraphQL(t, srv, tokG, `{ myHousehold { name myRole members { user { id } } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var gRes struct {
		HH struct {
			Name    *string `json:"name"`
			MyRole  string  `json:"myRole"`
			Members []struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			} `json:"members"`
		} `json:"myHousehold"`
	}
	decodeData(t, gr.Data, &gRes)
	require.Len(t, gRes.HH.Members, 1)
	assert.Equal(t, "OWNER", gRes.HH.MyRole)
	require.NotNil(t, gRes.HH.Name)
	assert.Equal(t, "Casa E", *gRes.HH.Name)
}

// TestIntegrationGroceryTogglePantrySync exercises the atomic toggle +
// pantry adjustment path in ToggleGroceryItemChecked (A2-01): checking a
// catalog item must add its quantity to the user's pantry, and
// unchecking must subtract it back — both inside one transaction.
func TestIntegrationGroceryTogglePantrySync(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	defer cleanup()

	invSvc := inventory.NewService(pool)
	grocerySvc := grocery.NewService(pool)
	upSvc := userprefs.NewService(pool)

	userID := testutil.MustUser(ctx, t, pool, "toggle-sync@example.com")

	brand, err := invSvc.CreateBrand(ctx, "IT Toggle Brand", "it")
	require.NoError(t, err)
	cat, err := invSvc.CreateCategory(ctx, "IT Toggle Category", "", "it")
	require.NoError(t, err)
	unit, err := invSvc.GetUnitByName(ctx, "each")
	require.NoError(t, err)
	netWeight := 1.0
	item, err := invSvc.CreateItem(ctx, inventory.Item{
		Name:       "IT Toggle Item",
		BrandID:    &brand.BrandID,
		CategoryID: cat.CategoryID,
		UnitID:     unit.UnitID,
		NetWeight:  &netWeight,
	}, "it")
	require.NoError(t, err)

	list, err := grocerySvc.CreateGroceryList(ctx, userID, nil, "it")
	require.NoError(t, err)
	gli, err := grocerySvc.AddGroceryListItem(ctx, grocery.GroceryListItem{
		GroceryListID:  list.GroceryListID,
		ItemID:         &item.ItemID,
		QuantityNeeded: 3.0,
		UnitID:         &item.UnitID,
	}, userID, "it")
	require.NoError(t, err)

	resolver := NewResolver(pool, Services{
		Analytics: analytics.NewService(pool),
		Grocery:   grocerySvc,
		Inventory: invSvc,
		MealPlan:  mealplan.NewService(pool),
		Recipe:    recipe.NewService(pool),
		UserPrefs: upSvc,
		Wine:      wine.NewService(pool),
		Identity:  identity.NewService(pool),
	}, Options{})

	uctx := currentuser.WithUser(ctx, currentuser.User{UserID: userID, HouseholdID: userID, Email: "toggle-sync@example.com"})
	gliID := graphql.ID(strconv.FormatInt(gli.GroceryListItemID, 10))

	toggled, err := resolver.ToggleGroceryItemChecked(uctx, struct{ GroceryListItemID graphql.ID }{GroceryListItemID: gliID})
	require.NoError(t, err)
	assert.True(t, toggled.IsChecked())

	pantry, err := upSvc.GetHouseholdItemByItem(ctx, userID, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, pantry, "checking the item should create a pantry row")
	assert.InDelta(t, 3.0, pantry.CurrentQty, 0.0001)

	untoggled, err := resolver.ToggleGroceryItemChecked(uctx, struct{ GroceryListItemID graphql.ID }{GroceryListItemID: gliID})
	require.NoError(t, err)
	assert.False(t, untoggled.IsChecked())

	pantry, err = upSvc.GetHouseholdItemByItem(ctx, userID, item.ItemID)
	require.NoError(t, err)
	require.NotNil(t, pantry)
	assert.InDelta(t, 0.0, pantry.CurrentQty, 0.0001)
}

// TestIntegrationGenerateGroceryList exercises the composed generation
// path (A1-04): two recipes sharing an ingredient produce one aggregated
// grocery line with the summed quantity minus pantry stock, all inside
// one unit of work.
func TestIntegrationGenerateGroceryList(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	pool, cleanup, err := testutil.NewTestDB(t, ctx)
	require.NoError(t, err)
	defer cleanup()

	invSvc := inventory.NewService(pool)
	grocerySvc := grocery.NewService(pool)
	mpSvc := mealplan.NewService(pool)
	recipeSvc := recipe.NewService(pool)
	upSvc := userprefs.NewService(pool)

	userID := testutil.MustUser(ctx, t, pool, "grocery-gen@example.com")

	brand, err := invSvc.CreateBrand(ctx, "IT Gen Brand", "it")
	require.NoError(t, err)
	cat, err := invSvc.CreateCategory(ctx, "IT Gen Category", "", "it")
	require.NoError(t, err)
	kg, err := invSvc.GetUnitByName(ctx, "kilogram")
	require.NoError(t, err)
	flour, err := invSvc.CreateItem(ctx, inventory.Item{
		Name: "IT Gen Flour", BrandID: &brand.BrandID, CategoryID: cat.CategoryID, UnitID: kg.UnitID,
	}, "it")
	require.NoError(t, err)

	servings := int32(4)
	mkRecipe := func(name string, flourKg float64) recipe.Recipe {
		rec, err := recipeSvc.CreateRecipeWithChildren(ctx,
			recipe.Recipe{Name: name, IsActive: true, Servings: &servings},
			[]recipe.RecipeItem{{ItemID: flour.ItemID, Quantity: flourKg, UnitID: kg.UnitID}},
			nil, "it")
		require.NoError(t, err)
		return rec
	}
	bread := mkRecipe("IT Gen Bread", 2)
	cake := mkRecipe("IT Gen Cake", 1)

	plan, err := mpSvc.CreateMealPlan(ctx, mealplan.MealPlan{
		HouseholdID: userID, Name: "IT Gen Plan", WeekStartDate: time.Now(), IsActive: true,
	}, "it")
	require.NoError(t, err)
	for i, rec := range []recipe.Recipe{bread, cake} {
		_, err = mpSvc.AddMealSlot(ctx, mealplan.MealSlot{
			MealPlanID: plan.MealPlanID, DayOfWeek: int16(i + 1), MealType: "dinner", RecipeID: &rec.RecipeID,
		}, userID, "it")
		require.NoError(t, err)
	}

	// Pantry already holds 1 kg of flour.
	_, err = upSvc.UpsertHouseholdItem(ctx, userprefs.HouseholdItem{
		HouseholdID: userID, ItemID: flour.ItemID, CurrentQty: 1,
	}, "it")
	require.NoError(t, err)

	resolver := NewResolver(pool, Services{
		Analytics: analytics.NewService(pool),
		Grocery:   grocerySvc,
		Inventory: invSvc,
		MealPlan:  mpSvc,
		Recipe:    recipeSvc,
		UserPrefs: upSvc,
		Wine:      wine.NewService(pool),
		Identity:  identity.NewService(pool),
	}, Options{})
	uctx := currentuser.WithUser(ctx, currentuser.User{UserID: userID, HouseholdID: userID, Email: "grocery-gen@example.com"})
	planID := graphql.ID(strconv.FormatInt(plan.MealPlanID, 10))

	res, err := resolver.GenerateGroceryList(uctx, struct{ MealPlanID graphql.ID }{MealPlanID: planID})
	require.NoError(t, err)
	items, err := res.Items(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1, "two recipes sharing flour must aggregate to one line")
	assert.InDelta(t, 2.0, items[0].QuantityNeeded(), 0.0001, "3 kg needed minus 1 kg on hand")

	// A fully stocked pantry omits the line entirely.
	_, err = upSvc.UpsertHouseholdItem(ctx, userprefs.HouseholdItem{
		HouseholdID: userID, ItemID: flour.ItemID, CurrentQty: 10,
	}, "it")
	require.NoError(t, err)
	res2, err := resolver.GenerateGroceryList(uctx, struct{ MealPlanID graphql.ID }{MealPlanID: planID})
	require.NoError(t, err)
	items2, err := res2.Items(ctx)
	require.NoError(t, err)
	assert.Empty(t, items2, "fully stocked pantry produces no grocery lines")
}

// doGraphQL posts a query and requires a clean response: decodable body
// and zero GraphQL errors. Negative tests that expect errors must use
// doGraphQLExpectErrors so a regression can never hide behind a swallowed
// error.
func doGraphQL(t *testing.T, srv *httptest.Server, token, query string, vars map[string]any) (int, graphqlResponse) {
	t.Helper()
	status, gr := doGraphQLExpectErrors(t, srv, token, query, vars)
	for _, e := range gr.Errors {
		t.Logf("graphql error: %s", e.Message)
	}
	require.Empty(t, gr.Errors, "unexpected graphql errors")
	return status, gr
}

// doGraphQLExpectErrors is doGraphQL without the error assertion, for
// tests that exercise rejection paths (401s, FORBIDDEN, validation).
func doGraphQLExpectErrors(t *testing.T, srv *httptest.Server, token, query string, vars map[string]any) (int, graphqlResponse) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": vars,
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/graphql", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var gr graphqlResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&gr), "graphql response must decode")
	return resp.StatusCode, gr
}

func decodeData(t *testing.T, data json.RawMessage, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(data, v))
}

func containsBrand(brands []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}, id string) bool {
	for _, b := range brands {
		if b.ID == id {
			return true
		}
	}
	return false
}

func findGroceryItemChecked(items []struct {
	ID             string  `json:"id"`
	ManualItemName string  `json:"manualItemName"`
	QuantityNeeded float64 `json:"quantityNeeded"`
	UnitOfMeasure  string  `json:"unitOfMeasure"`
	IsChecked      bool    `json:"isChecked"`
}, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return it.IsChecked
		}
	}
	return false
}

func containsUserItemID(items []struct {
	ID         string  `json:"id"`
	CurrentQty float64 `json:"currentQty"`
	Item       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"item"`
}, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func containsID(items []struct {
	ID string `json:"id"`
}, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

// runEventTests exercises the household-scoped event surface: a two-member
// household where both members see and mutate shared events, the
// event_created/event_updated/event_deleted notification fan-out, target-time
// validation, and isolation from other households.
func runEventTests(t *testing.T, srv *httptest.Server, issuer *testutil.TestIssuer) {
	tokH := issuer.Token(t, "ev-h", "ev-h@example.com", "EV H")
	tokI := issuer.Token(t, "ev-i", "ev-i@example.com", "EV I")
	tokJ := issuer.Token(t, "ev-j", "ev-j@example.com", "EV J")

	meID := func(token string) string {
		status, gr := doGraphQL(t, srv, token, `{ me { id } }`, nil)
		require.Equal(t, http.StatusOK, status)
		var res struct {
			Me struct {
				ID string `json:"id"`
			} `json:"me"`
		}
		decodeData(t, gr.Data, &res)
		return res.Me.ID
	}
	idI := meID(tokI)

	// H invites I; I accepts. J stays in a household of one.
	status, gr := doGraphQL(t, srv, tokH, `mutation Invite($userId: ID!) {
		inviteHouseholdMember(userId: $userId) { id }
	}`, map[string]any{"userId": idI})
	require.Equal(t, http.StatusOK, status)
	var inviteRes struct {
		Invite struct {
			ID string `json:"id"`
		} `json:"inviteHouseholdMember"`
	}
	decodeData(t, gr.Data, &inviteRes)

	status, gr = doGraphQL(t, srv, tokI, `mutation Accept($inviteId: ID!) {
		acceptHouseholdInvite(inviteId: $inviteId) { id }
	}`, map[string]any{"inviteId": inviteRes.Invite.ID})
	require.Equal(t, http.StatusOK, status)

	// Clear H's invite_accepted notification so the unread counts below
	// reflect only event activity.
	status, gr = doGraphQL(t, srv, tokH, `mutation { markAllNotificationsRead }`, nil)
	require.Equal(t, http.StatusOK, status)

	// H creates an event; I sees it listed (shared household scope) and is
	// notified with the event deep-link.
	status, gr = doGraphQL(t, srv, tokH, `mutation {
		createFoodEvent(input: { name: "Friendsgiving", eventDate: "2026-11-26", slotGranularityMinutes: 30 }) {
			id name eventDate slotGranularityMinutes isActive
		}
	}`, nil)
	require.Equal(t, http.StatusOK, status)
	var createRes struct {
		Event struct {
			ID                     string `json:"id"`
			Name                   string `json:"name"`
			EventDate              string `json:"eventDate"`
			SlotGranularityMinutes int    `json:"slotGranularityMinutes"`
			IsActive               bool   `json:"isActive"`
		} `json:"createFoodEvent"`
	}
	decodeData(t, gr.Data, &createRes)
	eventID := createRes.Event.ID
	assert.Equal(t, "Friendsgiving", createRes.Event.Name)
	assert.Equal(t, "2026-11-26", createRes.Event.EventDate)
	assert.Equal(t, 30, createRes.Event.SlotGranularityMinutes)
	assert.True(t, createRes.Event.IsActive)

	status, gr = doGraphQL(t, srv, tokI, `{
		foodEvents(page: 1, pageSize: 10) {
			items { id name }
			pageInfo { totalCount }
		}
	}`, nil)
	require.Equal(t, http.StatusOK, status)
	var listRes struct {
		FoodEvents struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
			PageInfo struct {
				TotalCount int `json:"totalCount"`
			} `json:"pageInfo"`
		} `json:"foodEvents"`
	}
	decodeData(t, gr.Data, &listRes)
	require.Len(t, listRes.FoodEvents.Items, 1)
	assert.Equal(t, eventID, listRes.FoodEvents.Items[0].ID)

	status, gr = doGraphQL(t, srv, tokI, `{ myNotifications(limit: 5) { id kind foodEventId } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var notifRes struct {
		Notifs []struct {
			ID          string  `json:"id"`
			Kind        string  `json:"kind"`
			FoodEventID *string `json:"foodEventId"`
		} `json:"myNotifications"`
	}
	decodeData(t, gr.Data, &notifRes)
	var sawEventCreated bool
	for _, n := range notifRes.Notifs {
		if n.Kind == "EVENT_CREATED" {
			sawEventCreated = true
			require.NotNil(t, n.FoodEventID)
			assert.Equal(t, eventID, *n.FoodEventID)
		}
	}
	assert.True(t, sawEventCreated, "invitee should get an event_created notification with the deep-link")

	// I adds a recipe slot (free-form, no recipe row — recipeId is optional).
	status, gr = doGraphQL(t, srv, tokI, `mutation AddSlot($eventId: ID!) {
		addEventRecipe(input: { foodEventId: $eventId, mealType: "dinner", targetTime: "2026-11-26T18:30:00Z", servings: 8, notes: "turkey" }) {
			id mealType servings notes
		}
	}`, map[string]any{"eventId": eventID})
	require.Equal(t, http.StatusOK, status)
	var addRes struct {
		Slot struct {
			ID       string  `json:"id"`
			MealType string  `json:"mealType"`
			Servings *int    `json:"servings"`
			Notes    *string `json:"notes"`
		} `json:"addEventRecipe"`
	}
	decodeData(t, gr.Data, &addRes)
	slotID := addRes.Slot.ID
	assert.Equal(t, "dinner", addRes.Slot.MealType)
	require.NotNil(t, addRes.Slot.Servings)
	assert.Equal(t, 8, *addRes.Slot.Servings)

	// Off-boundary and off-date target times are rejected with BAD_USER_INPUT.
	status, gr = doGraphQLExpectErrors(t, srv, tokI, `mutation BadSlot($eventId: ID!) {
		addEventRecipe(input: { foodEventId: $eventId, mealType: "dinner", targetTime: "2026-11-26T18:07:00Z" }) { id }
	}`, map[string]any{"eventId": eventID})
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, gr.Errors)
	assert.Equal(t, codeBadUserInput, gr.Errors[0].Extensions["code"])

	status, gr = doGraphQLExpectErrors(t, srv, tokI, `mutation BadDate($eventId: ID!) {
		addEventRecipe(input: { foodEventId: $eventId, mealType: "dinner", targetTime: "2026-11-27T18:00:00Z" }) { id }
	}`, map[string]any{"eventId": eventID})
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, gr.Errors)
	assert.Equal(t, codeBadUserInput, gr.Errors[0].Extensions["code"])

	// H reads the slot back through the event.
	status, gr = doGraphQL(t, srv, tokH, `query Event($id: ID!) {
		foodEvent(id: $id) { id recipes { id mealType notes } }
	}`, map[string]any{"id": eventID})
	require.Equal(t, http.StatusOK, status)
	var evRes struct {
		Event *struct {
			ID      string `json:"id"`
			Recipes []struct {
				ID       string `json:"id"`
				MealType string `json:"mealType"`
			} `json:"recipes"`
		} `json:"foodEvent"`
	}
	decodeData(t, gr.Data, &evRes)
	require.NotNil(t, evRes.Event)
	require.Len(t, evRes.Event.Recipes, 1)
	assert.Equal(t, slotID, evRes.Event.Recipes[0].ID)

	// The timeline computes on read; a free-form slot has no steps and is
	// flagged unschedulable rather than dropped.
	status, gr = doGraphQL(t, srv, tokI, `query Timeline($id: ID!) {
		eventTimeline(foodEventId: $id) {
			foodEventId warnings
			recipes { eventRecipeId name unschedulable warnings startBy }
		}
	}`, map[string]any{"id": eventID})
	require.Equal(t, http.StatusOK, status)
	var tlRes struct {
		Timeline *struct {
			FoodEventID string `json:"foodEventId"`
			Recipes     []struct {
				EventRecipeID string   `json:"eventRecipeId"`
				Name          string   `json:"name"`
				Unschedulable bool     `json:"unschedulable"`
				Warnings      []string `json:"warnings"`
				StartBy       *string  `json:"startBy"`
			} `json:"recipes"`
		} `json:"eventTimeline"`
	}
	decodeData(t, gr.Data, &tlRes)
	require.NotNil(t, tlRes.Timeline)
	assert.Equal(t, eventID, tlRes.Timeline.FoodEventID)
	require.Len(t, tlRes.Timeline.Recipes, 1)
	assert.Equal(t, slotID, tlRes.Timeline.Recipes[0].EventRecipeID)
	assert.Equal(t, "turkey", tlRes.Timeline.Recipes[0].Name)
	assert.True(t, tlRes.Timeline.Recipes[0].Unschedulable)
	assert.Nil(t, tlRes.Timeline.Recipes[0].StartBy)

	// Snapshot steps belong to the slot, not any shared recipe — a
	// free-form slot gets its own hand-entered schedule.
	status, gr = doGraphQL(t, srv, tokI, `mutation AddStep($slotId: ID!) {
		addEventRecipeStep(eventRecipeId: $slotId, input: { instruction: "pick up turkey", durationMinutes: 60, stepType: "other" }) {
			id stepNumber instruction durationMinutes
		}
	}`, map[string]any{"slotId": slotID})
	require.Equal(t, http.StatusOK, status)
	var stepRes struct {
		Step struct {
			ID              string `json:"id"`
			StepNumber      int    `json:"stepNumber"`
			Instruction     string `json:"instruction"`
			DurationMinutes *int   `json:"durationMinutes"`
		} `json:"addEventRecipeStep"`
	}
	decodeData(t, gr.Data, &stepRes)
	assert.Equal(t, 1, stepRes.Step.StepNumber)
	assert.Equal(t, "pick up turkey", stepRes.Step.Instruction)

	status, gr = doGraphQL(t, srv, tokI, `query Timeline($id: ID!) {
		eventTimeline(foodEventId: $id) {
			recipes { unschedulable startBy steps { stepNumber startTime endTime scheduledMinutes } }
		}
	}`, map[string]any{"id": eventID})
	require.Equal(t, http.StatusOK, status)
	var tlRes2 struct {
		Timeline *struct {
			Recipes []struct {
				Unschedulable bool    `json:"unschedulable"`
				StartBy       *string `json:"startBy"`
				Steps         []struct {
					StepNumber       int    `json:"stepNumber"`
					StartTime        string `json:"startTime"`
					EndTime          string `json:"endTime"`
					ScheduledMinutes int    `json:"scheduledMinutes"`
				} `json:"steps"`
			} `json:"recipes"`
		} `json:"eventTimeline"`
	}
	decodeData(t, gr.Data, &tlRes2)
	require.NotNil(t, tlRes2.Timeline)
	require.Len(t, tlRes2.Timeline.Recipes, 1)
	assert.False(t, tlRes2.Timeline.Recipes[0].Unschedulable)
	require.NotNil(t, tlRes2.Timeline.Recipes[0].StartBy)
	require.Len(t, tlRes2.Timeline.Recipes[0].Steps, 1)
	// 60 minutes on a 30-minute grid ends exactly at the 18:30 target.
	assert.Equal(t, "2026-11-26T17:30:00Z", tlRes2.Timeline.Recipes[0].Steps[0].StartTime)
	assert.Equal(t, "2026-11-26T18:30:00Z", tlRes2.Timeline.Recipes[0].Steps[0].EndTime)

	// The slot's steps field exposes the same snapshot.
	status, gr = doGraphQL(t, srv, tokH, `query EventSteps($id: ID!) {
		foodEvent(id: $id) { recipes { id steps { stepNumber instruction } } }
	}`, map[string]any{"id": eventID})
	require.Equal(t, http.StatusOK, status)
	var stepsRes struct {
		Event *struct {
			Recipes []struct {
				Steps []struct {
					StepNumber  int    `json:"stepNumber"`
					Instruction string `json:"instruction"`
				} `json:"steps"`
			} `json:"recipes"`
		} `json:"foodEvent"`
	}
	decodeData(t, gr.Data, &stepsRes)
	require.NotNil(t, stepsRes.Event)
	require.Len(t, stepsRes.Event.Recipes, 1)
	require.Len(t, stepsRes.Event.Recipes[0].Steps, 1)
	assert.Equal(t, "pick up turkey", stepsRes.Event.Recipes[0].Steps[0].Instruction)

	// A linked recipe materializes an ingredient snapshot scaled by
	// servings ÷ base_servings. The item ID comes from the global catalog.
	status, gr = doGraphQL(t, srv, tokI, `{ items(page: 1, pageSize: 1) { items { id } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var anyItem struct {
		Items struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"items"`
	}
	decodeData(t, gr.Data, &anyItem)
	require.NotEmpty(t, anyItem.Items.Items)

	// createRecipe is @admin — the recipe lives in the shared catalog, so
	// a global admin creates it and the member household links it.
	tokAdmin := issuer.Token(t, "ev-admin", "user-a@example.com", "EV Admin")
	status, gr = doGraphQL(t, srv, tokAdmin, `mutation CreateRecipe($input: CreateRecipeInput!) {
		createRecipe(input: $input) { id }
	}`, map[string]any{"input": map[string]any{
		"name":     "EV Gravy",
		"servings": 4,
		"items": []map[string]any{
			{"itemId": anyItem.Items.Items[0].ID, "quantity": 2.0, "unit": "cup"},
		},
		"steps": []map[string]any{
			{"stepNumber": 1, "instruction": "Simmer"},
		},
	}})
	require.Equal(t, http.StatusOK, status)
	var evRecipeRes struct {
		Recipe struct {
			ID string `json:"id"`
		} `json:"createRecipe"`
	}
	decodeData(t, gr.Data, &evRecipeRes)

	status, gr = doGraphQL(t, srv, tokI, `mutation AddLinkedSlot($eventId: ID!, $recipeId: ID!) {
		addEventRecipe(input: { foodEventId: $eventId, recipeId: $recipeId, mealType: "dinner", targetTime: "2026-11-26T18:00:00Z", servings: 8 }) {
			id servings baseServings scalingFactor
			items { id quantity baseQuantity unit item { id name } }
		}
	}`, map[string]any{"eventId": eventID, "recipeId": evRecipeRes.Recipe.ID})
	require.Equal(t, http.StatusOK, status)
	var linkedRes struct {
		Slot struct {
			ID            string  `json:"id"`
			Servings      *int    `json:"servings"`
			BaseServings  *int    `json:"baseServings"`
			ScalingFactor float64 `json:"scalingFactor"`
			Items         []struct {
				ID           string  `json:"id"`
				Quantity     float64 `json:"quantity"`
				BaseQuantity float64 `json:"baseQuantity"`
				Unit         string  `json:"unit"`
			} `json:"items"`
		} `json:"addEventRecipe"`
	}
	decodeData(t, gr.Data, &linkedRes)
	require.NotNil(t, linkedRes.Slot.BaseServings)
	assert.Equal(t, 4, *linkedRes.Slot.BaseServings)
	assert.Equal(t, 2.0, linkedRes.Slot.ScalingFactor)
	require.Len(t, linkedRes.Slot.Items, 1)
	// 8 servings on a recipe written for 4 doubles the copied amount.
	assert.Equal(t, 4.0, linkedRes.Slot.Items[0].Quantity)
	assert.Equal(t, 2.0, linkedRes.Slot.Items[0].BaseQuantity)
	linkedSlotID := linkedRes.Slot.ID
	linkedItemID := linkedRes.Slot.Items[0].ID

	// Editing the snapshot item changes the event's copy only; the shared
	// recipe item keeps its original quantity.
	status, gr = doGraphQL(t, srv, tokI, `mutation EditItem($id: ID!, $itemId: ID!) {
		updateEventRecipeItem(id: $id, input: { itemId: $itemId, quantity: 5, unit: "cup" }) {
			id baseQuantity
		}
	}`, map[string]any{"id": linkedItemID, "itemId": anyItem.Items.Items[0].ID})
	require.Equal(t, http.StatusOK, status)
	var editItemRes struct {
		Item struct {
			BaseQuantity float64 `json:"baseQuantity"`
		} `json:"updateEventRecipeItem"`
	}
	decodeData(t, gr.Data, &editItemRes)
	assert.Equal(t, 5.0, editItemRes.Item.BaseQuantity)

	status, gr = doGraphQL(t, srv, tokI, `query OrigRecipe($id: ID!) { recipe(id: $id) { items { quantity } } }`,
		map[string]any{"id": evRecipeRes.Recipe.ID})
	require.Equal(t, http.StatusOK, status)
	var origRecipe struct {
		Recipe *struct {
			Items []struct {
				Quantity float64 `json:"quantity"`
			} `json:"items"`
		} `json:"recipe"`
	}
	decodeData(t, gr.Data, &origRecipe)
	require.NotNil(t, origRecipe.Recipe)
	require.Len(t, origRecipe.Recipe.Items, 1)
	assert.Equal(t, 2.0, origRecipe.Recipe.Items[0].Quantity)

	// Syncing re-copies the recipe and restores the snapshot.
	status, gr = doGraphQL(t, srv, tokI, `mutation SyncSlot($id: ID!) {
		syncEventRecipe(eventRecipeId: $id) { items { baseQuantity } }
	}`, map[string]any{"id": linkedSlotID})
	require.Equal(t, http.StatusOK, status)
	var syncRes struct {
		Slot struct {
			Items []struct {
				BaseQuantity float64 `json:"baseQuantity"`
			} `json:"items"`
		} `json:"syncEventRecipe"`
	}
	decodeData(t, gr.Data, &syncRes)
	require.Len(t, syncRes.Slot.Items, 1)
	assert.Equal(t, 2.0, syncRes.Slot.Items[0].BaseQuantity)

	// J is a different household: no visibility, no mutation.
	status, gr = doGraphQL(t, srv, tokJ, `{ foodEvents(page: 1, pageSize: 10) { items { id } pageInfo { totalCount } } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var jList struct {
		FoodEvents struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"foodEvents"`
	}
	decodeData(t, gr.Data, &jList)
	assert.Empty(t, jList.FoodEvents.Items)

	status, gr = doGraphQLExpectErrors(t, srv, tokJ, `query Peek($id: ID!) { foodEvent(id: $id) { id } }`, map[string]any{"id": eventID})
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, gr.Errors, "foreign-household read must fail")
	assert.Equal(t, codeNotFound, gr.Errors[0].Extensions["code"])

	// I updates the slot and the event; H's unread count climbs.
	status, gr = doGraphQL(t, srv, tokI, `mutation Upd($id: ID!) {
		updateEventRecipe(id: $id, input: { servings: 10 }) { id servings }
	}`, map[string]any{"id": slotID})
	require.Equal(t, http.StatusOK, status)

	status, gr = doGraphQL(t, srv, tokH, `{ unreadNotificationCount }`, nil)
	require.Equal(t, http.StatusOK, status)
	var unreadH struct {
		Count int `json:"unreadNotificationCount"`
	}
	decodeData(t, gr.Data, &unreadH)
	// Slot add + slot update were both I-originated and notified H.
	assert.Equal(t, 2, unreadH.Count)

	// Delete the event; members get event_deleted (deep-link nulled by SET NULL).
	status, gr = doGraphQL(t, srv, tokH, `mutation Del($id: ID!) { deleteFoodEvent(id: $id) }`, map[string]any{"id": eventID})
	require.Equal(t, http.StatusOK, status)

	status, gr = doGraphQL(t, srv, tokI, `{ myNotifications(limit: 10) { kind foodEventId } }`, nil)
	require.Equal(t, http.StatusOK, status)
	decodeData(t, gr.Data, &notifRes)
	var sawDeleted bool
	for _, n := range notifRes.Notifs {
		if n.Kind == "EVENT_DELETED" {
			sawDeleted = true
		}
	}
	assert.True(t, sawDeleted)
}
