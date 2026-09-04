# LENA — GraphQL BFF Orchestration

## 1. The BFF's Role

The `internal/bff` package is the **only** package allowed to touch more than one domain module in a single request. It owns the GraphQL schema and resolvers. Domain modules (`inventory`, `recipe`, `mealplan`, etc.) must remain isolated and may not issue cross-domain SQL joins.

## 2. No Cross-Domain SQL Rule

- Each module's `sqlc` queries may only read tables in its own schema.
- A `mealplan.meal_slot` stores `recipe_id`. The `mealplan` module does not know recipe names.
- A `grocery.grocery_list_item` stores `item_id` and a `manual_item_name` fallback. The `grocery` module does not join `inventory.item`.
- The BFF resolves these IDs by calling the owning module's service.

## 3. Resolver Orchestration Pattern

```go
// Simplified pseudo-code
func (r *mealPlanResolver) Slots(ctx context.Context, obj *MealPlan) ([]*MealSlot, error) {
    slots, err := r.MealPlanService.GetSlotsForPlan(ctx, obj.ID)
    // slots contain only slot data + recipe_id
    return slots, err
}

func (r *mealSlotResolver) Recipe(ctx context.Context, obj *MealSlot) (*Recipe, error) {
    if obj.RecipeID == 0 {
        return nil, nil
    }
    return r.RecipeService.GetByID(ctx, obj.RecipeID)
}

func (r *groceryListItemResolver) Item(ctx context.Context, obj *GroceryListItem) (*Item, error) {
    if obj.ItemID == nil {
        return nil, nil
    }
    return r.InventoryService.GetByID(ctx, *obj.ItemID)
}

func (r *recipeResolver) IsFavorite(ctx context.Context, obj *Recipe) (bool, error) {
    u := currentuser.FromContext(ctx)
    return r.UserPrefService.RecipeIsFavorite(ctx, u.ID, obj.ID)
}
```

## 4. Domain Service Interfaces

Each module exposes a small, public Go interface that the BFF depends on:

```go
package inventory // module

type CatalogReader interface {
    GetItem(ctx context.Context, id int64) (*Item, error)
    SearchItems(ctx context.Context, q string, page, pageSize int) ([]*Item, int, error)
    GetBrand(ctx context.Context, id int64) (*Brand, error)
}

type CatalogWriter interface {
    CreateItem(ctx context.Context, in *CreateItemInput) (*Item, error)
    UpdateItem(ctx context.Context, id int64, in *UpdateItemInput) (*Item, error)
    DeleteItem(ctx context.Context, id int64) error
}
```

The BFF only knows these interfaces; it does not know SQL.

## 5. Avoiding N+1

The simplest N+1 mitigation is a **per-request data cache** or **DataLoader**:

1. On each HTTP request, the BFF creates an in-memory loader cache keyed by `(domain, id)`.
2. If the resolver for `MealSlot.recipe` asks for recipe ID `5` and a later resolver asks for the same ID, the second call returns the cached value.
3. A true DataLoader can be introduced later using `github.com/vikstrous/dataloadgen` to batch IDs into one `GetByIDs` call.

This is an optimization, not a relaxation of the no-join rule.

## 6. Error Handling

- Domain services return typed errors: `ErrNotFound`, `ErrValidation`, `ErrConflict`, etc.
- The BFF maps these to GraphQL `errors` with standard `message`, `path`, and `extensions.code`.
- Example extension code: `NOT_FOUND`, `VALIDATION`, `UNAUTHORIZED`.

## 7. GraphQL Implementation

The BFF uses `github.com/graph-gophers/graphql-go`, a schema-first library that maps GraphQL fields to Go struct methods by reflection.

- `internal/bff/schema.graphqls` — the single source of truth for the public schema.
- `internal/bff/resolver.go` — root `Resolver` plus per-type resolvers such as `itemResolver`, `recipeResolver`, `mealPlanResolver`, etc.
- `internal/bff/auth.go` — Echo middleware that validates OIDC ID tokens and injects `currentuser.User` into context.
- `internal/bff/model/scalars.go` — custom scalar helpers such as `Time`.

Resolvers return Go types that match the schema shape. For example, an `Item` resolver returns `*itemResolver`, which has methods `ID`, `Name`, `Brand(ctx)`, `Category(ctx)`, etc.

## 8. Scoping

- Every BFF resolver extracts `CurrentUser` from `context` (set by auth middleware).
- Catalog mutations are allowed for any authenticated user initially.
- Per-user mutations must pass `user_id` from `CurrentUser`; they never accept `userId` from the client.