# LENA GraphQL Schema

This document describes the public GraphQL API exposed by the BFF at `/graphql`. It is a curated overview of the primary types and operations — `internal/bff/schema.graphqls` is the authoritative, complete definition. Auth/session endpoints (`/auth/*`) are plain REST, not GraphQL; see `docs/auth-oidc.md`.

## Scalar types

- `ID` — opaque identifier, serialized as a string.
- `String`, `Int`, `Float`, `Boolean` — built-in GraphQL scalars.
- `Time` — ISO 8601 / RFC 3339 timestamp.
- `Date` — ISO 8601 calendar date (`YYYY-MM-DD`). Currently declared but not used by any resolver.

## Core types

### `User`

| Field | Type | Description |
|---|---|---|
| `id` | `ID!` | LENA internal user ID |
| `email` | `String!` | Primary email address (from the OIDC token) |
| `displayName` | `String` | Display name from the OIDC token |
| `firstName` | `String` | User-provided first name (self-service profile) |
| `lastName` | `String` | User-provided last name |
| `backupEmail` | `String` | Optional secondary contact email |
| `role` | `String!` | `member` or `admin` |
| `isActive` | `Boolean!` | `false` means banned — every request is rejected with 401 |
| `isProtected` | `Boolean!` | `true` when the email is in `LENA_PROTECTED_EMAILS` — can never be demoted or banned |
| `lastLoginAt` | `Time` | Last successful authentication |

### User management

- `users(page: Int, pageSize: Int): UserPage!` — **admin only.** Paged list of all users (`UserPage { items: [User!]!, pageInfo: PageInfo! }`).
- `setUserRole(userId: ID!, role: String!): User!` — **admin only.** `role` must be `member` or `admin`. Rejects self-modification, protected admins, and demoting the last active admin (`FORBIDDEN`).
- `setUserActive(userId: ID!, isActive: Boolean!): User!` — **admin only.** `isActive: false` bans the user and takes effect immediately: the cached identity is evicted (no 2-minute window) and every refresh-token session for the user is revoked in the same transaction. Same guards as `setUserRole`.
- `updateMyProfile(input: UpdateProfileInput!): User!` — any authenticated user, own record only. `UpdateProfileInput { firstName, lastName, backupEmail, birthdate, isSearchable }`; omitted fields are unchanged, empty strings clear the field. `backupEmail` must be a valid address (`BAD_USER_INPUT` otherwise). `isSearchable` is the discoverability opt-in — users are invisible to `searchUsers` and household invites until they set it true.
- `inviteHouseholdMember(userId: ID!): HouseholdInvite!` — invites a user into the caller's household. Target must be active **and** `isSearchable`; unknown, inactive, unsearchable, already-a-member, and pending-duplicate targets all return the identical `BAD_USER_INPUT: cannot invite this user` (no user enumeration). Rate-limited 10/min per caller.
- `acceptHouseholdInvite(inviteId: ID!): Household!` — the invitee accepts; their plans, lists, and events merge into the shared household.

### Catalog — `Brand`, `Category`, `Item`

```graphql
type Brand {
  id: ID!
  name: String!
}

type Category {
  id: ID!
  name: String!
  description: String
}

type Item {
  id: ID!
  name: String!
  brand: Brand
  upc12: String
  upc14: String
  category: Category!
  unit: String!
}
```

### Catalog — `Recipe`, `RecipeItem`, `RecipeStep`

```graphql
type Recipe {
  id: ID!
  name: String!
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  items: [RecipeItem!]!
  steps: [RecipeStep!]!
}

type RecipeItem {
  item: Item!
  quantity: Float!
  unit: String!
  notes: String
  isOptional: Boolean!
}

type RecipeStep {
  stepNumber: Int!
  instruction: String!
  # Timing metadata feeding the event master-timeline scheduler.
  durationMinutes: Int
  stepType: String
  isPassive: Boolean!
  dependsOnStepNumber: Int
  appliance: String
}
```

### Wine — `Bottle`

```graphql
type Bottle {
  id: ID!
  vineyard: String
  vintageYear: Int!
  abv: Float
  acidity: Int
  tanninLevel: Int
  body: Int
  sweetness: Int
  oakIntegration: Boolean
  bottleSize: String!
}
```

Wine reference data (`WineType`, `Country`, `Region`) exists in the database but is not exposed as separate GraphQL object types in the current schema.

### User preferences — `UserItem`, `UserBottle`

