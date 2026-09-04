# LENA — GraphQL Schema Specification

## 1. Endpoint

- Single public endpoint: `POST /graphql`
- All requests require `Authorization: Bearer <id_token>`.
- Response is JSON with `data` and/or `errors`.

## 2. Scalars

```graphql
scalar Time
scalar Date
```

## 3. Core Types

```graphql
type User {
  id: ID!
  email: String!
  displayName: String
  provider: String!
  externalSubject: String!
  lastLoginAt: Time
}

type PageInfo {
  pageNumber: Int!
  pageSize: Int!
  totalCount: Int!
}

type Category {
  id: ID!
  name: String!
  description: String
  isActive: Boolean!
}

type Brand {
  id: ID!
  name: String!
}

type Item {
  id: ID!
  name: String!
  brand: Brand
  upc12: String
  upc14: String
  category: Category!
  unit: String!
  isActive: Boolean!
}

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

type FlavorProfile {
  id: ID!
  name: String!
  isActive: Boolean!
}

type NutrientType {
  id: ID!
  name: String!
  unit: String
}

type Country {
  id: ID!
  name: String!
  isoCode: String!
  isActive: Boolean!
}

type Region {
  id: ID!
  country: Country!
  name: String!
  isActive: Boolean!
}

type WineType {
  id: ID!
  name: String!
  isActive: Boolean!
}

type Vintage {
  id: ID!
  year: Int!
  isActive: Boolean!
}

type GrapeVariety {
  id: ID!
  name: String!
}

type Bottle {
  id: ID!
  type: WineType!
  country: Country!
  region: Region!
  vintageYear: Int!
  vineyard: String
  abv: Float
  acidity: Int
  tanninLevel: Int
  body: Int
  sweetness: Int
  oakIntegration: Boolean
  bottleSize: String!
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

type Recipe {
  id: ID!
  name: String!
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  isActive: Boolean!
  isFavorite: Boolean!   # resolved from userprefs by BFF
  items: [RecipeItem!]!
  steps: [RecipeStep!]!
}

type RecipeItem {
  recipe: Recipe!
  item: Item!
  quantity: Float!
  unit: String!
  notes: String
  isOptional: Boolean!
}

type RecipeStep {
  id: ID!
  recipe: Recipe!
  stepNumber: Int!
  instruction: String!
}

type MealPlan {
  id: ID!
  name: String!
  weekStartDate: Date!
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

## 4. Inputs

```graphql
input CreateItemInput {
  name: String!
  brandId: ID
  upc12: String
  upc14: String
  categoryId: ID!
  unit: String!
}

input UpdateItemInput {
  name: String
  brandId: ID
  upc12: String
  upc14: String
  categoryId: ID
  unit: String
}

input CreateRecipeInput {
  name: String!
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  items: [RecipeItemInput!]!
  steps: [RecipeStepInput!]!
}

input RecipeItemInput {
  itemId: ID!
  quantity: Float!
  unit: String!
  notes: String
  isOptional: Boolean
}

input RecipeStepInput {
  stepNumber: Int!
  instruction: String!
}

input CreateMealPlanInput {
  name: String!
  weekStartDate: Date!
  weekStartDayOfWeek: Int
}

input AddMealSlotInput {
  mealPlanId: ID!
  dayOfWeek: Int!
  mealType: String!
  recipeId: ID
  servings: Int
  replacementNote: String
}
```

## 5. Queries

```graphql
type Query {
  me: User!

  # Inventory catalog
  items(page: Int = 1, pageSize: Int = 25, search: String, brand: String, inStock: Boolean, isFavorite: Boolean): ItemPage!
  item(id: ID!): Item

  # User inventory
  userItems(page: Int = 1, pageSize: Int = 25): UserItemPage!
  userItem(itemId: ID!): UserItem

  # Wine
  bottles(page: Int = 1, pageSize: Int = 25): BottlePage!
  bottle(id: ID!): Bottle
  userBottles(page: Int = 1, pageSize: Int = 25): UserBottlePage!

  # Recipes
  recipes(page: Int = 1, pageSize: Int = 25, favoritesOnly: Boolean): RecipePage!
  recipe(id: ID!): Recipe

  # Meal plans
  mealPlans(page: Int = 1, pageSize: Int = 25): MealPlanPage!
  mealPlan(id: ID!): MealPlan

  # Grocery
  groceryLists(page: Int = 1, pageSize: Int = 25): GroceryListPage!
  groceryList(id: ID!): GroceryList
}

type ItemPage { items: [Item!]!, pageInfo: PageInfo! }
type UserItemPage { items: [UserItem!]!, pageInfo: PageInfo! }
type BottlePage { items: [Bottle!]!, pageInfo: PageInfo! }
type UserBottlePage { items: [UserBottle!]!, pageInfo: PageInfo! }
type RecipePage { items: [Recipe!]!, pageInfo: PageInfo! }
type MealPlanPage { items: [MealPlan!]!, pageInfo: PageInfo! }
type GroceryListPage { items: [GroceryList!]!, pageInfo: PageInfo! }
```

## 6. Mutations

```graphql
type Mutation {
  # Catalog
  createItem(input: CreateItemInput!): Item!
  updateItem(id: ID!, input: UpdateItemInput!): Item!
  deleteItem(id: ID!): Boolean!

  # User inventory
  adjustUserItem(itemId: ID!, quantity: Float!, purchaseAt: Time): UserItem!
  setItemFavorite(itemId: ID!, isFavorite: Boolean!): UserItem!
  deleteUserItem(itemId: ID!): Boolean!

  # Recipes
  createRecipe(input: CreateRecipeInput!): Recipe!
  updateRecipe(id: ID!, input: CreateRecipeInput!): Recipe!
  deleteRecipe(id: ID!): Boolean!
  setRecipeFavorite(recipeId: ID!, isFavorite: Boolean!): Boolean!

  # Wine
  createBottle(input: CreateBottleInput!): Bottle!
  updateBottle(id: ID!, input: UpdateBottleInput!): Bottle!
  deleteBottle(id: ID!): Boolean!
  adjustUserBottle(bottleId: ID!, quantity: Int!): UserBottle!
  setBottleFavorite(bottleId: ID!, isFavorite: Boolean!): UserBottle!

  # Meal planning
  createMealPlan(input: CreateMealPlanInput!): MealPlan!
  updateMealPlan(id: ID!, input: CreateMealPlanInput!): MealPlan!
  deleteMealPlan(id: ID!): Boolean!
  addMealSlot(input: AddMealSlotInput!): MealSlot!
  removeMealSlot(slotId: ID!): Boolean!

  # Grocery
  generateGroceryList(mealPlanId: ID!): GroceryList!
  toggleGroceryItemChecked(groceryListItemId: ID!): GroceryListItem!
  deleteGroceryItem(groceryListItemId: ID!): Boolean!
}
```

> `CreateBottleInput` and `UpdateBottleInput` mirror the `Bottle` fields; omitted for brevity.

## 7. Example Queries

### Current user

```graphql
query {
  me { id email displayName lastLoginAt }
}
```

### Get grocery list with item details

```graphql
query {
  groceryList(id: "1") {
    id
    generatedAt
    items {
      id
      item { name unit }
      manualItemName
      quantityNeeded
      isChecked
    }
  }
}
```

### Generate a grocery list

```graphql
mutation {
  generateGroceryList(mealPlanId: "1") {
    id
    generatedAt
    items { id item { name } quantityNeeded isChecked }
  }
}
```