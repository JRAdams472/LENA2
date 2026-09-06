import {
  AuditableEntity,
  User,
  Item,
  Brand,
  Category,
  Bottle,
  BottleGrapeVariety,
  BottleFlavorProfile,
  Country,
  Region,
  WineType,
  Vintage,
  GrapeVariety,
  WineFlavorProfile,
  FlavorProfile,
  FoodFlavor,
  FoodNutrient,
  NutrientType,
  Recipe,
  RecipeItem,
  RecipeStep,
  MealPlan,
  MealSlot,
  MealSlotItem,
  MealPlanNutrition,
  GroceryList,
  GroceryListItem,
  PagedResult,
} from "./types";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "/graphql";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

// Default getter reads the persisted token so early requests (fired before
// AuthProvider's effect registers the real getter) still authenticate.
let authTokenGetter: (() => string | null) | null = () =>
  typeof window === "undefined"
    ? null
    : window.localStorage.getItem("lena_id_token");
let onUnauthorized: (() => void) | null = null;

export function setAuthTokenGetter(getter: () => string | null) {
  authTokenGetter = getter;
}

export function setOnUnauthorized(handler: () => void) {
  onUnauthorized = handler;
}

function getAuthToken(): string | null {
  return authTokenGetter ? authTokenGetter() : null;
}

interface GraphQLError {
  message: string;
  extensions?: { code?: string };
}

interface GraphQLResponse<T> {
  data?: T;
  errors?: GraphQLError[];
}

/**
 * Executes a GraphQL operation against the BFF endpoint.
 * POSTs `{ query, variables }` to API_BASE_URL and returns `data`.
 * Throws ApiError on non-OK HTTP or when the payload contains `errors`.
 */
async function request<T>(
  query: string,
  variables?: Record<string, unknown>
): Promise<T> {
  const idToken = getAuthToken();

  const res = await fetch(API_BASE_URL, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(idToken ? { Authorization: `Bearer ${idToken}` } : {}),
    },
    body: JSON.stringify({ query, variables: variables ?? {} }),
  });

  if (!res.ok) {
    if (res.status === 401) {
      onUnauthorized?.();
    }
    const text = await res.text().catch(() => "");
    throw new ApiError(res.status, text || `HTTP ${res.status}`);
  }

  const payload = (await res.json()) as GraphQLResponse<T>;

  if (payload.errors && payload.errors.length > 0) {
    const first = payload.errors[0];
    const code = first.extensions?.code;
    if (code === "UNAUTHENTICATED" || code === "UNAUTHORIZED") {
      onUnauthorized?.();
    }
    throw new ApiError(
      res.status,
      payload.errors.map((e) => e.message).join("; ")
    );
  }

  if (payload.data === undefined || payload.data === null) {
    throw new ApiError(res.status, "GraphQL response contained no data");
  }

  return payload.data;
}

export function asEntity<T extends object>(row: unknown): T {
  if (typeof row !== "object" || row === null) {
    throw new TypeError("asEntity expected a non-null object");
  }
  return row as T;
}

/* ------------------------------------------------------------------ */
/* GraphQL wire shapes (mirror internal/bff/schema.graphqls)           */
/* ------------------------------------------------------------------ */

interface GqlPageInfo {
  pageNumber: number;
  pageSize: number;
  totalCount: number;
}

interface GqlBrand {
  id: string;
  name: string;
  selectionCount: number;
  personalSelectionCount: number;
}

interface GqlCategory {
  id: string;
  name: string;
  description: string | null;
  isActive: boolean;
}

interface GqlFlavorProfile {
  id: string;
  name: string;
  isActive: boolean;
}

interface GqlNutrientType {
  id: string;
  name: string;
  unit: string;
}

interface GqlFoodNutrient {
  nutrient: GqlNutrientType;
  amount: number;
}

interface GqlFoodFlavor {
  flavor: GqlFlavorProfile;
  intensity: number;
}

interface GqlItem {
  id: string;
  name: string;
  brand: GqlBrand | null;
  upc12: string | null;
  upc14: string | null;
  category: GqlCategory;
  unit: string;
  nutrients: GqlFoodNutrient[];
  flavors: GqlFoodFlavor[];
  selectionCount: number;
  personalSelectionCount: number;
}

interface GqlItemPage {
  items: GqlItem[];
  pageInfo: GqlPageInfo;
}

interface GqlUserItem {
  id: string;
  item: GqlItem;
  currentQty: number;
  minQty: number | null;
  purchaseAt: string | null;
  expiresAt: string | null;
  notes: string | null;
  isFavorite: boolean;
}

interface GqlUserItemPage {
  items: GqlUserItem[];
  pageInfo: GqlPageInfo;
}

interface GqlRecipeItem {
  item: GqlItem;
  quantity: number;
  unit: string;
  notes: string | null;
  isOptional: boolean;
}

interface GqlRecipeStep {
  stepNumber: number;
  instruction: string;
}

interface GqlRecipe {
  id: string;
  name: string;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  items: GqlRecipeItem[];
  steps: GqlRecipeStep[];
  isFavorite: boolean;
  selectionCount: number;
  personalSelectionCount: number;
  myRating: number | null;
  averageRating: number | null;
  ratingCount: number;
}

interface GqlRecipePage {
  items: GqlRecipe[];
  pageInfo: GqlPageInfo;
}

interface GqlGrapeVariety {
  id: string;
  name: string;
  description: string | null;
  isActive: boolean;
}

interface GqlWineFlavorProfile {
  id: string;
  name: string;
  description: string | null;
  isActive: boolean;
}

interface GqlBottleGrapeVariety {
  grapeVariety: GqlGrapeVariety;
  percentage: number | null;
}

interface GqlBottleFlavorProfile {
  flavorProfile: GqlWineFlavorProfile;
  intensity: number;
}

interface GqlBottle {
  id: string;
  typeId: string;
  countryId: string;
  regionId: string;
  vineyard: string | null;
  vintageYear: number;
  abv: number | null;
  acidity: number | null;
  tanninLevel: number | null;
  body: number | null;
  sweetness: number | null;
  oakIntegration: boolean | null;
  bottleSize: string;
  grapeVarieties: GqlBottleGrapeVariety[];
  flavorProfiles: GqlBottleFlavorProfile[];
}

interface GqlBottlePage {
  items: GqlBottle[];
  pageInfo: GqlPageInfo;
}

interface GqlUserBottle {
  id: string;
  bottle: GqlBottle;
  bottleNumber: number | null;
  quantity: number;
  purchaseAt: string | null;
  purchasePrice: number | null;
  storageTemp: number | null;
  location: string | null;
  notes: string | null;
  isFavorite: boolean;
}

interface GqlUserBottlePage {
  items: GqlUserBottle[];
  pageInfo: GqlPageInfo;
}

interface GqlWineType {
  id: string;
  name: string;
  description: string | null;
}

interface GqlCountry {
  id: string;
  name: string;
  isoCode: string | null;
  description: string | null;
}

interface GqlRegion {
  id: string;
  name: string;
  description: string | null;
  country: GqlCountry;
}

interface GqlVintage {
  id: string;
  year: number;
  description: string | null;
  isActive: boolean;
}

interface GqlMealSlotItem {
  id: string;
  item: GqlItem | null;
  quantity: number;
  unit: string;
  isFromRecipe: boolean;
}

interface GqlMealSlot {
  id: string;
  dayOfWeek: number;
  mealType: string;
  recipe: GqlRecipe | null;
  servings: number | null;
  replacementNote: string | null;
  items: GqlMealSlotItem[];
}

interface GqlMealPlan {
  id: string;
  name: string;
  weekStartDate: string;
  isActive: boolean;
  slots: GqlMealSlot[];
}

interface GqlMealPlanPage {
  items: GqlMealPlan[];
  pageInfo: GqlPageInfo;
}

interface GqlNutritionSummary {
  name: string;
  unit: string;
  amount: number;
}

interface GqlGroceryListItem {
  id: string;
  item: GqlItem | null;
  manualItemName: string | null;
  quantityNeeded: number;
  unitOfMeasure: string | null;
  source: string;
  isChecked: boolean;
}

interface GqlGroceryList {
  id: string;
  generatedAt: string;
  items: GqlGroceryListItem[];
}

interface GqlGroceryListPage {
  items: GqlGroceryList[];
  pageInfo: GqlPageInfo;
}

interface GqlUser {
  id: string;
  email: string;
  displayName: string | null;
}

/* ------------------------------------------------------------------ */
/* Mappers: GraphQL shape -> UI type (lib/types.ts)                    */
/* ------------------------------------------------------------------ */

const num = (id: string | number | null | undefined): number => {
  const n = Number(id);
  return Number.isFinite(n) ? n : 0;
};

const audit = (): AuditableEntity => ({
  createdBy: "",
  createDate: "",
  lastUpdatedBy: null,
  lastUpdatedDate: null,
});

function toPaged<T>(items: T[], pageInfo: GqlPageInfo): PagedResult<T> {
  return {
    items,
    pageNumber: pageInfo.pageNumber,
    pageSize: pageInfo.pageSize,
    totalCount: pageInfo.totalCount,
    totalPages:
      pageInfo.pageSize > 0
        ? Math.ceil(pageInfo.totalCount / pageInfo.pageSize)
        : 0,
  };
}

function toNutrientType(n: GqlNutrientType): NutrientType {
  return {
    nutrientId: num(n.id),
    nutrientName: n.name,
    unitOfMeasure: n.unit,
  };
}

function toFlavorProfile(f: GqlFlavorProfile): FlavorProfile {
  return {
    flavorId: num(f.id),
    flavorName: f.name,
    isActive: f.isActive,
    foodFlavors: null,
  };
}

