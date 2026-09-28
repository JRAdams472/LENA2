# Recipe Categories — implementation plan

Add a grouped, per-group-exclusive category taxonomy for recipes
(admin-curated, member-assignable), with server-side search/category/favorite
filtering on `recipes()` and engagement-ranked ordering (favorites → used →
viewed → searched), plus the analytics events that feed the ranking, plus web
+ mobile UI — delivered in 4 phased PRs.

## Objective

Implement the `docs/newfeatures.md` "Recipe Categories" section plus the added
engagement-ranking requirement: recipes get categorized for easier search;
users filter by category in recipe search and meal planning; results are
ranked by the user's relationship to each recipe.

**Tags evaluation:** no recipe-tag feature exists today —
`FlavorProfile`/`WineFlavorProfile` are item/wine catalogs, unrelated to
recipes. Nothing to migrate; categories are the first recipe taxonomy.

**User decisions confirmed:**

- Admins curate the taxonomy; **members** may assign categories to recipes.
- Per-group `exclusive` flag; seeded groups all exclusive **except "Dish
  Type"** and "Dietary" (dietary attributes naturally combine).
- Filter semantics: **faceted** — AND across groups, OR within a group.
- Mobile gets full search + category filter (previously none).
- Ranking: **favorites first**, then used → viewed → searched → rest; other
  analytics signals act as in-tier tiebreakers.
- Whose history: **mixed** — "used" counts household menus (plans are
  shared); viewed/searched are personal.

## Current state (pre-change)

- `recipe.recipe` had no categorization; `recipes(page, pageSize)` took no
  filter args.
- Web `getRecipesPaged(search, isFavorite)` fetched **all** recipes and
  filtered client-side — removed by this feature.
- `menu_add` fired only in `AddMealSlot`; `AddEventRecipe` fired nothing.
  `recipe_selected`/`recipe_searched` event types existed but clients never
  fired them.
- `allowedSchemas` guard: `recipe` may only query `recipe`; `analytics` may
  read `analytics,recipe,mealplan,identity` — engagement sets are computed in
  the analytics layer and passed to the recipe query as ID/term arrays.
- `isFavorite` lives in `userprefs.user_recipe_preference` — same two-step
  treatment (favorite IDs fetched first, passed as include/exclude arrays).

## Analytics additions

- **`recipe_viewed` event** (weight 1) — `EventRecipeViewed` constant +
  `eventWeights` entry. Fired by clients when a recipe detail is opened.
