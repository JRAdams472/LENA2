// Package bffclient is a minimal GraphQL client for the LENA2 BFF API.
// It is used by the importer to snapshot the catalog and create missing items.
package bffclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client calls the LENA2 GraphQL API.
type Client struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// New creates a Client. baseURL is the root of the API, e.g. http://api:8080.
func New(baseURL, authToken string) *Client {
	if baseURL == "" {
		baseURL = "http://api:8080"
	}
	return &Client{
		baseURL:   baseURL,
		authToken: authToken,
		client:    http.DefaultClient,
	}
}

// WithHTTPClient replaces the underlying HTTP client.
func (c *Client) WithHTTPClient(client *http.Client) *Client {
	c.client = client
	return c
}

type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

func (e graphQLError) Error() string { return e.Message }

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

func (c *Client) do(ctx context.Context, query string, vars map[string]interface{}, result interface{}) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("graphql request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read graphql response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gr graphQLResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	if len(gr.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gr.Errors[0].Message)
	}

	if result != nil {
		if err := json.Unmarshal(gr.Data, result); err != nil {
			return fmt.Errorf("decode graphql data: %w", err)
		}
	}
	return nil
}

// Unit is a canonical unit of measure from the catalog.
type Unit struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Kind         string `json:"kind"`
}

// Category is a catalog category.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Item is a catalog item.
type Item struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Unit     string   `json:"unit"`
	Category Category `json:"category"`
}

// Ingredient is a generic ingredient.
type Ingredient struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DefaultUnit string    `json:"defaultUnit"`
	Category    *Category `json:"category"`
}

// Catalog holds a full snapshot of the inventory catalog.
type Catalog struct {
	Items       []Item
	Ingredients []Ingredient
	Units       []Unit
	Categories  []Category
}

// ListCatalog fetches all categories, units, and paged items/ingredients.
func (c *Client) ListCatalog(ctx context.Context) (*Catalog, error) {
	cat := &Catalog{}

	const unitsQuery = `query { units { id name abbreviation kind } categories { id name description } }`
	type unitsData struct {
		Units      []Unit     `json:"units"`
		Categories []Category `json:"categories"`
	}
	var ud unitsData
	if err := c.do(ctx, unitsQuery, nil, &ud); err != nil {
		return nil, fmt.Errorf("list units/categories: %w", err)
	}
	cat.Units = ud.Units
	cat.Categories = ud.Categories

	pageSize := 100
	for page := 1; ; page++ {
		vars := map[string]interface{}{"page": page, "pageSize": pageSize}
		const itemsQuery = `query($page: Int!, $pageSize: Int!) {
			items(page: $page, pageSize: $pageSize) {
				items { id name unit category { id name } }
				pageInfo { totalCount }
			}
		}`
		type itemsData struct {
			Items struct {
				Items    []Item   `json:"items"`
				PageInfo pageInfo `json:"pageInfo"`
			} `json:"items"`
		}
		var id itemsData
		if err := c.do(ctx, itemsQuery, vars, &id); err != nil {
			return nil, fmt.Errorf("list items page %d: %w", page, err)
		}
		cat.Items = append(cat.Items, id.Items.Items...)
		if len(cat.Items) >= id.Items.PageInfo.TotalCount {
			break
		}
	}

	for page := 1; ; page++ {
		vars := map[string]interface{}{"page": page, "pageSize": pageSize}
		const ingredientsQuery = `query($page: Int!, $pageSize: Int!) {
			ingredients(page: $page, pageSize: $pageSize) {
				items { id name defaultUnit category { id name } }
				pageInfo { totalCount }
			}
		}`
		type ingredientsData struct {
			Ingredients struct {
				Items    []Ingredient `json:"items"`
				PageInfo pageInfo     `json:"pageInfo"`
			} `json:"ingredients"`
		}
		var id ingredientsData
		if err := c.do(ctx, ingredientsQuery, vars, &id); err != nil {
			return nil, fmt.Errorf("list ingredients page %d: %w", page, err)
		}
		cat.Ingredients = append(cat.Ingredients, id.Ingredients.Items...)
		if len(cat.Ingredients) >= id.Ingredients.PageInfo.TotalCount {
			break
		}
	}

	return cat, nil
}

type pageInfo struct {
	TotalCount int `json:"totalCount"`
}

