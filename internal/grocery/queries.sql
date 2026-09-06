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
SELECT *
FROM grocery.grocery_list_item
WHERE grocery_list_id = $1
ORDER BY grocery_list_item_id;

-- name: ListGroceryListItemsByLists :many
SELECT *
FROM grocery.grocery_list_item
WHERE grocery_list_id = ANY(sqlc.arg(grocery_list_ids)::bigint[])
ORDER BY grocery_list_item_id;

-- name: GetGroceryListItemByID :one
SELECT *
FROM grocery.grocery_list_item
WHERE grocery_list_item_id = $1;

-- name: UpdateGroceryListItem :exec
UPDATE grocery.grocery_list_item
SET item_id          = $2,
    ingredient_id    = $3,
    manual_item_name = $4,
    quantity_needed  = $5,
    unit_id          = $6,
    source           = $7,
    is_checked       = $8,
    updated_by       = $9,
    updated_at       = now()
WHERE grocery_list_item_id = $1;

-- name: DeleteGroceryListItem :exec
DELETE FROM grocery.grocery_list_item
WHERE grocery_list_item_id = $1;