function toFoodNutrient(foodId: number, n: GqlFoodNutrient): FoodNutrient {
  return {
    foodId,
    nutrientId: num(n.nutrient.id),
    amountPerServing: n.amount,
    nutrientType: toNutrientType(n.nutrient),
  };
}

function toFoodFlavor(foodId: number, f: GqlFoodFlavor): FoodFlavor {
  return {
    foodId,
    flavorId: num(f.flavor.id),
    intensityScore: f.intensity,
    item: null,
    flavorProfile: toFlavorProfile(f.flavor),
  };
}

function toBrand(b: GqlBrand): Brand {
  return {
    brandID: num(b.id),
    brandName: b.name,
    selectionCount: b.selectionCount ?? 0,
    personalSelectionCount: b.personalSelectionCount ?? 0,
  };
}

function toItem(i: GqlItem, ui?: GqlUserItem): Item {
  const itemID = num(i.id);
  return {
    ...audit(),
    itemID,
    name: i.name,
    brand: i.brand?.name ?? null,
    upc12: i.upc12,
    upc14: i.upc14,
    categoryID: num(i.category?.id),
    unit: i.unit,
    currentQuantity: ui?.currentQty ?? 0,
    minQuantity: ui?.minQty ?? null,
    purchaseDate: ui?.purchaseAt ?? null,
    expiryDate: ui?.expiresAt ?? null,
    notes: ui?.notes ?? null,
    isFavorite: ui?.isFavorite ?? false,
    category: i.category
      ? {
          ...audit(),
          categoryID: num(i.category.id),
          categoryName: i.category.name,
          description: i.category.description,
          isActive: true,
        }
      : null,
    foodNutrients: (i.nutrients ?? []).map((n) => toFoodNutrient(itemID, n)),
    foodFlavors: (i.flavors ?? []).map((f) => toFoodFlavor(itemID, f)),
    selectionCount: i.selectionCount ?? 0,
    personalSelectionCount: i.personalSelectionCount ?? 0,
  };
}

function toRecipeItem(recipeID: number, r: GqlRecipeItem): RecipeItem {
  return {
    recipeID,
    itemID: num(r.item?.id),
    quantity: r.quantity,
    unitOfMeasure: r.unit,
    notes: r.notes,
    isOptional: r.isOptional,
    itemName: r.item?.name ?? null,
    itemBrand: r.item?.brand?.name ?? null,
    recipe: null,
    item: r.item ? toItem(r.item) : null,
  };
}

function toRecipeStep(recipeID: number, s: GqlRecipeStep): RecipeStep {
  return {
    ...audit(),
    recipeStepID: s.stepNumber,
    recipeID,
    stepNumber: s.stepNumber,
    instruction: s.instruction,
    recipe: null,
  };
}

function toRecipe(r: GqlRecipe): Recipe {
  const recipeID = num(r.id);
  return {
    ...audit(),
    recipeID,
    recipeName: r.name,
    description: r.description,
    servings: r.servings,
    prepTimeMinutes: r.prepTimeMinutes,
    cookTimeMinutes: r.cookTimeMinutes,
    isActive: true,
    isFavorite: r.isFavorite,
    recipeItems: (r.items ?? []).map((i) => toRecipeItem(recipeID, i)),
    recipeSteps: (r.steps ?? []).map((s) => toRecipeStep(recipeID, s)),
    selectionCount: r.selectionCount ?? 0,
    personalSelectionCount: r.personalSelectionCount ?? 0,
    myRating: r.myRating ?? null,
    averageRating: r.averageRating ?? null,
    ratingCount: r.ratingCount ?? 0,
  };
}

function toCategory(c: GqlCategory): Category {
  return {
    ...audit(),
    categoryID: num(c.id),
    categoryName: c.name,
    description: c.description,
    isActive: c.isActive,
  };
}

function toGrapeVariety(g: GqlGrapeVariety): GrapeVariety {
  return {
    ...audit(),
    grapeVarietyID: num(g.id),
    grapeVarietyName: g.name,
    description: g.description,
    isActive: g.isActive,
    bottleGrapeVarieties: [],
  };
}

function toWineFlavorProfile(f: GqlWineFlavorProfile): WineFlavorProfile {
  return {
    ...audit(),
    flavorProfileID: num(f.id),
    flavorProfileName: f.name,
    description: f.description,
    isActive: f.isActive,
  };
}

function toBottle(b: GqlBottle, ub?: GqlUserBottle): Bottle {
  const bottleID = num(b.id);
  return {
    ...audit(),
    bottleID,
    bottleNumber: ub?.bottleNumber ?? null,
    typeID: num(b.typeId),
    countryID: num(b.countryId),
    regionID: num(b.regionId),
    vintageYear: b.vintageYear,
    vineyard: b.vineyard,
    abv: b.abv,
    acidity: b.acidity,
    tanninLevel: b.tanninLevel,
    body: b.body,
    sweetness: b.sweetness,
    oakIntegration: b.oakIntegration,
    bottleSize: b.bottleSize,
    quantity: ub?.quantity ?? 0,
    purchaseDate: ub?.purchaseAt ?? null,
    purchasePrice: ub?.purchasePrice ?? null,
    storageTemp: ub?.storageTemp ?? null,
    location: ub?.location ?? null,
    notes: ub?.notes ?? null,
    isFavorite: ub?.isFavorite ?? false,
    type: null,
    country: null,
    region: null,
    vintage: null,
    bottleGrapeVarieties: (b.grapeVarieties ?? []).map(
      (g): BottleGrapeVariety => ({
        ...audit(),
        bottleID,
        grapeVarietyID: num(g.grapeVariety.id),
        percentage: g.percentage,
        bottle: null as unknown as Bottle,
        grapeVariety: toGrapeVariety(g.grapeVariety),
      })
    ),
    bottleFlavorProfiles: (b.flavorProfiles ?? []).map(
      (f): BottleFlavorProfile => ({
        ...audit(),
        flavorProfileID: num(f.flavorProfile.id),
        flavorProfileName: f.flavorProfile.name,
        description: f.flavorProfile.description,
        isActive: f.flavorProfile.isActive,
      })
    ),
  };
}

function toCountry(c: GqlCountry): Country {
  return {
    ...audit(),
    countryID: num(c.id),
    countryName: c.name,
    isoCode: c.isoCode ?? "",
    description: c.description,
    isActive: true,
    regions: [],
    bottles: [],
  };
}

function toRegion(r: GqlRegion): Region {
  return {
    ...audit(),
    regionID: num(r.id),
    regionName: r.name,
    countryID: num(r.country?.id),
    description: r.description,
    isActive: true,
    country: r.country ? toCountry(r.country) : null,
    bottles: [],
  };
}

function toWineType(t: GqlWineType): WineType {
  return {
    ...audit(),
    typeID: num(t.id),
    typeName: t.name,
    description: t.description,
    isActive: true,
    bottles: [],
  };
}

function toVintage(v: GqlVintage): Vintage {
  return {
    ...audit(),
    vintageID: num(v.id),
    year: v.year,
    description: v.description,
    isActive: v.isActive,
    bottles: [],
  };
}

function toMealSlotItem(slotID: number, i: GqlMealSlotItem): MealSlotItem {
  return {
    ...audit(),
    mealSlotItemID: num(i.id),
    mealSlotID: slotID,
    itemID: num(i.item?.id),
    quantity: i.quantity,
    unitOfMeasure: i.unit,
    isFromRecipe: i.isFromRecipe,
    mealSlot: null,
    item: i.item ? toItem(i.item) : null,
  };
}

const MEAL_TYPE_MAP: Record<string, number> = {
  breakfast: 0,
  lunch: 1,
  dinner: 2,
  snack: 3,
};

function mealTypeToNumber(mealType: string): number {
  const mapped = MEAL_TYPE_MAP[mealType.toLowerCase()];
  if (mapped !== undefined) return mapped;
  const parsed = Number(mealType);
  return Number.isFinite(parsed) ? parsed : 0;
}

function mealTypeToString(mealType: number | string): string {
  if (typeof mealType === "string") return mealType;
  const names = ["Breakfast", "Lunch", "Dinner", "Snack"];
  return names[mealType] ?? String(mealType);
}

function toMealSlot(planID: number, s: GqlMealSlot): MealSlot {
  const slotID = num(s.id);
  return {
    ...audit(),
    mealSlotID: slotID,
    mealPlanID: planID,
    dayOfWeek: s.dayOfWeek,
    mealType: mealTypeToNumber(s.mealType),
    recipeID: s.recipe ? num(s.recipe.id) : null,
    servings: s.servings ?? 0,
    replacementNote: s.replacementNote,
    mealPlan: null,
    recipe: s.recipe ? toRecipe(s.recipe) : null,
    mealSlotItems: (s.items ?? []).map((i) => toMealSlotItem(slotID, i)),
  };
}

function toMealPlan(p: GqlMealPlan): MealPlan {
  const planID = num(p.id);
  return {
    ...audit(),
    mealPlanID: planID,
    planName: p.name,
    weekStartDate: p.weekStartDate,
    weekStartDayOfWeek: 0,
    isActive: p.isActive,
    mealSlots: (p.slots ?? []).map((s) => toMealSlot(planID, s)),
  };
}

function toGroceryListItem(listID: number, i: GqlGroceryListItem): GroceryListItem {
  return {
    ...audit(),
    groceryListItemID: num(i.id),
    groceryListID: listID,
    itemID: i.item ? num(i.item.id) : null,
    itemName: i.item?.name ?? null,
    manualItemName: i.manualItemName,
    quantityNeeded: i.quantityNeeded,
    unitOfMeasure: i.unitOfMeasure,
    source: i.source,
    isChecked: i.isChecked,
    groceryList: null,
  };
}

