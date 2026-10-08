// Package instacartclient is a thin HTTP client for the Instacart
// Developer Platform (IDP) products_link endpoint, which returns a
// shareable URL to a pre-populated shopping-list page.
package instacartclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the IDP development server; production deployments
// override it with https://connect.instacart.com.
const DefaultBaseURL = "https://connect.dev.instacart.tools"

// Measurement is one quantity+unit option for a line item. Supplying
// more than one lets Instacart pick the best unit for its quantity
// calculation; the first entry wins when several are compatible.
type Measurement struct {
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

// Filters narrow Instacart's product matching for a line item.
type Filters struct {
	BrandFilters []string `json:"brand_filters,omitempty"`
}

// LineItem is one row of the shopping list. Name is required — Instacart
// uses it for product matching; UPCs and brand filters sharpen the match.
type LineItem struct {
	Name                 string        `json:"name"`
	DisplayText          string        `json:"display_text,omitempty"`
	LineItemMeasurements []Measurement `json:"line_item_measurements,omitempty"`
	UPCs                 []string      `json:"upcs,omitempty"`
	Filters              *Filters      `json:"filters,omitempty"`
}

// ShoppingListRequest is the products_link request body.
type ShoppingListRequest struct {
	Title     string     `json:"title"`
	LinkType  string     `json:"link_type"`
	LineItems []LineItem `json:"line_items"`
}

// APIError reports a non-2xx IDP response. Detail is a truncated body
// snippet for server-side logs; request headers (and therefore the API
// key) are never captured, and callers must not return Detail verbatim
// to GraphQL clients.
type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("instacart returned status %d", e.StatusCode)
	}
	return fmt.Sprintf("instacart returned status %d: %s", e.StatusCode, e.Detail)
}

// Client calls the IDP products_link endpoint.
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New creates a Client. baseURL defaults to the IDP development server
// and timeout to 15s.
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}
}

// CreateShoppingList POSTs the request to /idp/v1/products/products_link
// and returns the shareable products_link_url.
func (c *Client) CreateShoppingList(ctx context.Context, req ShoppingListRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("encode products_link request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/idp/v1/products/products_link", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("instacart request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", &APIError{StatusCode: resp.StatusCode, Detail: strings.TrimSpace(string(detail))}
	}

	var out struct {
		ProductsLinkURL string `json:"products_link_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode products_link response: %w", err)
	}
	if out.ProductsLinkURL == "" {
		return "", fmt.Errorf("products_link response missing products_link_url")
	}
	return out.ProductsLinkURL, nil
}
