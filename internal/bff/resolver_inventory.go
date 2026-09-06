package bff

import (
	"context"
	"strconv"
	"strings"

	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/graph-gophers/graphql-go"
)

// Brand resolves a single brand by ID.
func (r *Resolver) Brand(ctx context.Context, args struct{ ID graphql.ID }) (*brandResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	b, err := r.InventoryService.GetBrandByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}

// Brands resolves all catalog brands.
func (r *Resolver) Brands(ctx context.Context) ([]*brandResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	brands, err := r.InventoryService.ListBrands(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*brandResolver, len(brands))
	for i := range brands {
		out[i] = &brandResolver{b: brands[i]}
	}
	return out, nil
}

// Category resolves a single category by ID.
func (r *Resolver) Category(ctx context.Context, args struct{ ID graphql.ID }) (*categoryResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	c, err := r.InventoryService.GetCategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

// Categories resolves all catalog categories.
func (r *Resolver) Categories(ctx context.Context) ([]*categoryResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	cats, err := r.InventoryService.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*categoryResolver, len(cats))
	for i := range cats {
		out[i] = &categoryResolver{c: cats[i]}
	}
	return out, nil
}

// FlavorProfiles resolves all flavor profiles.
func (r *Resolver) FlavorProfiles(ctx context.Context) ([]*flavorProfileResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	profiles, err := r.InventoryService.ListFlavorProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*flavorProfileResolver, len(profiles))
	for i := range profiles {
		out[i] = &flavorProfileResolver{f: profiles[i]}
	}
	return out, nil
}

// NutrientTypes resolves all nutrient types.
func (r *Resolver) NutrientTypes(ctx context.Context) ([]*nutrientTypeResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	types, err := r.InventoryService.ListNutrientTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*nutrientTypeResolver, len(types))
	for i := range types {
		out[i] = &nutrientTypeResolver{n: types[i]}
	}
	return out, nil
}

// Item resolves a single catalog item by ID.
func (r *Resolver) Item(ctx context.Context, args struct{ ID graphql.ID }) (*itemResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	it, err := r.InventoryService.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.InventoryService, it: it}, nil
}