- **`recordView(entityType, entityId): Boolean!` mutation** — inserts the
  interaction event **without** upserting `user_selection_count` /
  `global_selection_count` (views aren't picks). Implemented as
  `analytics.Service.RecordView` → shared `recordEvent(..., countSelection=false)`.
- **`menu_add` on food events** — `AddEventRecipe` fires `menu_add` with
  `EntityType=recipe`.
- **`RecipeEngagementSets`** (analytics queries; `HouseholdUsedRecipeIDs`
  counts `menu_add` + `recipe_selected` events by household members —
  as-implemented delta from the plan's menu_add-only spec, since a picker
  pick is the same usage intent and usually co-occurs):
  - `UsedIDs` — household recipe menu adds, most-used first.
  - `ViewedIDs` — caller's recipe views, most-viewed first.
  - `SearchTerms` — caller's distinct recipe search terms.
- Client wiring (p2/p3): `recordView` on recipe open, `recordSearch` on the
  recipe search box, `recordSelection("recipe", id)` on picker picks.

## Data model — migration `0035_recipe_categories.{up,down}.sql`

```sql
recipe.category_group (category_group_id, name UNIQUE, exclusive, display_order, audit)
recipe.category       (category_id, category_group_id -> RESTRICT, name,
                       UNIQUE(category_group_id, name), audit)
recipe.recipe_category (recipe_id -> CASCADE, category_id -> CASCADE,
                        assigned_by, assigned_at, PK(recipe_id, category_id))
```

`lena_app` grants matching 0031–0034. Seed taxonomy:

| Group (order) | exclusive | Categories |
|---|---|---|
| Course (1) | yes | Breakfast, Lunch, Dinner, Dessert, Snack, Side |
| Dish Type (2) | no | Cocktail, Soup, Bread, Casserole, Single Pan |
| Dietary (3) | no | Low Calorie |
| Difficulty (4) | yes | Easy, Medium, Skilled |
| Main Ingredient (5) | yes | Chicken, Beef, Fish, Pork, Vegetarian |
| Cuisine (6) | yes | Italian, Spanish, Mexican, Southwestern US, French, American, Asian |

## GraphQL surface

```graphql
type RecipeCategoryGroup { id: ID!  name: String!  exclusive: Boolean!  displayOrder: Int!  categories: [RecipeCategory!]! }
type RecipeCategory      { id: ID!  name: String!  group: RecipeCategoryGroup! }

Recipe.categories: [RecipeCategory!]!

Query.recipeCategoryGroups: [RecipeCategoryGroup!]!
Query.recipes(page, pageSize, search, categoryIds, isFavorite): RecipePage!

Mutation.setRecipeCategories(recipeId, categoryIds): Recipe!            # member-level
Mutation.createRecipeCategoryGroup / updateRecipeCategoryGroup / deleteRecipeCategoryGroup  # @admin
Mutation.createRecipeCategory / updateRecipeCategory / deleteRecipeCategory                 # @admin
Mutation.recordView(entityType, entityId): Boolean!
CreateRecipeInput.categoryIds: [ID!]   # shared by createRecipe + updateRecipe
```

All new query args are nullable → backward compatible.

## Faceted search SQL (recipe schema only)

`SearchRecipes` filters `is_active`, optional `LIKE` name search, optional
category set, optional include/exclude ID sets (favorites). Category matching
is faceted: a recipe matches when the distinct **groups** its assignments
cover equals the distinct groups present in the filter — i.e. AND across
groups, OR within a group.

Ordering: `CASE` tier (favorite → used → viewed → searched-name-match →
rest), then `array_position` inside the used/viewed arrays (pre-sorted by
signal strength so position doubles as the in-tier tiebreaker), then name.

## Phases (branch + PR each, per AGENTS.md)

### Phase 1 — `recipe-categories-p1`: server ✅ (this PR)

- Migration 0035 + down.
- sqlc queries: group/category CRUD, `SetRecipeCategories` (clear + unnest
  insert inside the service's existing `withTx`), `ListCategoriesForRecipes`
  batch preload, `SearchRecipes`/`CountSearchRecipes`; mocks regenerated.
- `internal/recipe`: `CategoryGroup`/`Category` domain types; CRUD;
  `SetRecipeCategories` with exclusivity validation (`ValidationError` on two
  same-exclusive-group picks — surfaces as `BAD_USER_INPUT` via GraphQL);
  `SearchRecipes`/`CountSearchRecipes`.
- `internal/analytics`: `EventRecipeViewed`; `RecordView` (no count upserts);
  `RecipeEngagementSets`; `menu_add` fired in `AddEventRecipe`.
- BFF: schema + resolvers (types, `recipeCategoryGroups`, extended `recipes`,
  admin CRUD, `setRecipeCategories`, `recordView`); `Recipe.categories` via
  `loadRecipeChildren` (no N+1); `categoryIds` on `createRecipe`/`updateRecipe`
  composed inside the same `unitOfWork().InTx`.
- Tests: recipe service unit tests (CRUD, exclusivity, search params);
  BFF resolver tests (taxonomy query, categories field, assignment,
  exclusivity surfacing, admin guards, `recordView`); real-Postgres
  integration test (`TestIntegrationRecipeCategories`) covering seed data,
  assignment, exclusivity, faceted AND/OR, filters, and tier ordering.
- `userprefs.ListFavoriteRecipeIDs` added for the favorite filter/boost
  two-step.

### Phase 2 — `recipe-categories-p2`: web

- `lib/api.ts`/`types.ts`: category types; `categories` in recipe fields;
  `getRecipeCategoryGroups`, `setRecipeCategories`, `recordView`;
  `getRecipesPaged` → server-side `search`/`categoryIds`/`isFavorite`
  (delete fetch-all fallback).
- Recipes page: grouped filter bar (exclusive = single-select, otherwise
  multi), chips; debounced `recordSearch("recipe", term)`.
- Recipe detail: category chips; picker in edit form; `recordView` on open.
- Meal-plan picker: category filter; `recordSelection("recipe", id)` on pick.
- Admin page for group/category CRUD (follow `/inventory/categories`).
- Jest tests.

### Phase 3 — `recipe-categories-p3`: mobile

- Recipes screen: search field + grouped category filter; extended
  `recipes()` args; `recordSearch`/`recordView`.
- Edit recipe screen: category picker.
- Meal-plan picker: category filter + `recordSelection`.
- Category chips on list items; widget/unit tests.

### Phase 4 — `recipe-categories-p4`: closeout

- README feature note; wiki `Recipes.md` + fresh screenshots via
  `docs/wiki-screenshots.md`; `Tips-and-FAQ` tip.
- Playwright e2e: categorize → filter narrows; engagement smoke.
- Mark the section done in `docs/newfeatures.md`; branch cleanup.

## Verification per phase

- `go build ./...`, `go vet`, `gofmt`, `golangci-lint`,
  `go test -short ./cmd/... ./internal/...`; testcontainers integration.
- Web: `npx tsc --noEmit`, `npm test`, ESLint, `next build`.
  Mobile: `flutter analyze`, `flutter test`. e2e in CI.

## Risks / notes

- **Cold start:** engagement tiers are empty until clients fire events —
  ranking degrades gracefully to name order and improves as usage accrues.
- **Global assignments:** member-assigned categories affect the shared recipe
  catalog (per the admins-curate/members-assign decision). Assignment UI
  lives in the recipe edit form (admin); `setRecipeCategories` stays
  member-permitted at the API for future surfaces.
- **View ≠ selection:** `recordView` deliberately doesn't bump
  `user_selection_count`; in-tier ordering uses per-event-type counts from
  `interaction_event`.
- **"Searched" is fuzzy** — past search terms match against recipe names.
- `deleteRecipeCategory` cascades assignments; `deleteRecipeCategoryGroup` is
  rejected while categories exist.
- Event-local recipe snapshots unaffected — categories are search metadata,
  not event data.
