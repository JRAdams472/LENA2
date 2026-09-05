export interface PagedResult<T> {
  items: T[];
  pageNumber: number;
  pageSize: number;
  totalCount: number;
  totalPages: number;
}

export interface AuditableEntity {
  createdBy: string;
  createDate: string;
  lastUpdatedBy: string | null;
  lastUpdatedDate: string | null;
}

export interface User {
  userID: number;
  email: string;
  displayName: string | null;
  externalSubject: string | null;
  provider: string | null;
}

export interface Category extends AuditableEntity {
  categoryID: number;
  categoryName: string;
  description: string | null;
  isActive: boolean;
}

export interface FlavorProfile {
  flavorId: number;
  flavorName: string;
  isActive: boolean;
  foodFlavors?: FoodFlavor[] | null;
}

export interface FoodFlavor {
  foodId: number;
  flavorId: number;
  intensityScore: number;
  item: Item | null;
  flavorProfile: FlavorProfile | null;
}

export interface FoodNutrient {
  foodId: number;
  nutrientId: number;
  amountPerServing: number;
  nutrientType: NutrientType | null;
}

export interface Item extends AuditableEntity {
  itemID: number;
  name: string;
  brand: string | null;
  upc12: string | null;
  upc14: string | null;
  categoryID: number;
  unit: string;
  currentQuantity: number;
  minQuantity: number | null;
  purchaseDate: string | null;
  expiryDate: string | null;
  notes: string | null;
  isFavorite: boolean;
  category: Category | null;
  foodNutrients: FoodNutrient[] | null;
  foodFlavors: FoodFlavor[] | null;
}

export interface NutrientType {
  nutrientId: number;
  nutrientName: string;
  unitOfMeasure: string;
}

export interface Brand {
  brandID: number;
  brandName: string;
}

export interface Bottle extends AuditableEntity {
  bottleID: number;
  bottleNumber: number | null;
  typeID: number;
  countryID: number;
  regionID: number;
  vintageYear: number;
  vineyard: string | null;
  abv: number | null;
  acidity: number | null;
  tanninLevel: number | null;
  body: number | null;
  sweetness: number | null;
  oakIntegration: boolean | null;
  bottleSize: string;
  quantity: number;
  purchaseDate: string | null;
  purchasePrice: number | null;
  storageTemp: number | null;
  location: string | null;
  notes: string | null;
  isFavorite: boolean;
  type: WineType | null;
  country: Country | null;
  region: Region | null;
  vintage: Vintage | null;
  bottleGrapeVarieties: BottleGrapeVariety[];
  bottleFlavorProfiles: BottleFlavorProfile[];
}

export interface BottleFlavorProfile extends AuditableEntity {
  flavorProfileID: number;
  flavorProfileName: string;
  description: string | null;
  isActive: boolean;
}

export interface BottleGrapeVariety extends AuditableEntity {
  bottleID: number;
  grapeVarietyID: number;
  percentage: number | null;
  bottle: Bottle;
  grapeVariety: GrapeVariety;
}

export interface Country extends AuditableEntity {
  countryID: number;
  countryName: string;
  isoCode: string;
  description: string | null;
  isActive: boolean;
  regions: Region[];
  bottles: Bottle[];
}

export interface GrapeVariety extends AuditableEntity {
  grapeVarietyID: number;
  grapeVarietyName: string;
  description: string | null;
  isActive: boolean;
  bottleGrapeVarieties: BottleGrapeVariety[];
}

export interface WineFlavorProfile extends AuditableEntity {
  flavorProfileID: number;
  flavorProfileName: string;
  description: string | null;
  isActive: boolean;
}

export interface Region extends AuditableEntity {
  regionID: number;
  regionName: string;
  countryID: number;
  description: string | null;
  isActive: boolean;
  country?: Country | null;
  bottles?: Bottle[];
}

export interface WineType extends AuditableEntity {
  typeID: number;
  typeName: string;
  description: string | null;
  isActive: boolean;
  bottles: Bottle[];
}

export interface Vintage extends AuditableEntity {
  vintageID: number;
  year: number;
  description: string | null;
  isActive: boolean;
  bottles: Bottle[];
}

export interface Recipe extends AuditableEntity {
  recipeID: number;
  recipeName: string;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  isActive: boolean;
  isFavorite: boolean;
  recipeItems?: RecipeItem[];
  recipeSteps?: RecipeStep[];
}

export interface RecipeItem {
  recipeID: number;
  itemID: number;
  quantity: number;
  unitOfMeasure: string | null;
  notes: string | null;
  isOptional: boolean;
  itemName?: string | null;
  itemBrand?: string | null;
  recipe?: Recipe | null;
  item?: Item | null;
}

export interface RecipeStep extends AuditableEntity {
  recipeStepID: number;
  recipeID: number;
  stepNumber: number;
  instruction: string;
  recipe?: Recipe | null;
}

export interface MealPlan extends AuditableEntity {
  mealPlanID: number;
  planName: string;
  weekStartDate: string;
  weekStartDayOfWeek: number;
  isActive: boolean;
  mealSlots?: MealSlot[];
}

export interface MealSlot extends AuditableEntity {
  mealSlotID: number;
  mealPlanID: number;
  dayOfWeek: number;
  mealType: number;
  recipeID: number | null;
  servings: number;
  replacementNote: string | null;
  mealPlan?: MealPlan | null;
  recipe?: Recipe | null;
  mealSlotItems?: MealSlotItem[];
}

export interface MealSlotItem extends AuditableEntity {
  mealSlotItemID: number;
  mealSlotID: number;
  itemID: number;
  quantity: number;
  unitOfMeasure: string | null;
  isFromRecipe: boolean;
  mealSlot?: MealSlot | null;
  item?: Item | null;
}

export interface NutrientAmount {
  nutrientId: number;
  nutrientName: string;
  unitOfMeasure: string;
  amount: number;
}

export interface DailyNutrition {
  dayOfWeek: number;
  nutrients: NutrientAmount[];
}

export interface MealNutrition {
  dayOfWeek: number;
  mealType: number;
  mealSlotId: number;
  nutrients: NutrientAmount[];
}

export interface MealPlanNutrition {
  mealPlanId: number;
  dailyTotals: DailyNutrition[];
  meals: MealNutrition[];
}

export interface GroceryList extends AuditableEntity {
  groceryListID: number;
  mealPlanID: number | null;
  generatedDate: string;
  groceryListItems?: GroceryListItem[];
}

export interface GroceryListItem extends AuditableEntity {
  groceryListItemID: number;
  groceryListID: number;
  itemID: number | null;
  itemName: string | null;
  manualItemName: string | null;
  quantityNeeded: number;
  unitOfMeasure: string | null;
  source: string;
  isChecked: boolean;
  groceryList?: GroceryList | null;
}