// Items resolves a paginated list of catalog items.
func (r *Resolver) Items(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*itemPageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	items, err := r.InventoryService.ListItems(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.InventoryService.CountItems(ctx)
	if err != nil {
		return nil, err
	}
	ch, err := loadItemChildren(ctx, r.InventoryService, items)
	if err != nil {
		return nil, err
	}
	return &itemPageResolver{inv: r.InventoryService, items: items, ch: ch, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// CreateBrand adds a new brand.
func (r *Resolver) CreateBrand(ctx context.Context, args struct{ Input createBrandInput }) (*brandResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	b, err := r.InventoryService.CreateBrand(ctx, args.Input.Name)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}

// CreateCategory adds a new category.
func (r *Resolver) CreateCategory(ctx context.Context, args struct{ Input createCategoryInput }) (*categoryResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	c, err := r.InventoryService.CreateCategory(ctx, args.Input.Name, derefString(args.Input.Description), u.Email)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

// CreateFlavorProfile adds a new flavor profile.
func (r *Resolver) CreateFlavorProfile(ctx context.Context, args struct{ Input createFlavorProfileInput }) (*flavorProfileResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	f, err := r.InventoryService.CreateFlavorProfile(ctx, args.Input.Name, u.Email)
	if err != nil {
		return nil, err
	}
	return &flavorProfileResolver{f: f}, nil
}

// CreateNutrientType adds a new nutrient type.
func (r *Resolver) CreateNutrientType(ctx context.Context, args struct{ Input createNutrientTypeInput }) (*nutrientTypeResolver, error) {
	_, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	n, err := r.InventoryService.CreateNutrientType(ctx, args.Input.Name, args.Input.Unit)
	if err != nil {
		return nil, err
	}
	return &nutrientTypeResolver{n: n}, nil
}

// Ingredient resolves a single generic ingredient by ID.
func (r *Resolver) Ingredient(ctx context.Context, args struct{ ID graphql.ID }) (*ingredientResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	in, err := r.InventoryService.GetIngredientByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.InventoryService, in: in}, nil
}

// Ingredients resolves a paginated list of generic ingredients.
func (r *Resolver) Ingredients(ctx context.Context, args struct {
	Page     int32
	PageSize int32
}) (*ingredientPageResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	page := clamp(args.Page, 1, 1_000_000)
	pageSize := clamp(args.PageSize, 1, 100)
	ingredients, err := r.InventoryService.ListIngredients(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := r.InventoryService.CountIngredients(ctx)
	if err != nil {
		return nil, err
	}
	return &ingredientPageResolver{inv: r.InventoryService, ingredients: ingredients, page: page, pageSize: pageSize, total: int64ToInt32(total)}, nil
}

// CreateIngredient adds a new generic ingredient.
func (r *Resolver) CreateIngredient(ctx context.Context, args struct{ Input createIngredientInput }) (*ingredientResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	categoryID, err := optionalID(args.Input.CategoryID)
	if err != nil {
		return nil, err
	}
	var defaultUnitID *int64
	if u := strings.TrimSpace(derefString(args.Input.DefaultUnit)); u != "" {
		id, err := resolveUnitID(ctx, r.InventoryService, u)
		if err != nil {
			return nil, err
		}
		defaultUnitID = &id
	}
	in, err := r.InventoryService.CreateIngredient(ctx, inventory.Ingredient{
		Name:          args.Input.Name,
		CategoryID:    categoryID,
		DefaultUnitID: defaultUnitID,
		IsActive:      true,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.InventoryService, in: in}, nil
}

// UpdateIngredient modifies an existing ingredient.
func (r *Resolver) UpdateIngredient(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateIngredientInput
}) (*ingredientResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetIngredientByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	categoryID := existing.CategoryID
	if args.Input.CategoryID != nil {
		c, err := optionalID(args.Input.CategoryID)
		if err != nil {
			return nil, err
		}
		categoryID = c
	}
	defaultUnitID := existing.DefaultUnitID
	if args.Input.DefaultUnit != nil {
		defaultUnitID = nil
		if u := strings.TrimSpace(*args.Input.DefaultUnit); u != "" {
			id, err := resolveUnitID(ctx, r.InventoryService, u)
			if err != nil {
				return nil, err
			}
			defaultUnitID = &id
		}
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	in, err := r.InventoryService.UpdateIngredient(ctx, id, inventory.Ingredient{
		Name:          name,
		CategoryID:    categoryID,
		DefaultUnitID: defaultUnitID,
		IsActive:      isActive,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &ingredientResolver{inv: r.InventoryService, in: in}, nil
}

// DeleteIngredient removes an ingredient from the catalog.
func (r *Resolver) DeleteIngredient(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteIngredient(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// ingredientResolver resolves Ingredient fields.
type ingredientResolver struct {
	inv InventoryService
	in  inventory.Ingredient
}

func (r *ingredientResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.in.IngredientID, 10))
}

func (r *ingredientResolver) Name() string { return r.in.Name }

// DefaultUnit renders the ingredient's default unit name, when set.
func (r *ingredientResolver) DefaultUnit(ctx context.Context) (*string, error) {
	return unitNamePtr(ctx, r.inv, nil, r.in.DefaultUnitID)
}

func (r *ingredientResolver) IsActive() bool { return r.in.IsActive }

func (r *ingredientResolver) Category(ctx context.Context) (*categoryResolver, error) {
	if r.in.CategoryID == nil {
		return nil, nil
	}
	c, err := r.inv.GetCategoryByID(ctx, *r.in.CategoryID)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

type ingredientPageResolver struct {
	inv         InventoryService
	ingredients []inventory.Ingredient
	page        int32
	pageSize    int32
	total       int32
}

func (r *ingredientPageResolver) Items() []*ingredientResolver {
	out := make([]*ingredientResolver, len(r.ingredients))
	for i := range r.ingredients {
		out[i] = &ingredientResolver{inv: r.inv, in: r.ingredients[i]}
	}
	return out
}

func (r *ingredientPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

type createIngredientInput struct {
	Name        string
	CategoryID  *graphql.ID
	DefaultUnit *string
}

type updateIngredientInput struct {
	Name        *string
	CategoryID  *graphql.ID
	DefaultUnit *string
	IsActive    *bool
}

// Units resolves all canonical units of measure.
func (r *Resolver) Units(ctx context.Context) ([]*unitResolver, error) {
	if _, err := userFromContext(ctx); err != nil {
		return nil, err
	}
	units, err := r.InventoryService.ListUnits(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*unitResolver, len(units))
	for i := range units {
		out[i] = &unitResolver{u: units[i]}
	}
	return out, nil
}

// unitResolver resolves Unit fields.
type unitResolver struct{ u inventory.Unit }

func (r *unitResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.u.UnitID, 10)) }

func (r *unitResolver) Name() string { return r.u.Name }

func (r *unitResolver) Abbreviation() *string { return nilIfEmpty(r.u.Abbreviation) }

func (r *unitResolver) Kind() string { return r.u.Kind }

func (r *unitResolver) IsActive() bool { return r.u.IsActive }

func (r *unitResolver) ToBaseFactor() *float64 { return r.u.ToBaseFactor }

// CreateItem creates a new catalog item.
func (r *Resolver) CreateItem(ctx context.Context, args struct{ Input createItemInput }) (*itemResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	brandID, err := optionalID(args.Input.BrandID)
	if err != nil {
		return nil, err
	}
	catID, err := parseID(string(args.Input.CategoryID))
	if err != nil {
		return nil, err
	}
	unitID, err := resolveUnitID(ctx, r.InventoryService, args.Input.Unit)
	if err != nil {
		return nil, err
	}
	it, err := r.InventoryService.CreateItem(ctx, inventory.Item{
		Name:       args.Input.Name,
		BrandID:    brandID,
		Upc12:      derefString(args.Input.Upc12),
		Upc14:      derefString(args.Input.Upc14),
		CategoryID: catID,
		UnitID:     unitID,
	}, u.Email)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.InventoryService, it: it}, nil
}

// AddFoodNutrient adds a nutrient value to a catalog item.
func (r *Resolver) AddFoodNutrient(ctx context.Context, args struct{ Input addFoodNutrientInput }) (*foodNutrientResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.Input.ItemID))
	if err != nil {
		return nil, err
	}
	nutrientID, err := parseID(string(args.Input.NutrientID))
	if err != nil {
		return nil, err
	}
	n, err := r.InventoryService.CreateFoodNutrient(ctx, itemID, nutrientID, args.Input.Amount, u.Email)
	if err != nil {
		return nil, err
	}
	return &foodNutrientResolver{nutrient: n}, nil
}

// RemoveFoodNutrient removes a nutrient value from an item.
func (r *Resolver) RemoveFoodNutrient(ctx context.Context, args struct {
	ItemID     graphql.ID
	NutrientID graphql.ID
}) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	nutrientID, err := parseID(string(args.NutrientID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteFoodNutrient(ctx, itemID, nutrientID); err != nil {
		return false, err
	}
	return true, nil
}

// AddFoodFlavor adds a flavor profile to a catalog item.
func (r *Resolver) AddFoodFlavor(ctx context.Context, args struct{ Input addFoodFlavorInput }) (*foodFlavorResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(string(args.Input.ItemID))
	if err != nil {
		return nil, err
	}
	flavorID, err := parseID(string(args.Input.FlavorID))
	if err != nil {
		return nil, err
	}
	f, err := r.InventoryService.CreateFoodFlavor(ctx, itemID, flavorID, int16(args.Input.Intensity), u.Email)
	if err != nil {
		return nil, err
	}
	return &foodFlavorResolver{flavor: f}, nil
}

// RemoveFoodFlavor removes a flavor profile from an item.
func (r *Resolver) RemoveFoodFlavor(ctx context.Context, args struct {
	ItemID   graphql.ID
	FlavorID graphql.ID
}) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	itemID, err := parseID(string(args.ItemID))
	if err != nil {
		return false, err
	}
	flavorID, err := parseID(string(args.FlavorID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteFoodFlavor(ctx, itemID, flavorID); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateItem modifies an existing catalog item.
func (r *Resolver) UpdateItem(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateItemInput
}) (*itemResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	brandID := existing.BrandID
	if args.Input.BrandID != nil {
		b, err := optionalID(args.Input.BrandID)
		if err != nil {
			return nil, err
		}
		brandID = b
	}
	categoryID := existing.CategoryID
	if args.Input.CategoryID != nil {
		c, err := parseID(string(*args.Input.CategoryID))
		if err != nil {
			return nil, err
		}
		categoryID = c
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	unitID := existing.UnitID
	if args.Input.Unit != nil {
		id, err := resolveUnitID(ctx, r.InventoryService, *args.Input.Unit)
		if err != nil {
			return nil, err
		}
		unitID = id
	}
	upc12 := existing.Upc12
	if args.Input.Upc12 != nil {
		upc12 = *args.Input.Upc12
	}
	upc14 := existing.Upc14
	if args.Input.Upc14 != nil {
		upc14 = *args.Input.Upc14
	}
	if err := r.InventoryService.UpdateItem(ctx, id, inventory.Item{
		Name:       name,
		BrandID:    brandID,
		Upc12:      upc12,
		Upc14:      upc14,
		CategoryID: categoryID,
		UnitID:     unitID,
	}, u.Email); err != nil {
		return nil, err
	}
	updated, err := r.InventoryService.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &itemResolver{inv: r.InventoryService, it: updated}, nil
}

// DeleteItem removes a catalog item.
func (r *Resolver) DeleteItem(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteItem(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// itemResolver resolves Item fields. When ch is non-nil its batch-loaded
// maps are used instead of per-item service calls.
type itemResolver struct {
	inv InventoryService
	it  inventory.Item
	ch  *itemChildren
}

func (r *itemResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.it.ItemID, 10)) }

func (r *itemResolver) Name() string { return r.it.Name }

func (r *itemResolver) Upc12() *string { return nilIfEmpty(r.it.Upc12) }

func (r *itemResolver) Upc14() *string { return nilIfEmpty(r.it.Upc14) }

func (r *itemResolver) Unit(ctx context.Context) (string, error) {
	var units map[int64]inventory.Unit
	if r.ch != nil {
		units = r.ch.units
	}
	return unitName(ctx, r.inv, units, r.it.UnitID)
}

func (r *itemResolver) Brand(ctx context.Context) (*brandResolver, error) {
	if r.it.BrandID == nil {
		return nil, nil
	}
	if r.ch != nil {
		b, ok := r.ch.brands[*r.it.BrandID]
		if !ok {
			return nil, nil
		}
		return &brandResolver{b: b}, nil
	}
	b, err := r.inv.GetBrandByID(ctx, *r.it.BrandID)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}

func (r *itemResolver) Category(ctx context.Context) (*categoryResolver, error) {
	if r.ch != nil {
		c, ok := r.ch.categories[r.it.CategoryID]
		if !ok {
			return nil, nil
		}
		return &categoryResolver{c: c}, nil
	}
	c, err := r.inv.GetCategoryByID(ctx, r.it.CategoryID)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

func (r *itemResolver) Nutrients(ctx context.Context) ([]*foodNutrientResolver, error) {
	var nutrients []inventory.FoodNutrient
	if r.ch != nil {
		nutrients = r.ch.nutrients[r.it.ItemID]
	} else {
		var err error
		nutrients, err = r.inv.ListFoodNutrientsByItem(ctx, r.it.ItemID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*foodNutrientResolver, len(nutrients))
	for i := range nutrients {
		out[i] = &foodNutrientResolver{nutrient: nutrients[i]}
	}
	return out, nil
}

func (r *itemResolver) Flavors(ctx context.Context) ([]*foodFlavorResolver, error) {
	var flavors []inventory.FoodFlavor
	if r.ch != nil {
		flavors = r.ch.flavors[r.it.ItemID]
	} else {
		var err error
		flavors, err = r.inv.ListFoodFlavorsByItem(ctx, r.it.ItemID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*foodFlavorResolver, len(flavors))
	for i := range flavors {
		out[i] = &foodFlavorResolver{flavor: flavors[i]}
	}
	return out, nil
}

type brandResolver struct{ b inventory.Brand }

func (r *brandResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.b.BrandID, 10)) }

func (r *brandResolver) Name() string { return r.b.Name }

type categoryResolver struct{ c inventory.Category }

func (r *categoryResolver) ID() graphql.ID { return graphql.ID(strconv.FormatInt(r.c.CategoryID, 10)) }

func (r *categoryResolver) Name() string { return r.c.Name }

func (r *categoryResolver) Description() *string { return nilIfEmpty(r.c.Description) }

func (r *categoryResolver) IsActive() bool { return r.c.IsActive }

// flavorProfileResolver resolves an inventory flavor profile.
type flavorProfileResolver struct{ f inventory.FlavorProfile }

func (r *flavorProfileResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.f.FlavorID, 10))
}

func (r *flavorProfileResolver) Name() string { return r.f.Name }

func (r *flavorProfileResolver) IsActive() bool { return r.f.IsActive }

// nutrientTypeResolver resolves an inventory nutrient type.
type nutrientTypeResolver struct{ n inventory.NutrientType }

func (r *nutrientTypeResolver) ID() graphql.ID {
	return graphql.ID(strconv.FormatInt(r.n.NutrientID, 10))
}

func (r *nutrientTypeResolver) Name() string { return r.n.Name }

func (r *nutrientTypeResolver) Unit() string { return r.n.Unit }

// foodNutrientResolver resolves an item's nutrient value.
type foodNutrientResolver struct{ nutrient inventory.FoodNutrient }

func (r *foodNutrientResolver) Nutrient(ctx context.Context) (*nutrientTypeResolver, error) {
	return &nutrientTypeResolver{n: inventory.NutrientType{
		NutrientID: r.nutrient.NutrientID,
		Name:       r.nutrient.Name,
		Unit:       r.nutrient.Unit,
	}}, nil
}

func (r *foodNutrientResolver) Amount() float64 { return r.nutrient.Amount }

// foodFlavorResolver resolves an item's flavor profile.
type foodFlavorResolver struct{ flavor inventory.FoodFlavor }

func (r *foodFlavorResolver) Flavor(ctx context.Context) (*flavorProfileResolver, error) {
	return &flavorProfileResolver{f: inventory.FlavorProfile{
		FlavorID: r.flavor.FlavorID,
		Name:     r.flavor.Name,
	}}, nil
}

func (r *foodFlavorResolver) Intensity() int32 { return int32(r.flavor.Intensity) }

type itemPageResolver struct {
	inv      InventoryService
	items    []inventory.Item
	ch       *itemChildren
	page     int32
	pageSize int32
	total    int32
}

func (r *itemPageResolver) Items() []*itemResolver {
	out := make([]*itemResolver, len(r.items))
	for i := range r.items {
		out[i] = &itemResolver{inv: r.inv, it: r.items[i], ch: r.ch}
	}
	return out
}

func (r *itemPageResolver) PageInfo() *pageInfoResolver {
	return &pageInfoResolver{page: r.page, pageSize: r.pageSize, total: r.total}
}

// Inputs map directly to the GraphQL input types.
type createBrandInput struct {
	Name string
}

type createCategoryInput struct {
	Name        string
	Description *string
}

type createFlavorProfileInput struct {
	Name string
}

type createNutrientTypeInput struct {
	Name string
	Unit string
}

type createItemInput struct {
	Name       string
	BrandID    *graphql.ID
	Upc12      *string
	Upc14      *string
	CategoryID graphql.ID
	Unit       string
}

type updateItemInput struct {
	Name       *string
	BrandID    *graphql.ID
	Upc12      *string
	Upc14      *string
	CategoryID *graphql.ID
	Unit       *string
}

type addFoodNutrientInput struct {
	ItemID     graphql.ID
	NutrientID graphql.ID
	Amount     float64
}

type addFoodFlavorInput struct {
	ItemID    graphql.ID
	FlavorID  graphql.ID
	Intensity int32
}

// UpdateBrand renames an existing brand.
func (r *Resolver) UpdateBrand(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateBrandInput
}) (*brandResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	b, err := r.InventoryService.UpdateBrand(ctx, id, args.Input.Name)
	if err != nil {
		return nil, err
	}
	return &brandResolver{b: b}, nil
}

// DeleteBrand removes a brand.
func (r *Resolver) DeleteBrand(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteBrand(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateCategory modifies an existing category.
func (r *Resolver) UpdateCategory(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateCategoryInput
}) (*categoryResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetCategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	description := existing.Description
	if args.Input.Description != nil {
		description = *args.Input.Description
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	c, err := r.InventoryService.UpdateCategory(ctx, id, name, description, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &categoryResolver{c: c}, nil
}

// DeleteCategory removes a category.
func (r *Resolver) DeleteCategory(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteCategory(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateFlavorProfile modifies an existing flavor profile.
func (r *Resolver) UpdateFlavorProfile(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateFlavorProfileInput
}) (*flavorProfileResolver, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetFlavorProfileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	isActive := existing.IsActive
	if args.Input.IsActive != nil {
		isActive = *args.Input.IsActive
	}
	f, err := r.InventoryService.UpdateFlavorProfile(ctx, id, name, isActive, u.Email)
	if err != nil {
		return nil, err
	}
	return &flavorProfileResolver{f: f}, nil
}

// DeleteFlavorProfile removes a flavor profile.
func (r *Resolver) DeleteFlavorProfile(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteFlavorProfile(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateNutrientType modifies an existing nutrient type.
func (r *Resolver) UpdateNutrientType(ctx context.Context, args struct {
	ID    graphql.ID
	Input updateNutrientTypeInput
}) (*nutrientTypeResolver, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return nil, err
	}
	existing, err := r.InventoryService.GetNutrientTypeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if args.Input.Name != nil {
		name = *args.Input.Name
	}
	unit := existing.Unit
	if args.Input.Unit != nil {
		unit = *args.Input.Unit
	}
	n, err := r.InventoryService.UpdateNutrientType(ctx, id, name, unit)
	if err != nil {
		return nil, err
	}
	return &nutrientTypeResolver{n: n}, nil
}

// DeleteNutrientType removes a nutrient type.
func (r *Resolver) DeleteNutrientType(ctx context.Context, args struct{ ID graphql.ID }) (bool, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return false, err
	}
	id, err := parseID(string(args.ID))
	if err != nil {
		return false, err
	}
	if err := r.InventoryService.DeleteNutrientType(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

type updateBrandInput struct {
	Name string
}

type updateCategoryInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}

type updateFlavorProfileInput struct {
	Name     *string
	IsActive *bool
}

type updateNutrientTypeInput struct {
	Name *string
	Unit *string
}
