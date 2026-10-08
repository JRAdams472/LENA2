package bff

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/instacartclient"
	"github.com/graph-gophers/graphql-go"
)

// shopperProviderInstacart is the GraphQL enum value backed by the IDP
// products_link integration.
const shopperProviderInstacart = "INSTACART"

// shoppingLinkResolver resolves a ShoppingLink — the URL is the whole
// payload; the user completes the handoff in the provider's own session.
type shoppingLinkResolver struct {
	provider string
	url      string
}

func (r *shoppingLinkResolver) Provider() string { return r.provider }
func (r *shoppingLinkResolver) URL() string      { return r.url }

// ShopperProviders lists the shopper integrations configured on this
// deployment so clients can hide "shop this list" affordances when empty.
func (r *Resolver) ShopperProviders(ctx context.Context) ([]string, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	if r.ShoppingClient == nil {
		return nil, nil
	}
	return []string{shopperProviderInstacart}, nil
}

// CreateShoppingLink mints a shareable shopping-list URL on the configured
// shopper service from the list's unchecked lines. The call is stateless —
// each invocation produces a fresh link snapshot of the current list.
func (r *Resolver) CreateShoppingLink(ctx context.Context, args struct {
	GroceryListID  graphql.ID
	Provider       *string
	IncludeChecked *bool
}) (*shoppingLinkResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if args.Provider != nil && !strings.EqualFold(*args.Provider, shopperProviderInstacart) {
		return nil, badInputf("unknown shopping provider %q", *args.Provider)
	}
	if r.ShoppingClient == nil {
		return nil, errUnavailablef("shopping integration is not configured")
	}
	if !r.shoppingLimiter().allow(u.UserID) {
		return nil, &clientError{msg: "too many shopping-link requests; try again later", code: codeBusy}
	}
	id, err := parseID(string(args.GroceryListID))
	if err != nil {
		return nil, err
	}
	list, err := r.GroceryService.GetGroceryListByID(ctx, id, u.HouseholdID)
	if err != nil {
		return nil, err
	}
	gc, err := loadGroceryChildren(ctx, r.GroceryService, r.InventoryService, r.IdentityService, r.UserPrefsService, u.HouseholdID, []int64{id})
	if err != nil {
		return nil, err
	}
	lines := shoppingLineItems(gc, id, args.IncludeChecked != nil && *args.IncludeChecked)
	if len(lines) == 0 {
		return nil, badInputf("grocery list has no unchecked items to shop")
	}
	link, err := r.ShoppingClient.CreateShoppingList(ctx, instacartclient.ShoppingListRequest{
		Title:     fmt.Sprintf("LENA grocery list %s", list.GeneratedAt.Format("2006-01-02")),
		LinkType:  "shopping_list",
		LineItems: lines,
	})
	if err != nil {
		var ae *instacartclient.APIError
		if errors.As(err, &ae) {
			slog.Default().Warn("instacart products_link rejected the request",
				"status", ae.StatusCode, "grocery_list_id", id)
		} else {
			slog.Default().Error("instacart products_link request failed",
				"error", err, "grocery_list_id", id)
		}
		return nil, errUnavailablef("shopping provider request failed")
	}
	return &shoppingLinkResolver{provider: shopperProviderInstacart, url: link}, nil
}

// shoppingLineItems maps a list's grocery lines to IDP line items.
// Checked lines drop out unless includeChecked is set, and a line's name
// resolves bound item → usual-brand item → ingredient → manual text.
func shoppingLineItems(gc *groceryChildren, listID int64, includeChecked bool) []instacartclient.LineItem {
	var out []instacartclient.LineItem
	for _, li := range gc.itemsByList[listID] {
		if li.IsChecked && !includeChecked {
			continue
		}
		if line, ok := shoppingLine(gc, li); ok {
			out = append(out, line)
		}
	}
	return out
}

// shoppingLine maps one grocery line to an IDP line item; ok is false when
// the line has no usable name and should be skipped.
func shoppingLine(gc *groceryChildren, li grocery.GroceryListItem) (instacartclient.LineItem, bool) {
	it := groceryLineItem(gc, li)
	name := strings.TrimSpace(shoppingLineName(gc, li, it))
	if name == "" {
		return instacartclient.LineItem{}, false
	}
	line := instacartclient.LineItem{Name: name, UPCs: itemUPCs(it), Filters: brandFilter(gc, it)}
	if m, ok := instacartMeasurement(gc, li); ok {
		line.LineItemMeasurements = []instacartclient.Measurement{m}
		line.DisplayText = fmt.Sprintf("%s %s %s",
			strconv.FormatFloat(li.QuantityNeeded, 'f', -1, 64), unitDisplayName(gc, li), name)
	} else {
		line.DisplayText = name
	}
	return line, true
}

