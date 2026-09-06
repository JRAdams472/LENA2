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
INSERT INTO grocery.grocery_list_item (grocery_list_id, item_id, manual_item_name, quantity_needed, unit_of_measure, source, is_checked, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListGroceryListItems :many
SELECT *
FROM grocery.grocery_list_item
WHERE grocery_list_id = $1
ORDER BY grocery_list_item_id;

-- name: GetGroceryListItemByID :one
SELECT *
FROM grocery.grocery_list_item
WHERE grocery_list_item_id = $1;

-- name: UpdateGroceryListItem :exec
UPDATE grocery.grocery_list_item
SET item_id          = $2,
    manual_item_name = $3,
    quantity_needed  = $4,
    unit_of_measure  = $5,
    source           = $6,
    is_checked       = $7,
    updated_by       = $8,
    updated_at       = now()
WHERE grocery_list_item_id = $1;

-- name: DeleteGroceryListItem :exec
DELETE FROM grocery.grocery_list_item
WHERE grocery_list_item_id = $1;