```graphql
type UserItem {
  id: ID!
  item: Item!
  currentQty: Float!
  minQty: Float
  purchaseAt: Time
  expiresAt: Time
  notes: String
  isFavorite: Boolean!
}

type UserBottle {
  id: ID!
  bottle: Bottle!
  bottleNumber: Int
  quantity: Int!
  purchaseAt: Time
  purchasePrice: Float
  storageTemp: Float
  location: String
  notes: String
  isFavorite: Boolean!
}
```

### Meal planning — `MealPlan`, `MealSlot`, `MealSlotItem`

```graphql
type MealPlan {
  id: ID!
  name: String!
  weekStartDate: String!
  isActive: Boolean!
  slots: [MealSlot!]!
}

type MealSlot {
  id: ID!
  dayOfWeek: Int!
  mealType: String!
  recipe: Recipe
  servings: Int
  replacementNote: String
  items: [MealSlotItem!]!
}

type MealSlotItem {
  id: ID!
  item: Item
  quantity: Float!
  unit: String!
  isFromRecipe: Boolean!
}
```

### Grocery — `GroceryList`, `GroceryListItem`, store routing

```graphql
type GroceryList {
  id: ID!
  generatedAt: Time!
  items: [GroceryListItem!]!
  store: Store            # the store this list is routed through, if any
}

type GroceryListItem {
  id: ID!
  item: Item            # bound brand (optional preferred brand / check-off pick)
  ingredient: Ingredient
  usualBrand: Item      # household's remembered usual item for `ingredient`
  manualItemName: String
  quantityNeeded: Float!
  unitOfMeasure: String
  source: String!
  isChecked: Boolean!
}

type Store { id: ID!, name: String!, aisles: [StoreAisle!]! }
type StoreAisle { id: ID!, name: String!, position: Int! }

# Server-computed display order: aisle groups in walk order, then a
# trailing unassigned group (aisle: null). Clients render it verbatim —
# they never re-sort, so web and mobile always show the same order.
type GroceryRouteGroup {
  aisle: StoreAisle
  items: [GroceryRouteItem!]!
}
type GroceryRouteItem {
  item: GroceryListItem!
  suggested: Boolean!     # aisle inferred from learned order, not assigned
}
```

## Queries

| Query | Arguments | Returns | Description |
|---|---|---|---|
| `me` | — | `User!` | Current authenticated user |
| `brand(id)` | `ID!` | `Brand` | Single catalog brand |
| `brands` | `Int, Int` | `BrandPage!` | Paginated brands visible to the caller |
| `category(id)` | `ID!` | `Category` | Single category |
| `categories` | — | `[Category!]!` | All categories |
| `item(id)` | `ID!` | `Item` | Single catalog item |
| `items(page, pageSize)` | `Int, Int` | `ItemPage!` | Paginated catalog items |
| `recipe(id)` | `ID!` | `Recipe` | Single recipe |
| `recipes(page, pageSize, search, categoryIds, isFavorite, mealType, searchMode)` | `Int, Int, String, [ID!], Boolean, String, RecipeSearchMode` | `RecipePage!` | Paginated recipes — `searchMode: semantic` ranks embedded recipes by cosine distance + engagement (see `semanticSearchAvailable`) |
| `userItems(page, pageSize)` | `Int, Int` | `UserItemPage!` | Current user's pantry |
| `userBottles(page, pageSize)` | `Int, Int` | `UserBottlePage!` | Current user's cellar |
| `bottle(id)` | `ID!` | `Bottle` | Single wine bottle |
| `bottles(page, pageSize)` | `Int, Int` | `BottlePage!` | Paginated wine bottles |
| `mealPlan(id)` | `ID!` | `MealPlan` | Single meal plan |
| `mealPlans(page, pageSize)` | `Int, Int` | `MealPlanPage!` | Current user's plans |
| `groceryList(id)` | `ID!` | `GroceryList` | Single grocery list |
| `groceryLists(page, pageSize)` | `Int, Int` | `GroceryListPage!` | Current user's lists |
| `groceryStores` | — | `[Store!]!` | Household's stores with aisles |
| `groceryRouteGroups(groceryListId)` | `ID!` | `[GroceryRouteGroup!]!` | Server-computed route grouping |
| `foodEvent(id)` | `ID!` | `FoodEvent` | Single household event |
| `foodEvents(page, pageSize)` | `Int, Int` | `FoodEventPage!` | Household's events |
| `eventTimeline(foodEventId)` | `ID!` | `EventTimeline` | Backwards-scheduled master timeline |
| `aiAvailable` | — | `Boolean!` | Whether the AI assistant is configured |
| `semanticSearchAvailable` | — | `Boolean!` | Whether recipe embeddings (semantic search mode) are configured |
| `askAssistant(question)` | `String!` | `AssistantAnswer!` | Free-form Ask Dot question answered via read-only household-scoped tools |
| `suggestMeals(mealPlanId, maxSuggestions)` | `ID!, Int` | `[MealPlanSuggestion!]!` | Meal suggestions for open plan cells (server inference) |
| `suggestEventFixes(foodEventId, maxSuggestions)` | `ID!, Int` | `[EventFixSuggestion!]!` | Timeline-conflict fixes (server inference) |
| `suggestPairings(recipeId, maxSuggestions)` | `ID!, Int` | `[PairingSuggestion!]!` | Wine pairings — 21+ gated (server inference) |
| `suggestCocktails(maxSuggestions, inStockOnly)` | `Int, Boolean` | `[CocktailSuggestion!]!` | Cocktail picks — 21+ gated (server inference) |
| `assistantTools` | — | `[AssistantToolSpec!]!` | Read-only tool specs for client-side agents; household scope comes from auth context |
| `callAssistantTool(name, arguments)` | `String!, String!` | `String!` | Execute one read-only tool under the caller's scope — rate-limited, args JSON-schema-validated |
| `assistantPrompt(name)` | `String!` | `String!` | Server-owned system prompt for a named assistant flow |
| `prepareAssistantRequest(name, paramsJson)` | `String!, String!` | `PreparedAIRequest` | Server-assembled context + output contract so client-side inference produces identical prompts; alcohol-gated names keep the 21+ check |