function toGroceryList(g: GqlGroceryList): GroceryList {
  const listID = num(g.id);
  return {
    ...audit(),
    groceryListID: listID,
    mealPlanID: null,
    generatedDate: g.generatedAt,
    groceryListItems: (g.items ?? []).map((i) => toGroceryListItem(listID, i)),
  };
}

/* ------------------------------------------------------------------ */
/* Shared selection sets                                               */
/* ------------------------------------------------------------------ */

const BRAND_FIELDS = `
  id name selectionCount personalSelectionCount
`;

const ITEM_FIELDS = `
  id name upc12 upc14 unit selectionCount personalSelectionCount
  brand { ${BRAND_FIELDS} }
  category { id name description }
  nutrients { amount nutrient { id name unit } }
  flavors { intensity flavor { id name isActive } }
`;

const RECIPE_FIELDS = `
  id name description servings prepTimeMinutes cookTimeMinutes isFavorite selectionCount personalSelectionCount myRating averageRating ratingCount
  items { quantity unit notes isOptional item { ${ITEM_FIELDS} } }
  steps { stepNumber instruction }
`;

const BOTTLE_FIELDS = `
  id typeId countryId regionId vineyard vintageYear abv acidity tanninLevel
  body sweetness oakIntegration bottleSize
  grapeVarieties { percentage grapeVariety { id name description isActive } }
  flavorProfiles { intensity flavorProfile { id name description isActive } }
`;

const MEAL_PLAN_FIELDS = `
  id name weekStartDate isActive
  slots {
    id dayOfWeek mealType servings replacementNote
    recipe { ${RECIPE_FIELDS} }
    items { id quantity unit isFromRecipe item { ${ITEM_FIELDS} } }
  }
`;

const GROCERY_LIST_FIELDS = `
  id generatedAt
  items {
    id manualItemName quantityNeeded unitOfMeasure source isChecked
    item { ${ITEM_FIELDS} }
  }
`;

/* ------------------------------------------------------------------ */
/* Helpers to page through the API for client-side filtering           */
/* ------------------------------------------------------------------ */

function sortByFrequency(a: { personalSelectionCount: number; selectionCount: number }, b: { personalSelectionCount: number; selectionCount: number }): number {
  const pa = a.personalSelectionCount;
  const pb = b.personalSelectionCount;
  if (pa !== pb) return pb - pa;
  return b.selectionCount - a.selectionCount;
}

