# LENA GraphQL Schema

This document describes the public GraphQL API exposed by the BFF at `/graphql`.

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
| `email` | `String!` | Primary email address |
| `displayName` | `String` | Optional display name |

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

### Grocery — `GroceryList`, `GroceryListItem`

```graphql
type GroceryList {
  id: ID!
  generatedAt: Time!
  items: [GroceryListItem!]!
}

type GroceryListItem {
  id: ID!
  item: Item
  manualItemName: String
  quantityNeeded: Float!
  unitOfMeasure: String
  source: String!
  isChecked: Boolean!
}
```

## Queries

| Query | Arguments | Returns | Description |
|---|---|---|---|
| `me` | — | `User!` | Current authenticated user |
| `brand(id)` | `ID!` | `Brand` | Single catalog brand |
| `brands` | — | `[Brand!]!` | All brands |
| `category(id)` | `ID!` | `Category` | Single category |
| `categories` | — | `[Category!]!` | All categories |
| `item(id)` | `ID!` | `Item` | Single catalog item |
| `items(page, pageSize)` | `Int, Int` | `ItemPage!` | Paginated catalog items |
| `recipe(id)` | `ID!` | `Recipe` | Single recipe |
| `recipes(page, pageSize)` | `Int, Int` | `RecipePage!` | Paginated recipes |
| `userItems(page, pageSize)` | `Int, Int` | `UserItemPage!` | Current user's pantry |
| `userBottles(page, pageSize)` | `Int, Int` | `UserBottlePage!` | Current user's cellar |
| `bottle(id)` | `ID!` | `Bottle` | Single wine bottle |
| `bottles(page, pageSize)` | `Int, Int` | `BottlePage!` | Paginated wine bottles |
| `mealPlan(id)` | `ID!` | `MealPlan` | Single meal plan |
| `mealPlans(page, pageSize)` | `Int, Int` | `MealPlanPage!` | Current user's plans |
| `groceryList(id)` | `ID!` | `GroceryList` | Single grocery list |
| `groceryLists(page, pageSize)` | `Int, Int` | `GroceryListPage!` | Current user's lists |

## Mutations

### Inventory

- `createBrand(input: CreateBrandInput!): Brand!`
- `createCategory(input: CreateCategoryInput!): Category!`
- `createItem(input: CreateItemInput!): Item!`
- `updateItem(id: ID!, input: UpdateItemInput!): Item!`
- `deleteItem(id: ID!): Boolean!`

### Recipes

- `createRecipe(input: CreateRecipeInput!): Recipe!`
- `updateRecipe(id: ID!, input: CreateRecipeInput!): Recipe!`
- `deleteRecipe(id: ID!): Boolean!`
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
- `deleteGroceryItem(groceryListItemId: ID!): Boolean!`

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