## Mutations

### Inventory

- `createBrand(input: CreateBrandInput!): Brand!`
- `createCategory(input: CreateCategoryInput!): Category!`
- `createItem(input: CreateItemInput!): Item!`
- `updateItem(id: ID!, input: UpdateItemInput!): Item!`
- `deleteItem(id: ID!): Boolean!`

### Recipes

- `createRecipe(input: CreateRecipeInput!): Recipe!` — **admin only** (`@admin`); the recipe catalog is global across households.
- `updateRecipe(id: ID!, input: CreateRecipeInput!): Recipe!` — **admin only.**
- `deleteRecipe(id: ID!): Boolean!` — **admin only.**
- `setRecipeCategories(recipeId: ID!, categoryIds: [ID!]!): Recipe!` — **admin only.** Members see category chips read-only.
- `setRecipeFavorite(recipeId: ID!, isFavorite: Boolean!): Boolean!`

### User pantry and cellar

- `adjustUserItem(itemId: ID!, quantity: Float!, purchaseAt: Time): UserItem!`
- `setItemFavorite(itemId: ID!, isFavorite: Boolean!): UserItem!`
- `deleteUserItem(itemId: ID!): Boolean!`
- `adjustUserBottle(bottleId: ID!, quantity: Int!): UserBottle!`
- `setBottleFavorite(bottleId: ID!, isFavorite: Boolean!): UserBottle!`

### Meal planning and grocery

- `createMealPlan(input: CreateMealPlanInput!): MealPlan!`
- `updateMealPlan(id: ID!, input: CreateMealPlanInput!): MealPlan!`
- `deleteMealPlan(id: ID!): Boolean!`
- `addMealSlot(input: AddMealSlotInput!): MealSlot!`
- `removeMealSlot(slotId: ID!): Boolean!`
- `generateGroceryList(mealPlanId: ID!): GroceryList!`
- `toggleGroceryItemChecked(groceryListItemId: ID!): GroceryListItem!`
- `checkGroceryItemWithBrand(groceryListItemId: ID!, itemId: ID!): GroceryListItem!` — first-time check-off of an ingredient-only line: binds the picked item, marks checked, credits stock, and records it as the household's usual brand for that ingredient
- `deleteGroceryItem(groceryListItemId: ID!): Boolean!`

Ingredients (generic, unbranded):

- `ingredients(page: Int = 1, pageSize: Int = 25, search: String): IngredientPage!` (query)
- `getOrCreateIngredient(input: CreateIngredientInput!): Ingredient!` — member free-create with normalized-name dedupe; returns the existing row on conflict
- `mergeIngredient(fromId: ID!, intoId: ID!): Boolean!` (`@admin`) — repoints every FK to `intoId`, then deactivates `fromId` (the dedupe safety valve)
- `setItemIngredient(itemId: ID!, ingredientId: ID): Boolean!` (`@admin`) — catalog-level item→ingredient link; null clears
- `setHouseholdItemIngredient(itemId: ID!, ingredientId: ID): Boolean!` — household override of the catalog link; null clears; wins over `item.ingredient_id` everywhere