// shoppingLineName resolves the display/product name: bound or usual-brand
// item first, then the ingredient, then manual text. An ingredient-bound
// line whose ingredient failed to load has no name — it is skipped rather
// than falling back to manual text it may not carry.
func shoppingLineName(gc *groceryChildren, li grocery.GroceryListItem, it *inventory.Item) string {
	if it != nil {
		return it.Name
	}
	if li.IngredientID != nil {
		if ing, ok := gc.ingredients[*li.IngredientID]; ok {
			return ing.Name
		}
		return ""
	}
	return li.ManualItemName
}

// itemUPCs collects the catalog item's UPCs for exact-match prioritization.
func itemUPCs(it *inventory.Item) []string {
	if it == nil {
		return nil
	}
	var upcs []string
	if it.Upc12 != "" {
		upcs = append(upcs, it.Upc12)
	}
	if it.Upc14 != "" {
		upcs = append(upcs, it.Upc14)
	}
	return upcs
}

// brandFilter maps the item's brand onto an IDP brand_filters filter.
func brandFilter(gc *groceryChildren, it *inventory.Item) *instacartclient.Filters {
	if it == nil || it.BrandID == nil || gc.ch == nil {
		return nil
	}
	if b, ok := gc.ch.brands[*it.BrandID]; ok && b.Name != "" {
		return &instacartclient.Filters{BrandFilters: []string{b.Name}}
	}
	return nil
}

// groceryLineItem picks the catalog item a grocery line should shop for:
// the bound item wins, else the household's usual-brand item for the
// ingredient — an exact product (name + UPC + brand) beats a fuzzy match.
func groceryLineItem(gc *groceryChildren, li grocery.GroceryListItem) *inventory.Item {
	var itemID *int64
	switch {
	case li.ItemID != nil:
		itemID = li.ItemID
	case li.IngredientID != nil:
		if usualID, ok := gc.usualItems[*li.IngredientID]; ok {
			itemID = &usualID
		}
	}
	if itemID == nil {
		return nil
	}
	if it, ok := gc.items[*itemID]; ok {
		return &it
	}
	return nil
}

// instacartUnitByName maps LENA unit names and abbreviations onto the IDP
// measurement vocabulary (docs.instacart.com units_of_measurement).
var instacartUnitByName = map[string]string{
	"teaspoon": "teaspoon", "tsp": "teaspoon",
	"tablespoon": "tablespoon", "tbsp": "tablespoon", "tb": "tablespoon", "tbs": "tablespoon",
	"cup": "cup", "c": "cup",
	"fluid ounce": "fl oz", "fl oz": "fl oz",
	"milliliter": "milliliter", "ml": "milliliter",
	"liter": "liter", "l": "liter",
	"pint": "pint", "pt": "pint",
	"quart": "quart", "qt": "quart",
	"gallon": "gallon", "gal": "gallon",
	"gram": "gram", "g": "gram",
	"kilogram": "kilogram", "kg": "kilogram",
	"ounce": "ounce", "oz": "ounce",
	"pound": "pound", "lb": "pound",
	"each": "each", "ea": "each",
	"can":     "can",
	"package": "package", "pkg": "package",
	"bunch":    "bunch",
	"head":     "head",
	"packet":   "packet",
	"slice":    "each",
	"clove":    "each",
	"pinch":    "each",
	"to taste": "each",
}

// instacartMeasurement maps a LENA quantity+unit to an IDP Measurement.
// Unknown countable units degrade to "each" (IDP's recommended countable
// unit); unknown weight/volume units omit the measurement so Instacart
// never misreads e.g. a bag size as a count.
func instacartMeasurement(gc *groceryChildren, li grocery.GroceryListItem) (instacartclient.Measurement, bool) {
	if li.QuantityNeeded <= 0 {
		return instacartclient.Measurement{}, false
	}
	var unit string
	if li.UnitID != nil {
		if u, ok := gc.units[*li.UnitID]; ok {
			unit = instacartUnitByName[strings.ToLower(strings.TrimSpace(u.Name))]
			if unit == "" && u.Abbreviation != "" {
				unit = instacartUnitByName[strings.ToLower(strings.TrimSpace(u.Abbreviation))]
			}
			if unit == "" && u.Kind != "count" {
				return instacartclient.Measurement{}, false
			}
		}
	}
	if unit == "" {
		unit = "each"
	}
	return instacartclient.Measurement{Quantity: li.QuantityNeeded, Unit: unit}, true
}

// unitDisplayName renders the line's unit for display_text.
func unitDisplayName(gc *groceryChildren, li grocery.GroceryListItem) string {
	if li.UnitID != nil {
		if u, ok := gc.units[*li.UnitID]; ok {
			return u.Name
		}
	}
	return "x"
}
