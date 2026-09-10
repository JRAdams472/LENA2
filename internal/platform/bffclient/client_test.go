package bffclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/graphql", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req graphQLRequest
		require.NoError(t, json.Unmarshal(body, &req))

		w.Header().Set("Content-Type", "application/json")
		if contains(req.Query, "units") {
			_ = json.NewEncoder(w).Encode(graphQLResponse{Data: json.RawMessage(`{
				"units": [{"id":"1","name":"cup","abbreviation":"c","kind":"volume"}],
				"categories": [{"id":"1","name":"Baking"}]
			}`)})
			return
		}
		if contains(req.Query, "items(") {
			_ = json.NewEncoder(w).Encode(graphQLResponse{Data: json.RawMessage(`{
				"items": {
					"items": [{"id":"10","name":"Flour","unit":"cup","category":{"id":"1","name":"Baking"}}],
					"pageInfo": {"totalCount":1}
				}
			}`)})
			return
		}
		if contains(req.Query, "ingredients(") {
			_ = json.NewEncoder(w).Encode(graphQLResponse{Data: json.RawMessage(`{
				"ingredients": {
					"items": [],
					"pageInfo": {"totalCount":0}
				}
			}`)})
			return
		}
	}))
	defer server.Close()

	client := New(server.URL, "test-token")
	cat, err := client.ListCatalog(context.Background())
	require.NoError(t, err)
	assert.Len(t, cat.Units, 1)
	assert.Len(t, cat.Items, 1)
	assert.Len(t, cat.Categories, 1)
	assert.Len(t, cat.Ingredients, 0)
}

func TestCreateItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(graphQLResponse{Data: json.RawMessage(`{"createItem":{"id":"42","name":"Eggs"}}`)})
	}))
	defer server.Close()

	client := New(server.URL, "test-token")
	id, err := client.CreateItem(context.Background(), CreateItemInput{
		Name:       "Eggs",
		CategoryID: "1",
		Unit:       "each",
	})
	require.NoError(t, err)
	assert.Equal(t, "42", id)
}

func TestCreateRecipe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(graphQLResponse{Data: json.RawMessage(`{"createRecipe":{"id":"99","name":"Cake"}}`)})
	}))
	defer server.Close()

	client := New(server.URL, "test-token")
	id, err := client.CreateRecipe(context.Background(), CreateRecipeInput{
		Name:  "Cake",
		Items: []RecipeItemInput{{ItemID: "10", Quantity: 2, Unit: "cup"}},
		Steps: []RecipeStepInput{{StepNumber: 1, Instruction: "Mix"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "99", id)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