`Item.ingredient` is the catalog link; `Item.householdIngredient` is the override-aware resolved view. `itemId` is optional on `RecipeItemInput`, `AddMealSlotItemInput`, `EventRecipeItemInput`, and `AddGroceryItemInput` — all accept `ingredientId`, and at least one of the two is required.

Allergens and dietary restrictions:

- `allergens` (query) — `[Allergen!]!`, the registry rows.
- `myAllergies` (query) — `[MemberAllergen!]!`, the caller's records (`kind: allergy | dietary`).
- `setMyAllergy(allergenId: ID!, kind: MemberAllergyKind!, on: Boolean!): Boolean!` — set or clear one of the caller's own records.
- `createAllergen`, `updateAllergen` (`@admin`) — registry management; deactivate instead of delete.
- `setIngredientAllergen(ingredientId: ID!, allergenId: ID!, kind: AllergenFlagKind)` (`@admin`) — `contains`/`may_contain`, null clears.
- `setItemAllergen(itemId: ID!, allergenId: ID!, kind: AllergenFlagKind)` (`@admin`) — same, at the product level.
- `allergenSuggestions(status)` (`@admin`, query) — `[AllergenSuggestion!]!`, the AI review queue; `status` narrows to `pending`/`accepted`/`dismissed`.
- `suggestRecipeAllergens(recipeId: ID!, maxSuggestions: Int)` (`@admin`, AI budget) — runs the suggester over the recipe and returns the rows actually created; duplicates of open suggestions or curated flags drop server-side.
- `acceptAllergenSuggestion(id: ID!)` / `dismissAllergenSuggestion(id: ID!)` (`@admin`) — accept writes the real flag under the reviewer's attribution in the same transaction; both re-fetch and return the row.

`allergens`/`allergyWarnings` fields exist on `Item`, `Ingredient`, `Recipe`, `MealSlot`, `EventRecipe`, and `GroceryListItem`. Warnings name the household member, their record kind, and the entity's strongest flag kind. An entity with no flags means *no allergen information* — never "known safe"; clients must not render it as safe. Household `ingredient` overrides win over the catalog link; `contains` outranks `may_contain`.

Store routing — all household-scoped:

- `createStore(name: String!): Store!`, `renameStore(storeId: ID!, name: String!): Store!`, `deleteStore(storeId: ID!): Boolean!`
- `createStoreAisle(storeId: ID!, name: String!, position: Int!): StoreAisle!`, `renameStoreAisle(aisleId: ID!, name: String!): StoreAisle!`, `deleteStoreAisle(aisleId: ID!): Boolean!`, `reorderStoreAisles(storeId: ID!, aisleIds: [ID!]!): [StoreAisle!]!`
- `assignItemToAisle(storeId: ID!, aisleId: ID, itemId: ID, ingredientId: ID, manualItemName: String): Boolean!` — exactly one identity arg; `aisleId: null` unassigns
- `setGroceryListStore(groceryListId: ID!, storeId: ID): GroceryList!` — `storeId: null` clears; new lists inherit the household's most recent store
- `reorderGroceryListItems(groceryListId: ID!, entries: [GroceryReorderEntryInput!]!): Boolean!` — entries are the post-drag display order; an entry's `aisleId` moves that item (null = no change), so a cross-aisle drag lands rank + assignment atomically

### Events (household-scoped)

`FoodEvent` groups `EventRecipe` slots — a recipe (or free-form note) with a `mealType`, a `targetTime` serve time on the event date, and optional `servings`. `targetTime` must fall on the event's `slotGranularityMinutes` boundary (15 or 30) and on `eventDate`; violations return BAD_USER_INPUT. All members of the household can create and edit events; each mutation notifies the other members (`event_created`/`event_updated`/`event_deleted` kinds carrying `foodEventId` for deep-linking).

- `createFoodEvent(input: CreateFoodEventInput!): FoodEvent!`
- `updateFoodEvent(id: ID!, input: UpdateFoodEventInput!): FoodEvent!`
- `deleteFoodEvent(id: ID!): Boolean!`
- `addEventRecipe(input: AddEventRecipeInput!): EventRecipe!`
- `updateEventRecipe(id: ID!, input: UpdateEventRecipeInput!): EventRecipe!`
- `removeEventRecipe(id: ID!): Boolean!`