async function fetchAllItems(): Promise<GqlItem[]> {
  const pageSize = 200;
  let page = 1;
  const out: GqlItem[] = [];
  for (;;) {
    const data = await request<{ items: GqlItemPage }>(
      `query ($page: Int, $pageSize: Int) { items(page: $page, pageSize: $pageSize) { items { ${ITEM_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    out.push(...data.items.items);
    if (out.length >= data.items.pageInfo.totalCount || data.items.items.length === 0) break;
    page += 1;
  }
  return out.sort(sortByFrequency);
}

async function fetchAllUserItems(): Promise<GqlUserItem[]> {
  const pageSize = 200;
  let page = 1;
  const out: GqlUserItem[] = [];
  for (;;) {
    const data = await request<{ userItems: GqlUserItemPage }>(
      `query ($page: Int, $pageSize: Int) {
        userItems(page: $page, pageSize: $pageSize) {
          items { id currentQty minQty purchaseAt expiresAt notes isFavorite item { id } }
          pageInfo { pageNumber pageSize totalCount }
        }
      }`,
      { page, pageSize }
    );
    out.push(...data.userItems.items);
    if (out.length >= data.userItems.pageInfo.totalCount || data.userItems.items.length === 0) break;
    page += 1;
  }
  return out;
}

async function fetchItemsWithPrefs(): Promise<Item[]> {
  const [items, userItems] = await Promise.all([
    fetchAllItems(),
    fetchAllUserItems(),
  ]);
  const prefs = new Map(userItems.map((ui) => [num(ui.item.id), ui]));
  return items.map((i) => toItem(i, prefs.get(num(i.id))));
}

async function fetchAllBottles(): Promise<GqlBottle[]> {
  const pageSize = 200;
  let page = 1;
  const out: GqlBottle[] = [];
  for (;;) {
    const data = await request<{ bottles: GqlBottlePage }>(
      `query ($page: Int, $pageSize: Int) { bottles(page: $page, pageSize: $pageSize) { items { ${BOTTLE_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    out.push(...data.bottles.items);
    if (out.length >= data.bottles.pageInfo.totalCount || data.bottles.items.length === 0) break;
    page += 1;
  }
  return out;
}

function pagedSlice<T>(all: T[], pageNumber: number, pageSize: number): PagedResult<T> {
  const start = (pageNumber - 1) * pageSize;
  return {
    items: all.slice(start, start + pageSize),
    pageNumber,
    pageSize,
    totalCount: all.length,
    totalPages: pageSize > 0 ? Math.ceil(all.length / pageSize) : 0,
  };
}

/* ------------------------------------------------------------------ */
/* Public API                                                          */
/* ------------------------------------------------------------------ */

export const api = {
  // Auth
  getMe: async (): Promise<User> => {
    const data = await request<{ me: GqlUser }>(
      `query { me { id email displayName } }`
    );
    return {
      userID: num(data.me.id),
      email: data.me.email,
      displayName: data.me.displayName,
      externalSubject: null,
      provider: null,
    };
  },

  // Items
  getItems: async (): Promise<Item[]> => fetchItemsWithPrefs(),

  getItemsPaged: async (
    pageNumber: number,
    pageSize: number,
    search?: string,
    brand?: string,
    inStock?: boolean,
    isFavorite?: boolean
  ): Promise<PagedResult<Item>> => {
    let all = await fetchItemsWithPrefs();
    const s = (search ?? "").trim().toLowerCase();
    if (s) all = all.filter((i) => i.name.toLowerCase().includes(s));
    const b = (brand ?? "").trim().toLowerCase();
    if (b) all = all.filter((i) => (i.brand ?? "").toLowerCase() === b);
    if (inStock) all = all.filter((i) => i.currentQuantity > 0);
    if (isFavorite) all = all.filter((i) => i.isFavorite);
    all.sort(sortByFrequency);
    return pagedSlice(all, pageNumber, pageSize);
  },

  searchItems: async (search: string, brand?: string, limit: number = 50): Promise<Item[]> => {
    const s = search.trim().toLowerCase();
    const b = (brand ?? "").trim().toLowerCase();
    return (await fetchItemsWithPrefs())
      .filter((i) => i.name.toLowerCase().includes(s))
      .filter((i) => !b || (i.brand ?? "").toLowerCase() === b)
      .sort(sortByFrequency)
      .slice(0, limit);
  },

  getBrands: async (search?: string): Promise<Brand[]> => {
    const data = await request<{ brands: GqlBrand[] }>(
      `query { brands { ${BRAND_FIELDS} } }`
    );
    const s = (search ?? "").trim().toLowerCase();
    return data.brands
      .filter((b) => !s || b.name.toLowerCase().includes(s))
      .sort(sortByFrequency)
      .map(toBrand);
  },

  getFrequentBrands: async (limit = 10): Promise<Brand[]> => {
    const data = await request<{ frequentBrands: GqlBrand[] }>(
      `query ($limit: Int) { frequentBrands(limit: $limit) { ${BRAND_FIELDS} } }`,
      { limit }
    );
    return (data.frequentBrands ?? []).map(toBrand);
  },

  getFrequentItems: async (limit = 10): Promise<Item[]> => {
    const data = await request<{ frequentItems: GqlItem[] }>(
      `query ($limit: Int) { frequentItems(limit: $limit) { ${ITEM_FIELDS} } }`,
      { limit }
    );
    return (data.frequentItems ?? []).map((i) => toItem(i));
  },

  recordSelection: async (entityType: string, entityId: number): Promise<void> => {
    await request<{ recordSelection: boolean }>(
      `mutation ($entityType: String!, $entityId: ID!) { recordSelection(entityType: $entityType, entityId: $entityId) }`,
      { entityType, entityId: String(entityId) }
    );
  },

  recordSearch: async (entityType: string, term: string): Promise<void> => {
    await request<{ recordSearch: boolean }>(
      `mutation ($entityType: String!, $term: String!) { recordSearch(entityType: $entityType, term: $term) }`,
      { entityType, term }
    );
  },

  getItem: async (id: number): Promise<Item> => {
    const data = await request<{ item: GqlItem | null }>(
      `query ($id: ID!) { item(id: $id) { ${ITEM_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.item) throw new ApiError(404, `Item ${id} not found`);
    return toItem(data.item);
  },

  createItem: async (item: Omit<Item, keyof AuditableEntity>): Promise<Item> => {
    const data = await request<{ createItem: GqlItem }>(
      `mutation ($input: CreateItemInput!) { createItem(input: $input) { ${ITEM_FIELDS} } }`,
      {
        input: {
          name: item.name,
          brandId: null,
          upc12: item.upc12,
          upc14: item.upc14,
          categoryId: String(item.categoryID),
          unit: item.unit,
        },
      }
    );
    return toItem(data.createItem);
  },

  updateItem: async (id: number, item: Partial<Item>): Promise<Item> => {
    const input: Record<string, unknown> = {};
    if (item.name !== undefined) input.name = item.name;
    if (item.upc12 !== undefined) input.upc12 = item.upc12;
    if (item.upc14 !== undefined) input.upc14 = item.upc14;
    if (item.categoryID !== undefined) input.categoryId = String(item.categoryID);
    if (item.unit !== undefined) input.unit = item.unit;
    const data = await request<{ updateItem: GqlItem }>(
      `mutation ($id: ID!, $input: UpdateItemInput!) { updateItem(id: $id, input: $input) { ${ITEM_FIELDS} } }`,
      { id: String(id), input }
    );
    return toItem(data.updateItem);
  },

  deleteItem: async (id: number): Promise<Item | null> => {
    await request<{ deleteItem: boolean }>(
      `mutation ($id: ID!) { deleteItem(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  changeItemCategory: (id: number, categoryId: number): Promise<void> =>
    api.updateItem(id, { categoryID: categoryId }).then(() => undefined),

  setItemUPC12: (id: number, upc12: string): Promise<void> =>
    api.updateItem(id, { upc12 }).then(() => undefined),

  setItemUPC14: (id: number, upc14: string): Promise<void> =>
    api.updateItem(id, { upc14 }).then(() => undefined),

  adjustItemQuantity: async (id: number, quantity: number, purchaseDate?: string): Promise<void> => {
    await request<{ adjustUserItem: unknown }>(
      `mutation ($itemId: ID!, $quantity: Float!, $purchaseAt: Time) {
        adjustUserItem(itemId: $itemId, quantity: $quantity, purchaseAt: $purchaseAt) { id }
      }`,
      { itemId: String(id), quantity, purchaseAt: purchaseDate ?? null }
    );
  },

  setItemFavorite: async (id: number, isFavorite: boolean): Promise<void> => {
    await request<{ setItemFavorite: unknown }>(
      `mutation ($itemId: ID!, $isFavorite: Boolean!) {
        setItemFavorite(itemId: $itemId, isFavorite: $isFavorite) { id }
      }`,
      { itemId: String(id), isFavorite }
    );
  },

  // Inventory reference data
  getFlavorProfiles: async (): Promise<FlavorProfile[]> => {
    const data = await request<{ flavorProfiles: GqlFlavorProfile[] }>(
      `query { flavorProfiles { id name isActive } }`
    );
    return data.flavorProfiles.map(toFlavorProfile);
  },

  getActiveFlavorProfiles: async (): Promise<FlavorProfile[]> =>
    (await api.getFlavorProfiles()).filter((f) => f.isActive),

  createFlavorProfile: async (profile: Omit<FlavorProfile, keyof AuditableEntity>): Promise<FlavorProfile> => {
    const data = await request<{ createFlavorProfile: GqlFlavorProfile }>(
      `mutation ($input: CreateFlavorProfileInput!) { createFlavorProfile(input: $input) { id name isActive } }`,
      { input: { name: profile.flavorName } }
    );
    return toFlavorProfile(data.createFlavorProfile);
  },

  updateFlavorProfile: async (id: number, profile: Partial<FlavorProfile>): Promise<FlavorProfile> => {
    const input: Record<string, unknown> = {};
    if (profile.flavorName !== undefined) input.name = profile.flavorName;
    if (profile.isActive !== undefined) input.isActive = profile.isActive;
    const data = await request<{ updateFlavorProfile: GqlFlavorProfile }>(
      `mutation ($id: ID!, $input: UpdateFlavorProfileInput!) { updateFlavorProfile(id: $id, input: $input) { id name isActive } }`,
      { id: String(id), input }
    );
    return toFlavorProfile(data.updateFlavorProfile);
  },

  deleteFlavorProfile: async (id: number): Promise<FlavorProfile | null> => {
    await request<{ deleteFlavorProfile: boolean }>(
      `mutation ($id: ID!) { deleteFlavorProfile(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getBrandList: async (): Promise<Brand[]> => {
    const data = await request<{ brands: GqlBrand[] }>(
      `query { brands { ${BRAND_FIELDS} } }`
    );
    return data.brands.map(toBrand);
  },

  createBrand: async (brand: Omit<Brand, keyof AuditableEntity>): Promise<Brand> => {
    const data = await request<{ createBrand: GqlBrand }>(
      `mutation ($input: CreateBrandInput!) { createBrand(input: $input) { ${BRAND_FIELDS} } }`,
      { input: { name: brand.brandName } }
    );
    return toBrand(data.createBrand);
  },

  updateBrand: async (id: number, brand: Partial<Brand>): Promise<Brand> => {
    const input: Record<string, unknown> = {};
    if (brand.brandName !== undefined) input.name = brand.brandName;
    const data = await request<{ updateBrand: GqlBrand }>(
      `mutation ($id: ID!, $input: UpdateBrandInput!) { updateBrand(id: $id, input: $input) { ${BRAND_FIELDS} } }`,
      { id: String(id), input }
    );
    return toBrand(data.updateBrand);
  },

  deleteBrand: async (id: number): Promise<Brand | null> => {
    await request<{ deleteBrand: boolean }>(
      `mutation ($id: ID!) { deleteBrand(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getCategories: async (): Promise<Category[]> => {
    const data = await request<{ categories: GqlCategory[] }>(
      `query { categories { id name description isActive } }`
    );
    return data.categories.map(toCategory);
  },

  getActiveCategories: async (): Promise<Category[]> =>
    (await api.getCategories()).filter((c) => c.isActive),

  createCategory: async (category: Omit<Category, keyof AuditableEntity>): Promise<Category> => {
    const data = await request<{ createCategory: GqlCategory }>(
      `mutation ($input: CreateCategoryInput!) { createCategory(input: $input) { id name description isActive } }`,
      { input: { name: category.categoryName, description: category.description } }
    );
    return toCategory(data.createCategory);
  },

  updateCategory: async (id: number, category: Partial<Category>): Promise<Category> => {
    const input: Record<string, unknown> = {};
    if (category.categoryName !== undefined) input.name = category.categoryName;
    if (category.description !== undefined) input.description = category.description;
    if (category.isActive !== undefined) input.isActive = category.isActive;
    const data = await request<{ updateCategory: GqlCategory }>(
      `mutation ($id: ID!, $input: UpdateCategoryInput!) { updateCategory(id: $id, input: $input) { id name description isActive } }`,
      { id: String(id), input }
    );
    return toCategory(data.updateCategory);
  },

  deleteCategory: async (id: number): Promise<Category | null> => {
    await request<{ deleteCategory: boolean }>(
      `mutation ($id: ID!) { deleteCategory(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getFoodFlavors: async (): Promise<FoodFlavor[]> => {
    const items = await fetchAllItems();
    return items.flatMap((i) =>
      (i.flavors ?? []).map((f) => ({
        ...toFoodFlavor(num(i.id), f),
        item: toItem(i),
      }))
    );
  },

  createFoodFlavor: async (foodFlavor: Omit<FoodFlavor, keyof AuditableEntity>): Promise<FoodFlavor> => {
    const data = await request<{ addFoodFlavor: GqlFoodFlavor }>(
      `mutation ($input: AddFoodFlavorInput!) {
        addFoodFlavor(input: $input) { intensity flavor { id name isActive } }
      }`,
      {
        input: {
          itemId: String(foodFlavor.foodId),
          flavorId: String(foodFlavor.flavorId),
          intensity: foodFlavor.intensityScore,
        },
      }
    );
    return toFoodFlavor(foodFlavor.foodId, data.addFoodFlavor);
  },

  updateFoodFlavor: async (foodId: number, flavorId: number, foodFlavor: Partial<FoodFlavor>): Promise<FoodFlavor> => {
    await request<{ removeFoodFlavor: boolean }>(
      `mutation ($itemId: ID!, $flavorId: ID!) { removeFoodFlavor(itemId: $itemId, flavorId: $flavorId) }`,
      { itemId: String(foodId), flavorId: String(flavorId) }
    );
    const data = await request<{ addFoodFlavor: GqlFoodFlavor }>(
      `mutation ($input: AddFoodFlavorInput!) {
        addFoodFlavor(input: $input) { intensity flavor { id name isActive } }
      }`,
      {
        input: {
          itemId: String(foodId),
          flavorId: String(flavorId),
          intensity: foodFlavor.intensityScore ?? 0,
        },
      }
    );
    return toFoodFlavor(foodId, data.addFoodFlavor);
  },

  deleteFoodFlavor: async (foodId: number, flavorId: number): Promise<FoodFlavor | null> => {
    await request<{ removeFoodFlavor: boolean }>(
      `mutation ($itemId: ID!, $flavorId: ID!) { removeFoodFlavor(itemId: $itemId, flavorId: $flavorId) }`,
      { itemId: String(foodId), flavorId: String(flavorId) }
    );
    return null;
  },

  getFoodNutrients: async (): Promise<FoodNutrient[]> =>
    (await fetchAllItems()).flatMap((i) =>
      (i.nutrients ?? []).map((n) => toFoodNutrient(num(i.id), n))
    ),

  createFoodNutrient: async (foodNutrient: Omit<FoodNutrient, keyof AuditableEntity>): Promise<FoodNutrient> => {
    const data = await request<{ addFoodNutrient: GqlFoodNutrient }>(
      `mutation ($input: AddFoodNutrientInput!) {
        addFoodNutrient(input: $input) { amount nutrient { id name unit } }
      }`,
      {
        input: {
          itemId: String(foodNutrient.foodId),
          nutrientId: String(foodNutrient.nutrientId),
          amount: foodNutrient.amountPerServing,
        },
      }
    );
    return toFoodNutrient(foodNutrient.foodId, data.addFoodNutrient);
  },

  updateFoodNutrient: async (foodId: number, nutrientId: number, foodNutrient: Partial<FoodNutrient>): Promise<FoodNutrient> => {
    await request<{ removeFoodNutrient: boolean }>(
      `mutation ($itemId: ID!, $nutrientId: ID!) { removeFoodNutrient(itemId: $itemId, nutrientId: $nutrientId) }`,
      { itemId: String(foodId), nutrientId: String(nutrientId) }
    );
    const data = await request<{ addFoodNutrient: GqlFoodNutrient }>(
      `mutation ($input: AddFoodNutrientInput!) {
        addFoodNutrient(input: $input) { amount nutrient { id name unit } }
      }`,
      {
        input: {
          itemId: String(foodId),
          nutrientId: String(nutrientId),
          amount: foodNutrient.amountPerServing ?? 0,
        },
      }
    );
    return toFoodNutrient(foodId, data.addFoodNutrient);
  },

  deleteFoodNutrient: async (foodId: number, nutrientId: number): Promise<FoodNutrient | null> => {
    await request<{ removeFoodNutrient: boolean }>(
      `mutation ($itemId: ID!, $nutrientId: ID!) { removeFoodNutrient(itemId: $itemId, nutrientId: $nutrientId) }`,
      { itemId: String(foodId), nutrientId: String(nutrientId) }
    );
    return null;
  },

  getNutrientTypes: async (): Promise<NutrientType[]> => {
    const data = await request<{ nutrientTypes: GqlNutrientType[] }>(
      `query { nutrientTypes { id name unit } }`
    );
    return data.nutrientTypes.map(toNutrientType);
  },

  createNutrientType: async (nutrientType: Omit<NutrientType, keyof AuditableEntity>): Promise<NutrientType> => {
    const data = await request<{ createNutrientType: GqlNutrientType }>(
      `mutation ($input: CreateNutrientTypeInput!) { createNutrientType(input: $input) { id name unit } }`,
      {
        input: {
          name: nutrientType.nutrientName,
          unit: nutrientType.unitOfMeasure,
        },
      }
    );
    return toNutrientType(data.createNutrientType);
  },

  updateNutrientType: async (id: number, nutrientType: Partial<NutrientType>): Promise<NutrientType> => {
    const input: Record<string, unknown> = {};
    if (nutrientType.nutrientName !== undefined) input.name = nutrientType.nutrientName;
    if (nutrientType.unitOfMeasure !== undefined) input.unit = nutrientType.unitOfMeasure;
    const data = await request<{ updateNutrientType: GqlNutrientType }>(
      `mutation ($id: ID!, $input: UpdateNutrientTypeInput!) { updateNutrientType(id: $id, input: $input) { id name unit } }`,
      { id: String(id), input }
    );
    return toNutrientType(data.updateNutrientType);
  },

  deleteNutrientType: async (id: number): Promise<NutrientType | null> => {
    await request<{ deleteNutrientType: boolean }>(
      `mutation ($id: ID!) { deleteNutrientType(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  // Wine
  getBottlesPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<Bottle>> => {
    const data = await request<{ bottles: GqlBottlePage }>(
      `query ($page: Int, $pageSize: Int) { bottles(page: $page, pageSize: $pageSize) { items { ${BOTTLE_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page: pageNumber, pageSize }
    );
    return toPaged(data.bottles.items.map((b) => toBottle(b)), data.bottles.pageInfo);
  },

  getBottle: async (id: number): Promise<Bottle> => {
    const data = await request<{ bottle: GqlBottle | null }>(
      `query ($id: ID!) { bottle(id: $id) { ${BOTTLE_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.bottle) throw new ApiError(404, `Bottle ${id} not found`);
    return toBottle(data.bottle);
  },

  createBottle: async (bottle: Omit<Bottle, keyof AuditableEntity>): Promise<Bottle> => {
    const data = await request<{ createBottle: GqlBottle }>(
      `mutation ($input: CreateBottleInput!) { createBottle(input: $input) { ${BOTTLE_FIELDS} } }`,
      {
        input: {
          typeId: String(bottle.typeID),
          countryId: String(bottle.countryID),
          regionId: String(bottle.regionID),
          vintageYear: bottle.vintageYear,
          vineyard: bottle.vineyard,
          abv: bottle.abv,
          acidity: bottle.acidity,
          tanninLevel: bottle.tanninLevel,
          body: bottle.body,
          sweetness: bottle.sweetness,
          oakIntegration: bottle.oakIntegration,
          bottleSize: bottle.bottleSize,
        },
      }
    );
    return toBottle(data.createBottle);
  },

  updateBottle: async (id: number, bottle: Partial<Bottle>): Promise<Bottle> => {
    const input: Record<string, unknown> = {};
    if (bottle.typeID !== undefined) input.typeId = String(bottle.typeID);
    if (bottle.countryID !== undefined) input.countryId = String(bottle.countryID);
    if (bottle.regionID !== undefined) input.regionId = String(bottle.regionID);
    if (bottle.vintageYear !== undefined) input.vintageYear = bottle.vintageYear;
    if (bottle.vineyard !== undefined) input.vineyard = bottle.vineyard;
    if (bottle.abv !== undefined) input.abv = bottle.abv;
    if (bottle.acidity !== undefined) input.acidity = bottle.acidity;
    if (bottle.tanninLevel !== undefined) input.tanninLevel = bottle.tanninLevel;
    if (bottle.body !== undefined) input.body = bottle.body;
    if (bottle.sweetness !== undefined) input.sweetness = bottle.sweetness;
    if (bottle.oakIntegration !== undefined) input.oakIntegration = bottle.oakIntegration;
    if (bottle.bottleSize !== undefined) input.bottleSize = bottle.bottleSize;
    const data = await request<{ updateBottle: GqlBottle }>(
      `mutation ($id: ID!, $input: UpdateBottleInput!) { updateBottle(id: $id, input: $input) { ${BOTTLE_FIELDS} } }`,
      { id: String(id), input }
    );
    return toBottle(data.updateBottle);
  },

  deleteBottle: async (id: number): Promise<Bottle | null> => {
    await request<{ deleteBottle: boolean }>(
      `mutation ($id: ID!) { deleteBottle(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getBottlesByCountryId: async (countryId: number): Promise<Bottle[]> =>
    (await fetchAllBottles())
      .filter((b) => num(b.countryId) === countryId)
      .map((b) => toBottle(b)),

  getBottlesByRegionId: async (regionId: number): Promise<Bottle[]> =>
    (await fetchAllBottles())
      .filter((b) => num(b.regionId) === regionId)
      .map((b) => toBottle(b)),

  getBottlesByTypeId: async (typeId: number): Promise<Bottle[]> =>
    (await fetchAllBottles())
      .filter((b) => num(b.typeId) === typeId)
      .map((b) => toBottle(b)),

  getBottlesByVintageYear: async (year: number): Promise<Bottle[]> =>
    (await fetchAllBottles())
      .filter((b) => b.vintageYear === year)
      .map((b) => toBottle(b)),

  getFavoriteBottles: async (): Promise<Bottle[]> => {
    const pageSize = 200;
    let page = 1;
    const out: Bottle[] = [];
    for (;;) {
      const data = await request<{ userBottles: GqlUserBottlePage }>(
        `query ($page: Int, $pageSize: Int) {
          userBottles(page: $page, pageSize: $pageSize) {
            items { id bottleNumber quantity purchaseAt purchasePrice storageTemp location notes isFavorite bottle { ${BOTTLE_FIELDS} } }
            pageInfo { pageNumber pageSize totalCount }
          }
        }`,
        { page, pageSize }
      );
      out.push(
        ...data.userBottles.items
          .filter((ub) => ub.isFavorite)
          .map((ub) => toBottle(ub.bottle, ub))
      );
      if (
        page * pageSize >= data.userBottles.pageInfo.totalCount ||
        data.userBottles.items.length === 0
      )
        break;
      page += 1;
    }
    return out;
  },

  searchBottles: async (searchTerm: string): Promise<Bottle[]> => {
    const s = searchTerm.trim().toLowerCase();
    return (await fetchAllBottles())
      .filter((b) => (b.vineyard ?? "").toLowerCase().includes(s))
      .map((b) => toBottle(b));
  },

  getBottleCount: async (): Promise<number> => {
    const data = await request<{ bottles: { pageInfo: GqlPageInfo } }>(
      `query { bottles(page: 1, pageSize: 1) { pageInfo { pageNumber pageSize totalCount } } }`
    );
    return data.bottles.pageInfo.totalCount;
  },

  setBottleFavorite: async (id: number, isFavorite: boolean): Promise<void> => {
    await request<{ setBottleFavorite: unknown }>(
      `mutation ($bottleId: ID!, $isFavorite: Boolean!) {
        setBottleFavorite(bottleId: $bottleId, isFavorite: $isFavorite) { id }
      }`,
      { bottleId: String(id), isFavorite }
    );
  },

  // Wine reference data
  getCountries: async (): Promise<Country[]> => {
    const data = await request<{ countries: GqlCountry[] }>(
      `query { countries { id name isoCode description } }`
    );
    return data.countries.map(toCountry);
  },

  getCountriesPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<Country>> =>
    pagedSlice(await api.getCountries(), pageNumber, pageSize),

  getActiveCountries: async (): Promise<Country[]> => api.getCountries(),

  createCountry: async (country: Omit<Country, keyof AuditableEntity>): Promise<Country> => {
    const data = await request<{ createCountry: GqlCountry }>(
      `mutation ($input: CreateCountryInput!) { createCountry(input: $input) { id name isoCode description } }`,
      {
        input: {
          name: country.countryName,
          isoCode: country.isoCode,
          description: country.description,
        },
      }
    );
    return toCountry(data.createCountry);
  },

  updateCountry: async (id: number, country: Partial<Country>): Promise<Country> => {
    const input: Record<string, unknown> = {};
    if (country.countryName !== undefined) input.name = country.countryName;
    if (country.isoCode !== undefined) input.isoCode = country.isoCode;
    if (country.description !== undefined) input.description = country.description;
    if (country.isActive !== undefined) input.isActive = country.isActive;
    const data = await request<{ updateCountry: GqlCountry }>(
      `mutation ($id: ID!, $input: UpdateCountryInput!) { updateCountry(id: $id, input: $input) { id name isoCode description } }`,
      { id: String(id), input }
    );
    return toCountry(data.updateCountry);
  },

  deleteCountry: async (id: number): Promise<Country | null> => {
    await request<{ deleteCountry: boolean }>(
      `mutation ($id: ID!) { deleteCountry(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getRegions: async (): Promise<Region[]> => {
    const countries = await request<{ countries: GqlCountry[] }>(
      `query { countries { id name isoCode description } }`
    );
    const regions: Region[] = [];
    for (const c of countries.countries) {
      regions.push(...(await api.getRegionsByCountryId(num(c.id))));
    }
    return regions;
  },

  getRegionsPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<Region>> =>
    pagedSlice(await api.getRegions(), pageNumber, pageSize),

  getRegionsByCountryId: async (countryId: number): Promise<Region[]> => {
    const data = await request<{ regions: GqlRegion[] }>(
      `query ($countryId: ID!) { regions(countryId: $countryId) { id name description country { id name isoCode description } } }`,
      { countryId: String(countryId) }
    );
    return data.regions.map(toRegion);
  },

  createRegion: async (region: Omit<Region, keyof AuditableEntity>): Promise<Region> => {
    const data = await request<{ createRegion: GqlRegion }>(
      `mutation ($input: CreateRegionInput!) { createRegion(input: $input) { id name description country { id name isoCode description } } }`,
      {
        input: {
          countryId: String(region.countryID),
          name: region.regionName,
          description: region.description,
        },
      }
    );
    return toRegion(data.createRegion);
  },

  updateRegion: async (id: number, region: Partial<Region>): Promise<Region> => {
    const input: Record<string, unknown> = {};
    if (region.countryID !== undefined) input.countryId = String(region.countryID);
    if (region.regionName !== undefined) input.name = region.regionName;
    if (region.description !== undefined) input.description = region.description;
    if (region.isActive !== undefined) input.isActive = region.isActive;
    const data = await request<{ updateRegion: GqlRegion }>(
      `mutation ($id: ID!, $input: UpdateRegionInput!) { updateRegion(id: $id, input: $input) { id name description country { id name isoCode description } } }`,
      { id: String(id), input }
    );
    return toRegion(data.updateRegion);
  },

  deleteRegion: async (id: number): Promise<Region | null> => {
    await request<{ deleteRegion: boolean }>(
      `mutation ($id: ID!) { deleteRegion(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getTypes: async (): Promise<WineType[]> => {
    const data = await request<{ types: GqlWineType[] }>(
      `query { types { id name description } }`
    );
    return data.types.map(toWineType);
  },

  getTypesPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<WineType>> =>
    pagedSlice(await api.getTypes(), pageNumber, pageSize),

  createType: async (type: Omit<WineType, keyof AuditableEntity>): Promise<WineType> => {
    const data = await request<{ createType: GqlWineType }>(
      `mutation ($input: CreateTypeInput!) { createType(input: $input) { id name description } }`,
      {
        input: {
          name: type.typeName,
          description: type.description,
        },
      }
    );
    return toWineType(data.createType);
  },

  updateType: async (id: number, type: Partial<WineType>): Promise<WineType> => {
    const input: Record<string, unknown> = {};
    if (type.typeName !== undefined) input.name = type.typeName;
    if (type.description !== undefined) input.description = type.description;
    if (type.isActive !== undefined) input.isActive = type.isActive;
    const data = await request<{ updateType: GqlWineType }>(
      `mutation ($id: ID!, $input: UpdateTypeInput!) { updateType(id: $id, input: $input) { id name description } }`,
      { id: String(id), input }
    );
    return toWineType(data.updateType);
  },

  deleteType: async (id: number): Promise<WineType | null> => {
    await request<{ deleteType: boolean }>(
      `mutation ($id: ID!) { deleteType(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getVintages: async (): Promise<Vintage[]> => {
    const data = await request<{ vintages: GqlVintage[] }>(
      `query { vintages { id year description isActive } }`
    );
    return data.vintages.map(toVintage);
  },

  getVintagesPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<Vintage>> =>
    pagedSlice(await api.getVintages(), pageNumber, pageSize),

  getActiveVintages: async (): Promise<Vintage[]> =>
    (await api.getVintages()).filter((v) => v.isActive),

  createVintage: async (vintage: Omit<Vintage, keyof AuditableEntity>): Promise<Vintage> => {
    const data = await request<{ createVintage: GqlVintage }>(
      `mutation ($input: CreateVintageInput!) { createVintage(input: $input) { id year description isActive } }`,
      { input: { year: vintage.year, description: vintage.description } }
    );
    return toVintage(data.createVintage);
  },

  updateVintage: async (id: number, vintage: Partial<Vintage>): Promise<Vintage> => {
    const input: Record<string, unknown> = {};
    if (vintage.year !== undefined) input.year = vintage.year;
    if (vintage.description !== undefined) input.description = vintage.description;
    if (vintage.isActive !== undefined) input.isActive = vintage.isActive;
    const data = await request<{ updateVintage: GqlVintage }>(
      `mutation ($id: ID!, $input: UpdateVintageInput!) { updateVintage(id: $id, input: $input) { id year description isActive } }`,
      { id: String(id), input }
    );
    return toVintage(data.updateVintage);
  },

  deleteVintage: async (id: number): Promise<Vintage | null> => {
    await request<{ deleteVintage: boolean }>(
      `mutation ($id: ID!) { deleteVintage(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getGrapeVarieties: async (): Promise<GrapeVariety[]> => {
    const data = await request<{ grapeVarieties: GqlGrapeVariety[] }>(
      `query { grapeVarieties { id name description isActive } }`
    );
    return data.grapeVarieties.map(toGrapeVariety);
  },

  getActiveGrapeVarieties: async (): Promise<GrapeVariety[]> =>
    (await api.getGrapeVarieties()).filter((g) => g.isActive),

  createGrapeVariety: async (grapeVariety: Omit<GrapeVariety, keyof AuditableEntity>): Promise<GrapeVariety> => {
    const data = await request<{ createGrapeVariety: GqlGrapeVariety }>(
      `mutation ($input: CreateGrapeVarietyInput!) { createGrapeVariety(input: $input) { id name description isActive } }`,
      { input: { name: grapeVariety.grapeVarietyName, description: grapeVariety.description } }
    );
    return toGrapeVariety(data.createGrapeVariety);
  },

  updateGrapeVariety: async (id: number, grapeVariety: Partial<GrapeVariety>): Promise<GrapeVariety> => {
    const input: Record<string, unknown> = {};
    if (grapeVariety.grapeVarietyName !== undefined) input.name = grapeVariety.grapeVarietyName;
    if (grapeVariety.description !== undefined) input.description = grapeVariety.description;
    if (grapeVariety.isActive !== undefined) input.isActive = grapeVariety.isActive;
    const data = await request<{ updateGrapeVariety: GqlGrapeVariety }>(
      `mutation ($id: ID!, $input: UpdateGrapeVarietyInput!) { updateGrapeVariety(id: $id, input: $input) { id name description isActive } }`,
      { id: String(id), input }
    );
    return toGrapeVariety(data.updateGrapeVariety);
  },

  deleteGrapeVariety: async (id: number): Promise<GrapeVariety | null> => {
    await request<{ deleteGrapeVariety: boolean }>(
      `mutation ($id: ID!) { deleteGrapeVariety(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getWineFlavorProfiles: async (): Promise<WineFlavorProfile[]> => {
    const data = await request<{ wineFlavorProfiles: GqlWineFlavorProfile[] }>(
      `query { wineFlavorProfiles { id name description isActive } }`
    );
    return data.wineFlavorProfiles.map(toWineFlavorProfile);
  },

  getActiveWineFlavorProfiles: async (): Promise<WineFlavorProfile[]> =>
    (await api.getWineFlavorProfiles()).filter((f) => f.isActive),

  createWineFlavorProfile: async (flavorProfile: Omit<WineFlavorProfile, keyof AuditableEntity>): Promise<WineFlavorProfile> => {
    const data = await request<{ createWineFlavorProfile: GqlWineFlavorProfile }>(
      `mutation ($input: CreateWineFlavorProfileInput!) { createWineFlavorProfile(input: $input) { id name description isActive } }`,
      { input: { name: flavorProfile.flavorProfileName, description: flavorProfile.description } }
    );
    return toWineFlavorProfile(data.createWineFlavorProfile);
  },

  updateWineFlavorProfile: async (id: number, flavorProfile: Partial<WineFlavorProfile>): Promise<WineFlavorProfile> => {
    const input: Record<string, unknown> = {};
    if (flavorProfile.flavorProfileName !== undefined) input.name = flavorProfile.flavorProfileName;
    if (flavorProfile.description !== undefined) input.description = flavorProfile.description;
    if (flavorProfile.isActive !== undefined) input.isActive = flavorProfile.isActive;
    const data = await request<{ updateWineFlavorProfile: GqlWineFlavorProfile }>(
      `mutation ($id: ID!, $input: UpdateWineFlavorProfileInput!) { updateWineFlavorProfile(id: $id, input: $input) { id name description isActive } }`,
      { id: String(id), input }
    );
    return toWineFlavorProfile(data.updateWineFlavorProfile);
  },

  deleteWineFlavorProfile: async (id: number): Promise<WineFlavorProfile | null> => {
    await request<{ deleteWineFlavorProfile: boolean }>(
      `mutation ($id: ID!) { deleteWineFlavorProfile(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  // Recipes
  getRecipes: async (): Promise<Recipe[]> => {
    const pageSize = 200;
    let page = 1;
    const out: Recipe[] = [];
    for (;;) {
      const data = await request<{ recipes: GqlRecipePage }>(
        `query ($page: Int, $pageSize: Int) { recipes(page: $page, pageSize: $pageSize) { items { ${RECIPE_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
        { page, pageSize }
      );
      out.push(...data.recipes.items.map(toRecipe));
      if (
        page * pageSize >= data.recipes.pageInfo.totalCount ||
        data.recipes.items.length === 0
      )
        break;
      page += 1;
    }
    return out.sort(sortByFrequency);
  },

  getRecipesPaged: async (pageNumber: number, pageSize: number, search?: string, isFavorite?: boolean): Promise<PagedResult<Recipe>> => {
    if (!search && isFavorite === undefined) {
      const data = await request<{ recipes: GqlRecipePage }>(
        `query ($page: Int, $pageSize: Int) { recipes(page: $page, pageSize: $pageSize) { items { ${RECIPE_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
        { page: pageNumber, pageSize }
      );
      const items = data.recipes.items.map(toRecipe).sort(sortByFrequency);
      return toPaged(items, data.recipes.pageInfo);
    }
    let all = await api.getRecipes();
    const s = (search ?? "").trim().toLowerCase();
    if (s) all = all.filter((r) => r.recipeName.toLowerCase().includes(s));
    if (isFavorite !== undefined) all = all.filter((r) => r.isFavorite === isFavorite);
    all.sort(sortByFrequency);
    return pagedSlice(all, pageNumber, pageSize);
  },

  getRecipe: async (id: number): Promise<Recipe> => {
    const data = await request<{ recipe: GqlRecipe | null }>(
      `query ($id: ID!) { recipe(id: $id) { ${RECIPE_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.recipe) throw new ApiError(404, `Recipe ${id} not found`);
    return toRecipe(data.recipe);
  },

  createRecipe: async (recipe: Omit<Recipe, keyof AuditableEntity>): Promise<Recipe> => {
    const data = await request<{ createRecipe: GqlRecipe }>(
      `mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { ${RECIPE_FIELDS} } }`,
      { input: toRecipeInput(recipe) }
    );
    return toRecipe(data.createRecipe);
  },

  updateRecipe: async (id: number, recipe: Partial<Recipe>): Promise<Recipe> => {
    const existing = await api.getRecipe(id);
    const merged = { ...existing, ...recipe };
    const data = await request<{ updateRecipe: GqlRecipe }>(
      `mutation ($id: ID!, $input: CreateRecipeInput!) { updateRecipe(id: $id, input: $input) { ${RECIPE_FIELDS} } }`,
      { id: String(id), input: toRecipeInput(merged) }
    );
    return toRecipe(data.updateRecipe);
  },

  deleteRecipe: async (id: number): Promise<Recipe | null> => {
    await request<{ deleteRecipe: boolean }>(
      `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  setRecipeFavorite: async (id: number, isFavorite: boolean): Promise<void> => {
    await request<{ setRecipeFavorite: boolean }>(
      `mutation ($recipeId: ID!, $isFavorite: Boolean!) { setRecipeFavorite(recipeId: $recipeId, isFavorite: $isFavorite) }`,
      { recipeId: String(id), isFavorite }
    );
  },

  rateRecipe: async (id: number, rating: number): Promise<Recipe> => {
    const data = await request<{ rateRecipe: GqlRecipe }>(
      `mutation ($recipeId: ID!, $rating: Int!) { rateRecipe(recipeId: $recipeId, rating: $rating) { ${RECIPE_FIELDS} } }`,
      { recipeId: String(id), rating }
    );
    return toRecipe(data.rateRecipe);
  },

  getRecipeItems: async (recipeId: number): Promise<RecipeItem[]> =>
    (await api.getRecipe(recipeId)).recipeItems ?? [],

  addRecipeItem: async (recipeId: number, item: { itemId: number; portion: number; unit: string | null; isOptional: boolean }): Promise<RecipeItem> => {
    const recipe = await api.getRecipe(recipeId);
    const items = (recipe.recipeItems ?? []).map((i) => ({
      itemId: String(i.itemID),
      quantity: i.quantity,
      unit: i.unitOfMeasure ?? "",
      notes: i.notes,
      isOptional: i.isOptional,
    }));
    items.push({
      itemId: String(item.itemId),
      quantity: item.portion,
      unit: item.unit ?? "",
      notes: null,
      isOptional: item.isOptional,
    });
    const updated = await api.updateRecipe(recipeId, recipeInputOverride(recipe, { items }));
    return (
      (updated.recipeItems ?? []).find((i) => i.itemID === item.itemId) ?? {
        recipeID: recipeId,
        itemID: item.itemId,
        quantity: item.portion,
        unitOfMeasure: item.unit,
        notes: null,
        isOptional: item.isOptional,
      }
    );
  },

  removeRecipeItem: async (recipeId: number, itemId: number): Promise<void> => {
    const recipe = await api.getRecipe(recipeId);
    const items = (recipe.recipeItems ?? [])
      .filter((i) => i.itemID !== itemId)
      .map((i) => ({
        itemId: String(i.itemID),
        quantity: i.quantity,
        unit: i.unitOfMeasure ?? "",
        notes: i.notes,
        isOptional: i.isOptional,
      }));
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { items }));
  },

  getRecipeSteps: async (recipeId: number): Promise<RecipeStep[]> =>
    (await api.getRecipe(recipeId)).recipeSteps ?? [],

  addRecipeStep: async (recipeId: number, step: { stepNumber: number; instruction: string }): Promise<RecipeStep> => {
    const recipe = await api.getRecipe(recipeId);
    const steps = (recipe.recipeSteps ?? []).map((s) => ({
      stepNumber: s.stepNumber,
      instruction: s.instruction,
    }));
    steps.push(step);
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { steps }));
    return {
      ...audit(),
      recipeStepID: step.stepNumber,
      recipeID: recipeId,
      stepNumber: step.stepNumber,
      instruction: step.instruction,
      recipe: null,
    };
  },

  updateRecipeStep: async (recipeId: number, stepId: number, step: { stepNumber: number; instruction: string }): Promise<RecipeStep> => {
    const recipe = await api.getRecipe(recipeId);
    const steps = (recipe.recipeSteps ?? []).map((s) =>
      s.recipeStepID === stepId || s.stepNumber === stepId
        ? { stepNumber: step.stepNumber, instruction: step.instruction }
        : { stepNumber: s.stepNumber, instruction: s.instruction }
    );
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { steps }));
    return {
      ...audit(),
      recipeStepID: step.stepNumber,
      recipeID: recipeId,
      stepNumber: step.stepNumber,
      instruction: step.instruction,
      recipe: null,
    };
  },

  deleteRecipeStep: async (recipeId: number, stepId: number): Promise<void> => {
    const recipe = await api.getRecipe(recipeId);
    const steps = (recipe.recipeSteps ?? [])
      .filter((s) => s.recipeStepID !== stepId && s.stepNumber !== stepId)
      .map((s) => ({ stepNumber: s.stepNumber, instruction: s.instruction }));
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { steps }));
  },

  // Meal Plans
  getMealPlansPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<MealPlan>> => {
    const data = await request<{ mealPlans: GqlMealPlanPage }>(
      `query ($page: Int, $pageSize: Int) { mealPlans(page: $page, pageSize: $pageSize) { items { ${MEAL_PLAN_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page: pageNumber, pageSize }
    );
    return toPaged(data.mealPlans.items.map(toMealPlan), data.mealPlans.pageInfo);
  },

  getMealPlan: async (id: number): Promise<MealPlan> => {
    const data = await request<{ mealPlan: GqlMealPlan | null }>(
      `query ($id: ID!) { mealPlan(id: $id) { ${MEAL_PLAN_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.mealPlan) throw new ApiError(404, `MealPlan ${id} not found`);
    return toMealPlan(data.mealPlan);
  },

  createMealPlan: async (plan: Omit<MealPlan, keyof AuditableEntity | "mealPlanID" | "mealSlots">): Promise<MealPlan> => {
    const data = await request<{ createMealPlan: GqlMealPlan }>(
      `mutation ($input: CreateMealPlanInput!) { createMealPlan(input: $input) { ${MEAL_PLAN_FIELDS} } }`,
      {
        input: {
          name: plan.planName,
          weekStartDate: plan.weekStartDate,
          weekStartDayOfWeek: plan.weekStartDayOfWeek,
        },
      }
    );
    return toMealPlan(data.createMealPlan);
  },

  updateMealPlan: async (id: number, plan: Partial<MealPlan>): Promise<MealPlan> => {
    const existing = await api.getMealPlan(id);
    const data = await request<{ updateMealPlan: GqlMealPlan }>(
      `mutation ($id: ID!, $input: CreateMealPlanInput!) { updateMealPlan(id: $id, input: $input) { ${MEAL_PLAN_FIELDS} } }`,
      {
        id: String(id),
        input: {
          name: plan.planName ?? existing.planName,
          weekStartDate: plan.weekStartDate ?? existing.weekStartDate,
          weekStartDayOfWeek:
            plan.weekStartDayOfWeek ?? existing.weekStartDayOfWeek,
        },
      }
    );
    return toMealPlan(data.updateMealPlan);
  },

  deleteMealPlan: async (id: number): Promise<MealPlan | null> => {
    await request<{ deleteMealPlan: boolean }>(
      `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
      { id: String(id) }
    );
    return null;
  },

  getMealPlanNutrition: async (id: number): Promise<MealPlanNutrition> => {
    const data = await request<{ nutrition: GqlNutritionSummary[] }>(
      `query ($mealPlanId: ID!) { nutrition(mealPlanId: $mealPlanId) { name unit amount } }`,
      { mealPlanId: String(id) }
    );
    return {
      mealPlanId: id,
      dailyTotals: [
        {
          dayOfWeek: 0,
          nutrients: data.nutrition.map((n) => ({
            nutrientId: 0,
            nutrientName: n.name,
            unitOfMeasure: n.unit,
            amount: n.amount,
          })),
        },
      ],
      meals: [],
    };
  },

  getMealSlots: async (planId: number): Promise<MealSlot[]> =>
    (await api.getMealPlan(planId)).mealSlots ?? [],

  addMealSlot: async (planId: number, slot: Omit<MealSlot, "mealSlotID" | "mealPlanID" | "mealPlan" | "recipe" | "mealSlotItems">): Promise<MealSlot> => {
    const data = await request<{ addMealSlot: GqlMealSlot }>(
      `mutation ($input: AddMealSlotInput!) {
        addMealSlot(input: $input) {
          id dayOfWeek mealType servings replacementNote
          recipe { ${RECIPE_FIELDS} }
          items { id quantity unit isFromRecipe item { ${ITEM_FIELDS} } }
        }
      }`,
      {
        input: {
          mealPlanId: String(planId),
          dayOfWeek: slot.dayOfWeek,
          mealType: mealTypeToString(slot.mealType),
          recipeId: slot.recipeID != null ? String(slot.recipeID) : null,
          servings: slot.servings,
          replacementNote: slot.replacementNote,
        },
      }
    );
    return toMealSlot(planId, data.addMealSlot);
  },

  updateMealSlot: async (planId: number, slotId: number, slot: Partial<MealSlot>): Promise<MealSlot> => {
    const slots = await api.getMealSlots(planId);
    const existing = slots.find((s) => s.mealSlotID === slotId);
    if (!existing) throw new ApiError(404, `MealSlot ${slotId} not found`);
    await api.deleteMealSlot(planId, slotId);
    return api.addMealSlot(planId, {
      ...audit(),
      dayOfWeek: slot.dayOfWeek ?? existing.dayOfWeek,
      mealType: slot.mealType ?? existing.mealType,
      recipeID: slot.recipeID !== undefined ? slot.recipeID : existing.recipeID,
      servings: slot.servings ?? existing.servings,
      replacementNote:
        slot.replacementNote !== undefined
          ? slot.replacementNote
          : existing.replacementNote,
    });
  },

  deleteMealSlot: async (_planId: number, slotId: number): Promise<void> => {
    await request<{ removeMealSlot: boolean }>(
      `mutation ($slotId: ID!) { removeMealSlot(slotId: $slotId) }`,
      { slotId: String(slotId) }
    );
  },

  getMealSlotItems: async (slotId: number): Promise<MealSlotItem[]> => {
    const pageSize = 100;
    let page = 1;
    for (;;) {
      const data = await request<{ mealPlans: GqlMealPlanPage }>(
        `query ($page: Int, $pageSize: Int) { mealPlans(page: $page, pageSize: $pageSize) { items { ${MEAL_PLAN_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
        { page, pageSize }
      );
      for (const plan of data.mealPlans.items) {
        const slot = (plan.slots ?? []).find((s) => num(s.id) === slotId);
        if (slot) return (slot.items ?? []).map((i) => toMealSlotItem(slotId, i));
      }
      if (
        page * pageSize >= data.mealPlans.pageInfo.totalCount ||
        data.mealPlans.items.length === 0
      )
        break;
      page += 1;
    }
    return [];
  },

  addMealSlotItem: async (slotId: number, item: Omit<MealSlotItem, "mealSlotItemID" | "mealSlotID" | "mealSlot" | "item">): Promise<MealSlotItem> => {
    const data = await request<{ addMealSlotItem: GqlMealSlotItem }>(
      `mutation ($input: AddMealSlotItemInput!) {
        addMealSlotItem(input: $input) { id quantity unit isFromRecipe item { ${ITEM_FIELDS} } }
      }`,
      {
        input: {
          slotId: String(slotId),
          itemId: String(item.itemID),
          quantity: item.quantity,
          unit: item.unitOfMeasure ?? "",
          isFromRecipe: item.isFromRecipe,
        },
      }
    );
    return toMealSlotItem(slotId, data.addMealSlotItem);
  },

  deleteMealSlotItem: async (_slotId: number, itemId: number): Promise<void> => {
    await request<{ removeMealSlotItem: boolean }>(
      `mutation ($slotItemId: ID!) { removeMealSlotItem(slotItemId: $slotItemId) }`,
      { slotItemId: String(itemId) }
    );
  },

  // Grocery Lists
  getGroceryLists: async (): Promise<GroceryList[]> => {
    const data = await request<{ groceryLists: GqlGroceryListPage }>(
      `query { groceryLists(page: 1, pageSize: 200) { items { ${GROCERY_LIST_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`
    );
    return data.groceryLists.items.map(toGroceryList);
  },

  getGroceryList: async (id: number): Promise<GroceryList> => {
    const data = await request<{ groceryList: GqlGroceryList | null }>(
      `query ($id: ID!) { groceryList(id: $id) { ${GROCERY_LIST_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.groceryList) throw new ApiError(404, `GroceryList ${id} not found`);
    return toGroceryList(data.groceryList);
  },

  generateGroceryList: async (mealPlanId?: number): Promise<GroceryList> => {
    if (mealPlanId === undefined) {
      throw new ApiError(400, "generateGroceryList requires a mealPlanId");
    }
    const data = await request<{ generateGroceryList: GqlGroceryList }>(
      `mutation ($mealPlanId: ID!) { generateGroceryList(mealPlanId: $mealPlanId) { ${GROCERY_LIST_FIELDS} } }`,
      { mealPlanId: String(mealPlanId) }
    );
    return toGroceryList(data.generateGroceryList);
  },

  addGroceryListItem: async (listId: number, item: Omit<GroceryListItem, "groceryListItemID" | "groceryListID" | "groceryList">): Promise<GroceryListItem> => {
    const data = await request<{ addGroceryItem: GqlGroceryListItem }>(
      `mutation ($input: AddGroceryItemInput!) {
        addGroceryItem(input: $input) {
          id manualItemName quantityNeeded unitOfMeasure source isChecked
          item { ${ITEM_FIELDS} }
        }
      }`,
      {
        input: {
          groceryListId: String(listId),
          itemId: item.itemID != null ? String(item.itemID) : null,
          manualItemName: item.manualItemName,
          quantity: item.quantityNeeded,
          unit: item.unitOfMeasure ?? "",
        },
      }
    );
    return toGroceryListItem(listId, data.addGroceryItem);
  },

  toggleGroceryListItemChecked: async (id: number): Promise<GroceryListItem> => {
    const data = await request<{ toggleGroceryItemChecked: GqlGroceryListItem }>(
      `mutation ($groceryListItemId: ID!) {
        toggleGroceryItemChecked(groceryListItemId: $groceryListItemId) {
          id manualItemName quantityNeeded unitOfMeasure source isChecked
          item { ${ITEM_FIELDS} }
        }
      }`,
      { groceryListItemId: String(id) }
    );
    return toGroceryListItem(0, data.toggleGroceryItemChecked);
  },

  deleteGroceryListItem: async (id: number): Promise<void> => {
    await request<{ deleteGroceryItem: boolean }>(
      `mutation ($groceryListItemId: ID!) { deleteGroceryItem(groceryListItemId: $groceryListItemId) }`,
      { groceryListItemId: String(id) }
    );
  },
};

/* ------------------------------------------------------------------ */
/* Recipe input helpers                                                */
/* ------------------------------------------------------------------ */

interface RecipeInputShape {
  name: string;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  items: {
    itemId: string;
    quantity: number;
    unit: string;
    notes: string | null;
    isOptional: boolean;
  }[];
  steps: { stepNumber: number; instruction: string }[];
}

function toRecipeInput(recipe: Partial<Recipe>): RecipeInputShape {
  return {
    name: recipe.recipeName ?? "",
    description: recipe.description ?? null,
    servings: recipe.servings ?? null,
    prepTimeMinutes: recipe.prepTimeMinutes ?? null,
    cookTimeMinutes: recipe.cookTimeMinutes ?? null,
    items: (recipe.recipeItems ?? []).map((i) => ({
      itemId: String(i.itemID),
      quantity: i.quantity,
      unit: i.unitOfMeasure ?? "",
      notes: i.notes ?? null,
      isOptional: i.isOptional,
    })),
    steps: (recipe.recipeSteps ?? []).map((s) => ({
      stepNumber: s.stepNumber,
      instruction: s.instruction,
    })),
  };
}

function recipeInputOverride(
  recipe: Recipe,
  overrides: Partial<Pick<RecipeInputShape, "items" | "steps">>
): Partial<Recipe> {
  const input = toRecipeInput(recipe);
  if (overrides.items) input.items = overrides.items;
  if (overrides.steps) input.steps = overrides.steps;
  return {
    recipeName: input.name,
    description: input.description,
    servings: input.servings,
    prepTimeMinutes: input.prepTimeMinutes,
    cookTimeMinutes: input.cookTimeMinutes,
    recipeItems: input.items.map((i) => ({
      recipeID: recipe.recipeID,
      itemID: num(i.itemId),
      quantity: i.quantity,
      unitOfMeasure: i.unit,
      notes: i.notes,
      isOptional: i.isOptional,
    })),
    recipeSteps: input.steps.map((s) => ({
      ...audit(),
      recipeStepID: s.stepNumber,
      recipeID: recipe.recipeID,
      stepNumber: s.stepNumber,
      instruction: s.instruction,
    })),
  };
}
