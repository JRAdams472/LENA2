package recipeimport

import (
	"context"
	"fmt"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
)

// InventoryReader is the subset of inventory.Service needed by the importer.
type InventoryReader interface {
	ListUnits(ctx context.Context) ([]inventory.Unit, error)
	ListItems(ctx context.Context, userID int64, limit, offset int32) ([]inventory.Item, error)
	CountItems(ctx context.Context, userID int64) (int64, error)
	ListIngredients(ctx context.Context, limit, offset int32) ([]inventory.Ingredient, error)
	CountIngredients(ctx context.Context) (int64, error)
	GetItemByID(ctx context.Context, itemID int64) (inventory.Item, error)
	GetUnitByID(ctx context.Context, unitID int64) (inventory.Unit, error)
	GetUnitByName(ctx context.Context, name string) (inventory.Unit, error)
}

type catalogItem struct{ inv inventory.Item }

func (i catalogItem) ID() string   { return fmt.Sprintf("%d", i.inv.ItemID) }
func (i catalogItem) Name() string { return i.inv.Name }

type catalogUnit struct{ inv inventory.Unit }

func (u catalogUnit) ID() string           { return fmt.Sprintf("%d", u.inv.UnitID) }
func (u catalogUnit) Name() string         { return u.inv.Name }
func (u catalogUnit) Abbreviation() string { return u.inv.Abbreviation }

type catalogIngredient struct{ inv inventory.Ingredient }

func (i catalogIngredient) ID() string   { return fmt.Sprintf("%d", i.inv.IngredientID) }
func (i catalogIngredient) Name() string { return i.inv.Name }

// newCatalogSnapshot builds an ocrimport.CatalogSnapshot from the inventory
// catalog. It pages through approved items and all ingredients.
func newCatalogSnapshot(ctx context.Context, inv InventoryReader) (*ocrimport.CatalogSnapshot, error) {
	units, err := inv.ListUnits(ctx)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}

	const pageSize int32 = 100

	totalItems, err := inv.CountItems(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("count items: %w", err)
	}
	if totalItems > maxInt32 {
		totalItems = maxInt32
	}
	items := make([]ocrimport.CatalogItem, 0, totalItems)
	for offset := int64(0); offset < totalItems; offset += int64(pageSize) {
		page, err := inv.ListItems(ctx, 0, pageSize, int32(offset))
		if err != nil {
			return nil, fmt.Errorf("list items page offset %d: %w", offset, err)
		}
		for _, it := range page {
			if it.Status != inventory.ItemStatusApproved {
				continue
			}
			items = append(items, catalogItem{inv: it})
		}
	}

	totalIngs, err := inv.CountIngredients(ctx)
	if err != nil {
		return nil, fmt.Errorf("count ingredients: %w", err)
	}
	if totalIngs > maxInt32 {
		totalIngs = maxInt32
	}
	ingredients := make([]ocrimport.CatalogIngredient, 0, totalIngs)
	for offset := int64(0); offset < totalIngs; offset += int64(pageSize) {
		page, err := inv.ListIngredients(ctx, pageSize, int32(offset))
		if err != nil {
			return nil, fmt.Errorf("list ingredients page offset %d: %w", offset, err)
		}
		for _, in := range page {
			ingredients = append(ingredients, catalogIngredient{inv: in})
		}
	}

	cat := &ocrimport.StaticCatalog{
		ItemsField:       items,
		IngredientsField: ingredients,
		UnitsField:       make([]ocrimport.CatalogUnit, 0, len(units)),
		CategoriesField:  nil,
	}
	for _, u := range units {
		cat.UnitsField = append(cat.UnitsField, catalogUnit{inv: u})
	}
	return ocrimport.NewCatalogSnapshot(cat), nil
}

const maxInt32 = 2147483647
