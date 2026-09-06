-- name: UpsertUserItem :one
INSERT INTO inventory.user_item (
    user_id, item_id, current_qty, min_qty, purchase_at, expires_at, notes, is_favorite, created_by, updated_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id, item_id)
    DO UPDATE SET
        current_qty = EXCLUDED.current_qty,
        min_qty     = EXCLUDED.min_qty,
        purchase_at = EXCLUDED.purchase_at,
        expires_at  = EXCLUDED.expires_at,
        notes       = EXCLUDED.notes,
        is_favorite = EXCLUDED.is_favorite,
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: GetUserItemByID :one
SELECT *
FROM inventory.user_item
WHERE user_item_id = $1 AND user_id = $2;

-- name: ListUserItems :many
SELECT *
FROM inventory.user_item
WHERE user_id = $1
ORDER BY updated_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountUserItems :one
SELECT COUNT(*)
FROM inventory.user_item
WHERE user_id = $1;

-- name: DeleteUserItem :exec
DELETE FROM inventory.user_item
WHERE user_item_id = $1 AND user_id = $2;

-- name: UpsertUserBottle :one
INSERT INTO wine.user_bottle (
    user_id, bottle_id, bottle_number, quantity, purchase_at, purchase_price,
    storage_temp, location, notes, is_favorite, created_by, updated_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (user_id, bottle_id)
    DO UPDATE SET
        bottle_number  = EXCLUDED.bottle_number,
        quantity       = EXCLUDED.quantity,
        purchase_at    = EXCLUDED.purchase_at,
        purchase_price = EXCLUDED.purchase_price,
        storage_temp   = EXCLUDED.storage_temp,
        location       = EXCLUDED.location,
        notes          = EXCLUDED.notes,
        is_favorite    = EXCLUDED.is_favorite,
        updated_by     = EXCLUDED.updated_by,
        updated_at     = now()
RETURNING *;

-- name: GetUserBottleByID :one
SELECT *
FROM wine.user_bottle
WHERE user_bottle_id = $1 AND user_id = $2;

-- name: ListUserBottles :many
SELECT *
FROM wine.user_bottle
WHERE user_id = $1
ORDER BY updated_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountUserBottles :one
SELECT COUNT(*)
FROM wine.user_bottle
WHERE user_id = $1;

-- name: DeleteUserBottle :exec
DELETE FROM wine.user_bottle
WHERE user_bottle_id = $1 AND user_id = $2;

-- name: UpsertRecipeFavorite :one
INSERT INTO recipe.user_recipe_preference (user_id, recipe_id, is_favorite, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, recipe_id)
    DO UPDATE SET
        is_favorite = EXCLUDED.is_favorite,
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: GetRecipeFavorite :one
SELECT *
FROM recipe.user_recipe_preference
WHERE user_id = $1 AND recipe_id = $2;

-- name: ListRecipeFavorites :many
SELECT *
FROM recipe.user_recipe_preference
WHERE user_id = $1 AND recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[]);

-- name: DeleteRecipeFavorite :exec
DELETE FROM recipe.user_recipe_preference
WHERE user_id = $1 AND recipe_id = $2;
