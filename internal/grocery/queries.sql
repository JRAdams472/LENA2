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

-- name: UpdateGroceryListItem :exec
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