// CreateItemInput is the payload for the createItem mutation.
type CreateItemInput struct {
	Name       string   `json:"name"`
	BrandID    *string  `json:"brandId,omitempty"`
	UPC12      *string  `json:"upc12,omitempty"`
	UPC14      *string  `json:"upc14,omitempty"`
	CategoryID string   `json:"categoryId"`
	Unit       string   `json:"unit"`
	NetWeight  *float64 `json:"netWeight,omitempty"`
	IsMetric   *bool    `json:"isMetric,omitempty"`
}

// CreateItem calls the createItem admin mutation and returns the new item id.
func (c *Client) CreateItem(ctx context.Context, input CreateItemInput) (string, error) {
	const q = `mutation($input: CreateItemInput!) { createItem(input: $input) { id name } }`
	type createItemData struct {
		CreateItem struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"createItem"`
	}
	vars := map[string]interface{}{"input": input}
	var d createItemData
	if err := c.do(ctx, q, vars, &d); err != nil {
		return "", err
	}
	return d.CreateItem.ID, nil
}

// Recipe is the subset of recipe fields the importer needs.
type Recipe struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListRecipes fetches all active recipes by paging through the catalog.
func (c *Client) ListRecipes(ctx context.Context) ([]Recipe, error) {
	const q = `query($page: Int!, $pageSize: Int!) {
		recipes(page: $page, pageSize: $pageSize) {
			items { id name }
			pageInfo { totalCount }
		}
	}`

	var recipes []Recipe
	pageSize := 100
	for page := 1; ; page++ {
		vars := map[string]interface{}{"page": page, "pageSize": pageSize}
		type recipesData struct {
			Recipes struct {
				Items    []Recipe `json:"items"`
				PageInfo pageInfo `json:"pageInfo"`
			} `json:"recipes"`
		}
		var rd recipesData
		if err := c.do(ctx, q, vars, &rd); err != nil {
			return nil, err
		}
		recipes = append(recipes, rd.Recipes.Items...)
		if len(recipes) >= rd.Recipes.PageInfo.TotalCount {
			break
		}
	}
	return recipes, nil
}

// GetRecipeByName returns the first recipe whose name matches case-insensitively.
func (c *Client) GetRecipeByName(ctx context.Context, name string) (Recipe, bool, error) {
	recipes, err := c.ListRecipes(ctx)
	if err != nil {
		return Recipe{}, false, err
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for _, r := range recipes {
		if strings.ToLower(strings.TrimSpace(r.Name)) == want {
			return r, true, nil
		}
	}
	return Recipe{}, false, nil
}

// RecipeItemInput matches the GraphQL RecipeItemInput shape.
type RecipeItemInput struct {
	ItemID       string  `json:"itemId"`
	IngredientID *string `json:"ingredientId,omitempty"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Section      *string `json:"section,omitempty"`
	DisplayOrder *int    `json:"displayOrder,omitempty"`
	Notes        *string `json:"notes,omitempty"`
	IsOptional   *bool   `json:"isOptional,omitempty"`
}

// RecipeStepInput matches the GraphQL RecipeStepInput shape.
type RecipeStepInput struct {
	StepNumber  int    `json:"stepNumber"`
	Instruction string `json:"instruction"`
}

// CreateRecipeInput matches the GraphQL CreateRecipeInput shape.
type CreateRecipeInput struct {
	Name            string            `json:"name"`
	Description     *string           `json:"description,omitempty"`
	Servings        *int              `json:"servings,omitempty"`
	PrepTimeMinutes *int              `json:"prepTimeMinutes,omitempty"`
	CookTimeMinutes *int              `json:"cookTimeMinutes,omitempty"`
	Items           []RecipeItemInput `json:"items"`
	Steps           []RecipeStepInput `json:"steps"`
}

// CreateRecipe calls the createRecipe admin mutation and returns the recipe id.
func (c *Client) CreateRecipe(ctx context.Context, input CreateRecipeInput) (string, error) {
	const q = `mutation($input: CreateRecipeInput!) { createRecipe(input: $input) { id name } }`
	type createRecipeData struct {
		CreateRecipe struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"createRecipe"`
	}
	vars := map[string]interface{}{"input": input}
	var d createRecipeData
	if err := c.do(ctx, q, vars, &d); err != nil {
		return "", err
	}
	return d.CreateRecipe.ID, nil
}
