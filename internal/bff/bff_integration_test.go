package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/testenv"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func TestBFF_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	pool, cleanup, err := testenv.NewTestDB(t, ctx)
	require.NoError(t, err)
	defer cleanup()

	issuer := testenv.NewTestIssuer(t)

	identitySvc := identity.NewService(pool)
	authenticator := NewAuthenticator(AuthConfig{
		Issuers:   []string{issuer.URL},
		Audiences: []string{issuer.Audience},
	}, identitySvc)

	resolver := NewResolver(
		grocery.NewService(pool),
		inventory.NewService(pool),
		mealplan.NewService(pool),
		recipe.NewService(pool),
		userprefs.NewService(pool),
		wine.NewService(pool),
	)

	e := echo.New()
	e.HideBanner = true
	e.POST("/graphql", NewGraphQLHandler(resolver), authenticator.Middleware())
	srv := httptest.NewServer(e)
	defer srv.Close()

	t.Run("auth", func(t *testing.T) {
		runAuthTests(t, srv, issuer)
	})
	t.Run("end to end", func(t *testing.T) {
		runEndToEndTests(t, srv, issuer)
	})
}

func runAuthTests(t *testing.T, srv *httptest.Server, issuer *testenv.TestIssuer) {
	t.Run("no token returns 401", func(t *testing.T) {
		status, _ := doGraphQL(t, srv, "", `{ me { email } }`, nil)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("garbage token returns 401", func(t *testing.T) {
		status, _ := doGraphQL(t, srv, "not.a.jwt", `{ me { email } }`, nil)
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("untrusted issuer token returns 401", func(t *testing.T) {
		otherIssuer := testenv.NewTestIssuer(t)
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
}

func runEndToEndTests(t *testing.T, srv *httptest.Server, issuer *testenv.TestIssuer) {
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

	status, gr = doGraphQL(t, srv, tokA, `{ brands { id name } }`, nil)
	require.Equal(t, http.StatusOK, status)
	var brandsRes struct {
		Brands []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"brands"`
	}
	decodeData(t, gr.Data, &brandsRes)
	require.True(t, containsBrand(brandsRes.Brands, brandRes.CreateBrand.ID))

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
	status, gr = doGraphQL(t, srv, tokA, `mutation CreateItem($input: CreateItemInput!) { createItem(input: $input) { id name category { id name } unit } }`, map[string]any{
		"input": map[string]any{
			"name":       "Integration Milk",
			"categoryId": catRes.CreateCategory.ID,
			"unit":       "gallon",
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

	// cross-user isolation: user B cannot read user A's meal plan
	status, gr = doGraphQL(t, srv, tokB, `query MealPlan($id: ID!) { mealPlan(id: $id) { id name } }`, map[string]any{
		"id": mealPlanRes.CreateMealPlan.ID,
	})
	require.Equal(t, http.StatusOK, status)
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

func doGraphQL(t *testing.T, srv *httptest.Server, token, query string, vars map[string]any) (int, graphqlResponse) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": vars,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/graphql", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var gr graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	for _, e := range gr.Errors {
		t.Logf("graphql error: %s", e.Message)
	}
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
