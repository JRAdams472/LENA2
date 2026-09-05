package bff

import (
	"context"

	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

// GroceryService is the subset of *grocery.Service used by the resolver.
type GroceryService interface {
	GetGroceryListByID(ctx context.Context, groceryListID, userID int64) (grocery.GroceryList, error)
	ListGroceryLists(ctx context.Context, userID int64, limit, offset int32) ([]grocery.GroceryList, error)
	Generate(ctx context.Context, userID int64, mealPlanID int64, by string) (grocery.GroceryList, error)
	GetGroceryListItemByID(ctx context.Context, groceryListItemID int64) (grocery.GroceryListItem, error)
	UpdateGroceryListItem(ctx context.Context, groceryListItemID int64, arg grocery.GroceryListItem, by string) error
	DeleteGroceryListItem(ctx context.Context, groceryListItemID int64) error
	AddGroceryListItem(ctx context.Context, arg grocery.GroceryListItem, by string) (grocery.GroceryListItem, error)
	ListGroceryListItems(ctx context.Context, groceryListID int64) ([]grocery.GroceryListItem, error)
}

var _ GroceryService = (*grocery.Service)(nil)

// InventoryService is the subset of *inventory.Service used by the resolver.
type InventoryService interface {
	GetBrandByID(ctx context.Context, brandID int64) (inventory.Brand, error)
	ListBrands(ctx context.Context) ([]inventory.Brand, error)
	GetCategoryByID(ctx context.Context, categoryID int64) (inventory.Category, error)
	ListCategories(ctx context.Context) ([]inventory.Category, error)
	ListFlavorProfiles(ctx context.Context) ([]inventory.FlavorProfile, error)
	ListNutrientTypes(ctx context.Context) ([]inventory.NutrientType, error)
	GetItemByID(ctx context.Context, itemID int64) (inventory.Item, error)
	ListItems(ctx context.Context, limit, offset int32) ([]inventory.Item, error)
	CreateBrand(ctx context.Context, name string) (inventory.Brand, error)
	CreateCategory(ctx context.Context, name, description, by string) (inventory.Category, error)
	CreateFlavorProfile(ctx context.Context, name, by string) (inventory.FlavorProfile, error)
	CreateNutrientType(ctx context.Context, name, unit string) (inventory.NutrientType, error)
	CreateItem(ctx context.Context, arg inventory.Item, by string) (inventory.Item, error)
	CreateFoodNutrient(ctx context.Context, itemID, nutrientID int64, amount float64, by string) (inventory.FoodNutrient, error)
	DeleteFoodNutrient(ctx context.Context, itemID, nutrientID int64) error
	CreateFoodFlavor(ctx context.Context, itemID, flavorID int64, intensity int16, by string) (inventory.FoodFlavor, error)
	DeleteFoodFlavor(ctx context.Context, itemID, flavorID int64) error
	ListFoodNutrientsByItem(ctx context.Context, itemID int64) ([]inventory.FoodNutrient, error)
	ListFoodFlavorsByItem(ctx context.Context, itemID int64) ([]inventory.FoodFlavor, error)
	UpdateItem(ctx context.Context, itemID int64, arg inventory.Item, by string) error
	DeleteItem(ctx context.Context, itemID int64) error
	UpdateBrand(ctx context.Context, brandID int64, name string) (inventory.Brand, error)
	DeleteBrand(ctx context.Context, brandID int64) error
	UpdateCategory(ctx context.Context, categoryID int64, name, description string, isActive bool, by string) (inventory.Category, error)
	DeleteCategory(ctx context.Context, categoryID int64) error
	GetFlavorProfileByID(ctx context.Context, flavorID int64) (inventory.FlavorProfile, error)
	UpdateFlavorProfile(ctx context.Context, flavorID int64, name string, isActive bool, by string) (inventory.FlavorProfile, error)
	DeleteFlavorProfile(ctx context.Context, flavorID int64) error
	GetNutrientTypeByID(ctx context.Context, nutrientID int64) (inventory.NutrientType, error)
	UpdateNutrientType(ctx context.Context, nutrientID int64, name, unit string) (inventory.NutrientType, error)
	DeleteNutrientType(ctx context.Context, nutrientID int64) error
}

var _ InventoryService = (*inventory.Service)(nil)

// MealPlanService is the subset of *mealplan.Service used by the resolver.
type MealPlanService interface {
	GetMealPlanByID(ctx context.Context, mealPlanID, userID int64) (mealplan.MealPlan, error)
	ListMealPlans(ctx context.Context, userID int64, limit, offset int32) ([]mealplan.MealPlan, error)
	ListMealSlotsForPlan(ctx context.Context, mealPlanID int64) ([]mealplan.MealSlot, error)
	ListMealSlotItems(ctx context.Context, slotID int64) ([]mealplan.MealSlotItem, error)
	CreateMealPlan(ctx context.Context, arg mealplan.MealPlan, by string) (mealplan.MealPlan, error)
	UpdateMealPlan(ctx context.Context, mealPlanID, userID int64, arg mealplan.MealPlan, by string) error
	DeleteMealPlan(ctx context.Context, mealPlanID, userID int64) error
	AddMealSlot(ctx context.Context, arg mealplan.MealSlot, by string) (mealplan.MealSlot, error)
	DeleteMealSlot(ctx context.Context, slotID int64) error
	AddMealSlotItem(ctx context.Context, arg mealplan.MealSlotItem, by string) (mealplan.MealSlotItem, error)
	DeleteMealSlotItem(ctx context.Context, slotItemID int64) error
}

var _ MealPlanService = (*mealplan.Service)(nil)

// RecipeService is the subset of *recipe.Service used by the resolver.
type RecipeService interface {
	GetRecipeByID(ctx context.Context, recipeID int64) (recipe.Recipe, error)
	ListRecipes(ctx context.Context, active bool, limit, offset int32) ([]recipe.Recipe, error)
	CreateRecipe(ctx context.Context, arg recipe.Recipe, by string) (recipe.Recipe, error)
	AddRecipeItem(ctx context.Context, arg recipe.RecipeItem) error
	AddRecipeStep(ctx context.Context, recipeID int64, stepNumber int32, instruction, by string) (recipe.RecipeStep, error)
	UpdateRecipe(ctx context.Context, recipeID int64, arg recipe.Recipe, by string) error
	ListRecipeItems(ctx context.Context, recipeID int64) ([]recipe.RecipeItem, error)
	RemoveRecipeItem(ctx context.Context, recipeID, itemID int64) error
	ListRecipeSteps(ctx context.Context, recipeID int64) ([]recipe.RecipeStep, error)
	DeleteRecipeStep(ctx context.Context, stepID int64) error
	DeleteRecipe(ctx context.Context, recipeID int64) error
}

var _ RecipeService = (*recipe.Service)(nil)

// UserPrefsService is the subset of *userprefs.Service used by the resolver.
type UserPrefsService interface {
	ListUserBottles(ctx context.Context, userID int64, limit, offset int32) ([]userprefs.UserBottle, error)
	ListUserItems(ctx context.Context, userID int64, limit, offset int32) ([]userprefs.UserItem, error)
	SetRecipeFavorite(ctx context.Context, userID, recipeID int64, isFavorite bool, by string) (userprefs.RecipeFavorite, error)
	UpsertUserItem(ctx context.Context, arg userprefs.UserItem, by string) (userprefs.UserItem, error)
	DeleteUserItem(ctx context.Context, userItemID, userID int64) error
	UpsertUserBottle(ctx context.Context, arg userprefs.UserBottle, by string) (userprefs.UserBottle, error)
	GetRecipeFavorite(ctx context.Context, userID, recipeID int64) (userprefs.RecipeFavorite, error)
}

var _ UserPrefsService = (*userprefs.Service)(nil)

// WineService is the subset of *wine.Service used by the resolver.
type WineService interface {
	GetBottleByID(ctx context.Context, bottleID int64) (wine.Bottle, error)
	ListBottles(ctx context.Context, limit, offset int32) ([]wine.Bottle, error)
	ListTypes(ctx context.Context) ([]wine.Type, error)
	ListCountries(ctx context.Context) ([]wine.Country, error)
	ListRegions(ctx context.Context, countryID int64) ([]wine.Region, error)
	ListVintages(ctx context.Context) ([]wine.Vintage, error)
	ListGrapeVarieties(ctx context.Context) ([]wine.GrapeVariety, error)
	ListWineFlavorProfiles(ctx context.Context) ([]wine.WineFlavorProfile, error)
	CreateWineFlavorProfile(ctx context.Context, name, description, by string) (wine.WineFlavorProfile, error)
	GetWineFlavorProfileByID(ctx context.Context, flavorProfileID int64) (wine.WineFlavorProfile, error)
	UpdateWineFlavorProfile(ctx context.Context, flavorProfileID int64, name, description string, isActive bool, by string) (wine.WineFlavorProfile, error)
	DeleteWineFlavorProfile(ctx context.Context, flavorProfileID int64) error
	CreateVintage(ctx context.Context, year int32, description, by string) (wine.Vintage, error)
	CreateGrapeVariety(ctx context.Context, name, description, by string) (wine.GrapeVariety, error)
	CreateBottle(ctx context.Context, arg wine.Bottle, by string) (wine.Bottle, error)
	UpdateBottle(ctx context.Context, bottleID int64, arg wine.Bottle, by string) error
	DeleteBottle(ctx context.Context, bottleID int64) error
	AddBottleGrapeVariety(ctx context.Context, bottleID, grapeVarietyID int64, percentage int16, by string) (wine.BottleGrapeVariety, error)
	RemoveBottleGrapeVariety(ctx context.Context, bottleID, grapeVarietyID int64) error
	AddBottleFlavorProfile(ctx context.Context, bottleID, flavorProfileID int64, intensity int16, by string) (wine.BottleFlavorProfile, error)
	RemoveBottleFlavorProfile(ctx context.Context, bottleID, flavorProfileID int64) error
	ListBottleGrapeVarieties(ctx context.Context, bottleID int64) ([]wine.BottleGrapeVariety, error)
	ListBottleFlavorProfiles(ctx context.Context, bottleID int64) ([]wine.BottleFlavorProfile, error)
	CreateCountry(ctx context.Context, name, isoCode, description, by string) (wine.Country, error)
	GetCountryByID(ctx context.Context, countryID int64) (wine.Country, error)
	UpdateCountry(ctx context.Context, countryID int64, name, isoCode, description string, isActive bool, by string) (wine.Country, error)
	DeleteCountry(ctx context.Context, countryID int64) error
	CreateRegion(ctx context.Context, arg wine.Region, by string) (wine.Region, error)
	GetRegionByID(ctx context.Context, regionID int64) (wine.Region, error)
	UpdateRegion(ctx context.Context, regionID, countryID int64, name, description string, isActive bool, by string) (wine.Region, error)
	DeleteRegion(ctx context.Context, regionID int64) error
	CreateType(ctx context.Context, name, description, by string) (wine.Type, error)
	GetTypeByID(ctx context.Context, typeID int64) (wine.Type, error)
	UpdateType(ctx context.Context, typeID int64, name, description string, isActive bool, by string) (wine.Type, error)
	DeleteType(ctx context.Context, typeID int64) error
	GetVintageByID(ctx context.Context, vintageID int64) (wine.Vintage, error)
	UpdateVintage(ctx context.Context, vintageID int64, year int32, description string, isActive bool, by string) (wine.Vintage, error)
	DeleteVintage(ctx context.Context, vintageID int64) error
	GetGrapeVarietyByID(ctx context.Context, grapeVarietyID int64) (wine.GrapeVariety, error)
	UpdateGrapeVariety(ctx context.Context, grapeVarietyID int64, name, description string, isActive bool, by string) (wine.GrapeVariety, error)
	DeleteGrapeVariety(ctx context.Context, grapeVarietyID int64) error
}

var _ WineService = (*wine.Service)(nil)
