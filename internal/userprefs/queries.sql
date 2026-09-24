-- name: UpsertHouseholdItem :one
INSERT INTO userprefs.household_item (
    household_id, item_id, current_qty, min_qty, purchase_at, expires_at, notes, created_by, updated_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (household_id, item_id)
    DO UPDATE SET
        current_qty = EXCLUDED.current_qty,
        min_qty     = EXCLUDED.min_qty,
        purchase_at = EXCLUDED.purchase_at,
        expires_at  = EXCLUDED.expires_at,
        notes       = EXCLUDED.notes,
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: AdjustHouseholdItemQuantity :one
-- Atomically adjust the household's pantry quantity by delta, clamping at 0.
-- Creates the row if it does not yet exist, preserving all other fields
-- on an existing row.
INSERT INTO userprefs.household_item (
    household_id, item_id, current_qty, min_qty, purchase_at, expires_at, notes, created_by, updated_by
)
VALUES ($1, $2, GREATEST(0::numeric, sqlc.arg(delta)::numeric), 0::numeric, NULL, NULL, NULL, $3, $3)
ON CONFLICT (household_id, item_id)
    DO UPDATE SET
        current_qty = GREATEST(0::numeric, userprefs.household_item.current_qty + sqlc.arg(delta)::numeric),
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: GetHouseholdItemByID :one
SELECT *
FROM userprefs.household_item
WHERE household_item_id = $1 AND household_id = $2;

-- name: GetHouseholdItemByItem :one
SELECT *
FROM userprefs.household_item
WHERE household_id = $1 AND item_id = $2;

-- name: ListHouseholdItems :many
SELECT *
FROM userprefs.household_item
WHERE household_id = $1
ORDER BY updated_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountHouseholdItems :one
SELECT COUNT(*)
FROM userprefs.household_item
WHERE household_id = $1;

-- name: DeleteHouseholdItem :execrows
DELETE FROM userprefs.household_item
WHERE household_item_id = $1 AND household_id = $2;

-- name: UpsertHouseholdBottle :one
INSERT INTO userprefs.household_bottle (
    household_id, bottle_id, bottle_number, quantity, purchase_at, purchase_price,
    storage_temp, location, notes, created_by, updated_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (household_id, bottle_id)
    DO UPDATE SET
        bottle_number  = EXCLUDED.bottle_number,
        quantity       = EXCLUDED.quantity,
        purchase_at    = EXCLUDED.purchase_at,
        purchase_price = EXCLUDED.purchase_price,
        storage_temp   = EXCLUDED.storage_temp,
        location       = EXCLUDED.location,
        notes          = EXCLUDED.notes,
        updated_by     = EXCLUDED.updated_by,
        updated_at     = now()
RETURNING *;

-- name: GetHouseholdBottleByID :one
SELECT *
FROM userprefs.household_bottle
WHERE household_bottle_id = $1 AND household_id = $2;

-- name: GetHouseholdBottleByBottle :one
SELECT *
FROM userprefs.household_bottle
WHERE household_id = $1 AND bottle_id = $2;

-- name: ListHouseholdBottles :many
SELECT *
FROM userprefs.household_bottle
WHERE household_id = $1
ORDER BY updated_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountHouseholdBottles :one
SELECT COUNT(*)
FROM userprefs.household_bottle
WHERE household_id = $1;

-- name: DeleteHouseholdBottle :execrows
DELETE FROM userprefs.household_bottle
WHERE household_bottle_id = $1 AND household_id = $2;

-- ---------- per-user favorites (never household-scoped) ----------

-- name: SetUserItemFavorite :one
INSERT INTO userprefs.user_item_favorite (user_id, item_id, is_favorite, created_by, updated_by)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (user_id, item_id)
    DO UPDATE SET
        is_favorite = EXCLUDED.is_favorite,
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: ListUserItemFavorites :many
SELECT *
FROM userprefs.user_item_favorite
WHERE user_id = $1 AND item_id = ANY(sqlc.arg(item_ids)::bigint[]);

-- name: DeleteUserItemFavorite :exec
DELETE FROM userprefs.user_item_favorite
WHERE user_id = $1 AND item_id = $2;

-- name: SetUserBottleFavorite :one
INSERT INTO userprefs.user_bottle_favorite (user_id, bottle_id, is_favorite, created_by, updated_by)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (user_id, bottle_id)
    DO UPDATE SET
        is_favorite = EXCLUDED.is_favorite,
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: ListUserBottleFavorites :many
SELECT *
FROM userprefs.user_bottle_favorite
WHERE user_id = $1 AND bottle_id = ANY(sqlc.arg(bottle_ids)::bigint[]);

-- name: DeleteUserBottleFavorite :exec
DELETE FROM userprefs.user_bottle_favorite
WHERE user_id = $1 AND bottle_id = $2;

-- ---------- household merge (invite accept) ----------

-- name: MergeHouseholdItemConflicts :exec
-- For items present in both households, fold the source row into the
-- target: quantities sum, min_qty takes the max, timestamps keep the most
-- recent non-null, notes prefer the incoming non-null value.
UPDATE userprefs.household_item dst
SET current_qty = dst.current_qty + src.current_qty,
    min_qty     = GREATEST(dst.min_qty, src.min_qty),
    purchase_at = GREATEST(dst.purchase_at, src.purchase_at),
    expires_at  = GREATEST(dst.expires_at, src.expires_at),
    notes       = COALESCE(src.notes, dst.notes),
    updated_by  = sqlc.arg(updated_by)::varchar,
    updated_at  = now()
FROM userprefs.household_item src
WHERE src.household_id = sqlc.arg(from_household_id)
  AND dst.household_id = sqlc.arg(to_household_id)
  AND dst.item_id      = src.item_id;

-- name: ReassignHouseholdItems :exec
-- Move every source row not already folded into a target row.
UPDATE userprefs.household_item hi
SET household_id = sqlc.arg(to_household_id),
    updated_by   = sqlc.arg(updated_by)::varchar,
    updated_at   = now()
WHERE hi.household_id = sqlc.arg(from_household_id)
  AND NOT EXISTS (
      SELECT 1 FROM userprefs.household_item dst
      WHERE dst.household_id = sqlc.arg(to_household_id)
        AND dst.item_id      = hi.item_id
  );

-- name: DeleteMergedHouseholdItems :exec
-- Drop source rows that were folded into a target row.
DELETE FROM userprefs.household_item src
WHERE src.household_id = sqlc.arg(from_household_id)
  AND EXISTS (
      SELECT 1 FROM userprefs.household_item dst
      WHERE dst.household_id = sqlc.arg(to_household_id)
        AND dst.item_id      = src.item_id
  );

-- name: MergeHouseholdBottleConflicts :exec
UPDATE userprefs.household_bottle dst
SET quantity       = dst.quantity + src.quantity,
    bottle_number  = COALESCE(src.bottle_number, dst.bottle_number),
    purchase_at    = GREATEST(dst.purchase_at, src.purchase_at),
    purchase_price = COALESCE(src.purchase_price, dst.purchase_price),
    storage_temp   = COALESCE(src.storage_temp, dst.storage_temp),
    location       = COALESCE(src.location, dst.location),
    notes          = COALESCE(src.notes, dst.notes),
    updated_by     = sqlc.arg(updated_by)::varchar,
    updated_at     = now()
FROM userprefs.household_bottle src
WHERE src.household_id = sqlc.arg(from_household_id)
  AND dst.household_id = sqlc.arg(to_household_id)
  AND dst.bottle_id    = src.bottle_id;

-- name: ReassignHouseholdBottles :exec
UPDATE userprefs.household_bottle hb
SET household_id = sqlc.arg(to_household_id),
    updated_by   = sqlc.arg(updated_by)::varchar,
    updated_at   = now()
WHERE hb.household_id = sqlc.arg(from_household_id)
  AND NOT EXISTS (
      SELECT 1 FROM userprefs.household_bottle dst
      WHERE dst.household_id = sqlc.arg(to_household_id)
        AND dst.bottle_id    = hb.bottle_id
  );

-- name: DeleteMergedHouseholdBottles :exec
DELETE FROM userprefs.household_bottle src
WHERE src.household_id = sqlc.arg(from_household_id)
  AND EXISTS (
      SELECT 1 FROM userprefs.household_bottle dst
      WHERE dst.household_id = sqlc.arg(to_household_id)
        AND dst.bottle_id    = src.bottle_id
  );

-- ---------- recipe favorites (unchanged, per-user) ----------

-- name: UpsertRecipeFavorite :one
INSERT INTO userprefs.user_recipe_preference (user_id, recipe_id, is_favorite, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, recipe_id)
    DO UPDATE SET
        is_favorite = EXCLUDED.is_favorite,
        updated_by  = EXCLUDED.updated_by,
        updated_at  = now()
RETURNING *;

-- name: GetRecipeFavorite :one
SELECT *
FROM userprefs.user_recipe_preference
WHERE user_id = $1 AND recipe_id = $2;

-- name: ListRecipeFavorites :many
SELECT *
FROM userprefs.user_recipe_preference
WHERE user_id = $1 AND recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[]);

-- name: DeleteRecipeFavorite :exec
DELETE FROM userprefs.user_recipe_preference
WHERE user_id = $1 AND recipe_id = $2;
