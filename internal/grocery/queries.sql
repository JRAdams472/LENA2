-- name: CreateGroceryList :one
INSERT INTO grocery.grocery_list (household_id, meal_plan_id, created_by, updated_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetGroceryListByID :one
SELECT *
FROM grocery.grocery_list
WHERE grocery_list_id = $1 AND household_id = $2;

-- name: ListGroceryLists :many
SELECT *
FROM grocery.grocery_list
WHERE household_id = $1
ORDER BY generated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountGroceryLists :one
SELECT COUNT(*)
FROM grocery.grocery_list
WHERE household_id = $1;

-- name: DeleteGroceryList :exec
DELETE FROM grocery.grocery_list
WHERE grocery_list_id = $1 AND household_id = $2;

-- name: GetLatestGroceryListByPlan :one
SELECT *
FROM grocery.grocery_list
WHERE household_id = $1 AND meal_plan_id = $2
ORDER BY generated_at DESC, grocery_list_id DESC
LIMIT 1;

-- name: DeleteGeneratedGroceryListItems :exec
-- Regenerate-in-place: generated lines (source <> 'manual') are replaced
-- wholesale; manual lines are preserved.
DELETE FROM grocery.grocery_list_item
WHERE grocery_list_id = $1 AND source <> 'manual';

-- name: TouchGroceryListGeneratedAt :one
UPDATE grocery.grocery_list
SET generated_at = now(),
    updated_by   = $3,
    updated_at   = now()
WHERE grocery_list_id = $1 AND household_id = $2
RETURNING *;

-- name: ReassignGroceryListsToHousehold :exec
-- Invite-accept merge: repoint all of the source household's lists. Zero
-- rows is not an error.
UPDATE grocery.grocery_list
SET household_id = sqlc.arg(to_household_id),
    updated_by   = sqlc.arg(updated_by)::varchar,
    updated_at   = now()
WHERE household_id = sqlc.arg(from_household_id);

-- name: AddGroceryListItem :one
INSERT INTO grocery.grocery_list_item (grocery_list_id, item_id, ingredient_id, manual_item_name, quantity_needed, unit_id, source, is_checked, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListGroceryListItems :many
SELECT gli.*
FROM grocery.grocery_list_item gli
JOIN grocery.grocery_list gl ON gli.grocery_list_id = gl.grocery_list_id
WHERE gli.grocery_list_id = $1 AND gl.household_id = $2
ORDER BY gli.grocery_list_item_id;

-- name: ListGroceryListItemsByLists :many
SELECT gli.*
FROM grocery.grocery_list_item gli
JOIN grocery.grocery_list gl ON gli.grocery_list_id = gl.grocery_list_id
WHERE gli.grocery_list_id = ANY(sqlc.arg(grocery_list_ids)::bigint[]) AND gl.household_id = sqlc.arg(household_id)
ORDER BY gli.grocery_list_item_id;

-- name: GetGroceryListItemByID :one
SELECT gli.*
FROM grocery.grocery_list_item gli
JOIN grocery.grocery_list gl ON gli.grocery_list_id = gl.grocery_list_id
WHERE gli.grocery_list_item_id = $1 AND gl.household_id = $2;

-- name: UpdateGroceryListItem :execrows
UPDATE grocery.grocery_list_item gli
SET item_id          = $3,
    ingredient_id    = $4,
    manual_item_name = $5,
    quantity_needed  = $6,
    unit_id          = $7,
    source           = $8,
    is_checked       = $9,
    updated_by       = $10,
    updated_at       = now()
FROM grocery.grocery_list gl
WHERE gli.grocery_list_id = gl.grocery_list_id
  AND gli.grocery_list_item_id = $1 AND gl.household_id = $2;

-- name: DeleteGroceryListItem :exec
DELETE FROM grocery.grocery_list_item gli
USING grocery.grocery_list gl
WHERE gli.grocery_list_id = gl.grocery_list_id
  AND gli.grocery_list_item_id = $1 AND gl.household_id = $2;

-- name: ToggleGroceryListItemChecked :one
-- Checking stamps checked_at and assigns the next per-list checked_seq so
-- the order items were checked off survives for store-routing analytics;
-- unchecking clears both.
UPDATE grocery.grocery_list_item gli
SET is_checked = NOT gli.is_checked,
    checked_at = CASE WHEN NOT gli.is_checked THEN now() ELSE NULL END,
    checked_seq = CASE WHEN NOT gli.is_checked
        THEN COALESCE((
            SELECT MAX(i2.checked_seq)
            FROM grocery.grocery_list_item i2
            WHERE i2.grocery_list_id = gli.grocery_list_id
        ), 0) + 1
        ELSE NULL END,
    updated_by = $3,
    updated_at = now()
FROM grocery.grocery_list gl
WHERE gli.grocery_list_id = gl.grocery_list_id
  AND gli.grocery_list_item_id = $1
  AND gl.household_id = $2
RETURNING gli.*;

-- name: CreateStore :one
INSERT INTO grocery.store (household_id, name, created_by, updated_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetStoreByID :one
SELECT * FROM grocery.store
WHERE store_id = $1 AND household_id = $2;

-- name: ListStores :many
SELECT * FROM grocery.store
WHERE household_id = $1
ORDER BY name, store_id;

-- name: RenameStore :one
UPDATE grocery.store
SET name = $3, updated_by = $4, updated_at = now()
WHERE store_id = $1 AND household_id = $2
RETURNING *;

-- name: DeleteStore :exec
DELETE FROM grocery.store
WHERE store_id = $1 AND household_id = $2;

-- name: GetLatestListStore :one
-- New lists inherit the household's most recently used store so web and
-- mobile default identically.
SELECT store_id FROM grocery.grocery_list
WHERE household_id = $1 AND store_id IS NOT NULL
ORDER BY generated_at DESC, grocery_list_id DESC
LIMIT 1;

-- name: SetGroceryListStore :execrows
UPDATE grocery.grocery_list
SET store_id = $3, updated_by = $4, updated_at = now()
WHERE grocery_list_id = $1 AND household_id = $2;

-- name: CreateAisle :one
INSERT INTO grocery.store_aisle (store_id, name, position, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAisleByID :one
SELECT sa.* FROM grocery.store_aisle sa
JOIN grocery.store s ON sa.store_id = s.store_id
WHERE sa.aisle_id = $1 AND s.household_id = $2;

-- name: ListAisles :many
SELECT sa.* FROM grocery.store_aisle sa
JOIN grocery.store s ON sa.store_id = s.store_id
WHERE sa.store_id = $1 AND s.household_id = $2
ORDER BY sa.position, sa.aisle_id;

-- name: RenameAisle :execrows
UPDATE grocery.store_aisle sa
SET name = $3, updated_by = $4, updated_at = now()
FROM grocery.store s
WHERE sa.store_id = s.store_id
  AND sa.aisle_id = $1 AND s.household_id = $2;

-- name: DeleteAisle :exec
DELETE FROM grocery.store_aisle sa
USING grocery.store s
WHERE sa.store_id = s.store_id
  AND sa.aisle_id = $1 AND s.household_id = $2;

-- name: ReorderAisles :exec
-- Rewrite every aisle's position from the submitted order in one
-- statement: position is the row's index in the id array.
UPDATE grocery.store_aisle sa
SET position = ord.idx - 1,
    updated_by = sqlc.arg(updated_by)::varchar,
    updated_at = now()
FROM (
    SELECT aisle_id, ordinality AS idx
    FROM unnest(sqlc.arg(aisle_ids)::bigint[]) WITH ORDINALITY AS t(aisle_id, ordinality)
) ord
JOIN grocery.store s ON s.store_id = sqlc.arg(store_id)
WHERE sa.aisle_id = ord.aisle_id
  AND sa.store_id = s.store_id
  AND s.household_id = sqlc.arg(household_id);

-- name: AssignItemToAisle :one
-- Identity is exactly one of item_id/ingredient_id/manual_name; the
-- three partial unique indexes make each variant an upsert.
INSERT INTO grocery.aisle_assignment
    (store_id, aisle_id, item_id, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (store_id, item_id) WHERE item_id IS NOT NULL
    DO UPDATE SET aisle_id = EXCLUDED.aisle_id, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: AssignIngredientToAisle :one
INSERT INTO grocery.aisle_assignment
    (store_id, aisle_id, ingredient_id, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (store_id, ingredient_id) WHERE ingredient_id IS NOT NULL
    DO UPDATE SET aisle_id = EXCLUDED.aisle_id, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: AssignManualToAisle :one
INSERT INTO grocery.aisle_assignment
    (store_id, aisle_id, manual_name, created_by, updated_by)
VALUES (sqlc.arg(store_id), sqlc.arg(aisle_id), sqlc.arg(manual_name)::varchar,
        sqlc.arg(created_by), sqlc.arg(updated_by))
ON CONFLICT (store_id, manual_name) WHERE manual_name IS NOT NULL
    DO UPDATE SET aisle_id = EXCLUDED.aisle_id, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: UnassignItem :execrows
DELETE FROM grocery.aisle_assignment aa
USING grocery.store s
WHERE aa.store_id = s.store_id
  AND aa.store_id = $1 AND s.household_id = $2
  AND (aa.item_id = $3 OR aa.ingredient_id = $4 OR aa.manual_name = $5);

-- name: ListAssignments :many
SELECT aa.* FROM grocery.aisle_assignment aa
JOIN grocery.store s ON aa.store_id = s.store_id
WHERE aa.store_id = $1 AND s.household_id = $2;

-- name: UpsertRouteObservationItem :one
-- One check-off contributes normalized position seq/total to the running
-- learned mean for the item's identity at the list's store (0 = generic).
INSERT INTO grocery.item_route
    (household_id, store_id, item_id, learned_sum, learned_count, created_by, updated_by)
VALUES ($1, $2, $3, $4, 1, $5, $6)
ON CONFLICT (household_id, store_id, item_id) WHERE item_id IS NOT NULL
    DO UPDATE SET learned_sum = grocery.item_route.learned_sum + EXCLUDED.learned_sum,
                  learned_count = grocery.item_route.learned_count + 1,
                  updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: UpsertRouteObservationIngredient :one
INSERT INTO grocery.item_route
    (household_id, store_id, ingredient_id, learned_sum, learned_count, created_by, updated_by)
VALUES ($1, $2, $3, $4, 1, $5, $6)
ON CONFLICT (household_id, store_id, ingredient_id) WHERE ingredient_id IS NOT NULL
    DO UPDATE SET learned_sum = grocery.item_route.learned_sum + EXCLUDED.learned_sum,
                  learned_count = grocery.item_route.learned_count + 1,
                  updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: UpsertRouteObservationManual :one
INSERT INTO grocery.item_route
    (household_id, store_id, manual_name, learned_sum, learned_count, created_by, updated_by)
VALUES (sqlc.arg(household_id), sqlc.arg(store_id), sqlc.arg(manual_name)::varchar,
        sqlc.arg(learned_sum), 1, sqlc.arg(created_by), sqlc.arg(updated_by))
ON CONFLICT (household_id, store_id, manual_name) WHERE manual_name IS NOT NULL
    DO UPDATE SET learned_sum = grocery.item_route.learned_sum + EXCLUDED.learned_sum,
                  learned_count = grocery.item_route.learned_count + 1,
                  updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: ListItemRoutes :many
-- Store-specific rows plus the generic (store_id = 0) fallback.
SELECT ir.* FROM grocery.item_route ir
WHERE ir.household_id = $1 AND ir.store_id IN ($2, 0);

-- name: UpsertManualRankItem :exec
-- Manual arrangement is written even when no learned row exists yet.
INSERT INTO grocery.item_route
    (household_id, store_id, item_id, manual_rank, manual_at, created_by, updated_by)
VALUES ($1, $2, $3, $4, now(), $5, $6)
ON CONFLICT (household_id, store_id, item_id) WHERE item_id IS NOT NULL
    DO UPDATE SET manual_rank = EXCLUDED.manual_rank, manual_at = now(),
                  updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: UpsertManualRankIngredient :exec
INSERT INTO grocery.item_route
    (household_id, store_id, ingredient_id, manual_rank, manual_at, created_by, updated_by)
VALUES ($1, $2, $3, $4, now(), $5, $6)
ON CONFLICT (household_id, store_id, ingredient_id) WHERE ingredient_id IS NOT NULL
    DO UPDATE SET manual_rank = EXCLUDED.manual_rank, manual_at = now(),
                  updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: UpsertManualRankManual :exec
INSERT INTO grocery.item_route
    (household_id, store_id, manual_name, manual_rank, manual_at, created_by, updated_by)
VALUES (sqlc.arg(household_id), sqlc.arg(store_id), sqlc.arg(manual_name)::varchar,
        sqlc.arg(manual_rank), now(), sqlc.arg(created_by), sqlc.arg(updated_by))
ON CONFLICT (household_id, store_id, manual_name) WHERE manual_name IS NOT NULL
    DO UPDATE SET manual_rank = EXCLUDED.manual_rank, manual_at = now(),
                  updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: ResetStoreRoute :exec
DELETE FROM grocery.item_route ir
USING grocery.store s
WHERE s.store_id = sqlc.arg(store_id)
  AND s.household_id = sqlc.arg(household_id)
  AND ir.household_id = s.household_id
  AND (ir.store_id = s.store_id OR ir.store_id = 0);
