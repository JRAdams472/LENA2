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
-- Plain recency paging for internal consumers (pantry stock scans); ranked
-- listing goes through SearchHouseholdItems.
SELECT *
FROM userprefs.household_item
WHERE household_id = $1
ORDER BY updated_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountHouseholdItems :one
SELECT COUNT(*)
FROM userprefs.household_item
WHERE household_id = $1;

-- name: SearchHouseholdItems :many
-- Ranked pantry listing. Tiers: 0 expiring within 7 days (smallest
-- expires_at first — urgency over habit), 1 the caller's favorite catalog
-- items, 2 personally-used, 3 household-used, 4 rest by recency.
-- include_ids scopes by catalog item (the BFF resolves a name term to item
-- IDs because this schema cannot join inventory). NULL means no filter.
SELECT *
FROM userprefs.household_item
WHERE household_id = $1
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR item_id = ANY(sqlc.arg(include_ids)::bigint[]))
ORDER BY
  CASE
    WHEN expires_at IS NOT NULL AND expires_at <= now() + interval '7 days' THEN 0
    WHEN item_id = ANY(sqlc.arg(favorite_ids)::bigint[]) THEN 1
    WHEN item_id = ANY(sqlc.arg(personal_ids)::bigint[]) THEN 2
    WHEN item_id = ANY(sqlc.arg(household_ids)::bigint[]) THEN 3
    ELSE 4
  END,
  expires_at ASC NULLS LAST,
  array_position(sqlc.arg(personal_ids)::bigint[], item_id),
  array_position(sqlc.arg(household_ids)::bigint[], item_id),
  updated_at DESC NULLS LAST,
  household_item_id
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchHouseholdItems :one
SELECT COUNT(*)
FROM userprefs.household_item
WHERE household_id = $1
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR item_id = ANY(sqlc.arg(include_ids)::bigint[]));

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
-- Plain recency paging for internal consumers; ranked listing goes through
-- SearchHouseholdBottles.
SELECT *
FROM userprefs.household_bottle
WHERE household_id = $1
ORDER BY updated_at DESC NULLS LAST
LIMIT $2 OFFSET $3;

-- name: CountHouseholdBottles :one
SELECT COUNT(*)
FROM userprefs.household_bottle
WHERE household_id = $1;

-- name: SearchHouseholdBottles :many
-- Ranked cellar listing. Tiers: 0 the caller's favorite bottles,
-- 1 personally-used, 2 household-used, 3 rest by recency. include_ids
-- scopes by catalog bottle (the BFF resolves a name term to bottle IDs
-- because this schema cannot join wine). NULL means no filter.
SELECT *
FROM userprefs.household_bottle
WHERE household_id = $1
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR bottle_id = ANY(sqlc.arg(include_ids)::bigint[]))
ORDER BY
  CASE
    WHEN bottle_id = ANY(sqlc.arg(favorite_ids)::bigint[]) THEN 0
    WHEN bottle_id = ANY(sqlc.arg(personal_ids)::bigint[]) THEN 1
    WHEN bottle_id = ANY(sqlc.arg(household_ids)::bigint[]) THEN 2
    ELSE 3
  END,
  array_position(sqlc.arg(favorite_ids)::bigint[], bottle_id),
  array_position(sqlc.arg(personal_ids)::bigint[], bottle_id),
  array_position(sqlc.arg(household_ids)::bigint[], bottle_id),
  updated_at DESC NULLS LAST,
  household_bottle_id
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchHouseholdBottles :one
SELECT COUNT(*)
FROM userprefs.household_bottle
WHERE household_id = $1
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR bottle_id = ANY(sqlc.arg(include_ids)::bigint[]));

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

-- name: ListFavoriteItemIDs :many
-- Every item the user has favorited — feeds the ranking tier on catalog
-- and pantry listings.
SELECT item_id
FROM userprefs.user_item_favorite
WHERE user_id = $1 AND is_favorite = TRUE;

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

-- name: ListFavoriteBottleIDs :many
-- Every bottle the user has favorited — feeds the ranking tier on catalog
-- and cellar listings.
SELECT bottle_id
FROM userprefs.user_bottle_favorite
WHERE user_id = $1 AND is_favorite = TRUE;

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

-- name: ListFavoriteRecipeIDs :many
-- Every recipe the user has favorited — feeds the search ranking boost and
-- the isFavorite filter (kept in userprefs; SQL never crosses schemas, so
-- the BFF passes these IDs into the recipe query).
SELECT recipe_id
FROM userprefs.user_recipe_preference
WHERE user_id = $1 AND is_favorite = TRUE;
