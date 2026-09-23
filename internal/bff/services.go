package bff

import (
	"context"
	"time"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/app/recipeimport"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/inventory/nutritionparse"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/ocrimport"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

// The BFF service surfaces are decomposed into role interfaces: readers for
// child resolvers and queries, writers for member mutations, and admin
// roles for moderation paths. The domain services satisfy the composed
// interfaces; child resolvers hold only the narrow role they need so a
// nested type can never reach back into a write or admin surface.

// GroceryReader is the read side of the grocery domain.
type GroceryReader interface {
	GetGroceryListByID(ctx context.Context, groceryListID, userID int64) (grocery.GroceryList, error)
	ListGroceryLists(ctx context.Context, userID int64, limit, offset int32) ([]grocery.GroceryList, error)
	CountGroceryLists(ctx context.Context, userID int64) (int64, error)
	GetGroceryListItemByID(ctx context.Context, groceryListItemID, userID int64) (grocery.GroceryListItem, error)
	ListGroceryListItems(ctx context.Context, groceryListID, userID int64) ([]grocery.GroceryListItem, error)
	ListGroceryListItemsByLists(ctx context.Context, groceryListIDs []int64, userID int64) ([]grocery.GroceryListItem, error)
}

// GroceryWriter is the write side of the grocery domain.
type GroceryWriter interface {
	CreateGroceryList(ctx context.Context, userID int64, mealPlanID *int64, by string) (grocery.GroceryList, error)
	AddGroceryListItems(ctx context.Context, items []grocery.GroceryListItem, userID int64, by string) ([]grocery.GroceryListItem, error)
	UpdateGroceryListItem(ctx context.Context, groceryListItemID, userID int64, arg grocery.GroceryListItem, by string) error
	ToggleGroceryListItemChecked(ctx context.Context, groceryListItemID, userID int64, by string) (grocery.GroceryListItem, error)
	DeleteGroceryListItem(ctx context.Context, groceryListItemID, userID int64) error
	AddGroceryListItem(ctx context.Context, arg grocery.GroceryListItem, userID int64, by string) (grocery.GroceryListItem, error)
}

// GroceryService is the subset of *grocery.Service used by the resolver.
type GroceryService interface {
	GroceryReader
	GroceryWriter
}

var _ GroceryService = (*grocery.Service)(nil)

// ItemReader is the read-only catalog surface used by nested item,
// ingredient, and unit resolvers as well as the batch preload helpers.
type ItemReader interface {
	GetBrandByID(ctx context.Context, brandID int64) (inventory.Brand, error)
	ListBrands(ctx context.Context) ([]inventory.Brand, error)
	GetCategoryByID(ctx context.Context, categoryID int64) (inventory.Category, error)
	ListCategories(ctx context.Context) ([]inventory.Category, error)
	ListFlavorProfiles(ctx context.Context) ([]inventory.FlavorProfile, error)
	ListNutrientTypes(ctx context.Context) ([]inventory.NutrientType, error)
	GetItemByID(ctx context.Context, itemID int64) (inventory.Item, error)
	GetItemByUpc(ctx context.Context, code string, userID int64) (inventory.Item, error)
	ListItems(ctx context.Context, userID int64, limit, offset int32) ([]inventory.Item, error)
	CountItems(ctx context.Context, userID int64) (int64, error)
	SearchBrands(ctx context.Context, term string, userID int64, limit int32) ([]inventory.Brand, error)
	ListBrandsVisible(ctx context.Context, userID int64, limit, offset int32) ([]inventory.Brand, error)
	CountBrandsVisible(ctx context.Context, userID int64) (int64, error)
	GetItemsByIDs(ctx context.Context, itemIDs []int64) ([]inventory.Item, error)
	GetBrandsByIDs(ctx context.Context, brandIDs []int64) ([]inventory.Brand, error)
	GetCategoriesByIDs(ctx context.Context, categoryIDs []int64) ([]inventory.Category, error)
	ListFoodNutrientsByItem(ctx context.Context, itemID int64) ([]inventory.FoodNutrient, error)
	ListFoodNutrientsByItems(ctx context.Context, itemIDs []int64) ([]inventory.FoodNutrient, error)
	ListFoodFlavorsByItem(ctx context.Context, itemID int64) ([]inventory.FoodFlavor, error)
	ListFoodFlavorsByItems(ctx context.Context, itemIDs []int64) ([]inventory.FoodFlavor, error)
	GetIngredientByID(ctx context.Context, ingredientID int64) (inventory.Ingredient, error)
	GetIngredientsByIDs(ctx context.Context, ingredientIDs []int64) ([]inventory.Ingredient, error)
	ListIngredients(ctx context.Context, limit, offset int32) ([]inventory.Ingredient, error)
	CountIngredients(ctx context.Context) (int64, error)
	GetUnitByID(ctx context.Context, unitID int64) (inventory.Unit, error)
	GetUnitByName(ctx context.Context, name string) (inventory.Unit, error)
	GetUnitsByIDs(ctx context.Context, unitIDs []int64) ([]inventory.Unit, error)
	ListUnits(ctx context.Context) ([]inventory.Unit, error)
	GetFlavorProfileByID(ctx context.Context, flavorProfileID int64) (inventory.FlavorProfile, error)
	GetNutrientTypeByID(ctx context.Context, nutrientID int64) (inventory.NutrientType, error)
	GetNutrientTypeByName(ctx context.Context, name string) (inventory.NutrientType, error)
}

// ItemWriter is the member-facing catalog write surface (submissions and
// user-editable fields).
type ItemWriter interface {
	SubmitItem(ctx context.Context, arg inventory.Item, userID int64, by string) (inventory.Item, error)
	SubmitBrand(ctx context.Context, name string, userID int64, by string) (inventory.Brand, error)
	UpdateItem(ctx context.Context, itemID int64, arg inventory.Item, by string) error
	ApplyNutritionLabel(ctx context.Context, itemID int64, parsed []nutritionparse.Nutrient, by string) error
}

// CatalogAdmin is the admin moderation surface for the catalog.
type CatalogAdmin interface {
	ListPendingItems(ctx context.Context, limit, offset int32) ([]inventory.Item, error)
	CountPendingItems(ctx context.Context) (int64, error)
	SetItemStatus(ctx context.Context, itemID int64, status string, approverUserID int64, by string) error
	ListPendingBrands(ctx context.Context, limit, offset int32) ([]inventory.Brand, error)
	CountPendingBrands(ctx context.Context) (int64, error)
	SetBrandStatus(ctx context.Context, brandID int64, status string, approverUserID int64, by string) error
	CreateItem(ctx context.Context, arg inventory.Item, by string) (inventory.Item, error)
	DeleteItem(ctx context.Context, itemID int64) error
	CreateBrand(ctx context.Context, name, by string) (inventory.Brand, error)
	UpdateBrand(ctx context.Context, brandID int64, name string) (inventory.Brand, error)
	DeleteBrand(ctx context.Context, brandID int64) error
	CreateCategory(ctx context.Context, name, description, by string) (inventory.Category, error)
	UpdateCategory(ctx context.Context, categoryID int64, name, description string, isActive bool, by string) (inventory.Category, error)
	DeleteCategory(ctx context.Context, categoryID int64) error
	CreateFlavorProfile(ctx context.Context, name, by string) (inventory.FlavorProfile, error)
	UpdateFlavorProfile(ctx context.Context, flavorProfileID int64, name string, isActive bool, by string) (inventory.FlavorProfile, error)
	DeleteFlavorProfile(ctx context.Context, flavorProfileID int64) error
	CreateNutrientType(ctx context.Context, name, unit string) (inventory.NutrientType, error)
	UpdateNutrientType(ctx context.Context, nutrientID int64, name, unit string) (inventory.NutrientType, error)
	DeleteNutrientType(ctx context.Context, nutrientID int64) error
	CreateIngredient(ctx context.Context, arg inventory.Ingredient, by string) (inventory.Ingredient, error)
	UpdateIngredient(ctx context.Context, ingredientID int64, arg inventory.Ingredient, by string) (inventory.Ingredient, error)
	DeleteIngredient(ctx context.Context, ingredientID int64) error
	SetItemNutrients(ctx context.Context, itemID int64, entries []inventory.NutrientEntry, by string) error
	CreateFoodNutrient(ctx context.Context, itemID, nutrientID int64, amount float64, by string) (inventory.FoodNutrient, error)
	DeleteFoodNutrient(ctx context.Context, itemID, nutrientID int64) error
	CreateFoodFlavor(ctx context.Context, itemID, flavorID int64, intensity int16, by string) (inventory.FoodFlavor, error)
	DeleteFoodFlavor(ctx context.Context, itemID, flavorID int64) error
}

// InventoryService is the subset of *inventory.Service used by the resolver.
type InventoryService interface {
	ItemReader
	ItemWriter
	CatalogAdmin
}

var _ InventoryService = (*inventory.Service)(nil)

// MealPlanReader is the read side of the meal-plan domain.
type MealPlanReader interface {
	GetMealPlanByID(ctx context.Context, mealPlanID, userID int64) (mealplan.MealPlan, error)
	ListMealPlans(ctx context.Context, userID int64, limit, offset int32) ([]mealplan.MealPlan, error)
	CountMealPlans(ctx context.Context, userID int64) (int64, error)
	ListMealSlotsForPlan(ctx context.Context, mealPlanID, userID int64) ([]mealplan.MealSlot, error)
	ListMealSlotsByPlans(ctx context.Context, mealPlanIDs []int64, userID int64) ([]mealplan.MealSlot, error)
	ListMealSlotItems(ctx context.Context, slotID, userID int64) ([]mealplan.MealSlotItem, error)
	ListMealSlotItemsByPlan(ctx context.Context, mealPlanID, userID int64) ([]mealplan.MealSlotItem, error)
	ListMealSlotItemsByPlans(ctx context.Context, mealPlanIDs []int64, userID int64) ([]mealplan.MealSlotItem, error)
	LastPlannedDates(ctx context.Context, userID int64, recipeIDs []int64) (map[int64]time.Time, error)
}

// MealPlanWriter is the write side of the meal-plan domain.
type MealPlanWriter interface {
	CreateMealPlan(ctx context.Context, arg mealplan.MealPlan, by string) (mealplan.MealPlan, error)
	UpdateMealPlan(ctx context.Context, mealPlanID, userID int64, arg mealplan.MealPlan, by string) error
	DeleteMealPlan(ctx context.Context, mealPlanID, userID int64) error
	AddMealSlot(ctx context.Context, arg mealplan.MealSlot, userID int64, by string) (mealplan.MealSlot, error)
	DeleteMealSlot(ctx context.Context, slotID, userID int64) error
	AddMealSlotItem(ctx context.Context, arg mealplan.MealSlotItem, userID int64, by string) (mealplan.MealSlotItem, error)
	DeleteMealSlotItem(ctx context.Context, slotItemID, userID int64) error
}

// MealPlanService is the subset of *mealplan.Service used by the resolver.
type MealPlanService interface {
	MealPlanReader
	MealPlanWriter
}

var _ MealPlanService = (*mealplan.Service)(nil)

// RecipeReader is the read side of the recipe domain.
type RecipeReader interface {
	GetRecipeByID(ctx context.Context, recipeID int64) (recipe.Recipe, error)
	ScaleRecipe(ctx context.Context, recipeID int64, servings int32) (recipe.ScaledRecipe, error)
	ListRecipes(ctx context.Context, active bool, limit, offset int32) ([]recipe.Recipe, error)
	CountRecipes(ctx context.Context, active bool) (int64, error)
	GetRecipesByIDs(ctx context.Context, recipeIDs []int64) ([]recipe.Recipe, error)
	ListRecipeItemsByRecipes(ctx context.Context, recipeIDs []int64) ([]recipe.RecipeItem, error)
	ListRecipeItems(ctx context.Context, recipeID int64) ([]recipe.RecipeItem, error)
	ListRecipeSteps(ctx context.Context, recipeID int64) ([]recipe.RecipeStep, error)
	ListRecipeStepsByRecipes(ctx context.Context, recipeIDs []int64) ([]recipe.RecipeStep, error)
}

// RecipeWriter is the write side of the recipe domain.
type RecipeWriter interface {
	CreateRecipe(ctx context.Context, arg recipe.Recipe, by string) (recipe.Recipe, error)
	CreateRecipeWithChildren(ctx context.Context, arg recipe.Recipe, items []recipe.RecipeItem, steps []recipe.RecipeStep, by string) (recipe.Recipe, error)
	UpdateRecipeWithChildren(ctx context.Context, recipeID int64, arg recipe.Recipe, items []recipe.RecipeItem, steps []recipe.RecipeStep, by string) error
	AddRecipeItem(ctx context.Context, arg recipe.RecipeItem) error
	AddRecipeStep(ctx context.Context, recipeID int64, stepNumber int32, instruction, by string) (recipe.RecipeStep, error)
	UpdateRecipe(ctx context.Context, recipeID int64, arg recipe.Recipe, by string) error
	RemoveRecipeItem(ctx context.Context, recipeItemID int64) error
	DeleteRecipeStep(ctx context.Context, stepID int64) error
	DeleteRecipe(ctx context.Context, recipeID int64) error
}

// RecipeRater is the rating surface of the recipe domain.
type RecipeRater interface {
	SetRating(ctx context.Context, userID, recipeID int64, rating int16, by string) (recipe.RecipeRating, error)
	GetUserRating(ctx context.Context, userID, recipeID int64) (recipe.RecipeRating, error)
	ListRecipeRatings(ctx context.Context, userID int64, recipeIDs []int64) ([]recipe.RecipeRating, error)
	ListRatingSummaries(ctx context.Context, recipeIDs []int64) ([]recipe.RatingSummary, error)
	ListRatedAtLeast(ctx context.Context, userID int64, minRating int16) ([]recipe.RecipeRating, error)
}

// RecipeService is the subset of *recipe.Service used by the resolver.
type RecipeService interface {
	RecipeReader
	RecipeWriter
	RecipeRater
}

var _ RecipeService = (*recipe.Service)(nil)

// ImportSubmitter is the member-facing recipe import surface.
type ImportSubmitter interface {
	Submit(ctx context.Context, mediaType string, data []byte, submittedByUserID *int64, by string) (*recipeimport.RecipeImport, error)
	Get(ctx context.Context, id int64) (*recipeimport.RecipeImport, error)
}

// ImportReviewer is the admin review surface of recipe imports.
type ImportReviewer interface {
	List(ctx context.Context, status string, page, pageSize int32) ([]recipeimport.RecipeImport, error)
	Count(ctx context.Context, status string) (int64, error)
	ListPending(ctx context.Context, page, pageSize int32) ([]recipeimport.RecipeImport, error)
	CountPending(ctx context.Context) (int64, error)
	UpdateReview(ctx context.Context, id int64, review *ocrimport.ReviewRecipe, updatedBy string) (*recipeimport.RecipeImport, error)
	Approve(ctx context.Context, id int64, approvedBy currentuser.User) (*recipe.Recipe, *recipeimport.RecipeImport, error)
	Reject(ctx context.Context, id int64) error
	Retry(ctx context.Context, id int64) error
}

// RecipeImportService is the subset of *recipeimport.Service used by the
// resolver, plus lifecycle draining for graceful shutdown.
type RecipeImportService interface {
	ImportSubmitter
	ImportReviewer
	Shutdown(ctx context.Context) error
}

var _ RecipeImportService = (*recipeimport.Service)(nil)

// PantryStore is the user-item (pantry) surface of userprefs.
type PantryStore interface {
	ListUserItems(ctx context.Context, userID int64, limit, offset int32) ([]userprefs.UserItem, error)
	GetUserItemByUserAndItem(ctx context.Context, userID, itemID int64) (*userprefs.UserItem, error)
	CountUserItems(ctx context.Context, userID int64) (int64, error)
	UpsertUserItem(ctx context.Context, arg userprefs.UserItem, by string) (userprefs.UserItem, error)
	AdjustUserItemQuantity(ctx context.Context, userID, itemID int64, delta float64, by string) (userprefs.UserItem, error)
	DeleteUserItem(ctx context.Context, userItemID, userID int64) error
}

// CellarStore is the user-bottle (cellar) surface of userprefs.
type CellarStore interface {
	ListUserBottles(ctx context.Context, userID int64, limit, offset int32) ([]userprefs.UserBottle, error)
	GetUserBottleByUserAndBottle(ctx context.Context, userID, bottleID int64) (*userprefs.UserBottle, error)
	CountUserBottles(ctx context.Context, userID int64) (int64, error)
	UpsertUserBottle(ctx context.Context, arg userprefs.UserBottle, by string) (userprefs.UserBottle, error)
}

// FavoriteStore is the recipe-favorite surface of userprefs.
type FavoriteStore interface {
	SetRecipeFavorite(ctx context.Context, userID, recipeID int64, isFavorite bool, by string) (userprefs.RecipeFavorite, error)
	GetRecipeFavorite(ctx context.Context, userID, recipeID int64) (userprefs.RecipeFavorite, error)
	ListRecipeFavorites(ctx context.Context, userID int64, recipeIDs []int64) ([]userprefs.RecipeFavorite, error)
}

// UserPrefsService is the subset of *userprefs.Service used by the resolver.
type UserPrefsService interface {
	PantryStore
	CellarStore
	FavoriteStore
}

var _ UserPrefsService = (*userprefs.Service)(nil)

// UserReader is the profile-read surface of identity.
type UserReader interface {
	GetByID(ctx context.Context, userID int64) (identity.User, error)
	IsProtected(provider, email string) bool
}

// UserAdmin is the admin user-management surface of identity.
type UserAdmin interface {
	ListUsers(ctx context.Context, limit, offset int32) ([]identity.User, error)
	CountUsers(ctx context.Context) (int64, error)
	AdminSetRole(ctx context.Context, actorID, targetID int64, role string) error
	AdminSetActive(ctx context.Context, actorID, targetID int64, active bool, by string) error
}

// IdentityService is the subset of *identity.Service used by the resolver
// for profile reads and admin user-management operations.
type IdentityService interface {
	UserReader
	UserAdmin
	UpdateProfile(ctx context.Context, userID int64, firstName, lastName, backupEmail, by string) error
}

var _ IdentityService = (*identity.Service)(nil)

// EventRecorder is the event-ingest surface of analytics.
type EventRecorder interface {
	RecordEvent(ctx context.Context, e analytics.Event, by string) error
	ComputeIngredientOverlapSuggestions(ctx context.Context, newRecipeID int64) (int, error)
}

// RecommendationReader is the read-model surface of analytics.
type RecommendationReader interface {
	GetUserSelectionCounts(ctx context.Context, userID int64, entityType string, entityIDs []int64) ([]analytics.SelectionCount, error)
	GetGlobalSelectionCounts(ctx context.Context, entityType string, entityIDs []int64) ([]analytics.SelectionCount, error)
	TopUserSelections(ctx context.Context, userID int64, entityType string, limit int32) ([]analytics.SelectionCount, error)
	TopGlobalSelections(ctx context.Context, entityType string, limit int32) ([]analytics.SelectionCount, error)
	ListRecipeRecommendations(ctx context.Context, userID int64, reason string, limit int32) ([]analytics.Recommendation, error)
}

// AnalyticsService is the subset of *analytics.Service used by the resolver.
type AnalyticsService interface {
	EventRecorder
	RecommendationReader
}

var _ AnalyticsService = (*analytics.Service)(nil)

// BottleReader is the read-only wine surface used by nested bottle
// resolvers and the bottle preload helper.
type BottleReader interface {
	GetBottleByID(ctx context.Context, bottleID int64) (wine.Bottle, error)
	ListBottles(ctx context.Context, limit, offset int32) ([]wine.Bottle, error)
	CountBottles(ctx context.Context) (int64, error)
	ListTypes(ctx context.Context) ([]wine.Type, error)
	ListCountries(ctx context.Context) ([]wine.Country, error)
	ListRegions(ctx context.Context, countryID int64) ([]wine.Region, error)
	ListVintages(ctx context.Context) ([]wine.Vintage, error)
	ListGrapeVarieties(ctx context.Context) ([]wine.GrapeVariety, error)
	ListWineFlavorProfiles(ctx context.Context) ([]wine.WineFlavorProfile, error)
	GetWineFlavorProfileByID(ctx context.Context, flavorProfileID int64) (wine.WineFlavorProfile, error)
	GetBottlesByIDs(ctx context.Context, bottleIDs []int64) ([]wine.Bottle, error)
	ListBottleGrapeVarieties(ctx context.Context, bottleID int64) ([]wine.BottleGrapeVariety, error)
	ListBottleGrapeVarietiesByBottles(ctx context.Context, bottleIDs []int64) ([]wine.BottleGrapeVariety, error)
	ListBottleFlavorProfiles(ctx context.Context, bottleID int64) ([]wine.BottleFlavorProfile, error)
	ListBottleFlavorProfilesByBottles(ctx context.Context, bottleIDs []int64) ([]wine.BottleFlavorProfile, error)
	GetCountryByID(ctx context.Context, countryID int64) (wine.Country, error)
	GetRegionByID(ctx context.Context, regionID int64) (wine.Region, error)
	GetTypeByID(ctx context.Context, typeID int64) (wine.Type, error)
	GetVintageByID(ctx context.Context, vintageID int64) (wine.Vintage, error)
	GetGrapeVarietyByID(ctx context.Context, grapeVarietyID int64) (wine.GrapeVariety, error)
}

// BottleWriter is the member-facing bottle write surface.
type BottleWriter interface {
	CreateBottle(ctx context.Context, arg wine.Bottle, by string) (wine.Bottle, error)
	UpdateBottle(ctx context.Context, bottleID int64, arg wine.Bottle, by string) error
	DeleteBottle(ctx context.Context, bottleID int64) error
	AddBottleGrapeVariety(ctx context.Context, bottleID, grapeVarietyID int64, percentage *int16, by string) (wine.BottleGrapeVariety, error)
	RemoveBottleGrapeVariety(ctx context.Context, bottleID, grapeVarietyID int64) error
	AddBottleFlavorProfile(ctx context.Context, bottleID, flavorProfileID int64, intensity int16, by string) (wine.BottleFlavorProfile, error)
	RemoveBottleFlavorProfile(ctx context.Context, bottleID, flavorProfileID int64) error
}

// WineCatalogAdmin is the admin surface for wine taxonomy and reference data.
type WineCatalogAdmin interface {
	CreateWineFlavorProfile(ctx context.Context, name, description, by string) (wine.WineFlavorProfile, error)
	UpdateWineFlavorProfile(ctx context.Context, flavorProfileID int64, name, description string, isActive bool, by string) (wine.WineFlavorProfile, error)
	DeleteWineFlavorProfile(ctx context.Context, flavorProfileID int64) error
	CreateVintage(ctx context.Context, year int32, description, by string) (wine.Vintage, error)
	CreateGrapeVariety(ctx context.Context, name, description, by string) (wine.GrapeVariety, error)
	CreateCountry(ctx context.Context, name, isoCode, description, by string) (wine.Country, error)
	UpdateCountry(ctx context.Context, countryID int64, name, isoCode, description string, isActive bool, by string) (wine.Country, error)
	DeleteCountry(ctx context.Context, countryID int64) error
	CreateRegion(ctx context.Context, arg wine.Region, by string) (wine.Region, error)
	UpdateRegion(ctx context.Context, regionID, countryID int64, name, description string, isActive bool, by string) (wine.Region, error)
	DeleteRegion(ctx context.Context, regionID int64) error
	CreateType(ctx context.Context, name, description, by string) (wine.Type, error)
	UpdateType(ctx context.Context, typeID int64, name, description string, isActive bool, by string) (wine.Type, error)
	DeleteType(ctx context.Context, typeID int64) error
	UpdateVintage(ctx context.Context, vintageID int64, year int32, description string, isActive bool, by string) (wine.Vintage, error)
	DeleteVintage(ctx context.Context, vintageID int64) error
	UpdateGrapeVariety(ctx context.Context, grapeVarietyID int64, name, description string, isActive bool, by string) (wine.GrapeVariety, error)
	DeleteGrapeVariety(ctx context.Context, grapeVarietyID int64) error
}

// WineService is the subset of *wine.Service used by the resolver.
type WineService interface {
	BottleReader
	BottleWriter
	WineCatalogAdmin
}

var _ WineService = (*wine.Service)(nil)
