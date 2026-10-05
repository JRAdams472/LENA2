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
  RecipeRecommendation,
  RecipeCategory,
  RecipeCategoryGroup,
  MealPlan,
  MealSlot,
  MealSlotItem,
  MealPlanNutrition,
  MealPlanSuggestion,
  PairingSuggestion,
  CocktailSuggestion,
  GroceryList,
  GroceryListItem,
  GroceryRouteGroup,
  Store,
  StoreAisle,
  Household,
  HouseholdInvite,
  HouseholdMember,
  HouseholdNotification,
  HouseholdRole,
  HouseholdUser,
  FoodEvent,
  EventRecipe,
  EventRecipeItem,
  EventRecipeStep,
  EventTimeline,
  EventFixSuggestion,
  AssistantAnswer,
  AssistantToolSpec,
  PreparedAIRequest,
  InviteStatus,
  NotificationCategoryPreference,
  NotificationKind,
  Unit,
  Ingredient,
  Allergen,
  AllergenFlag,
  AllergenSuggestion,
  AllergyWarning,
  MemberAllergen,
  RecipeImport,
  RecipeImportDraft,
  RecipeImportDraftItem,
  RecipeImportReview,
  RecipeImportReviewItem,
  PagedResult,
} from "./types";
import { brandedName } from "./format";

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
// The token lives in sessionStorage (per-tab); AuthProvider migrates any
// legacy localStorage copy.
let authTokenGetter: (() => string | null) | null = () =>
  typeof window === "undefined"
    ? null
    : window.sessionStorage.getItem("lena_id_token") ??
      window.localStorage.getItem("lena_id_token");
let onUnauthorized: (() => void) | null = null;
// sessionRefresher is registered by AuthProvider: on a 401 it attempts one
// refresh-token rotation and reports whether a usable access token now
// exists. Single-flighted below so a burst of 401s shares one rotation.
let sessionRefresher: (() => Promise<boolean>) | null = null;
let refreshInFlight: Promise<boolean> | null = null;

export function setAuthTokenGetter(getter: () => string | null) {
  authTokenGetter = getter;
}

export function setOnUnauthorized(handler: () => void) {
  onUnauthorized = handler;
}

export function setSessionRefresher(refresher: (() => Promise<boolean>) | null) {
  sessionRefresher = refresher;
}

async function tryRefreshSession(): Promise<boolean> {
  if (!sessionRefresher) return false;
  refreshInFlight ??= sessionRefresher().finally(() => {
    refreshInFlight = null;
  });
  return refreshInFlight;
}

function getAuthToken(): string | null {
  return authTokenGetter ? authTokenGetter() : null;
}

/* ------------------------------------------------------------------ */
/* Session endpoints (rt-p2): /auth/session* lives outside GraphQL so   */
/* refresh works after the access token expires.                        */
/* ------------------------------------------------------------------ */

export interface SessionBundle {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
}

// API_BASE_URL targets the GraphQL endpoint; session routes share the
// same origin (Caddy routes /auth/* to the API).
function sessionUrl(path: string): string {
  const origin =
    typeof window === "undefined" ? "http://localhost" : window.location.origin;
  const u = new URL(API_BASE_URL, origin);
  u.pathname = path;
  u.search = "";
  return u.toString();
}