`eventTimeline(foodEventId)` computes the master schedule on read: each slot's **snapshot** step DAG is placed backwards from its `targetTime` (sinks end at the target, each step ends by its dependents' latest start). Durations round up to the event's slot granularity so all boundaries align; a missing `durationMinutes` is estimated as one slot and flagged `estimated`. Steps that name the same `appliance` and overlap are marked in `conflicts` and surfaced in recipe/event `warnings` — the engine reports contention but does not resolve it. `EventTimelineRecipe.startBy` is the earliest required start; `unschedulable` covers free-form slots and dependency cycles.

`EventRecipe.steps` is the slot's own copy of the recipe's steps (snapshot): linking a recipe copies `recipe_step` rows into `event_recipe_step`, and the step mutations below edit that copy — the shared recipe is never altered. `EventRecipe.items` is the same pattern for ingredients: `recipe_item` rows copy into `event_recipe_item` at link time, `baseServings` freezes the recipe's servings as the scaling denominator, and `items[].quantity` is reported scaled by `scalingFactor` (slot `servings` ÷ `baseServings`, or 1 when either is unset) while `baseQuantity` stays the raw per-base-servings amount. `syncEventRecipe` re-copies steps, items, and base servings, discarding slot edits.

- `addEventRecipeStep(eventRecipeId: ID!, input: EventRecipeStepInput!): EventRecipeStep!`
- `updateEventRecipeStep(id: ID!, input: EventRecipeStepInput!): EventRecipeStep!`
- `removeEventRecipeStep(id: ID!): Boolean!`
- `addEventRecipeItem(eventRecipeId: ID!, input: EventRecipeItemInput!): EventRecipeItem!`
- `updateEventRecipeItem(id: ID!, input: EventRecipeItemInput!): EventRecipeItem!`
- `removeEventRecipeItem(id: ID!): Boolean!`
- `syncEventRecipe(eventRecipeId: ID!): EventRecipe!`

`EventRecipeItemInput` takes `unit` as a name or abbreviation resolved through the shared unit catalog (like `RecipeItemInput`), plus `itemId`, `quantity` (the unscaled base amount), `section`, `displayOrder`, `notes`, and `isOptional`.

## Example operations

### Fetch current user and pantry

```graphql
query Dashboard {
  me {
    id
    email
    displayName
  }
  userItems(page: 1, pageSize: 20) {
    items {
      id
      item {
        name
        category { name }
      }
      currentQty
      minQty
    }
    pageInfo {
      pageNumber
      pageSize
      totalCount
    }
  }
}
```

### Create a recipe

```graphql
mutation CreatePasta {
  createRecipe(input: {
    name: "Pasta Marinara",
    servings: 4,
    items: [
      { itemId: "1", quantity: 1, unit: "box", isOptional: false },
      { itemId: "2", quantity: 24, unit: "oz", isOptional: false }
    ],
    steps: [
      { stepNumber: 1, instruction: "Boil water and cook pasta." },
      { stepNumber: 2, instruction: "Heat sauce and toss with pasta." }
    ]
  }) {
    id
    name
    items { item { name } quantity unit }
  }
}
```

### Generate a grocery list from a meal plan

```graphql
mutation GenerateGroceries($mealPlanId: ID!) {
  generateGroceryList(mealPlanId: $mealPlanId) {
    id
    generatedAt
    items {
      id
      item { name unit }
      manualItemName
      quantityNeeded
      unitOfMeasure
      source
      isChecked
    }
  }
}
```

### Toggle a grocery item

```graphql
mutation Toggle($id: ID!) {
  toggleGroceryItemChecked(groceryListItemId: $id) {
    id
    isChecked
  }
}
```

## Notes for clients

- All queries and mutations require a valid `Authorization: Bearer <id_token>` header.
- Pagination defaults to `page: 1` and `pageSize: 25`.
- Nullable `pageInfo.totalCount` in the resolver currently reflects the number of records returned for the requested page, not a global database count.

## Mobile redesign additions

Added for the Flutter mobile redesign (`mobile-redesign-p0` through `p4`):

- `Item.status: String!` and `Item.submittedByMe: Boolean!` for admin approval visibility.
- `itemByUpc(code: String!): Item` — UPC lookup respecting status/visibility rules.
- `submitItem(input: CreateItemInput!): Item!` — user item submission (creates a `pending` item).
- `approveItem(id: ID!): Item!` / `rejectItem(id: ID!): Item!` — admin moderation.
- `setItemNutrients(itemId: ID!, nutrients: [FoodNutrientInput!]!): [FoodNutrient!]!` — write path for nutrition data.
- `incrementUserItem(itemId: ID!, delta: Float!): UserItem` — delta-based pantry adjustment used by grocery sync and scan add/remove.
- `pendingItems(page: Int = 1, pageSize: Int = 25): ItemPage!` — admin-only list of pending items.
