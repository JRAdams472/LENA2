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
  // YYYY-MM-DD; gates sommelier/cocktail AI suggestions at 21+.
  birthdate: string | null;
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
  | "EVENT_DELETED"
  | "PROTEIN_DEFROST"
  | "MEAL_PREP_ADVANCE"
  | "ITEM_EXPIRING";

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
  // Server-rendered feed text for scheduled reminders; null on
  // event-driven rows (clients render those from kind).
  title: string | null;
  body: string | null;
  // Deep-link target for PROTEIN_DEFROST / MEAL_PREP_ADVANCE reminders.
  recipeId: number | null;
  // Deep-link + replacement-action target for ITEM_EXPIRING reminders.
  itemId: number | null;
  createdAt: string;
}

// One notification opt-out bucket as seen by the current user: "_all" is
// the global mute; every other value is a notification_type category.
export interface NotificationCategoryPreference {
  category: string;
  label: string;
  enabled: boolean;
  pushEnabled: boolean;
  mutedUntil: string | null;
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
  // Whether items in this category count as protein for defrost reminders.
  isProtein: boolean;
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
  item?: Item | null;
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
  // ingredient is the global catalog link; householdIngredient resolves
  // the household override (falling through to the catalog link).
  ingredient?: Ingredient | null;
  householdIngredient?: Ingredient | null;
  foodNutrients: FoodNutrient[] | null;
  foodFlavors: FoodFlavor[] | null;
  selectionCount: number;
  personalSelectionCount: number;
  allergens?: AllergenFlag[];
  allergyWarnings?: AllergyWarning[];
}

// A generic ingredient ("corn") distinct from a branded catalog item
// ("Green Giant corn"). Recipes and grocery needs key on ingredients.
export interface Ingredient {
  ingredientID: number;
  name: string;
  category: Category | null;
  defaultUnit: string | null;
  isActive: boolean;
  allergens?: AllergenFlag[];
  allergyWarnings?: AllergyWarning[];
}

// An allergen from the registry — the taxonomy member records and
// entity flags both key on.
export interface Allergen {
  allergenID: number;
  name: string;
  description: string | null;
  isActive: boolean;
}

// An entity's declared allergen flag. "contains" is confirmed;
// "may_contain" is advisory (cross-contamination, ingredient ambiguity).
export type AllergenFlagKind = "contains" | "may_contain";

export interface AllergenFlag {
  allergen: Allergen;
  kind: AllergenFlagKind;
}

// A member's own allergy or dietary-restriction record.
export type MemberAllergyKind = "allergy" | "dietary";

export interface MemberAllergen {
  allergen: Allergen;
  kind: MemberAllergyKind;
}

// A conflict between an entity's allergen set and one household member's
// records. memberKind is the member's record kind; entityKind is the
// strongest flag the entity carries for that allergen.
export interface AllergyWarning {
  member: HouseholdUser;
  allergen: Allergen;
  memberKind: MemberAllergyKind;
  entityKind: AllergenFlagKind;
}

export interface NutrientType {
  nutrientId: number;
  nutrientName: string;
  unitOfMeasure: string;
}

// One AI-proposed flag in the admin review queue. Accepting writes a real
// flag under the reviewer's attribution; dismissing keeps the audit row.
export type AllergenSuggestionStatus = "pending" | "accepted" | "dismissed";

export interface AllergenSuggestion {
  id: number;
  recipeId: number | null;
  recipeName: string | null;
  targetKind: "ingredient" | "item";
  ingredientId: number | null;
  ingredientName: string | null;
  itemId: number | null;
  itemName: string | null;
  allergen: Allergen;
  kind: AllergenFlagKind;
  rationale: string | null;
  status: AllergenSuggestionStatus;
  reviewedAt: string | null;
}

export interface Brand {
  brandID: number;
  brandName: string;
  selectionCount: number;
  personalSelectionCount: number;
}

export interface Unit {
  unitID: number;
  name: string;
  abbreviation: string | null;
  kind: string;
  isActive: boolean;
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
  // Present on the detail fetch — the household's tweak set, when one
  // exists. Effective items/steps already have it applied.
  householdDelta?: RecipeDelta | null;
  selectionCount: number;
  personalSelectionCount: number;
  myRating: number | null;
  averageRating: number | null;
  ratingCount: number;
  categories?: RecipeCategory[];
  allergens?: AllergenFlag[];
  allergyWarnings?: AllergyWarning[];
}

