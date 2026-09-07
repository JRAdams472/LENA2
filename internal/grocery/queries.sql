-- name: CreateGroceryList :one
INSERT INTO grocery.grocery_list (user_id, meal_plan_id, created_by, updated_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetGroceryListByID :one
SELECT *
FROM grocery.grocery_list
WHERE grocery_list_id = $1 AND user_id = $2;

-- name: ListGroceryLists :many
SELECT *
FROM grocery.grocery_list
WHERE user_id = $1
ORDER BY generated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountGroceryLists :one
SELECT COUNT(*)
FROM grocery.grocery_list
WHERE user_id = $1;

-- name: DeleteGroceryList :exec
DELETE FROM grocery.grocery_list
WHERE grocery_list_id = $1 AND user_id = $2;

-- name: AddGroceryListItem :one
INSERT INTO grocery.grocery_list_item (grocery_list_id, item_id, ingredient_id, manual_item_name, quantity_needed, unit_id, source, is_checked, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListGroceryListItems :many
SELECT gli.*
FROM grocery.grocery_list_item gli
JOIN grocery.grocery_list gl ON gli.grocery_list_id = gl.grocery_list_id
WHERE gli.grocery_list_id = $1 AND gl.user_id = $2
ORDER BY gli.grocery_list_item_id;

-- name: ListGroceryListItemsByLists :many
SELECT gli.*
FROM grocery.grocery_list_item gli
JOIN grocery.grocery_list gl ON gli.grocery_list_id = gl.grocery_list_id
WHERE gli.grocery_list_id = ANY(sqlc.arg(grocery_list_ids)::bigint[]) AND gl.user_id = sqlc.arg(user_id)
ORDER BY gli.grocery_list_item_id;

-- name: GetGroceryListItemByID :one
SELECT gli.*
FROM grocery.grocery_list_item gli
JOIN grocery.grocery_list gl ON gli.grocery_list_id = gl.grocery_list_id
WHERE gli.grocery_list_item_id = $1 AND gl.user_id = $2;

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
  AND gli.grocery_list_item_id = $1 AND gl.user_id = $2;

-- name: DeleteGroceryListItem :exec
DELETE FROM grocery.grocery_list_item gli
USING grocery.grocery_list gl
WHERE gli.grocery_list_id = gl.grocery_list_id
  AND gli.grocery_list_item_id = $1 AND gl.user_id = $2;
