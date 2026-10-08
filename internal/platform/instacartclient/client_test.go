package instacartclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateShoppingList(t *testing.T) {
	var gotAuth, gotContentType string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"products_link_url":"https://instacart.example/list/abc"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", 5*time.Second)
	url, err := c.CreateShoppingList(context.Background(), ShoppingListRequest{
		Title:    "LENA grocery list",
		LinkType: "shopping_list",
		LineItems: []LineItem{{
			Name:        "Organic Whole Milk",
			DisplayText: "1 gallon Organic Whole Milk",
			LineItemMeasurements: []Measurement{
				{Quantity: 1, Unit: "gallon"},
			},
			UPCs:    []string{"012345678905"},
			Filters: &Filters{BrandFilters: []string{"Horizon"}},
		}},
	})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	if url != "https://instacart.example/list/abc" {
		t.Fatalf("url = %q", url)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if gotBody["link_type"] != "shopping_list" {
		t.Fatalf("link_type = %v", gotBody)
	}
	items, ok := gotBody["line_items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("line_items = %v", gotBody["line_items"])
	}
	li := items[0].(map[string]any)
	if li["name"] != "Organic Whole Milk" {
		t.Fatalf("name = %v", li["name"])
	}
	m := li["line_item_measurements"].([]any)[0].(map[string]any)
	if m["unit"] != "gallon" || m["quantity"] != float64(1) {
		t.Fatalf("measurement = %v", m)
	}
	if li["upcs"].([]any)[0] != "012345678905" {
		t.Fatalf("upcs = %v", li["upcs"])
	}
	f := li["filters"].(map[string]any)
	if f["brand_filters"].([]any)[0] != "Horizon" {
		t.Fatalf("brand_filters = %v", f)
	}
}

func TestCreateShoppingListErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, "bad-key", 5*time.Second)
	_, err := c.CreateShoppingList(context.Background(), ShoppingListRequest{
		Title: "t", LinkType: "shopping_list",
		LineItems: []LineItem{{Name: "milk"}},
	})
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
	if ae.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d", ae.StatusCode)
	}
}

func TestCreateShoppingListMissingURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k", 5*time.Second)
	if _, err := c.CreateShoppingList(context.Background(), ShoppingListRequest{
		Title: "t", LinkType: "shopping_list",
		LineItems: []LineItem{{Name: "milk"}},
	}); err == nil {
		t.Fatal("expected error for missing products_link_url")
	}
}

func TestNewDefaults(t *testing.T) {
	c := New("", "k", 0)
	if c.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %q", c.baseURL)
	}
	c = New("https://connect.instacart.com/", "k", 0)
	if c.baseURL != "https://connect.instacart.com" {
		t.Fatalf("trailing slash not trimmed: %q", c.baseURL)
	}
}