export interface RecipeRecommendation {
  recipe: Recipe;
  reason: string;
  score: number;
}

export interface RecipeCategory {
  categoryID: number;
  categoryName: string;
  group: RecipeCategoryGroup;
}

export interface RecipeCategoryGroup {
  categoryGroupID: number;
  groupName: string;
  exclusive: boolean;
  displayOrder: number;
  categories: RecipeCategory[];
}

export interface RecipeItem {
  recipeID: number;
  // Canonical recipe_item identity — household delta rows anchor on it.
  // Delta-added lines carry 0.
  recipeItemID?: number;
  // Null when the line is ingredient-only.
  itemID: number | null;
  ingredientID?: number | null;
  ingredientName?: string | null;
  quantity: number;
  unitOfMeasure: string | null;
  section?: string | null;
  displayOrder?: number;
  notes: string | null;
  isOptional: boolean;
  itemName?: string | null;
  itemBrand?: string | null;
  // Set on effective lines produced by a household delta —
  // substitute/adjust/add; removed lines never surface.
  deltaKind?: string | null;
  recipe?: Recipe | null;
  item?: Item | null;
  ingredient?: Ingredient | null;
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
  // Set on effective steps produced by a household delta — replace/add;
  // removed steps never surface.
  deltaKind?: string | null;
  recipe?: Recipe | null;
}

// Which recipe rows the API returns: the household delta-applied view or
// the untouched canonical recipe.
export type RecipeView = "effective" | "canonical";

// One ingredient-line change in a household recipe delta.
export interface RecipeDeltaItem {
  recipeDeltaItemID: number;
  // Anchored canonical line — null for added lines and orphans.
  recipeItemID: number | null;
  kind: "substitute" | "adjust" | "remove" | "add";
  itemID: number | null;
  itemName: string | null;
  itemBrand: string | null;
  ingredientID: number | null;
  ingredientName: string | null;
  quantity: number | null;
  unitOfMeasure: string | null;
  unitID: number | null;
  section: string | null;
  displayOrder: number | null;
  notes: string | null;
  isOptional: boolean | null;
  orphaned: boolean;
}

// One step change in a household recipe delta.
export interface RecipeDeltaStep {
  recipeDeltaStepID: number;
  // Anchored canonical step — null for added steps and orphans.
  stepID: number | null;
  kind: "replace" | "remove" | "add";
  stepNumber: number | null;
  instruction: string | null;
  durationMinutes: number | null;
  stepType: string | null;
  isPassive: boolean | null;
  dependsOnStepNumber: number | null;
  appliance: string | null;
  orphaned: boolean;
}

// A household's tweak set for one canonical recipe (LEN-25). stale fires
// when the canonical recipe changed after the delta was last written or
// acknowledged.
export interface RecipeDelta {
  recipeDeltaID: number;
  stale: boolean;
  orphanedItemCount: number;
  orphanedStepCount: number;
  items: RecipeDeltaItem[];
  steps: RecipeDeltaStep[];
  updatedAt: string | null;
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
  // The linked recipe's servings frozen at link time — the scaling
  // denominator. Null for free-form slots or recipes without servings.
  baseServings: number | null;
  // servings ÷ baseServings, or 1 when scaling does not apply.
  scalingFactor: number;
  notes: string | null;
  recipe?: Recipe | null;
  // The slot's own copies of the recipe's steps and items — edits here
  // never touch the original recipe.
  steps?: EventRecipeStep[];
  items?: EventRecipeItem[];
  allergens?: AllergenFlag[];
  allergyWarnings?: AllergyWarning[];
}

// One ingredient in a slot's snapshot. quantity is already scaled by the
// slot's scalingFactor; baseQuantity is the per-base-servings amount
// copied from the recipe.
export interface EventRecipeItem {
  eventRecipeItemID: number;
  // Null when the line is ingredient-only.
  itemID: number | null;
  ingredientID?: number | null;
  ingredientName?: string | null;
  itemName: string | null;
  quantity: number;
  baseQuantity: number;
  unit: string;
  section: string | null;
  displayOrder: number;
  notes: string | null;
  isOptional: boolean;
}

export interface EventRecipeStep {
  eventRecipeStepID: number;
  stepNumber: number;
  instruction: string;
  durationMinutes: number | null;
  stepType: string | null;
  isPassive: boolean;
  dependsOnStepNumber: number | null;
  appliance: string | null;
}

// The master schedule for an event: every recipe's steps backwards-
// scheduled from their serve times, computed on read.
export interface EventTimeline {
  foodEventID: number;
  warnings: string[];
  recipes: EventTimelineRecipe[];
}

