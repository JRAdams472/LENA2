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
  firstName: string | null;
  lastName: string | null;
  backupEmail: string | null;
  role: "member" | "admin";
  isActive: boolean;
  isProtected: boolean;
  lastLoginAt: string | null;
  externalSubject: string | null;
  provider: string | null;
  isSearchable: boolean;
  household: Household | null;
}

export type InviteStatus = "PENDING" | "ACCEPTED" | "DECLINED" | "CANCELLED";

// Restricted user projection for household flows — the API never returns
// email, role, or activity fields here.
export interface HouseholdUser {
  userID: number;
  displayName: string | null;
  firstName: string | null;
  lastName: string | null;
}

export type HouseholdRole = "OWNER" | "ADMIN" | "MEMBER";

export type NotificationKind =
  | "INVITE_RECEIVED"
  | "INVITE_ACCEPTED"
  | "INVITE_DECLINED"
  | "INVITE_CANCELLED"
  | "MEMBER_JOINED"
  | "MEMBER_LEFT"
  | "MEMBER_REMOVED"
  | "ROLE_CHANGED"
  | "HOUSEHOLD_RENAMED"
  | "EVENT_CREATED"
  | "EVENT_UPDATED"
  | "EVENT_DELETED";

// A member entry pairs the restricted user projection with household
// role metadata — roles never appear on HouseholdUser itself so invite
// and search results cannot leak them.
export interface HouseholdMember {
  user: HouseholdUser;
  role: HouseholdRole;
  isMe: boolean;
}

export interface Household {
  householdID: number;
  name: string | null;
  members: HouseholdMember[];
  myRole: HouseholdRole;
  createdAt: string;
}

export interface HouseholdNotification {
  notificationID: number;
  kind: NotificationKind;
  actor: HouseholdUser | null;
  // Deep-link target for EVENT_* kinds; null otherwise or once the event
  // row is gone.
  foodEventId: number | null;
  createdAt: string;
}

export interface HouseholdInvite {
  inviteID: number;
  fromUser: HouseholdUser;
  toUser: HouseholdUser;
  status: InviteStatus;
  createdAt: string;
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
  status: string;
  submittedByMe: boolean;
  category: Category | null;
  foodNutrients: FoodNutrient[] | null;
  foodFlavors: FoodFlavor[] | null;
  selectionCount: number;
  personalSelectionCount: number;
}

export interface NutrientType {
  nutrientId: number;
  nutrientName: string;
  unitOfMeasure: string;
}

export interface Brand {
  brandID: number;
  brandName: string;
  selectionCount: number;
  personalSelectionCount: number;
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
  selectionCount: number;
  personalSelectionCount: number;
  myRating: number | null;
  averageRating: number | null;
  ratingCount: number;
}

export interface RecipeRecommendation {
  recipe: Recipe;
  reason: string;
  score: number;
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
  // Timing metadata feeding the event master-timeline scheduler.
  durationMinutes?: number | null;
  stepType?: string | null;
  isPassive?: boolean;
  dependsOnStepNumber?: number | null;
  appliance?: string | null;
  recipe?: Recipe | null;
}

// A household food event groups recipes (or free-form slots) scheduled to
// be served at absolute times on one date.
export interface FoodEvent {
  foodEventID: number;
  name: string;
  eventDate: string;
  // Serve times must land on this boundary: 15 or 30 minutes.
  slotGranularityMinutes: number;
  isActive: boolean;
  eventRecipes?: EventRecipe[];
}

export interface EventRecipe {
  eventRecipeID: number;
  foodEventID: number;
  recipeID: number | null;
  mealType: string;
  targetTime: string;
  servings: number | null;
  notes: string | null;
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
  warnings?: string[];
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

export interface RecipeImportDraftItem {
  quantity: number | null;
  unit: string | null;
  ingredient: string;
  section: string | null;
  notes: string | null;
  isOptional: boolean;
}

export interface RecipeImportDraftStep {
  stepNumber: number;
  instruction: string;
}

export interface RecipeImportDraft {
  name: string | null;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  sourceHint: string | null;
  items: RecipeImportDraftItem[];
  steps: RecipeImportDraftStep[];
}

export interface RecipeImportSuggestion {
  id: string;
  name: string;
  kind: string;
  score: number;
}

export interface RecipeImportReviewItem extends RecipeImportDraftItem {
  itemId: string | null;
  itemName: string | null;
  unitId: string | null;
  confidence: number;
  suggestions: RecipeImportSuggestion[];
  status: string;
  approved: boolean;
}

export interface RecipeImportReviewStep {
  stepNumber: number;
  instruction: string;
}

export interface RecipeImportReview {
  pageId: string | null;
  name: string | null;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  sourceHint: string | null;
  items: RecipeImportReviewItem[];
  steps: RecipeImportReviewStep[];
  approved: boolean;
}

export interface RecipeImport extends AuditableEntity {
  recipeImportID: number;
  status: string;
  sourceFilename: string;
  ocrText: string | null;
  draft: RecipeImportDraft | null;
  review: RecipeImportReview | null;
  recipeID: number | null;
  recipe: Recipe | null;
  profanityFlag: boolean;
  profanityReason: string | null;
  errorMessage: string | null;
}