// createSession exchanges a provider credential for a LENA session.
// Returns null on any failure so the caller can fall back to OIDC-only
// mode (token still works as the bearer for its remaining lifetime).
export async function createSession(
  idToken: string,
  device?: string
): Promise<SessionBundle | null> {
  const res = await fetch(sessionUrl("/auth/session"), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${idToken}`,
    },
    body: JSON.stringify({ device: device ?? "web" }),
  });
  if (!res.ok) return null;
  return (await res.json()) as SessionBundle;
}

// refreshSessionRequest rotates the session. Browser clients omit the
// token — the HttpOnly refresh cookie carries the credential; mobile
// clients pass the stored token explicitly.
export async function refreshSessionRequest(
  refreshToken?: string,
  device?: string
): Promise<SessionBundle | null> {
  const res = await fetch(sessionUrl("/auth/session/refresh"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      refreshToken: refreshToken ?? "",
      device: device ?? "web",
    }),
  });
  if (!res.ok) return null;
  return (await res.json()) as SessionBundle;
}

// createProviderSession exchanges an OAuth2 authorization code for a
// LENA session at /auth/session/{provider}. There is no OIDC fallback —
// a code is not a bearer token, so this fails hard when the provider or
// sessions are not configured on the server. nonce is the authorize-
// request value the provider echoes in its id_token (Microsoft,
// Facebook); providers without id_tokens ignore it. codeVerifier is
// the PKCE secret the server forwards to the token exchange — required
// since the authorize redirect carried a code_challenge.
export async function createProviderSession(
  provider: string,
  code: string,
  nonce?: string,
  device?: string,
  codeVerifier?: string
): Promise<SessionBundle> {
  const res = await fetch(sessionUrl(`/auth/session/${provider}`), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code, nonce, device: device ?? "web", codeVerifier }),
  });
  if (!res.ok) {
    throw new Error(`${provider} sign-in failed (HTTP ${res.status})`);
  }
  return (await res.json()) as SessionBundle;
}

// revokeSession is best-effort sign-out — the session dies with the
// refresh token's expiry regardless, so transport errors are ignored.
// Browsers omit the token (the refresh cookie carries it); the server
// always clears the cookie.
export async function revokeSession(refreshToken?: string): Promise<void> {
  await fetch(sessionUrl("/auth/session/revoke"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refreshToken: refreshToken ?? "" }),
  }).catch(() => undefined);
}

interface GraphQLError {
  message: string;
  extensions?: { code?: string };
}

interface GraphQLResponse<T> {
  data?: T;
  errors?: GraphQLError[];
}

// All mutation strings in this module declare the keyword, so a leading
// check is enough — a miss only skips the dedup header, never blocks.
function isMutation(query: string): boolean {
  return query.trimStart().toLowerCase().startsWith("mutation");
}

// Idempotency keys need uniqueness, not secrecy: a fresh key per logical
// operation lets the server dedup retries, while a network-failure retry
// below reuses the same key so the attempt that actually landed is
// replayed rather than re-executed.
let idempotencyCounter = 0;

function newIdempotencyKey(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  // crypto.getRandomValues works in non-secure contexts (http on LAN dev
  // hosts) where randomUUID is gated out.
  if (typeof crypto !== "undefined" && crypto.getRandomValues) {
    return Array.from(crypto.getRandomValues(new Uint8Array(16)))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  }
  return `${Date.now().toString(36)}-${(++idempotencyCounter).toString(36)}`;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// One retry, after a short pause, on transport failure only — fetch's
// TypeError means the request may or may not have reached the server; the
// reused idempotency key makes the retry safe either way.
const NETWORK_RETRY_DELAY_MS = 300;

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
  const init: RequestInit = {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(idToken ? { Authorization: `Bearer ${idToken}` } : {}),
      ...(isMutation(query)
        ? { "Idempotency-Key": newIdempotencyKey() }
        : {}),
    },
    body: JSON.stringify({ query, variables: variables ?? {} }),
  };

  let res: Response;
  try {
    res = await fetch(API_BASE_URL, init);
  } catch {
    await sleep(NETWORK_RETRY_DELAY_MS);
    res = await fetch(API_BASE_URL, init);
  }

  // Expired access token: rotate the refresh token once (single-flight
  // across concurrent requests) and retry with the new access token.
  if (res.status === 401 && (await tryRefreshSession())) {
    const fresh = getAuthToken();
    res = await fetch(API_BASE_URL, {
      ...init,
      headers: {
        ...init.headers,
        ...(fresh ? { Authorization: `Bearer ${fresh}` } : {}),
      },
    });
  }

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

interface GqlUnit {
  id: string;
  name: string;
  abbreviation: string | null;
  kind: string;
  isActive: boolean;
}

interface GqlCategory {
  id: string;
  name: string;
  description: string | null;
  isActive: boolean;
  isProtein: boolean;
}

interface GqlIngredient {
  id: string;
  name: string;
  category: GqlCategory | null;
  defaultUnit: string | null;
  isActive: boolean;
  allergens?: GqlAllergenFlag[];
  allergyWarnings?: GqlAllergyWarning[];
}

interface GqlIngredientPage {
  items: GqlIngredient[];
  pageInfo: GqlPageInfo;
}

interface GqlAllergen {
  id: string;
  name: string;
  description: string | null;
  isActive: boolean;
}

interface GqlAllergenFlag {
  kind: string;
  allergen: GqlAllergen;
}

interface GqlAllergyWarning {
  memberKind: string;
  entityKind: string;
  member: GqlHouseholdUser;
  allergen: GqlAllergen;
}

interface GqlMemberAllergen {
  kind: string;
  allergen: GqlAllergen;
}

interface GqlAllergenSuggestion {
  id: string;
  recipeId: string | null;
  recipeName: string | null;
  targetKind: string;
  ingredientId: string | null;
  ingredientName: string | null;
  itemId: string | null;
  itemName: string | null;
  allergen: GqlAllergen;
  kind: string;
  rationale: string | null;
  status: string;
  reviewedAt: string | null;
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
  ingredient?: GqlIngredient | null;
  householdIngredient?: GqlIngredient | null;
  nutrients: GqlFoodNutrient[];
  flavors: GqlFoodFlavor[];
  status: string;
  submittedByMe: boolean;
  selectionCount: number;
  personalSelectionCount: number;
  allergens?: GqlAllergenFlag[];
  allergyWarnings?: GqlAllergyWarning[];
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
  item: GqlItem | null;
  ingredient?: GqlIngredient | null;
  quantity: number;
  unit: string;
  notes: string | null;
  isOptional: boolean;
}

interface GqlRecipeStep {
  stepNumber: number;
  instruction: string;
  durationMinutes: number | null;
  stepType: string | null;
  isPassive: boolean;
  dependsOnStepNumber: number | null;
  appliance: string | null;
}

interface GqlRecipeCategory {
  id: string;
  name: string;
  group: GqlRecipeCategoryGroup;
}

interface GqlRecipeCategoryGroup {
  id: string;
  name: string;
  exclusive: boolean;
  displayOrder: number;
  categories?: GqlRecipeCategory[];
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
  categories: GqlRecipeCategory[] | null;
  allergens?: GqlAllergenFlag[];
  allergyWarnings?: GqlAllergyWarning[];
}

interface GqlRecipePage {
  items: GqlRecipe[];
  pageInfo: GqlPageInfo;
}

interface GqlRecipeRecommendation {
  recipe: GqlRecipe;
  reason: string;
  score: number;
}

interface GqlRecipeImportDraftItem {
  quantity: number | null;
  unit: string | null;
  ingredient: string;
  section: string | null;
  notes: string | null;
  isOptional: boolean;
}

interface GqlRecipeImportDraftStep {
  stepNumber: number;
  instruction: string;
}

interface GqlRecipeImportDraft {
  name: string | null;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  sourceHint: string | null;
  items: GqlRecipeImportDraftItem[];
  steps: GqlRecipeImportDraftStep[];
}

interface GqlRecipeImportSuggestion {
  id: string;
  name: string;
  kind: string;
  score: number;
}

interface GqlRecipeImportReviewItem {
  draftItem: GqlRecipeImportDraftItem;
  itemId: string | null;
  itemKind: string | null;
  itemName: string | null;
  unit: string | null;
  unitId: string | null;
  confidence: number;
  suggestions: GqlRecipeImportSuggestion[];
  status: string;
  notes: string | null;
  approved: boolean;
}

interface GqlRecipeImportReviewStep {
  stepNumber: number;
  instruction: string;
}

interface GqlRecipeImportReview {
  pageId: string | null;
  name: string | null;
  description: string | null;
  servings: number | null;
  prepTimeMinutes: number | null;
  cookTimeMinutes: number | null;
  sourceHint: string | null;
  items: GqlRecipeImportReviewItem[];
  steps: GqlRecipeImportReviewStep[];
  approved: boolean;
}

interface GqlRecipeImport {
  id: string;
  status: string;
  sourceFilename: string;
  ocrText: string | null;
  draft: GqlRecipeImportDraft | null;
  review: GqlRecipeImportReview | null;
  recipe: GqlRecipe | null;
  profanityFlag: boolean;
  profanityReason: string | null;
  errorMessage: string | null;
  createdAt: string;
  updatedAt: string | null;
  createdBy: string;
}

interface GqlRecipeImportPage {
  items: GqlRecipeImport[];
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
  ingredient?: GqlIngredient | null;
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
  allergens?: GqlAllergenFlag[];
  allergyWarnings?: GqlAllergyWarning[];
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

interface GqlMealPlanSuggestion {
  recipe: GqlRecipe;
  dayOfWeek: number;
  mealType: string;
  reason: string;
  usesExpiringItems: string[];
}

interface GqlPairingSuggestion {
  bottleId: string | null;
  name: string;
  reason: string;
  inCellar: boolean;
}

interface GqlCocktailSuggestion {
  recipe: GqlRecipe;
  reason: string;
  missingIngredients: string[];
}

interface GqlAssistantAnswer {
  answer: string;
  toolCalls: { name: string }[] | null;
}

interface GqlPreparedRequest {
  prompt: string;
  contextJson: string;
  outputSchemaJson: string;
}

interface GqlEventFixSuggestion {
  eventRecipeId: string;
  recipeName: string;
  stepNumber: number | null;
  action: EventFixSuggestion["action"];
  minutes: number | null;
  appliance: string | null;
  durationMinutes: number | null;
  dependsOnStepNumber: number | null;
  reason: string;
}

interface GqlEventRecipeStep {
  id: string;
  stepNumber: number;
  instruction: string;
  durationMinutes: number | null;
  stepType: string | null;
  isPassive: boolean;
  dependsOnStepNumber: number | null;
  appliance: string | null;
}

interface GqlEventRecipeItem {
  id: string;
  item: { id: string; name: string } | null;
  ingredient?: { id: string; name: string } | null;
  quantity: number;
  baseQuantity: number;
  unit: string;
  section: string | null;
  displayOrder: number;
  notes: string | null;
  isOptional: boolean;
}

interface GqlEventRecipe {
  id: string;
  mealType: string;
  targetTime: string;
  servings: number | null;
  baseServings: number | null;
  scalingFactor: number;
  notes: string | null;
  recipe: GqlRecipe | null;
  steps: GqlEventRecipeStep[];
  items: GqlEventRecipeItem[];
  allergens?: GqlAllergenFlag[];
  allergyWarnings?: GqlAllergyWarning[];
}

interface GqlFoodEvent {
  id: string;
  name: string;
  eventDate: string;
  slotGranularityMinutes: number;
  isActive: boolean;
  recipes: GqlEventRecipe[];
}

interface GqlFoodEventPage {
  items: GqlFoodEvent[];
  pageInfo: GqlPageInfo;
}

interface GqlTimelineStep {
  stepNumber: number;
  instruction: string;
  stepType: string | null;
  isPassive: boolean;
  appliance: string | null;
  durationMinutes: number | null;
  scheduledMinutes: number;
  estimated: boolean;
  startTime: string;
  endTime: string;
  conflicts: string[];
}

interface GqlEventTimelineRecipe {
  eventRecipeId: string;
  name: string;
  targetTime: string;
  servings: number | null;
  baseServings: number | null;
  startBy: string | null;
  unschedulable: boolean;
  warnings: string[];
  steps: GqlTimelineStep[];
}

interface GqlEventTimeline {
  foodEventId: string;
  warnings: string[];
  recipes: GqlEventTimelineRecipe[];
}

interface GqlNutritionSummary {
  name: string;
  unit: string;
  amount: number;
}

interface GqlGroceryListItem {
  id: string;
  item: GqlItem | null;
  ingredient?: { id: string; name: string } | null;
  usualBrand?: { id: string; name: string; brand: GqlBrand | null } | null;
  manualItemName: string | null;
  quantityNeeded: number;
  unitOfMeasure: string | null;
  source: string;
  isChecked: boolean;
  allergens?: GqlAllergenFlag[];
  allergyWarnings?: GqlAllergyWarning[];
}

interface GqlStoreAisle {
  id: string;
  name: string;
  position: number;
}

interface GqlStore {
  id: string;
  name: string;
  aisles?: GqlStoreAisle[];
}

interface GqlGroceryList {
  id: string;
  generatedAt: string;
  store: GqlStore | null;
  items: GqlGroceryListItem[];
}

interface GqlGroceryRouteItem {
  suggested: boolean;
  item: GqlGroceryListItem;
}

interface GqlGroceryRouteGroup {
  aisle: GqlStoreAisle | null;
  items: GqlGroceryRouteItem[];
}

interface GqlGroceryListPage {
  items: GqlGroceryList[];
  pageInfo: GqlPageInfo;
}

interface GqlUser {
  id: string;
  email: string;
  displayName: string | null;
  firstName: string | null;
  lastName: string | null;
  backupEmail: string | null;
  birthdate?: string | null;
  role: string;
  isActive: boolean;
  isProtected: boolean;
  lastLoginAt: string | null;
  isSearchable?: boolean;
  household?: GqlHousehold | null;
}

interface GqlUserPage {
  items: GqlUser[];
  pageInfo: GqlPageInfo;
}

interface GqlHouseholdUser {
  id: string;
  displayName: string | null;
  firstName: string | null;
  lastName: string | null;
}

interface GqlHouseholdMember {
  user: GqlHouseholdUser;
  role: string;
  isMe: boolean;
}

interface GqlHousehold {
  id: string;
  name: string | null;
  members: GqlHouseholdMember[];
  myRole: string;
  createdAt: string;
}

interface GqlHouseholdNotification {
  id: string;
  kind: string;
  actor: GqlHouseholdUser | null;
  foodEventId: string | null;
  title: string | null;
  body: string | null;
  recipeId: string | null;
  itemId: string | null;
  createdAt: string;
}

interface GqlNotificationCategoryPreference {
  category: string;
  label: string;
  enabled: boolean;
  mutedUntil: string | null;
}

interface GqlHouseholdInvite {
  id: string;
  fromUser: GqlHouseholdUser;
  toUser: GqlHouseholdUser;
  status: string;
  createdAt: string;
}

/* ------------------------------------------------------------------ */
/* Mappers: GraphQL shape -> UI type (lib/types.ts)                    */
/* ------------------------------------------------------------------ */

const num = (id: string | number | null | undefined): number => {
  const n = Number(id);
  return Number.isFinite(n) ? n : 0;
};

const toUser = (u: GqlUser): User => ({
  userID: num(u.id),
  email: u.email,
  displayName: u.displayName ?? null,
  firstName: u.firstName ?? null,
  lastName: u.lastName ?? null,
  backupEmail: u.backupEmail ?? null,
  birthdate: u.birthdate ?? null,
  role: u.role === "admin" ? "admin" : "member",
  isActive: u.isActive !== false,
  isProtected: u.isProtected === true,
  lastLoginAt: u.lastLoginAt ?? null,
  externalSubject: null,
  provider: null,
  isSearchable: u.isSearchable !== false,
  household: u.household ? toHousehold(u.household) : null,
});

const toHouseholdUser = (u: GqlHouseholdUser): HouseholdUser => ({
  userID: num(u.id),
  displayName: u.displayName ?? null,
  firstName: u.firstName ?? null,
  lastName: u.lastName ?? null,
});

const toHouseholdRole = (r: string): HouseholdRole =>
  r === "OWNER" || r === "ADMIN" ? r : "MEMBER";

const toHouseholdMember = (m: GqlHouseholdMember): HouseholdMember => ({
  user: toHouseholdUser(m.user),
  role: toHouseholdRole(m.role),
  isMe: m.isMe === true,
});

const toHousehold = (h: GqlHousehold): Household => ({
  householdID: num(h.id),
  name: h.name ?? null,
  members: (h.members ?? []).map(toHouseholdMember),
  myRole: toHouseholdRole(h.myRole),
  createdAt: h.createdAt,
});

const toHouseholdNotification = (
  n: GqlHouseholdNotification
): HouseholdNotification => ({
  notificationID: num(n.id),
  kind: n.kind as NotificationKind,
  actor: n.actor ? toHouseholdUser(n.actor) : null,
  foodEventId: n.foodEventId ? num(n.foodEventId) : null,
  title: n.title,
  body: n.body,
  recipeId: n.recipeId ? num(n.recipeId) : null,
  itemId: n.itemId ? num(n.itemId) : null,
  createdAt: n.createdAt,
});

const toHouseholdInvite = (i: GqlHouseholdInvite): HouseholdInvite => ({
  inviteID: num(i.id),
  fromUser: toHouseholdUser(i.fromUser),
  toUser: toHouseholdUser(i.toUser),
  status: (i.status as InviteStatus) ?? "PENDING",
  createdAt: i.createdAt,
});

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

function toUnit(u: GqlUnit): Unit {
  return {
    unitID: num(u.id),
    name: u.name,
    abbreviation: u.abbreviation,
    kind: u.kind,
    isActive: u.isActive,
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

const toAllergen = (a: GqlAllergen): Allergen => ({
  allergenID: num(a.id),
  name: a.name,
  description: a.description ?? null,
  isActive: a.isActive !== false,
});

const toAllergenFlag = (f: GqlAllergenFlag): AllergenFlag => ({
  allergen: toAllergen(f.allergen),
  kind: f.kind === "contains" ? "contains" : "may_contain",
});

const toAllergyWarning = (w: GqlAllergyWarning): AllergyWarning => ({
  member: toHouseholdUser(w.member),
  allergen: toAllergen(w.allergen),
  memberKind: w.memberKind === "allergy" ? "allergy" : "dietary",
  entityKind: w.entityKind === "contains" ? "contains" : "may_contain",
});

const toMemberAllergen = (m: GqlMemberAllergen): MemberAllergen => ({
  allergen: toAllergen(m.allergen),
  kind: m.kind === "allergy" ? "allergy" : "dietary",
});

const ALLERGEN_SUGGESTION_FIELDS = `id recipeId recipeName targetKind ingredientId ingredientName itemId itemName
  allergen { id name description isActive } kind rationale status reviewedAt`;

const toAllergenSuggestion = (s: GqlAllergenSuggestion): AllergenSuggestion => ({
  id: num(s.id),
  recipeId: s.recipeId ? num(s.recipeId) : null,
  recipeName: s.recipeName,
  targetKind: s.targetKind === "item" ? "item" : "ingredient",
  ingredientId: s.ingredientId ? num(s.ingredientId) : null,
  ingredientName: s.ingredientName,
  itemId: s.itemId ? num(s.itemId) : null,
  itemName: s.itemName,
  allergen: toAllergen(s.allergen),
  kind: s.kind === "contains" ? "contains" : "may_contain",
  rationale: s.rationale,
  status: (s.status as AllergenSuggestion["status"]) || "pending",
  reviewedAt: s.reviewedAt,
});

function toIngredient(g: GqlIngredient): Ingredient {
  return {
    ingredientID: num(g.id),
    name: g.name,
    category: g.category ? toCategory(g.category) : null,
    defaultUnit: g.defaultUnit,
    isActive: g.isActive,
    allergens: (g.allergens ?? []).map(toAllergenFlag),
    allergyWarnings: (g.allergyWarnings ?? []).map(toAllergyWarning),
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
    ingredient: i.ingredient ? toIngredient(i.ingredient) : null,
    householdIngredient: i.householdIngredient ? toIngredient(i.householdIngredient) : null,
    unit: i.unit,
    currentQuantity: ui?.currentQty ?? 0,
    minQuantity: ui?.minQty ?? null,
    purchaseDate: ui?.purchaseAt ?? null,
    expiryDate: ui?.expiresAt ?? null,
    notes: ui?.notes ?? null,
    isFavorite: ui?.isFavorite ?? false,
    status: i.status,
    submittedByMe: i.submittedByMe,
    category: i.category
      ? {
          ...audit(),
          categoryID: num(i.category.id),
          categoryName: i.category.name,
          description: i.category.description,
          isActive: true,
          isProtein: i.category.isProtein === true,
        }
      : null,
    foodNutrients: (i.nutrients ?? []).map((n) => toFoodNutrient(itemID, n)),
    foodFlavors: (i.flavors ?? []).map((f) => toFoodFlavor(itemID, f)),
    selectionCount: i.selectionCount ?? 0,
    personalSelectionCount: i.personalSelectionCount ?? 0,
    allergens: (i.allergens ?? []).map(toAllergenFlag),
    allergyWarnings: (i.allergyWarnings ?? []).map(toAllergyWarning),
  };
}

function toRecipeItem(recipeID: number, r: GqlRecipeItem): RecipeItem {
  return {
    recipeID,
    itemID: r.item ? num(r.item.id) : null,
    ingredientID: r.ingredient ? num(r.ingredient.id) : null,
    ingredientName: r.ingredient?.name ?? null,
    quantity: r.quantity,
    unitOfMeasure: r.unit,
    notes: r.notes,
    isOptional: r.isOptional,
    itemName: r.item?.name ?? null,
    itemBrand: r.item?.brand?.name ?? null,
    recipe: null,
    item: r.item ? toItem(r.item) : null,
    ingredient: r.ingredient ? toIngredient(r.ingredient) : null,
  };
}

function toRecipeStep(recipeID: number, s: GqlRecipeStep): RecipeStep {
  return {
    ...audit(),
    recipeStepID: s.stepNumber,
    recipeID,
    stepNumber: s.stepNumber,
    instruction: s.instruction,
    durationMinutes: s.durationMinutes,
    stepType: s.stepType,
    isPassive: s.isPassive,
    dependsOnStepNumber: s.dependsOnStepNumber,
    appliance: s.appliance,
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
    categories: (r.categories ?? []).map(toRecipeCategory),
    allergens: (r.allergens ?? []).map(toAllergenFlag),
    allergyWarnings: (r.allergyWarnings ?? []).map(toAllergyWarning),
  };
}

function toRecipeCategory(c: GqlRecipeCategory): RecipeCategory {
  return {
    categoryID: num(c.id),
    categoryName: c.name,
    group: toRecipeCategoryGroup(c.group),
  };
}

function toRecipeCategoryGroup(g: GqlRecipeCategoryGroup): RecipeCategoryGroup {
  return {
    categoryGroupID: num(g.id),
    groupName: g.name,
    exclusive: g.exclusive,
    displayOrder: g.displayOrder,
    categories: (g.categories ?? []).map(toRecipeCategory),
  };
}

function toRecipeImportDraftItem(item: GqlRecipeImportDraftItem): RecipeImportDraftItem {
  return {
    quantity: item.quantity,
    unit: item.unit,
    ingredient: item.ingredient,
    section: item.section,
    notes: item.notes,
    isOptional: item.isOptional,
  };
}

function toRecipeImportReviewItem(item: GqlRecipeImportReviewItem): RecipeImportReviewItem {
  const draft = toRecipeImportDraftItem(item.draftItem);
  return {
    ...draft,
    unit: item.unit,
    notes: item.notes ?? draft.notes,
    itemId: item.itemId,
    itemKind: item.itemKind,
    itemName: item.itemName,
    unitId: item.unitId,
    confidence: item.confidence,
    suggestions: item.suggestions ?? [],
    status: item.status,
    approved: item.approved,
  };
}

function toRecipeImportDraft(draft: GqlRecipeImportDraft | null): RecipeImportDraft | null {
  if (!draft) return null;
  return {
    name: draft.name,
    description: draft.description,
    servings: draft.servings,
    prepTimeMinutes: draft.prepTimeMinutes,
    cookTimeMinutes: draft.cookTimeMinutes,
    sourceHint: draft.sourceHint,
    items: (draft.items ?? []).map(toRecipeImportDraftItem),
    steps: (draft.steps ?? []).map((s) => ({ stepNumber: s.stepNumber, instruction: s.instruction })),
  };
}

function toRecipeImportReview(review: GqlRecipeImportReview | null): RecipeImportReview | null {
  if (!review) return null;
  return {
    pageId: review.pageId,
    name: review.name,
    description: review.description,
    servings: review.servings,
    prepTimeMinutes: review.prepTimeMinutes,
    cookTimeMinutes: review.cookTimeMinutes,
    sourceHint: review.sourceHint,
    items: (review.items ?? []).map(toRecipeImportReviewItem),
    steps: (review.steps ?? []).map((s) => ({ stepNumber: s.stepNumber, instruction: s.instruction })),
    approved: review.approved,
  };
}

function toRecipeImport(r: GqlRecipeImport): RecipeImport {
  return {
    createdBy: r.createdBy,
    createDate: r.createdAt,
    lastUpdatedBy: null,
    lastUpdatedDate: r.updatedAt,
    recipeImportID: num(r.id),
    status: r.status,
    sourceFilename: r.sourceFilename,
    ocrText: r.ocrText,
    draft: toRecipeImportDraft(r.draft),
    review: toRecipeImportReview(r.review),
    recipeID: r.recipe ? num(r.recipe.id) : null,
    recipe: r.recipe ? toRecipe(r.recipe) : null,
    profanityFlag: r.profanityFlag,
    profanityReason: r.profanityReason,
    errorMessage: r.errorMessage,
  };
}

function toRecipeImportReviewInput(review: RecipeImportReview): Record<string, unknown> {
  return {
    name: review.name,
    description: review.description,
    servings: review.servings,
    prepTimeMinutes: review.prepTimeMinutes,
    cookTimeMinutes: review.cookTimeMinutes,
    sourceHint: review.sourceHint,
    items: review.items.map((it) => ({
      ingredient: it.ingredient,
      quantity: it.quantity,
      unit: it.unit,
      section: it.section,
      notes: it.notes,
      isOptional: it.isOptional,
      itemId: it.itemId,
      itemKind: it.itemKind,
      itemName: it.itemName,
      unitId: it.unitId,
      confidence: it.confidence,
      suggestions: it.suggestions.map((s) => ({ id: s.id, name: s.name, kind: s.kind, score: s.score })),
      // "accepted" has no input field — carry it through as approved so a
      // save doesn't un-resolve rows the matcher already cleared.
      approved: it.approved || it.status === "accepted",
    })),
    steps: review.steps.map((s) => ({
      stepNumber: s.stepNumber,
      instruction: s.instruction,
    })),
  };
}

function toCategory(c: GqlCategory): Category {
  return {
    ...audit(),
    categoryID: num(c.id),
    categoryName: c.name,
    description: c.description,
    isActive: c.isActive,
    isProtein: c.isProtein === true,
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
    itemID: i.item ? num(i.item.id) : null,
    ingredientID: i.ingredient ? num(i.ingredient.id) : null,
    ingredientName: i.ingredient?.name ?? null,
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

export function mealTypeToNumber(mealType: string): number {
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
    allergens: (s.allergens ?? []).map(toAllergenFlag),
    allergyWarnings: (s.allergyWarnings ?? []).map(toAllergyWarning),
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

// EventRecipeStepInput is the writable shape of an event slot's snapshot
// step — step_number is server-assigned on add.
export interface EventRecipeStepInput {
  instruction: string;
  durationMinutes?: number | null;
  stepType?: string | null;
  isPassive?: boolean | null;
  dependsOnStepNumber?: number | null;
  appliance?: string | null;
}

function toEventStepVariables(step: EventRecipeStepInput) {
  return {
    instruction: step.instruction,
    durationMinutes: step.durationMinutes ?? null,
    stepType: step.stepType ?? null,
    isPassive: step.isPassive ?? null,
    dependsOnStepNumber: step.dependsOnStepNumber ?? null,
    appliance: step.appliance ?? null,
  };
}

// EventRecipeItemInput is the writable shape of an event slot's snapshot
// ingredient — quantity is the unscaled (per-base-servings) amount, and
// unit is a name or abbreviation resolved by the shared unit catalog.
export interface EventRecipeItemInput {
  itemID?: number | null;
  ingredientID?: number | null;
  quantity: number;
  unit: string;
  section?: string | null;
  displayOrder?: number | null;
  notes?: string | null;
  isOptional?: boolean | null;
}

function toEventItemVariables(item: EventRecipeItemInput) {
  return {
    itemId: item.itemID != null ? String(item.itemID) : null,
    ingredientId: item.ingredientID != null ? String(item.ingredientID) : null,
    quantity: item.quantity,
    unit: item.unit,
    section: item.section ?? null,
    displayOrder: item.displayOrder ?? null,
    notes: item.notes ?? null,
    isOptional: item.isOptional ?? null,
  };
}

function toEventRecipeStep(s: GqlEventRecipeStep): EventRecipeStep {
  return {
    eventRecipeStepID: num(s.id),
    stepNumber: s.stepNumber,
    instruction: s.instruction,
    durationMinutes: s.durationMinutes,
    stepType: s.stepType,
    isPassive: s.isPassive,
    dependsOnStepNumber: s.dependsOnStepNumber,
    appliance: s.appliance,
  };
}

function toEventRecipeItem(i: GqlEventRecipeItem): EventRecipeItem {
  return {
    eventRecipeItemID: num(i.id),
    itemID: i.item ? num(i.item.id) : null,
    ingredientID: i.ingredient ? num(i.ingredient.id) : null,
    ingredientName: i.ingredient?.name ?? null,
    itemName: i.item?.name ?? null,
    quantity: i.quantity,
    baseQuantity: i.baseQuantity,
    unit: i.unit,
    section: i.section,
    displayOrder: i.displayOrder,
    notes: i.notes,
    isOptional: i.isOptional,
  };
}

function toEventRecipe(foodEventID: number, er: GqlEventRecipe): EventRecipe {
  return {
    eventRecipeID: num(er.id),
    foodEventID,
    recipeID: er.recipe ? num(er.recipe.id) : null,
    mealType: er.mealType,
    targetTime: er.targetTime,
    servings: er.servings,
    baseServings: er.baseServings,
    scalingFactor: er.scalingFactor,
    notes: er.notes,
    recipe: er.recipe ? toRecipe(er.recipe) : null,
    steps: (er.steps ?? []).map(toEventRecipeStep),
    items: (er.items ?? []).map(toEventRecipeItem),
    allergens: (er.allergens ?? []).map(toAllergenFlag),
    allergyWarnings: (er.allergyWarnings ?? []).map(toAllergyWarning),
  };
}

function toFoodEvent(e: GqlFoodEvent): FoodEvent {
  const foodEventID = num(e.id);
  return {
    foodEventID,
    name: e.name,
    eventDate: e.eventDate,
    slotGranularityMinutes: e.slotGranularityMinutes,
    isActive: e.isActive,
    eventRecipes: (e.recipes ?? []).map((r) => toEventRecipe(foodEventID, r)),
  };
}

function toEventTimeline(t: GqlEventTimeline): EventTimeline {
  return {
    foodEventID: num(t.foodEventId),
    warnings: t.warnings ?? [],
    recipes: (t.recipes ?? []).map((r) => ({
      eventRecipeID: num(r.eventRecipeId),
      name: r.name,
      targetTime: r.targetTime,
      servings: r.servings,
      baseServings: r.baseServings,
      startBy: r.startBy,
      unschedulable: r.unschedulable,
      warnings: r.warnings ?? [],
      steps: (r.steps ?? []).map((s) => ({
        stepNumber: s.stepNumber,
        instruction: s.instruction,
        stepType: s.stepType,
        isPassive: s.isPassive,
        appliance: s.appliance,
        durationMinutes: s.durationMinutes,
        scheduledMinutes: s.scheduledMinutes,
        estimated: s.estimated,
        startTime: s.startTime,
        endTime: s.endTime,
        conflicts: s.conflicts ?? [],
      })),
    })),
  };
}

function usualBrandLabel(
  b: NonNullable<GqlGroceryListItem["usualBrand"]> | null | undefined
): string | null {
  if (!b) return null;
  return brandedName(b.brand?.name, b.name);
}

function toGroceryListItem(listID: number, i: GqlGroceryListItem): GroceryListItem {
  return {
    ...audit(),
    groceryListItemID: num(i.id),
    groceryListID: listID,
    itemID: i.item ? num(i.item.id) : null,
    itemName: i.item?.name ?? null,
    ingredientID: i.ingredient ? num(i.ingredient.id) : null,
    ingredientName: i.ingredient?.name ?? null,
    usualBrandItemID: i.usualBrand ? num(i.usualBrand.id) : null,
    usualBrandName: usualBrandLabel(i.usualBrand),
    manualItemName: i.manualItemName,
    quantityNeeded: i.quantityNeeded,
    unitOfMeasure: i.unitOfMeasure,
    source: i.source,
    isChecked: i.isChecked,
    groceryList: null,
    allergens: (i.allergens ?? []).map(toAllergenFlag),
    allergyWarnings: (i.allergyWarnings ?? []).map(toAllergyWarning),
  };
}

function toStoreAisle(a: GqlStoreAisle): StoreAisle {
  return { aisleID: num(a.id), name: a.name, position: a.position };
}

function toStore(s: GqlStore): Store {
  return {
    storeID: num(s.id),
    name: s.name,
    aisles: (s.aisles ?? []).map(toStoreAisle),
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
    store: g.store ? toStore(g.store) : null,
  };
}

/* ------------------------------------------------------------------ */
/* Shared selection sets                                               */
/* ------------------------------------------------------------------ */

const ALLERGEN_FLAG_FIELDS = `
  kind
  allergen { id name description isActive }
`;

const ALLERGY_WARNING_FIELDS = `
  memberKind entityKind
  member { id displayName firstName lastName }
  allergen { id name }
`;

// Entity allergen flags + household-member conflicts. Empty allergens
// means "no allergen data recorded" — never render it as "known safe".
const ALLERGY_FIELDS = `
  allergens { ${ALLERGEN_FLAG_FIELDS} }
  allergyWarnings { ${ALLERGY_WARNING_FIELDS} }
`;

const BRAND_FIELDS = `
  id name selectionCount personalSelectionCount
`;

const ITEM_FIELDS = `
  id name upc12 upc14 unit status submittedByMe selectionCount personalSelectionCount
  brand { ${BRAND_FIELDS} }
  category { id name description }
  ingredient { id name }
  householdIngredient { id name }
  nutrients { amount nutrient { id name unit } }
  flavors { intensity flavor { id name isActive } }
  ${ALLERGY_FIELDS}
`;

const INGREDIENT_FIELDS = `
  id name defaultUnit isActive
  category { id name description isActive isProtein }
  ${ALLERGY_FIELDS}
`;

const RECIPE_CATEGORY_FIELDS = `
  id name group { id name exclusive displayOrder }
`;

const RECIPE_FIELDS = `
  id name description servings prepTimeMinutes cookTimeMinutes isFavorite selectionCount personalSelectionCount myRating averageRating ratingCount
  items { quantity unit notes isOptional ingredient { id name } item { ${ITEM_FIELDS} } }
  steps { stepNumber instruction durationMinutes stepType isPassive dependsOnStepNumber appliance }
  categories { ${RECIPE_CATEGORY_FIELDS} }
  ${ALLERGY_FIELDS}
`;

const RECIPE_IMPORT_FIELDS = `
  id status sourceFilename ocrText createdAt updatedAt createdBy
  profanityFlag profanityReason errorMessage
  draft { name description servings prepTimeMinutes cookTimeMinutes sourceHint items { quantity unit ingredient section notes isOptional } steps { stepNumber instruction } }
  review { pageId name description servings prepTimeMinutes cookTimeMinutes sourceHint approved items { draftItem { quantity unit ingredient section notes isOptional } itemId itemKind itemName unit unitId confidence status notes approved suggestions { id name kind score } } steps { stepNumber instruction } }
  recipe { ${RECIPE_FIELDS} }
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
    ${ALLERGY_FIELDS}
  }
`;

const EVENT_RECIPE_STEP_FIELDS = `
  id stepNumber instruction durationMinutes stepType isPassive
  dependsOnStepNumber appliance
`;

const EVENT_RECIPE_ITEM_FIELDS = `
  id quantity baseQuantity unit section displayOrder notes isOptional
  item { id name }
  ingredient { id name }
`;

const EVENT_RECIPE_FIELDS = `
  id mealType targetTime servings baseServings scalingFactor notes
  recipe { ${RECIPE_FIELDS} }
  steps { ${EVENT_RECIPE_STEP_FIELDS} }
  items { ${EVENT_RECIPE_ITEM_FIELDS} }
  ${ALLERGY_FIELDS}
`;

const FOOD_EVENT_FIELDS = `
  id name eventDate slotGranularityMinutes isActive
  recipes { ${EVENT_RECIPE_FIELDS} }
`;

const GROCERY_LIST_FIELDS = `
  id generatedAt
  store { id name }
  items {
    id manualItemName quantityNeeded unitOfMeasure source isChecked
          ingredient { id name }
          usualBrand { id name brand { id name } }
    item { ${ITEM_FIELDS} }
    ${ALLERGY_FIELDS}
  }
`;

const STORE_FIELDS = `
  id name
  aisles { id name position }
`;

/* ------------------------------------------------------------------ */
/* Helpers to page through the API for client-side filtering           */
/* ------------------------------------------------------------------ */

// Catalog order is engagement-ranked by the server (favorites → personal →
// household → global → name); preserve it — never re-sort fetched pages.

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
  return out;
}

async function fetchAllUserItems(search?: string): Promise<GqlUserItem[]> {
  const pageSize = 200;
  let page = 1;
  const term = (search ?? "").trim();
  const out: GqlUserItem[] = [];
  for (;;) {
    const data = await request<{ userItems: GqlUserItemPage }>(
      `query ($page: Int, $pageSize: Int, $search: String) {
        userItems(page: $page, pageSize: $pageSize, search: $search) {
          items { id currentQty minQty purchaseAt expiresAt notes isFavorite item { ${ITEM_FIELDS} } }
          pageInfo { pageNumber pageSize totalCount }
        }
      }`,
      { page, pageSize, search: term || null }
    );
    out.push(...data.userItems.items);
    if (out.length >= data.userItems.pageInfo.totalCount || data.userItems.items.length === 0) break;
    page += 1;
  }
  return out;
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
      `query { me { id email displayName firstName lastName backupEmail birthdate role isActive isProtected lastLoginAt isSearchable household { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } } }`
    );
    return toUser(data.me);
  },

  // Profile (self-service)
  updateMyProfile: async (input: {
    firstName?: string;
    lastName?: string;
    backupEmail?: string;
    birthdate?: string;
    isSearchable?: boolean;
  }): Promise<User> => {
    const data = await request<{ updateMyProfile: GqlUser }>(
      `mutation ($input: UpdateProfileInput!) { updateMyProfile(input: $input) { id email displayName firstName lastName backupEmail birthdate role isActive isProtected lastLoginAt isSearchable } }`,
      { input }
    );
    return toUser(data.updateMyProfile);
  },

  // Household
  getMyHousehold: async (): Promise<Household | null> => {
    const data = await request<{ myHousehold: GqlHousehold | null }>(
      `query { myHousehold { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } }`
    );
    return data.myHousehold ? toHousehold(data.myHousehold) : null;
  },

  getHouseholdInvites: async (): Promise<HouseholdInvite[]> => {
    const data = await request<{ householdInvites: GqlHouseholdInvite[] }>(
      `query { householdInvites { id status createdAt fromUser { id displayName firstName lastName } toUser { id displayName firstName lastName } } }`
    );
    return (data.householdInvites ?? []).map(toHouseholdInvite);
  },

  searchHouseholdUsers: async (
    term: string,
    limit = 20
  ): Promise<HouseholdUser[]> => {
    const data = await request<{ searchHouseholdUsers: GqlHouseholdUser[] }>(
      `query ($term: String!, $limit: Int) { searchHouseholdUsers(term: $term, limit: $limit) { id displayName firstName lastName } }`,
      { term, limit }
    );
    return (data.searchHouseholdUsers ?? []).map(toHouseholdUser);
  },

  inviteHouseholdMember: async (userID: number): Promise<HouseholdInvite> => {
    const data = await request<{ inviteHouseholdMember: GqlHouseholdInvite }>(
      `mutation ($userId: ID!) { inviteHouseholdMember(userId: $userId) { id status createdAt fromUser { id displayName firstName lastName } toUser { id displayName firstName lastName } } }`,
      { userId: String(userID) }
    );
    return toHouseholdInvite(data.inviteHouseholdMember);
  },

  acceptHouseholdInvite: async (inviteID: number): Promise<Household> => {
    const data = await request<{ acceptHouseholdInvite: GqlHousehold }>(
      `mutation ($inviteId: ID!) { acceptHouseholdInvite(inviteId: $inviteId) { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } }`,
      { inviteId: String(inviteID) }
    );
    return toHousehold(data.acceptHouseholdInvite);
  },

  declineHouseholdInvite: async (inviteID: number): Promise<HouseholdInvite> => {
    const data = await request<{ declineHouseholdInvite: GqlHouseholdInvite }>(
      `mutation ($inviteId: ID!) { declineHouseholdInvite(inviteId: $inviteId) { id status createdAt fromUser { id displayName firstName lastName } toUser { id displayName firstName lastName } } }`,
      { inviteId: String(inviteID) }
    );
    return toHouseholdInvite(data.declineHouseholdInvite);
  },

  cancelHouseholdInvite: async (inviteID: number): Promise<HouseholdInvite> => {
    const data = await request<{ cancelHouseholdInvite: GqlHouseholdInvite }>(
      `mutation ($inviteId: ID!) { cancelHouseholdInvite(inviteId: $inviteId) { id status createdAt fromUser { id displayName firstName lastName } toUser { id displayName firstName lastName } } }`,
      { inviteId: String(inviteID) }
    );
    return toHouseholdInvite(data.cancelHouseholdInvite);
  },

  leaveHousehold: async (): Promise<boolean> => {
    const data = await request<{ leaveHousehold: boolean }>(
      `mutation { leaveHousehold }`
    );
    return data.leaveHousehold;
  },

  renameHousehold: async (name: string): Promise<Household> => {
    const data = await request<{ renameHousehold: GqlHousehold }>(
      `mutation ($name: String!) { renameHousehold(name: $name) { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } }`,
      { name }
    );
    return toHousehold(data.renameHousehold);
  },

  setHouseholdRole: async (
    userID: number,
    role: HouseholdRole
  ): Promise<Household> => {
    const data = await request<{ setHouseholdRole: GqlHousehold }>(
      `mutation ($userId: ID!, $role: HouseholdRole!) { setHouseholdRole(userId: $userId, role: $role) { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } }`,
      { userId: String(userID), role }
    );
    return toHousehold(data.setHouseholdRole);
  },

  removeHouseholdMember: async (userID: number): Promise<Household> => {
    const data = await request<{ removeHouseholdMember: GqlHousehold }>(
      `mutation ($userId: ID!) { removeHouseholdMember(userId: $userId) { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } }`,
      { userId: String(userID) }
    );
    return toHousehold(data.removeHouseholdMember);
  },

  transferHouseholdOwnership: async (
    userID: number
  ): Promise<Household> => {
    const data = await request<{ transferHouseholdOwnership: GqlHousehold }>(
      `mutation ($userId: ID!) { transferHouseholdOwnership(userId: $userId) { id name myRole members { user { id displayName firstName lastName } role isMe } createdAt } }`,
      { userId: String(userID) }
    );
    return toHousehold(data.transferHouseholdOwnership);
  },

  getMyNotifications: async (
    limit = 20
  ): Promise<HouseholdNotification[]> => {
    const data = await request<{ myNotifications: GqlHouseholdNotification[] }>(
      `query ($limit: Int) { myNotifications(limit: $limit) { id kind foodEventId title body recipeId itemId createdAt actor { id displayName firstName lastName } } }`,
      { limit }
    );
    return (data.myNotifications ?? []).map(toHouseholdNotification);
  },

  getUnreadNotificationCount: async (): Promise<number> => {
    const data = await request<{ unreadNotificationCount: number }>(
      `query { unreadNotificationCount }`
    );
    return data.unreadNotificationCount ?? 0;
  },

  markAllNotificationsRead: async (): Promise<boolean> => {
    const data = await request<{ markAllNotificationsRead: boolean }>(
      `mutation { markAllNotificationsRead }`
    );
    return data.markAllNotificationsRead;
  },

  getMyNotificationPreferences: async (): Promise<
    NotificationCategoryPreference[]
  > => {
    const data = await request<{
      myNotificationPreferences: GqlNotificationCategoryPreference[];
    }>(
      `query { myNotificationPreferences { category label enabled mutedUntil } }`
    );
    return data.myNotificationPreferences ?? [];
  },

  setNotificationCategoryEnabled: async (
    category: string,
    enabled: boolean
  ): Promise<boolean> => {
    const data = await request<{ setNotificationCategoryEnabled: boolean }>(
      `mutation ($category: String!, $enabled: Boolean!) { setNotificationCategoryEnabled(category: $category, enabled: $enabled) }`,
      { category, enabled }
    );
    return data.setNotificationCategoryEnabled;
  },

  muteNotifications: async (
    category: string | null,
    until: string
  ): Promise<boolean> => {
    const data = await request<{ muteNotifications: boolean }>(
      `mutation ($category: String, $until: Time!) { muteNotifications(category: $category, until: $until) }`,
      { category, until }
    );
    return data.muteNotifications;
  },

  clearNotificationMute: async (category: string | null): Promise<boolean> => {
    const data = await request<{ clearNotificationMute: boolean }>(
      `mutation ($category: String) { clearNotificationMute(category: $category) }`,
      { category }
    );
    return data.clearNotificationMute;
  },

  addItemToCurrentGroceryList: async (
    itemID: number
  ): Promise<GroceryListItem | null> => {
    const data = await request<{
      addItemToCurrentGroceryList: GqlGroceryListItem;
    }>(
      `mutation ($itemId: ID!) { addItemToCurrentGroceryList(itemId: $itemId) { id item { id name } ingredient { id name } usualBrand { id name brand { id name } } manualItemName quantityNeeded unitOfMeasure source isChecked } }`,
      { itemId: String(itemID) }
    );
    // The returned row's list id isn't on the GraphQL type; the caller only
    // needs the created row to confirm the add and name the item.
    return data.addItemToCurrentGroceryList
      ? toGroceryListItem(0, data.addItemToCurrentGroceryList)
      : null;
  },

  // Admin user management
  getUsers: async (
    page: number,
    pageSize: number
  ): Promise<PagedResult<User>> => {
    const data = await request<{ users: GqlUserPage }>(
      `query ($page: Int, $pageSize: Int) { users(page: $page, pageSize: $pageSize) { items { id email displayName firstName lastName backupEmail role isActive isProtected lastLoginAt } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    return {
      items: data.users.items.map(toUser),
      pageNumber: data.users.pageInfo.pageNumber,
      pageSize: data.users.pageInfo.pageSize,
      totalCount: data.users.pageInfo.totalCount,
      totalPages: Math.ceil(data.users.pageInfo.totalCount / data.users.pageInfo.pageSize) || 1,
    };
  },

  setUserRole: async (userId: number, role: "member" | "admin"): Promise<User> => {
    const data = await request<{ setUserRole: GqlUser }>(
      `mutation ($userId: ID!, $role: Role!) { setUserRole(userId: $userId, role: $role) { id email displayName firstName lastName backupEmail role isActive isProtected lastLoginAt } }`,
      { userId: String(userId), role }
    );
    return toUser(data.setUserRole);
  },

  setUserActive: async (userId: number, isActive: boolean): Promise<User> => {
    const data = await request<{ setUserActive: GqlUser }>(
      `mutation ($userId: ID!, $isActive: Boolean!) { setUserActive(userId: $userId, isActive: $isActive) { id email displayName firstName lastName backupEmail role isActive isProtected lastLoginAt } }`,
      { userId: String(userId), isActive }
    );
    return toUser(data.setUserActive);
  },

  // Items
  getItemsByIds: async (ids: number[]): Promise<Item[]> =>
    (
      await Promise.all(
        [...new Set(ids)].map((id) => api.getItem(id).catch(() => null))
      )
    ).filter((i): i is Item => i !== null),

  getItemsPaged: async (
    pageNumber: number,
    pageSize: number,
    search?: string,
    brandId?: number,
    inStock?: boolean,
    isFavorite?: boolean
  ): Promise<PagedResult<Item>> => {
    const term = (search ?? "").trim();
    if (inStock || isFavorite) {
      // Pantry-scoped filters drive from userItems — ranked server-side, and a
      // household pantry is small enough to filter and page client-side.
      const userItems = await fetchAllUserItems(term);
      let rows = userItems.map((ui) => ({ ui, item: toItem(ui.item, ui) }));
      if (brandId) rows = rows.filter((r) => num(r.ui.item.brand?.id) === brandId);
      if (inStock) rows = rows.filter((r) => r.item.currentQuantity > 0);
      if (isFavorite) rows = rows.filter((r) => r.item.isFavorite);
      return pagedSlice(rows.map((r) => r.item), pageNumber, pageSize);
    }
    const [data, userItems] = await Promise.all([
      request<{ items: GqlItemPage }>(
        `query ($page: Int, $pageSize: Int, $search: String, $brandId: ID) {
          items(page: $page, pageSize: $pageSize, search: $search, brandId: $brandId) {
            items { ${ITEM_FIELDS} }
            pageInfo { pageNumber pageSize totalCount }
          }
        }`,
        { page: pageNumber, pageSize, search: term || null, brandId: brandId ? String(brandId) : null }
      ),
      fetchAllUserItems(),
    ]);
    const prefs = new Map(userItems.map((ui) => [num(ui.item.id), ui]));
    return toPaged(
      data.items.items.map((i) => toItem(i, prefs.get(num(i.id)))),
      data.items.pageInfo
    );
  },

  searchItems: async (search: string, brandId?: number, limit: number = 50): Promise<Item[]> => {
    const data = await request<{ items: GqlItemPage }>(
      `query ($search: String, $brandId: ID, $limit: Int) {
        items(page: 1, pageSize: $limit, search: $search, brandId: $brandId) {
          items { ${ITEM_FIELDS} }
        }
      }`,
      { search: search.trim() || null, brandId: brandId ? String(brandId) : null, limit }
    );
    return (data.items.items ?? []).map((i) => toItem(i));
  },

  getUnits: async (): Promise<Unit[]> => {
    const data = await request<{ units: GqlUnit[] }>(
      `query { units { id name abbreviation kind isActive } }`
    );
    return (data.units ?? []).map(toUnit);
  },

  getBrands: async (search?: string): Promise<Brand[]> => {
    const s = (search ?? "").trim();
    if (s === "") return [];
    const data = await request<{ searchBrands: GqlBrand[] }>(
      `query ($term: String!, $limit: Int) { searchBrands(term: $term, limit: $limit) { ${BRAND_FIELDS} } }`,
      { term: s, limit: 50 }
    );
    return (data.searchBrands ?? []).map(toBrand);
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
      `mutation ($entityType: EntityType!, $entityId: ID!) { recordSelection(entityType: $entityType, entityId: $entityId) }`,
      { entityType, entityId: String(entityId) }
    );
  },

  recordSearch: async (entityType: string, term: string): Promise<void> => {
    await request<{ recordSearch: boolean }>(
      `mutation ($entityType: EntityType!, $term: String!) { recordSearch(entityType: $entityType, term: $term) }`,
      { entityType, term }
    );
  },

  recordView: async (entityType: string, entityId: number): Promise<void> => {
    await request<{ recordView: boolean }>(
      `mutation ($entityType: EntityType!, $entityId: ID!) { recordView(entityType: $entityType, entityId: $entityId) }`,
      { entityType, entityId: String(entityId) }
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

  getItemByUpc: async (code: string): Promise<Item | null> => {
    const data = await request<{ itemByUpc: GqlItem | null }>(
      `query ($code: String!) { itemByUpc(code: $code) { ${ITEM_FIELDS} } }`,
      { code }
    );
    return data.itemByUpc ? toItem(data.itemByUpc) : null;
  },

  getPendingItems: async (
    page: number,
    pageSize: number
  ): Promise<PagedResult<Item>> => {
    const data = await request<{ pendingItems: GqlItemPage }>(
      `query ($page: Int, $pageSize: Int) { pendingItems(page: $page, pageSize: $pageSize) { items { ${ITEM_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    return {
      items: (data.pendingItems.items ?? []).map((i) => toItem(i)),
      pageNumber: data.pendingItems.pageInfo.pageNumber,
      pageSize: data.pendingItems.pageInfo.pageSize,
      totalCount: data.pendingItems.pageInfo.totalCount,
      totalPages: Math.max(
        1,
        Math.ceil(
          data.pendingItems.pageInfo.totalCount / data.pendingItems.pageInfo.pageSize
        )
      ),
    };
  },

  approveItem: async (id: number): Promise<Item> => {
    const data = await request<{ approveItem: GqlItem }>(
      `mutation ($id: ID!) { approveItem(id: $id) { ${ITEM_FIELDS} } }`,
      { id: String(id) }
    );
    return toItem(data.approveItem);
  },

  rejectItem: async (id: number): Promise<Item> => {
    const data = await request<{ rejectItem: GqlItem }>(
      `mutation ($id: ID!) { rejectItem(id: $id) { ${ITEM_FIELDS} } }`,
      { id: String(id) }
    );
    return toItem(data.rejectItem);
  },

  createItem: async (
    item: Omit<Item, keyof AuditableEntity | "status" | "submittedByMe">
  ): Promise<Item> => {
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

  // Generic ingredients — the semantic identity recipes and grocery needs
  // key on; branded items are optional purchasing representatives.

  getIngredients: async (page: number, pageSize: number, search?: string): Promise<PagedResult<Ingredient>> => {
    const data = await request<{ ingredients: GqlIngredientPage }>(
      `query ($page: Int, $pageSize: Int, $search: String) { ingredients(page: $page, pageSize: $pageSize, search: $search) { items { ${INGREDIENT_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize, search: search || null }
    );
    return toPaged(data.ingredients.items.map(toIngredient), data.ingredients.pageInfo);
  },

  searchIngredients: async (search: string, pageSize = 20): Promise<Ingredient[]> => {
    const page = await api.getIngredients(1, pageSize, search);
    return page.items;
  },

  createIngredient: async (input: { name: string; categoryId?: number | null; defaultUnit?: string | null }): Promise<Ingredient> => {
    const data = await request<{ createIngredient: GqlIngredient }>(
      `mutation ($input: CreateIngredientInput!) { createIngredient(input: $input) { ${INGREDIENT_FIELDS} } }`,
      {
        input: {
          name: input.name,
          categoryId: input.categoryId != null ? String(input.categoryId) : null,
          defaultUnit: input.defaultUnit ?? null,
        },
      }
    );
    return toIngredient(data.createIngredient);
  },

  updateIngredient: async (
    id: number,
    input: { name?: string; categoryId?: number | null; defaultUnit?: string | null; isActive?: boolean }
  ): Promise<Ingredient> => {
    const vars: Record<string, unknown> = {};
    if (input.name !== undefined) vars.name = input.name;
    if (input.categoryId !== undefined) vars.categoryId = input.categoryId != null ? String(input.categoryId) : null;
    if (input.defaultUnit !== undefined) vars.defaultUnit = input.defaultUnit;
    if (input.isActive !== undefined) vars.isActive = input.isActive;
    const data = await request<{ updateIngredient: GqlIngredient }>(
      `mutation ($id: ID!, $input: UpdateIngredientInput!) { updateIngredient(id: $id, input: $input) { ${INGREDIENT_FIELDS} } }`,
      { id: String(id), input: vars }
    );
    return toIngredient(data.updateIngredient);
  },

  deleteIngredient: async (id: number): Promise<void> => {
    await request<{ deleteIngredient: boolean }>(
      `mutation ($id: ID!) { deleteIngredient(id: $id) }`,
      { id: String(id) }
    );
  },

  // Members can free-create here — the backend returns the existing row
  // for a normalized-name match, so callers can't mint duplicates.
  getOrCreateIngredient: async (input: { name: string; categoryId?: number | null; defaultUnit?: string | null }): Promise<Ingredient> => {
    const data = await request<{ getOrCreateIngredient: GqlIngredient }>(
      `mutation ($input: CreateIngredientInput!) { getOrCreateIngredient(input: $input) { ${INGREDIENT_FIELDS} } }`,
      {
        input: {
          name: input.name,
          categoryId: input.categoryId != null ? String(input.categoryId) : null,
          defaultUnit: input.defaultUnit ?? null,
        },
      }
    );
    return toIngredient(data.getOrCreateIngredient);
  },

  // Admin dedupe: repoints every reference from fromId onto intoId.
  mergeIngredient: async (fromId: number, intoId: number): Promise<void> => {
    await request<{ mergeIngredient: boolean }>(
      `mutation ($fromId: ID!, $intoId: ID!) { mergeIngredient(fromId: $fromId, intoId: $intoId) }`,
      { fromId: String(fromId), intoId: String(intoId) }
    );
  },

  // Admin: catalog-level item -> ingredient link (null clears).
  setItemIngredient: async (itemId: number, ingredientId: number | null): Promise<void> => {
    await request<{ setItemIngredient: boolean }>(
      `mutation ($itemId: ID!, $ingredientId: ID) { setItemIngredient(itemId: $itemId, ingredientId: $ingredientId) }`,
      { itemId: String(itemId), ingredientId: ingredientId != null ? String(ingredientId) : null }
    );
  },

  // Household remap: "this item is a different ingredient for us".
  setHouseholdItemIngredient: async (itemId: number, ingredientId: number | null): Promise<void> => {
    await request<{ setHouseholdItemIngredient: boolean }>(
      `mutation ($itemId: ID!, $ingredientId: ID) { setHouseholdItemIngredient(itemId: $itemId, ingredientId: $ingredientId) }`,
      { itemId: String(itemId), ingredientId: ingredientId != null ? String(ingredientId) : null }
    );
  },

  /* ------------------------------ allergens ------------------------------ */

  getAllergens: async (): Promise<Allergen[]> => {
    const data = await request<{ allergens: GqlAllergen[] }>(
      `query { allergens { id name description isActive } }`
    );
    return (data.allergens ?? []).map(toAllergen);
  },

  getMyAllergies: async (): Promise<MemberAllergen[]> => {
    const data = await request<{ myAllergies: GqlMemberAllergen[] }>(
      `query { myAllergies { kind allergen { id name description isActive } } }`
    );
    return (data.myAllergies ?? []).map(toMemberAllergen);
  },

  // Set or clear one of the caller's records. kind is "allergy" (avoid
  // always) or "dietary" (preference); on=false removes the record.
  setMyAllergy: async (allergenId: number, kind: "allergy" | "dietary", on: boolean): Promise<void> => {
    await request<{ setMyAllergy: boolean }>(
      `mutation ($allergenId: ID!, $kind: MemberAllergyKind!, $on: Boolean!) { setMyAllergy(allergenId: $allergenId, kind: $kind, on: $on) }`,
      { allergenId: String(allergenId), kind, on }
    );
  },

  createAllergen: async (input: { name: string; description?: string | null }): Promise<Allergen> => {
    const data = await request<{ createAllergen: GqlAllergen }>(
      `mutation ($input: CreateAllergenInput!) { createAllergen(input: $input) { id name description isActive } }`,
      { input: { name: input.name, description: input.description ?? null } }
    );
    return toAllergen(data.createAllergen);
  },

  updateAllergen: async (
    id: number,
    input: { name?: string; description?: string | null; isActive?: boolean }
  ): Promise<Allergen> => {
    const vars: Record<string, unknown> = {};
    if (input.name !== undefined) vars.name = input.name;
    if (input.description !== undefined) vars.description = input.description;
    if (input.isActive !== undefined) vars.isActive = input.isActive;
    const data = await request<{ updateAllergen: GqlAllergen }>(
      `mutation ($id: ID!, $input: UpdateAllergenInput!) { updateAllergen(id: $id, input: $input) { id name description isActive } }`,
      { id: String(id), input: vars }
    );
    return toAllergen(data.updateAllergen);
  },

  // Admin curation: set an ingredient/item flag ("contains" or
  // "may_contain"); null kind clears the flag.
  setIngredientAllergen: async (ingredientId: number, allergenId: number, kind: "contains" | "may_contain" | null): Promise<void> => {
    await request<{ setIngredientAllergen: boolean }>(
      `mutation ($ingredientId: ID!, $allergenId: ID!, $kind: AllergenFlagKind) { setIngredientAllergen(ingredientId: $ingredientId, allergenId: $allergenId, kind: $kind) }`,
      { ingredientId: String(ingredientId), allergenId: String(allergenId), kind }
    );
  },

  setItemAllergen: async (itemId: number, allergenId: number, kind: "contains" | "may_contain" | null): Promise<void> => {
    await request<{ setItemAllergen: boolean }>(
      `mutation ($itemId: ID!, $allergenId: ID!, $kind: AllergenFlagKind) { setItemAllergen(itemId: $itemId, allergenId: $allergenId, kind: $kind) }`,
      { itemId: String(itemId), allergenId: String(allergenId), kind }
    );
  },

  /* -------------------- allergen flag review queue -------------------- */

  getAllergenSuggestions: async (status?: "pending" | "accepted" | "dismissed"): Promise<AllergenSuggestion[]> => {
    const data = await request<{ allergenSuggestions: GqlAllergenSuggestion[] }>(
      `query ($status: String) { allergenSuggestions(status: $status) { ${ALLERGEN_SUGGESTION_FIELDS} } }`,
      { status: status ?? null }
    );
    return (data.allergenSuggestions ?? []).map(toAllergenSuggestion);
  },

  // Admin: run the AI suggester over one recipe and enqueue the validated
  // proposals. Returns the rows actually created (dupes drop server-side).
  suggestRecipeAllergens: async (recipeId: number, maxSuggestions?: number): Promise<AllergenSuggestion[]> => {
    const data = await request<{ suggestRecipeAllergens: GqlAllergenSuggestion[] }>(
      `mutation ($recipeId: ID!, $max: Int) { suggestRecipeAllergens(recipeId: $recipeId, maxSuggestions: $max) { ${ALLERGEN_SUGGESTION_FIELDS} } }`,
      { recipeId: String(recipeId), max: maxSuggestions ?? null }
    );
    return (data.suggestRecipeAllergens ?? []).map(toAllergenSuggestion);
  },

  acceptAllergenSuggestion: async (id: number): Promise<AllergenSuggestion> => {
    const data = await request<{ acceptAllergenSuggestion: GqlAllergenSuggestion }>(
      `mutation ($id: ID!) { acceptAllergenSuggestion(id: $id) { ${ALLERGEN_SUGGESTION_FIELDS} } }`,
      { id: String(id) }
    );
    return toAllergenSuggestion(data.acceptAllergenSuggestion);
  },

  dismissAllergenSuggestion: async (id: number): Promise<AllergenSuggestion> => {
    const data = await request<{ dismissAllergenSuggestion: GqlAllergenSuggestion }>(
      `mutation ($id: ID!) { dismissAllergenSuggestion(id: $id) { ${ALLERGEN_SUGGESTION_FIELDS} } }`,
      { id: String(id) }
    );
    return toAllergenSuggestion(data.dismissAllergenSuggestion);
  },

  adjustItemQuantity: async (
    id: number,
    quantity: number,
    purchaseDate?: string,
    expiryDate?: string,
    minQuantity?: number
  ): Promise<void> => {
    await request<{ adjustUserItem: unknown }>(
      `mutation ($itemId: ID!, $quantity: Float!, $purchaseAt: Time, $expiresAt: Time, $minQty: Float) {
        adjustUserItem(itemId: $itemId, quantity: $quantity, purchaseAt: $purchaseAt, expiresAt: $expiresAt, minQty: $minQty) { id }
      }`,
      {
        itemId: String(id),
        quantity,
        purchaseAt: purchaseDate ?? null,
        expiresAt: expiryDate ?? null,
        minQty: minQuantity ?? null,
      }
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

  getBrandsPaged: async (page: number, pageSize: number): Promise<PagedResult<Brand>> => {
    const data = await request<{
      brands: { items: GqlBrand[]; pageInfo: GqlPageInfo };
    }>(
      `query ($page: Int!, $pageSize: Int!) { brands(page: $page, pageSize: $pageSize) { items { ${BRAND_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    return toPaged(data.brands.items.map(toBrand), data.brands.pageInfo);
  },

  getBrandList: async (): Promise<Brand[]> => {
    const out: Brand[] = [];
    const pageSize = 100;
    for (let page = 1; ; page++) {
      const result = await api.getBrandsPaged(page, pageSize);
      out.push(...result.items);
      if (result.items.length < pageSize) break;
    }
    return out;
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
      `query { categories { id name description isActive isProtein } }`
    );
    return data.categories.map(toCategory);
  },

  getActiveCategories: async (): Promise<Category[]> =>
    (await api.getCategories()).filter((c) => c.isActive),

  createCategory: async (category: Omit<Category, keyof AuditableEntity>): Promise<Category> => {
    const data = await request<{ createCategory: GqlCategory }>(
      `mutation ($input: CreateCategoryInput!) { createCategory(input: $input) { id name description isActive isProtein } }`,
      { input: { name: category.categoryName, description: category.description, isProtein: category.isProtein } }
    );
    return toCategory(data.createCategory);
  },

  updateCategory: async (id: number, category: Partial<Category>): Promise<Category> => {
    const input: Record<string, unknown> = {};
    if (category.categoryName !== undefined) input.name = category.categoryName;
    if (category.description !== undefined) input.description = category.description;
    if (category.isActive !== undefined) input.isActive = category.isActive;
    if (category.isProtein !== undefined) input.isProtein = category.isProtein;
    const data = await request<{ updateCategory: GqlCategory }>(
      `mutation ($id: ID!, $input: UpdateCategoryInput!) { updateCategory(id: $id, input: $input) { id name description isActive isProtein } }`,
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

  // Flavors hang off items, so paging is over items — a page's row count
  // varies with how many flavors each item carries.
  getFoodFlavorsPaged: async (page: number, pageSize: number): Promise<PagedResult<FoodFlavor>> => {
    const data = await request<{ items: GqlItemPage }>(
      `query ($page: Int, $pageSize: Int) { items(page: $page, pageSize: $pageSize) { items { ${ITEM_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    const rows = data.items.items.flatMap((i) =>
      (i.flavors ?? []).map((f) => ({
        ...toFoodFlavor(num(i.id), f),
        item: toItem(i),
      }))
    );
    return toPaged(rows, data.items.pageInfo);
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

  // Nutrients hang off items — paging is over items, so a page's row
  // count varies with how many nutrients each item carries.
  getFoodNutrientsPaged: async (page: number, pageSize: number): Promise<PagedResult<FoodNutrient>> => {
    const data = await request<{ items: GqlItemPage }>(
      `query ($page: Int, $pageSize: Int) { items(page: $page, pageSize: $pageSize) { items { ${ITEM_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page, pageSize }
    );
    const rows = data.items.items.flatMap((i) =>
      (i.nutrients ?? []).map((n) => ({
        ...toFoodNutrient(num(i.id), n),
        item: toItem(i),
      }))
    );
    return toPaged(rows, data.items.pageInfo);
  },

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
    const data = await request<{ bottles: GqlBottlePage }>(
      `query ($search: String) { bottles(page: 1, pageSize: 50, search: $search) { items { ${BOTTLE_FIELDS} } } }`,
      { search: searchTerm.trim() || null }
    );
    return (data.bottles.items ?? []).map((b) => toBottle(b));
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
  getRecipes: async (mealType?: string): Promise<Recipe[]> => {
    const pageSize = 200;
    let page = 1;
    const out: Recipe[] = [];
    for (;;) {
      const data = await request<{ recipes: GqlRecipePage }>(
        `query ($page: Int, $pageSize: Int, $mealType: String) { recipes(page: $page, pageSize: $pageSize, mealType: $mealType) { items { ${RECIPE_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
        { page, pageSize, mealType: mealType ?? null }
      );
      out.push(...data.recipes.items.map(toRecipe));
      if (
        page * pageSize >= data.recipes.pageInfo.totalCount ||
        data.recipes.items.length === 0
      )
        break;
      page += 1;
    }
    return out;
  },

  getRecipesPaged: async (
    pageNumber: number,
    pageSize: number,
    search?: string,
    isFavorite?: boolean,
    categoryIds?: number[],
    searchMode?: "keyword" | "semantic"
  ): Promise<PagedResult<Recipe>> => {
    const data = await request<{ recipes: GqlRecipePage }>(
      `query ($page: Int, $pageSize: Int, $search: String, $categoryIds: [ID!], $isFavorite: Boolean, $searchMode: RecipeSearchMode) {
        recipes(page: $page, pageSize: $pageSize, search: $search, categoryIds: $categoryIds, isFavorite: $isFavorite, searchMode: $searchMode) {
          items { ${RECIPE_FIELDS} } pageInfo { pageNumber pageSize totalCount }
        }
      }`,
      {
        page: pageNumber,
        pageSize,
        search: search?.trim() || null,
        categoryIds: categoryIds?.length ? categoryIds.map(String) : null,
        isFavorite: isFavorite ?? null,
        searchMode: searchMode ?? "keyword",
      }
    );
    const items = data.recipes.items.map(toRecipe);
    return toPaged(items, data.recipes.pageInfo);
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

  // ---------- recipe categories ----------

  getRecipeCategoryGroups: async (): Promise<RecipeCategoryGroup[]> => {
    const data = await request<{ recipeCategoryGroups: GqlRecipeCategoryGroup[] }>(
      `query { recipeCategoryGroups { id name exclusive displayOrder categories { ${RECIPE_CATEGORY_FIELDS} } } }`
    );
    return data.recipeCategoryGroups.map(toRecipeCategoryGroup);
  },

  setRecipeCategories: async (recipeId: number, categoryIds: number[]): Promise<Recipe> => {
    const data = await request<{ setRecipeCategories: GqlRecipe }>(
      `mutation ($recipeId: ID!, $categoryIds: [ID!]!) { setRecipeCategories(recipeId: $recipeId, categoryIds: $categoryIds) { ${RECIPE_FIELDS} } }`,
      { recipeId: String(recipeId), categoryIds: categoryIds.map(String) }
    );
    return toRecipe(data.setRecipeCategories);
  },

  createRecipeCategoryGroup: async (group: { name: string; exclusive?: boolean; displayOrder?: number }): Promise<RecipeCategoryGroup> => {
    const data = await request<{ createRecipeCategoryGroup: GqlRecipeCategoryGroup }>(
      `mutation ($input: CreateRecipeCategoryGroupInput!) { createRecipeCategoryGroup(input: $input) { id name exclusive displayOrder } }`,
      { input: group }
    );
    return toRecipeCategoryGroup(data.createRecipeCategoryGroup);
  },

  updateRecipeCategoryGroup: async (id: number, group: { name?: string; exclusive?: boolean; displayOrder?: number }): Promise<RecipeCategoryGroup> => {
    const data = await request<{ updateRecipeCategoryGroup: GqlRecipeCategoryGroup }>(
      `mutation ($id: ID!, $input: UpdateRecipeCategoryGroupInput!) { updateRecipeCategoryGroup(id: $id, input: $input) { id name exclusive displayOrder } }`,
      { id: String(id), input: group }
    );
    return toRecipeCategoryGroup(data.updateRecipeCategoryGroup);
  },

  deleteRecipeCategoryGroup: async (id: number): Promise<void> => {
    await request<{ deleteRecipeCategoryGroup: boolean }>(
      `mutation ($id: ID!) { deleteRecipeCategoryGroup(id: $id) }`,
      { id: String(id) }
    );
  },

  createRecipeCategory: async (groupId: number, name: string): Promise<RecipeCategory> => {
    const data = await request<{ createRecipeCategory: GqlRecipeCategory }>(
      `mutation ($input: CreateRecipeCategoryInput!) { createRecipeCategory(input: $input) { ${RECIPE_CATEGORY_FIELDS} } }`,
      { input: { groupId: String(groupId), name } }
    );
    return toRecipeCategory(data.createRecipeCategory);
  },

  updateRecipeCategory: async (id: number, name: string): Promise<RecipeCategory> => {
    const data = await request<{ updateRecipeCategory: GqlRecipeCategory }>(
      `mutation ($id: ID!, $input: UpdateRecipeCategoryInput!) { updateRecipeCategory(id: $id, input: $input) { ${RECIPE_CATEGORY_FIELDS} } }`,
      { id: String(id), input: { name } }
    );
    return toRecipeCategory(data.updateRecipeCategory);
  },

  deleteRecipeCategory: async (id: number): Promise<void> => {
    await request<{ deleteRecipeCategory: boolean }>(
      `mutation ($id: ID!) { deleteRecipeCategory(id: $id) }`,
      { id: String(id) }
    );
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

  // Admin-only: upload a recipe scan to the import inbox and enqueue
  // server-side OCR processing. Returns the created import job.
  submitRecipeScan: async (fileBase64: string): Promise<RecipeImport> => {
    const data = await request<{ submitRecipeScan: GqlRecipeImport }>(
      `mutation ($fileBase64: String!) { submitRecipeScan(fileBase64: $fileBase64) { ${RECIPE_IMPORT_FIELDS} } }`,
      { fileBase64 }
    );
    return toRecipeImport(data.submitRecipeScan);
  },

  getPendingRecipeImports: async (pageNumber = 1, pageSize = 25): Promise<PagedResult<RecipeImport>> => {
    const data = await request<{ pendingRecipeImports: GqlRecipeImportPage }>(
      `query ($page: Int, $pageSize: Int) { pendingRecipeImports(page: $page, pageSize: $pageSize) { items { ${RECIPE_IMPORT_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page: pageNumber, pageSize }
    );
    return toPaged(data.pendingRecipeImports.items.map(toRecipeImport), data.pendingRecipeImports.pageInfo);
  },

  getRecipeImport: async (id: number): Promise<RecipeImport> => {
    const data = await request<{ recipeImport: GqlRecipeImport | null }>(
      `query ($id: ID!) { recipeImport(id: $id) { ${RECIPE_IMPORT_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.recipeImport) throw new ApiError(404, `Recipe import ${id} not found`);
    return toRecipeImport(data.recipeImport);
  },

  updateRecipeImport: async (id: number, review: RecipeImportReview): Promise<RecipeImport> => {
    const data = await request<{ updateRecipeImport: GqlRecipeImport }>(
      `mutation ($id: ID!, $input: RecipeImportReviewInput!) { updateRecipeImport(id: $id, input: $input) { ${RECIPE_IMPORT_FIELDS} } }`,
      { id: String(id), input: toRecipeImportReviewInput(review) }
    );
    return toRecipeImport(data.updateRecipeImport);
  },

  approveRecipeImport: async (id: number): Promise<Recipe> => {
    const data = await request<{ approveRecipeImport: GqlRecipe | null }>(
      `mutation ($id: ID!) { approveRecipeImport(id: $id) { ${RECIPE_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.approveRecipeImport) throw new ApiError(404, `Recipe import ${id} not found`);
    return toRecipe(data.approveRecipeImport);
  },

  rejectRecipeImport: async (id: number): Promise<RecipeImport> => {
    const data = await request<{ rejectRecipeImport: GqlRecipeImport }>(
      `mutation ($id: ID!) { rejectRecipeImport(id: $id) { ${RECIPE_IMPORT_FIELDS} } }`,
      { id: String(id) }
    );
    return toRecipeImport(data.rejectRecipeImport);
  },

  retryRecipeImport: async (id: number): Promise<RecipeImport> => {
    const data = await request<{ retryRecipeImport: GqlRecipeImport }>(
      `mutation ($id: ID!) { retryRecipeImport(id: $id) { ${RECIPE_IMPORT_FIELDS} } }`,
      { id: String(id) }
    );
    return toRecipeImport(data.retryRecipeImport);
  },

  getRecommendedRecipes: async (limit = 10): Promise<RecipeRecommendation[]> => {
    const data = await request<{ recommendedRecipes: GqlRecipeRecommendation[] }>(
      `query ($limit: Int) { recommendedRecipes(limit: $limit) { recipe { ${RECIPE_FIELDS} } reason score } }`,
      { limit }
    );
    return (data.recommendedRecipes ?? []).map((r) => ({
      recipe: toRecipe(r.recipe),
      reason: r.reason,
      score: r.score,
    }));
  },

  getSuggestedRestockItems: async (limit = 10): Promise<Item[]> => {
    const data = await request<{ suggestedRestockItems: GqlItem[] }>(
      `query ($limit: Int) { suggestedRestockItems(limit: $limit) { ${ITEM_FIELDS} } }`,
      { limit }
    );
    return (data.suggestedRestockItems ?? []).map((i) => toItem(i));
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

  addRecipeItem: async (recipeId: number, item: { itemId?: number | null; ingredientId?: number | null; portion: number; unit: string | null; isOptional: boolean }): Promise<RecipeItem> => {
    const recipe = await api.getRecipe(recipeId);
    const items = (recipe.recipeItems ?? []).map((i) => ({
      itemId: i.itemID != null ? String(i.itemID) : null,
      ingredientId: i.ingredientID != null ? String(i.ingredientID) : null,
      quantity: i.quantity,
      unit: i.unitOfMeasure ?? "",
      notes: i.notes,
      isOptional: i.isOptional,
    }));
    items.push({
      itemId: item.itemId != null ? String(item.itemId) : null,
      ingredientId: item.ingredientId != null ? String(item.ingredientId) : null,
      quantity: item.portion,
      unit: item.unit ?? "",
      notes: null,
      isOptional: item.isOptional,
    });
    const updated = await api.updateRecipe(recipeId, recipeInputOverride(recipe, { items }));
    return (
      (updated.recipeItems ?? []).find((i) =>
        item.itemId != null ? i.itemID === item.itemId : i.ingredientID === item.ingredientId
      ) ?? {
        recipeID: recipeId,
        itemID: item.itemId ?? null,
        ingredientID: item.ingredientId ?? null,
        quantity: item.portion,
        unitOfMeasure: item.unit,
        notes: null,
        isOptional: item.isOptional,
      }
    );
  },

  // Removes a line by whichever key it carries — a branded item or, on
  // ingredient-only lines, the ingredient.
  removeRecipeItem: async (
    recipeId: number,
    item: { itemID?: number | null; ingredientID?: number | null }
  ): Promise<void> => {
    const recipe = await api.getRecipe(recipeId);
    const items = (recipe.recipeItems ?? [])
      .filter((i) => !(
        (item.itemID != null && i.itemID === item.itemID) ||
        (item.itemID == null && i.itemID == null && i.ingredientID === item.ingredientID)
      ))
      .map((i) => ({
        itemId: i.itemID != null ? String(i.itemID) : null,
        ingredientId: i.ingredientID != null ? String(i.ingredientID) : null,
        quantity: i.quantity,
        unit: i.unitOfMeasure ?? "",
        notes: i.notes,
        isOptional: i.isOptional,
      }));
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { items }));
  },

  getRecipeSteps: async (recipeId: number): Promise<RecipeStep[]> =>
    (await api.getRecipe(recipeId)).recipeSteps ?? [],

  addRecipeStep: async (recipeId: number, step: Partial<RecipeStep> & { stepNumber: number; instruction: string }): Promise<RecipeStep> => {
    const recipe = await api.getRecipe(recipeId);
    const steps = (recipe.recipeSteps ?? []).map(toStepInput);
    steps.push(toStepInput(step));
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { steps }));
    return {
      ...audit(),
      recipeStepID: step.stepNumber,
      recipeID: recipeId,
      stepNumber: step.stepNumber,
      instruction: step.instruction,
      durationMinutes: step.durationMinutes ?? null,
      stepType: step.stepType ?? null,
      isPassive: step.isPassive ?? false,
      dependsOnStepNumber: step.dependsOnStepNumber ?? null,
      appliance: step.appliance ?? null,
      recipe: null,
    };
  },

  updateRecipeStep: async (recipeId: number, stepId: number, step: Partial<RecipeStep> & { stepNumber: number; instruction: string }): Promise<RecipeStep> => {
    const recipe = await api.getRecipe(recipeId);
    const steps = (recipe.recipeSteps ?? []).map((s) =>
      s.recipeStepID === stepId || s.stepNumber === stepId
        ? toStepInput(step)
        : toStepInput(s)
    );
    await api.updateRecipe(recipeId, recipeInputOverride(recipe, { steps }));
    return {
      ...audit(),
      recipeStepID: step.stepNumber,
      recipeID: recipeId,
      stepNumber: step.stepNumber,
      instruction: step.instruction,
      durationMinutes: step.durationMinutes ?? null,
      stepType: step.stepType ?? null,
      isPassive: step.isPassive ?? false,
      dependsOnStepNumber: step.dependsOnStepNumber ?? null,
      appliance: step.appliance ?? null,
      recipe: null,
    };
  },

  deleteRecipeStep: async (recipeId: number, stepId: number): Promise<void> => {
    const recipe = await api.getRecipe(recipeId);
    const steps = (recipe.recipeSteps ?? [])
      .filter((s) => s.recipeStepID !== stepId && s.stepNumber !== stepId)
      .map(toStepInput);
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
    const data = await request<{
      nutrition: { entries: GqlNutritionSummary[]; warnings: string[] };
    }>(
      `query ($mealPlanId: ID!) { nutrition(mealPlanId: $mealPlanId) { entries { name unit amount } warnings } }`,
      { mealPlanId: String(id) }
    );
    return {
      mealPlanId: id,
      warnings: data.nutrition.warnings,
      dailyTotals: [
        {
          dayOfWeek: 0,
          nutrients: data.nutrition.entries.map((n) => ({
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
          items { id quantity unit isFromRecipe ingredient { id name } item { ${ITEM_FIELDS} } }
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
    const merged = {
      ...audit(),
      dayOfWeek: slot.dayOfWeek ?? existing.dayOfWeek,
      mealType: slot.mealType ?? existing.mealType,
      recipeID: existing.recipeID,
      servings: slot.servings ?? existing.servings,
      replacementNote: existing.replacementNote,
    };
    // `null` is meaningful here (clears the field), so `??` would be wrong.
    if (slot.recipeID !== undefined) merged.recipeID = slot.recipeID;
    if (slot.replacementNote !== undefined) merged.replacementNote = slot.replacementNote;
    return api.addMealSlot(planId, merged);
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
        addMealSlotItem(input: $input) { id quantity unit isFromRecipe ingredient { id name } item { ${ITEM_FIELDS} } }
      }`,
      {
        input: {
          slotId: String(slotId),
          itemId: item.itemID != null ? String(item.itemID) : null,
          ingredientId: item.ingredientID != null ? String(item.ingredientID) : null,
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

  // Food Events (household-scoped gatherings grouping recipes by serve time)
  getFoodEventsPaged: async (pageNumber: number, pageSize: number): Promise<PagedResult<FoodEvent>> => {
    const data = await request<{ foodEvents: GqlFoodEventPage }>(
      `query ($page: Int, $pageSize: Int) { foodEvents(page: $page, pageSize: $pageSize) { items { ${FOOD_EVENT_FIELDS} } pageInfo { pageNumber pageSize totalCount } } }`,
      { page: pageNumber, pageSize }
    );
    return toPaged(data.foodEvents.items.map(toFoodEvent), data.foodEvents.pageInfo);
  },

  getFoodEvent: async (id: number): Promise<FoodEvent> => {
    const data = await request<{ foodEvent: GqlFoodEvent | null }>(
      `query ($id: ID!) { foodEvent(id: $id) { ${FOOD_EVENT_FIELDS} } }`,
      { id: String(id) }
    );
    if (!data.foodEvent) throw new ApiError(404, `FoodEvent ${id} not found`);
    return toFoodEvent(data.foodEvent);
  },

  createFoodEvent: async (ev: { name: string; eventDate: string; slotGranularityMinutes?: number | null }): Promise<FoodEvent> => {
    const data = await request<{ createFoodEvent: GqlFoodEvent }>(
      `mutation ($input: CreateFoodEventInput!) { createFoodEvent(input: $input) { ${FOOD_EVENT_FIELDS} } }`,
      {
        input: {
          name: ev.name,
          eventDate: ev.eventDate,
          slotGranularityMinutes: ev.slotGranularityMinutes ?? null,
        },
      }
    );
    return toFoodEvent(data.createFoodEvent);
  },

  updateFoodEvent: async (id: number, ev: { name?: string; eventDate?: string; slotGranularityMinutes?: number; isActive?: boolean }): Promise<FoodEvent> => {
    const data = await request<{ updateFoodEvent: GqlFoodEvent }>(
      `mutation ($id: ID!, $input: UpdateFoodEventInput!) { updateFoodEvent(id: $id, input: $input) { ${FOOD_EVENT_FIELDS} } }`,
      {
        id: String(id),
        input: {
          name: ev.name ?? null,
          eventDate: ev.eventDate ?? null,
          slotGranularityMinutes: ev.slotGranularityMinutes ?? null,
          isActive: ev.isActive ?? null,
        },
      }
    );
    return toFoodEvent(data.updateFoodEvent);
  },

  deleteFoodEvent: async (id: number): Promise<void> => {
    await request<{ deleteFoodEvent: boolean }>(
      `mutation ($id: ID!) { deleteFoodEvent(id: $id) }`,
      { id: String(id) }
    );
  },

  addEventRecipe: async (foodEventId: number, slot: { recipeID?: number | null; mealType: string; targetTime: string; servings?: number | null; notes?: string | null }): Promise<EventRecipe> => {
    const data = await request<{ addEventRecipe: GqlEventRecipe }>(
      `mutation ($input: AddEventRecipeInput!) {
        addEventRecipe(input: $input) { ${EVENT_RECIPE_FIELDS} }
      }`,
      {
        input: {
          foodEventId: String(foodEventId),
          recipeId: slot.recipeID != null ? String(slot.recipeID) : null,
          mealType: slot.mealType,
          targetTime: slot.targetTime,
          servings: slot.servings ?? null,
          notes: slot.notes ?? null,
        },
      }
    );
    return toEventRecipe(foodEventId, data.addEventRecipe);
  },

  updateEventRecipe: async (id: number, slot: { recipeID?: number | null; mealType?: string; targetTime?: string; servings?: number | null; notes?: string | null }): Promise<EventRecipe> => {
    const data = await request<{ updateEventRecipe: GqlEventRecipe }>(
      `mutation ($id: ID!, $input: UpdateEventRecipeInput!) {
        updateEventRecipe(id: $id, input: $input) { ${EVENT_RECIPE_FIELDS} }
      }`,
      {
        id: String(id),
        input: {
          recipeId: slot.recipeID != null ? String(slot.recipeID) : null,
          mealType: slot.mealType ?? null,
          targetTime: slot.targetTime ?? null,
          servings: slot.servings ?? null,
          notes: slot.notes ?? null,
        },
      }
    );
    return toEventRecipe(0, data.updateEventRecipe);
  },

  removeEventRecipe: async (id: number): Promise<void> => {
    await request<{ removeEventRecipe: boolean }>(
      `mutation ($id: ID!) { removeEventRecipe(id: $id) }`,
      { id: String(id) }
    );
  },

  addEventRecipeStep: async (eventRecipeId: number, step: EventRecipeStepInput): Promise<EventRecipeStep> => {
    const data = await request<{ addEventRecipeStep: GqlEventRecipeStep }>(
      `mutation ($id: ID!, $input: EventRecipeStepInput!) {
        addEventRecipeStep(eventRecipeId: $id, input: $input) { ${EVENT_RECIPE_STEP_FIELDS} }
      }`,
      { id: String(eventRecipeId), input: toEventStepVariables(step) }
    );
    return toEventRecipeStep(data.addEventRecipeStep);
  },

  updateEventRecipeStep: async (id: number, step: EventRecipeStepInput): Promise<EventRecipeStep> => {
    const data = await request<{ updateEventRecipeStep: GqlEventRecipeStep }>(
      `mutation ($id: ID!, $input: EventRecipeStepInput!) {
        updateEventRecipeStep(id: $id, input: $input) { ${EVENT_RECIPE_STEP_FIELDS} }
      }`,
      { id: String(id), input: toEventStepVariables(step) }
    );
    return toEventRecipeStep(data.updateEventRecipeStep);
  },

  removeEventRecipeStep: async (id: number): Promise<void> => {
    await request<{ removeEventRecipeStep: boolean }>(
      `mutation ($id: ID!) { removeEventRecipeStep(id: $id) }`,
      { id: String(id) }
    );
  },

  addEventRecipeItem: async (eventRecipeId: number, item: EventRecipeItemInput): Promise<EventRecipeItem> => {
    const data = await request<{ addEventRecipeItem: GqlEventRecipeItem }>(
      `mutation ($id: ID!, $input: EventRecipeItemInput!) {
        addEventRecipeItem(eventRecipeId: $id, input: $input) { ${EVENT_RECIPE_ITEM_FIELDS} }
      }`,
      { id: String(eventRecipeId), input: toEventItemVariables(item) }
    );
    return toEventRecipeItem(data.addEventRecipeItem);
  },

  updateEventRecipeItem: async (id: number, item: EventRecipeItemInput): Promise<EventRecipeItem> => {
    const data = await request<{ updateEventRecipeItem: GqlEventRecipeItem }>(
      `mutation ($id: ID!, $input: EventRecipeItemInput!) {
        updateEventRecipeItem(id: $id, input: $input) { ${EVENT_RECIPE_ITEM_FIELDS} }
      }`,
      { id: String(id), input: toEventItemVariables(item) }
    );
    return toEventRecipeItem(data.updateEventRecipeItem);
  },

  removeEventRecipeItem: async (id: number): Promise<void> => {
    await request<{ removeEventRecipeItem: boolean }>(
      `mutation ($id: ID!) { removeEventRecipeItem(id: $id) }`,
      { id: String(id) }
    );
  },

  syncEventRecipe: async (eventRecipeId: number): Promise<EventRecipe> => {
    const data = await request<{ syncEventRecipe: GqlEventRecipe }>(
      `mutation ($id: ID!) {
        syncEventRecipe(eventRecipeId: $id) { ${EVENT_RECIPE_FIELDS} }
      }`,
      { id: String(eventRecipeId) }
    );
    return toEventRecipe(0, data.syncEventRecipe);
  },

  getEventTimeline: async (foodEventId: number): Promise<EventTimeline> => {
    const data = await request<{ eventTimeline: GqlEventTimeline | null }>(
      `query ($id: ID!) {
        eventTimeline(foodEventId: $id) {
          foodEventId
          warnings
          recipes {
            eventRecipeId name targetTime servings baseServings startBy
            unschedulable warnings
            steps {
              stepNumber instruction stepType isPassive appliance
              durationMinutes scheduledMinutes estimated
              startTime endTime conflicts
            }
          }
        }
      }`,
      { id: String(foodEventId) }
    );
    if (!data.eventTimeline) throw new ApiError(404, `Timeline for event ${foodEventId} not found`);
    return toEventTimeline(data.eventTimeline);
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
          ingredient { id name }
          usualBrand { id name brand { id name } }
          item { ${ITEM_FIELDS} }
        }
      }`,
      {
        input: {
          groceryListId: String(listId),
          itemId: item.itemID != null ? String(item.itemID) : null,
          ingredientId: item.ingredientID != null ? String(item.ingredientID) : null,
          manualItemName: item.manualItemName,
          quantity: item.quantityNeeded,
          unit: item.unitOfMeasure ?? "",
          source: item.source || null,
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
          ingredient { id name }
          usualBrand { id name brand { id name } }
          item { ${ITEM_FIELDS} }
        }
      }`,
      { groceryListItemId: String(id) }
    );
    return toGroceryListItem(0, data.toggleGroceryItemChecked);
  },

  checkGroceryItemWithBrand: async (id: number, itemId: number): Promise<GroceryListItem> => {
    const data = await request<{ checkGroceryItemWithBrand: GqlGroceryListItem }>(
      `mutation ($groceryListItemId: ID!, $itemId: ID!) {
        checkGroceryItemWithBrand(groceryListItemId: $groceryListItemId, itemId: $itemId) {
          id manualItemName quantityNeeded unitOfMeasure source isChecked
          ingredient { id name }
          usualBrand { id name brand { id name } }
          item { ${ITEM_FIELDS} }
        }
      }`,
      { groceryListItemId: String(id), itemId: String(itemId) }
    );
    return toGroceryListItem(0, data.checkGroceryItemWithBrand);
  },

  deleteGroceryListItem: async (id: number): Promise<void> => {
    await request<{ deleteGroceryItem: boolean }>(
      `mutation ($groceryListItemId: ID!) { deleteGroceryItem(groceryListItemId: $groceryListItemId) }`,
      { groceryListItemId: String(id) }
    );
  },

  // Store routing — server computes the order; the client renders
  // routeGroups verbatim so web and mobile always agree.

  getGroceryStores: async (): Promise<Store[]> => {
    const data = await request<{ groceryStores: GqlStore[] }>(
      `query { groceryStores { ${STORE_FIELDS} } }`
    );
    return data.groceryStores.map(toStore);
  },

  getGroceryRouteGroups: async (listId: number): Promise<GroceryRouteGroup[]> => {
    const data = await request<{ groceryRouteGroups: GqlGroceryRouteGroup[] }>(
      `query ($groceryListId: ID!) {
        groceryRouteGroups(groceryListId: $groceryListId) {
          aisle { id name position }
          items {
            suggested
            item { id manualItemName quantityNeeded unitOfMeasure source isChecked
          ingredient { id name } usualBrand { id name brand { id name } } item { ${ITEM_FIELDS} }
          ${ALLERGY_FIELDS} }
          }
        }
      }`,
      { groceryListId: String(listId) }
    );
    return data.groceryRouteGroups.map((g) => ({
      aisle: g.aisle ? toStoreAisle(g.aisle) : null,
      items: g.items.map((i) => ({ suggested: i.suggested, item: toGroceryListItem(listId, i.item) })),
    }));
  },

  createStore: async (name: string): Promise<Store> => {
    const data = await request<{ createStore: GqlStore }>(
      `mutation ($name: String!) { createStore(name: $name) { ${STORE_FIELDS} } }`,
      { name }
    );
    return toStore(data.createStore);
  },

  renameStore: async (storeId: number, name: string): Promise<Store> => {
    const data = await request<{ renameStore: GqlStore }>(
      `mutation ($storeId: ID!, $name: String!) { renameStore(storeId: $storeId, name: $name) { ${STORE_FIELDS} } }`,
      { storeId: String(storeId), name }
    );
    return toStore(data.renameStore);
  },

  deleteStore: async (storeId: number): Promise<void> => {
    await request<{ deleteStore: boolean }>(
      `mutation ($storeId: ID!) { deleteStore(storeId: $storeId) }`,
      { storeId: String(storeId) }
    );
  },

  createStoreAisle: async (storeId: number, name: string, position: number): Promise<StoreAisle> => {
    const data = await request<{ createStoreAisle: GqlStoreAisle }>(
      `mutation ($storeId: ID!, $name: String!, $position: Int!) {
        createStoreAisle(storeId: $storeId, name: $name, position: $position) { id name position }
      }`,
      { storeId: String(storeId), name, position }
    );
    return toStoreAisle(data.createStoreAisle);
  },

  renameStoreAisle: async (aisleId: number, name: string): Promise<StoreAisle> => {
    const data = await request<{ renameStoreAisle: GqlStoreAisle }>(
      `mutation ($aisleId: ID!, $name: String!) { renameStoreAisle(aisleId: $aisleId, name: $name) { id name position } }`,
      { aisleId: String(aisleId), name }
    );
    return toStoreAisle(data.renameStoreAisle);
  },

  deleteStoreAisle: async (aisleId: number): Promise<void> => {
    await request<{ deleteStoreAisle: boolean }>(
      `mutation ($aisleId: ID!) { deleteStoreAisle(aisleId: $aisleId) }`,
      { aisleId: String(aisleId) }
    );
  },

  reorderStoreAisles: async (storeId: number, aisleIds: number[]): Promise<StoreAisle[]> => {
    const data = await request<{ reorderStoreAisles: GqlStoreAisle[] }>(
      `mutation ($storeId: ID!, $aisleIds: [ID!]!) {
        reorderStoreAisles(storeId: $storeId, aisleIds: $aisleIds) { id name position }
      }`,
      { storeId: String(storeId), aisleIds: aisleIds.map(String) }
    );
    return data.reorderStoreAisles.map(toStoreAisle);
  },

  assignItemToAisle: async (
    storeId: number,
    aisleId: number | null,
    identity: { itemID?: number | null; ingredientID?: number | null; manualItemName?: string | null }
  ): Promise<void> => {
    await request<{ assignItemToAisle: boolean }>(
      `mutation ($storeId: ID!, $aisleId: ID, $itemId: ID, $ingredientId: ID, $manualItemName: String) {
        assignItemToAisle(storeId: $storeId, aisleId: $aisleId, itemId: $itemId, ingredientId: $ingredientId, manualItemName: $manualItemName)
      }`,
      {
        storeId: String(storeId),
        aisleId: aisleId != null ? String(aisleId) : null,
        itemId: identity.itemID != null ? String(identity.itemID) : null,
        ingredientId: identity.ingredientID != null ? String(identity.ingredientID) : null,
        manualItemName: identity.manualItemName ?? null,
      }
    );
  },

  setGroceryListStore: async (listId: number, storeId: number | null): Promise<GroceryList> => {
    const data = await request<{ setGroceryListStore: GqlGroceryList }>(
      `mutation ($groceryListId: ID!, $storeId: ID) {
        setGroceryListStore(groceryListId: $groceryListId, storeId: $storeId) { ${GROCERY_LIST_FIELDS} }
      }`,
      { groceryListId: String(listId), storeId: storeId != null ? String(storeId) : null }
    );
    return toGroceryList(data.setGroceryListStore);
  },

  reorderGroceryListItems: async (
    listId: number,
    entries: { groceryListItemID: number; aisleID?: number | null }[]
  ): Promise<void> => {
    await request<{ reorderGroceryListItems: boolean }>(
      `mutation ($groceryListId: ID!, $entries: [GroceryReorderEntryInput!]!) {
        reorderGroceryListItems(groceryListId: $groceryListId, entries: $entries)
      }`,
      {
        groceryListId: String(listId),
        entries: entries.map((e) => ({
          groceryListItemId: String(e.groceryListItemID),
          aisleId: e.aisleID != null ? String(e.aisleID) : null,
        })),
      }
    );
  },

  /* ---------------------------- AI assistant ---------------------------- */

  getAIAvailable: async (): Promise<boolean> => {
    const data = await request<{ aiAvailable: boolean }>(`query { aiAvailable }`);
    return data.aiAvailable ?? false;
  },

  getSemanticSearchAvailable: async (): Promise<boolean> => {
    const data = await request<{ semanticSearchAvailable: boolean }>(
      `query { semanticSearchAvailable }`
    );
    return data.semanticSearchAvailable ?? false;
  },

  suggestMeals: async (mealPlanId: number, maxSuggestions = 6): Promise<MealPlanSuggestion[]> => {
    const data = await request<{ suggestMeals: GqlMealPlanSuggestion[] }>(
      `query ($mealPlanId: ID!, $maxSuggestions: Int) {
        suggestMeals(mealPlanId: $mealPlanId, maxSuggestions: $maxSuggestions) {
          recipe { ${RECIPE_FIELDS} }
          dayOfWeek mealType reason usesExpiringItems
        }
      }`,
      { mealPlanId: String(mealPlanId), maxSuggestions }
    );
    return data.suggestMeals.map((s) => ({
      recipe: toRecipe(s.recipe),
      dayOfWeek: s.dayOfWeek,
      mealType: mealTypeToNumber(s.mealType),
      reason: s.reason,
      usesExpiringItems: s.usesExpiringItems ?? [],
    }));
  },

  suggestEventFixes: async (foodEventId: number, maxSuggestions = 6): Promise<EventFixSuggestion[]> => {
    const data = await request<{ suggestEventFixes: GqlEventFixSuggestion[] }>(
      `query ($foodEventId: ID!, $maxSuggestions: Int) {
        suggestEventFixes(foodEventId: $foodEventId, maxSuggestions: $maxSuggestions) {
          eventRecipeId recipeName stepNumber action minutes appliance
          durationMinutes dependsOnStepNumber reason
        }
      }`,
      { foodEventId: String(foodEventId), maxSuggestions }
    );
    return data.suggestEventFixes.map((f) => ({
      eventRecipeId: Number(f.eventRecipeId),
      recipeName: f.recipeName,
      stepNumber: f.stepNumber ?? null,
      action: f.action,
      minutes: f.minutes ?? null,
      appliance: f.appliance ?? null,
      durationMinutes: f.durationMinutes ?? null,
      dependsOnStepNumber: f.dependsOnStepNumber ?? null,
      reason: f.reason,
    }));
  },

  suggestPairings: async (recipeId: number, maxSuggestions = 4): Promise<PairingSuggestion[]> => {
    const data = await request<{ suggestPairings: GqlPairingSuggestion[] }>(
      `query ($recipeId: ID!, $maxSuggestions: Int) {
        suggestPairings(recipeId: $recipeId, maxSuggestions: $maxSuggestions) {
          bottleId name reason inCellar
        }
      }`,
      { recipeId: String(recipeId), maxSuggestions }
    );
    return data.suggestPairings.map((p) => ({
      bottleId: p.bottleId == null ? null : Number(p.bottleId),
      name: p.name,
      reason: p.reason,
      inCellar: p.inCellar,
    }));
  },

  suggestCocktails: async (inStockOnly = false, maxSuggestions = 6): Promise<CocktailSuggestion[]> => {
    const data = await request<{ suggestCocktails: GqlCocktailSuggestion[] }>(
      `query ($maxSuggestions: Int, $inStockOnly: Boolean) {
        suggestCocktails(maxSuggestions: $maxSuggestions, inStockOnly: $inStockOnly) {
          recipe { ${RECIPE_FIELDS} }
          reason missingIngredients
        }
      }`,
      { maxSuggestions, inStockOnly }
    );
    return data.suggestCocktails.map((c) => ({
      recipe: toRecipe(c.recipe),
      reason: c.reason,
      missingIngredients: c.missingIngredients ?? [],
    }));
  },

  askAssistant: async (question: string): Promise<AssistantAnswer> => {
    const data = await request<{ askAssistant: GqlAssistantAnswer }>(
      `query ($question: String!) {
        askAssistant(question: $question) { answer toolCalls { name } }
      }`,
      { question }
    );
    return {
      answer: data.askAssistant.answer,
      toolCalls: data.askAssistant.toolCalls ?? [],
    };
  },

  /* -------- Client-side inference surface (local AI assistant) -------- */

  getAssistantTools: async (): Promise<AssistantToolSpec[]> => {
    const data = await request<{ assistantTools: AssistantToolSpec[] }>(
      `query { assistantTools { name description parametersJson } }`
    );
    return data.assistantTools ?? [];
  },

  callAssistantTool: async (name: string, argumentsJson: string): Promise<string> => {
    const data = await request<{ callAssistantTool: string }>(
      `query ($name: String!, $arguments: String!) {
        callAssistantTool(name: $name, arguments: $arguments)
      }`,
      { name, arguments: argumentsJson }
    );
    return data.callAssistantTool;
  },

  getAssistantPrompt: async (name: string): Promise<string> => {
    const data = await request<{ assistantPrompt: string }>(
      `query ($name: String!) { assistantPrompt(name: $name) }`,
      { name }
    );
    return data.assistantPrompt;
  },

  prepareAssistantRequest: async (
    name: string,
    paramsJson: string
  ): Promise<PreparedAIRequest | null> => {
    const data = await request<{ prepareAssistantRequest: GqlPreparedRequest | null }>(
      `query ($name: String!, $paramsJson: String!) {
        prepareAssistantRequest(name: $name, paramsJson: $paramsJson) {
          prompt contextJson outputSchemaJson
        }
      }`,
      { name, paramsJson }
    );
    const p = data.prepareAssistantRequest;
    return p ? { prompt: p.prompt, contextJson: p.contextJson, outputSchemaJson: p.outputSchemaJson } : null;
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
  categoryIds: string[] | null;
  items: {
    itemId: string | null;
    ingredientId: string | null;
    quantity: number;
    unit: string;
    notes: string | null;
    isOptional: boolean;
  }[];
  steps: {
    stepNumber: number;
    instruction: string;
    durationMinutes?: number | null;
    stepType?: string | null;
    isPassive?: boolean;
    dependsOnStepNumber?: number | null;
    appliance?: string | null;
  }[];
}

// toStepInput preserves the step-timing fields through the
// fetch-modify-updateRecipe round trip — dropping them would silently
// erase duration/appliance data on every web-side step save.
function toStepInput(s: Partial<RecipeStep> & { stepNumber: number; instruction: string }) {
  return {
    stepNumber: s.stepNumber,
    instruction: s.instruction,
    durationMinutes: s.durationMinutes ?? null,
    stepType: s.stepType ?? null,
    isPassive: s.isPassive ?? false,
    dependsOnStepNumber: s.dependsOnStepNumber ?? null,
    appliance: s.appliance ?? null,
  };
}

function toRecipeInput(recipe: Partial<Recipe>): RecipeInputShape {
  return {
    name: recipe.recipeName ?? "",
    description: recipe.description ?? null,
    servings: recipe.servings ?? null,
    prepTimeMinutes: recipe.prepTimeMinutes ?? null,
    cookTimeMinutes: recipe.cookTimeMinutes ?? null,
    categoryIds: recipe.categories
      ? recipe.categories.map((c) => String(c.categoryID))
      : null,
    items: (recipe.recipeItems ?? []).map((i) => ({
      itemId: i.itemID != null ? String(i.itemID) : null,
      ingredientId: i.ingredientID != null ? String(i.ingredientID) : null,
      quantity: i.quantity,
      unit: i.unitOfMeasure ?? "",
      notes: i.notes ?? null,
      isOptional: i.isOptional,
    })),
    steps: (recipe.recipeSteps ?? []).map(toStepInput),
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
      itemID: i.itemId != null ? num(i.itemId) : null,
      ingredientID: i.ingredientId != null ? num(i.ingredientId) : null,
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
      durationMinutes: s.durationMinutes ?? null,
      stepType: s.stepType ?? null,
      isPassive: s.isPassive ?? false,
      dependsOnStepNumber: s.dependsOnStepNumber ?? null,
      appliance: s.appliance ?? null,
    })),
  };
}