export interface EventTimelineRecipe {
  eventRecipeID: number;
  name: string;
  targetTime: string;
  servings: number | null;
  baseServings: number | null;
  // Earliest moment work must begin; null when unschedulable.
  startBy: string | null;
  unschedulable: boolean;
  warnings: string[];
  steps: TimelineStep[];
}

export interface TimelineStep {
  stepNumber: number;
  instruction: string;
  stepType: string | null;
  isPassive: boolean;
  appliance: string | null;
  durationMinutes: number | null;
  // Effective slot time used (duration rounded up to slot granularity).
  scheduledMinutes: number;
  // Duration was missing and estimated as one slot.
  estimated: boolean;
  startTime: string;
  endTime: string;
  conflicts: string[];
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
  allergens?: AllergenFlag[];
  allergyWarnings?: AllergyWarning[];
}

export interface MealSlotItem extends AuditableEntity {
  mealSlotItemID: number;
  mealSlotID: number;
  // Null when the line is ingredient-only.
  itemID: number | null;
  ingredientID?: number | null;
  ingredientName?: string | null;
  quantity: number;
  unitOfMeasure: string | null;
  isFromRecipe: boolean;
  mealSlot?: MealSlot | null;
  item?: Item | null;
}

export interface MealPlanSuggestion {
  recipe: Recipe;
  dayOfWeek: number;
  mealType: number;
  reason: string;
  usesExpiringItems: string[];
}

// One AI wine pairing. bottleId/inCellar identify a household cellar
// bottle; when null, name is a general style suggestion.
export interface PairingSuggestion {
  bottleId: number | null;
  name: string;
  reason: string;
  inCellar: boolean;
}

// One AI cocktail pick — a catalog recipe tagged with the Cocktail dish
// type. missingIngredients lists pantry gaps the model identified.
export interface CocktailSuggestion {
  recipe: Recipe;
  reason: string;
  missingIngredients: string[];
}

// One AI schedule fix for an event timeline problem. `action` selects
// which payload fields are meaningful; applied via the existing event
// recipe / recipe-step mutations.
export interface EventFixSuggestion {
  eventRecipeId: number;
  recipeName: string;
  stepNumber: number | null;
  action: "shift_serve" | "set_appliance" | "set_duration" | "set_dependency";
  minutes: number | null;
  appliance: string | null;
  durationMinutes: number | null;
  dependsOnStepNumber: number | null;
  reason: string;
}

// One household-data lookup the assistant made while answering — the
// "checked your pantry" trace shown under each reply.
export interface AssistantToolCall {
  name: string;
}

// The assistant's reply plus the tools it used.
export interface AssistantAnswer {
  answer: string;
  toolCalls: AssistantToolCall[];
}

// One read-only assistant tool spec (assistantTools) — advertised to
// client-side inference agents for the JSON tool protocol.
export interface AssistantToolSpec {
  name: string;
  description: string;
  parametersJson: string;
}

// A one-shot AI request assembled server-side for client-side generation
// (prepareAssistantRequest).
export interface PreparedAIRequest {
  prompt: string;
  contextJson: string;
  outputSchemaJson: string;
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
  store?: Store | null;
}

export interface Store {
  storeID: number;
  name: string;
  aisles?: StoreAisle[];
}

export interface StoreAisle {
  aisleID: number;
  name: string;
  position: number;
}

// GroceryRouteGroup is one stop on the store walk — an aisle's items in
// server-computed order, or the trailing unassigned bucket (aisle null).
export interface GroceryRouteGroup {
  aisle: StoreAisle | null;
  items: GroceryRouteItem[];
}

export interface GroceryRouteItem {
  item: GroceryListItem;
  suggested: boolean;
}

export interface GroceryListItem extends AuditableEntity {
  groceryListItemID: number;
  groceryListID: number;
  itemID: number | null;
  itemName: string | null;
  ingredientID?: number | null;
  ingredientName?: string | null;
  // The household's usual brand for the line's ingredient — recorded by
  // brand-picked check-offs.
  usualBrandItemID?: number | null;
  usualBrandName?: string | null;
  manualItemName: string | null;
  quantityNeeded: number;
  unitOfMeasure: string | null;
  source: string;
  isChecked: boolean;
  groceryList?: GroceryList | null;
  allergens?: AllergenFlag[];
  allergyWarnings?: AllergyWarning[];
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
  itemKind: string | null;
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
