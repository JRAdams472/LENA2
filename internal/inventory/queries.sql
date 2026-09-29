-- name: CreateBrand :one
-- Admin-only fast path: brand is immediately approved.
INSERT INTO inventory.brand (name, status, created_by, updated_by)
VALUES ($1, 'approved', $2, $2)
RETURNING *;

-- name: CreateBrandPending :one
-- User-submitted brand: starts pending, visible only to the submitter
-- until an admin approves it.
INSERT INTO inventory.brand (name, status, submitted_by_user_id, created_by, updated_by)
VALUES ($1, 'pending', $2, $3, $3)
RETURNING *;

-- name: GetBrandByID :one
SELECT *
FROM inventory.brand
WHERE brand_id = $1;

-- name: UpsertBrand :one
-- Race-free submit: if an approved or own-pending normalized name already
-- exists, return the existing row; otherwise create a new pending brand.
INSERT INTO inventory.brand (name, status, submitted_by_user_id, created_by, updated_by)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (name_normalized) WHERE status <> 'rejected' DO NOTHING
RETURNING *;

-- name: FindBrandByNormalizedName :one
-- Special characters (apostrophes, periods, etc.) and case are ignored so
-- "Bush", "Bushs", and "Bush's" all match the same brand. Rejected rows
-- are never resurfaced.
SELECT *
FROM inventory.brand
WHERE name_normalized = lower(regexp_replace($1, '[^a-zA-Z0-9]', '', 'g'))
  AND status <> 'rejected';

-- name: SearchBrands :many
-- Engagement-ranked brand picker search. Tiers come from ID arrays the BFF
-- computes from analytics/userprefs (other schemas — SQL never crosses them):
--   0 personal-used, 1 household-used, 2 prior-search-term match,
--   3 global-popular, 4 rest. The score arrays arrive pre-sorted so
--   array_position doubles as the in-tier tiebreaker.
SELECT *
FROM inventory.brand
WHERE (status = 'approved' OR submitted_by_user_id = $1)
  AND name_normalized LIKE '%' || lower(regexp_replace($2, '[^a-zA-Z0-9]', '', 'g')) || '%'
ORDER BY
  CASE
    WHEN brand_id = ANY(sqlc.arg(personal_ids)::bigint[]) THEN 0
    WHEN brand_id = ANY(sqlc.arg(household_ids)::bigint[]) THEN 1
    WHEN EXISTS (
      SELECT 1 FROM unnest(sqlc.arg(search_terms)::text[]) t
      WHERE position(lower(t) in lower(name)) > 0
    ) THEN 2
    WHEN brand_id = ANY(sqlc.arg(global_ids)::bigint[]) THEN 3
    ELSE 4
  END,
  array_position(sqlc.arg(personal_ids)::bigint[], brand_id),
  array_position(sqlc.arg(household_ids)::bigint[], brand_id),
  array_position(sqlc.arg(global_ids)::bigint[], brand_id),
  name,
  brand_id
LIMIT $3;

-- name: ListBrands :many
SELECT *
FROM inventory.brand
ORDER BY name;

-- name: ListBrandsVisible :many
-- Brands are visible when approved, or when the caller submitted them.
SELECT *
FROM inventory.brand
WHERE status = 'approved' OR submitted_by_user_id = $1
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: CountBrandsVisible :one
SELECT COUNT(*)
FROM inventory.brand
WHERE status = 'approved' OR submitted_by_user_id = $1;

-- name: ListPendingBrands :many
SELECT *
FROM inventory.brand
WHERE status = 'pending'
ORDER BY created_at
LIMIT $1 OFFSET $2;

-- name: CountPendingBrands :one
SELECT COUNT(*)
FROM inventory.brand
WHERE status = 'pending';

-- name: SetBrandStatus :exec
UPDATE inventory.brand
SET status              = $2,
    approved_by_user_id = $3,
    approved_at         = $4,
    updated_by          = $5,
    updated_at          = now()
WHERE brand_id = $1;

-- name: CreateCategory :one
INSERT INTO inventory.category (name, description, is_active, is_protein, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCategoryByID :one
SELECT *
FROM inventory.category
WHERE category_id = $1;

-- name: GetFlavorProfileByID :one
SELECT *
FROM inventory.flavor_profile
WHERE flavor_id = $1;

-- name: GetNutrientTypeByID :one
SELECT *
FROM inventory.nutrient_type
WHERE nutrient_id = $1;

-- name: GetNutrientTypeByName :one
SELECT *
FROM inventory.nutrient_type
WHERE lower(name) = lower($1);

-- name: ListCategories :many
SELECT *
FROM inventory.category
ORDER BY name;

-- name: CreateItem :one
INSERT INTO inventory.item (name, brand_id, upc12, upc14, category_id, unit_id, status, submitted_by_user_id, created_by, updated_by, net_weight, is_metric)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetItemByID :one
SELECT *
FROM inventory.item
WHERE item_id = $1;

-- name: ListItems :many
-- Items are visible when approved, or when the caller submitted them.
-- Plain alphabetical paging for internal consumers (recipe import); ranked
-- listing goes through SearchItems.
SELECT *
FROM inventory.item
WHERE status = 'approved' OR submitted_by_user_id = $1
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: CountItems :one
SELECT COUNT(*)
FROM inventory.item
WHERE status = 'approved' OR submitted_by_user_id = $1;

-- name: RankedItems :many
-- The engaged slice of an item search: every catalog row matching the
-- visibility/term filters whose ID is in the caller's engagement set. At
-- most a few thousand rows — fetched whole and tier-sorted in Go, which is
-- far cheaper than ORDER BY CASE over the full ~110k-row catalog.
SELECT *
FROM inventory.item
WHERE (status = 'approved' OR submitted_by_user_id = $1)
  AND (sqlc.narg('search')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('search')) || '%')
  AND (sqlc.narg('brand_id')::bigint IS NULL OR brand_id = sqlc.narg('brand_id'))
  AND item_id IN (SELECT unnest(sqlc.arg(engaged_ids)::bigint[]));

-- name: SearchItemsRemainder :many
-- The non-engaged slice, served in (name, item_id) index order with a
-- hashed NOT IN probe — no sort, so deep pagination stays cheap on the
-- large catalog.
SELECT *
FROM inventory.item
WHERE (status = 'approved' OR submitted_by_user_id = $1)
  AND (sqlc.narg('search')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('search')) || '%')
  AND (sqlc.narg('brand_id')::bigint IS NULL OR brand_id = sqlc.narg('brand_id'))
  AND item_id NOT IN (SELECT unnest(sqlc.arg(engaged_ids)::bigint[]))
ORDER BY name, item_id
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchItems :one
SELECT COUNT(*)
FROM inventory.item
WHERE (status = 'approved' OR submitted_by_user_id = $1)
  AND (sqlc.narg('search')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('search')) || '%')
  AND (sqlc.narg('brand_id')::bigint IS NULL OR brand_id = sqlc.narg('brand_id'));

-- name: MatchItemIDsByTerms :many
-- Items matching any of the user's prior search terms — the "searched"
-- engagement tier resolved to IDs so it can join the ranked set. One
-- per-page scan, only run when the user has recorded terms.
SELECT DISTINCT item_id
FROM inventory.item
WHERE (status = 'approved' OR submitted_by_user_id = $1)
  AND (sqlc.narg('search')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('search')) || '%')
  AND EXISTS (
    SELECT 1 FROM unnest(sqlc.arg(search_terms)::text[]) t
    WHERE position(lower(t) in lower(name)) > 0
  )
LIMIT 500;

-- name: GetItemByUpc :one
-- Barcode lookup: the caller passes the normalized code plus their user id
-- so pending items they submitted are still found.
SELECT *
FROM inventory.item
WHERE (upc12 = $1 OR upc14 = $1)
  AND (status = 'approved' OR submitted_by_user_id = $2);

-- name: ListPendingItems :many
SELECT *
FROM inventory.item
WHERE status = 'pending'
ORDER BY created_at
LIMIT $1 OFFSET $2;

-- name: CountPendingItems :one
SELECT COUNT(*)
FROM inventory.item
WHERE status = 'pending';

-- name: SetItemStatus :exec
UPDATE inventory.item
SET status              = $2,
    approved_by_user_id = $3,
    approved_at         = $4,
    updated_by          = $5,
    updated_at          = now()
WHERE item_id = $1;

-- name: UpdateItem :exec
UPDATE inventory.item
SET name        = $2,
    brand_id    = $3,
    upc12       = $4,
    upc14       = $5,
    category_id = $6,
    unit_id     = $7,
    updated_by  = $8,
    updated_at  = now(),
    net_weight  = $9,
    is_metric   = $10
WHERE item_id = $1;

-- name: DeleteItem :exec
DELETE FROM inventory.item
WHERE item_id = $1;

-- name: CreateFlavorProfile :one
INSERT INTO inventory.flavor_profile (name, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListFlavorProfiles :many
SELECT *
FROM inventory.flavor_profile
ORDER BY name;

-- name: CreateNutrientType :one
INSERT INTO inventory.nutrient_type (name, unit)
VALUES ($1, $2)
RETURNING *;

-- name: ListNutrientTypes :many
SELECT *
FROM inventory.nutrient_type
ORDER BY name;

-- name: ListFoodNutrientsByItem :many
SELECT nt.nutrient_id, nt.name, nt.unit, fn.amount, fn.basis_quantity, fn.basis_unit_id
FROM inventory.food_nutrient fn
JOIN inventory.nutrient_type nt ON fn.nutrient_id = nt.nutrient_id
WHERE fn.food_id = $1
ORDER BY nt.name;

-- name: ListFoodNutrientsByItems :many
SELECT fn.food_id, nt.nutrient_id, nt.name, nt.unit, fn.amount, fn.basis_quantity, fn.basis_unit_id
FROM inventory.food_nutrient fn
JOIN inventory.nutrient_type nt ON fn.nutrient_id = nt.nutrient_id
WHERE fn.food_id = ANY(sqlc.arg(item_ids)::bigint[])
ORDER BY nt.name;

-- name: CreateFoodNutrient :one
-- The basis defaults to per-100g when the caller does not declare one.
WITH ins AS (
    INSERT INTO inventory.food_nutrient (food_id, nutrient_id, amount, basis_quantity, basis_unit_id, created_by)
    VALUES (
        $1, $2, $3,
        COALESCE(sqlc.narg(basis_quantity), 100),
        COALESCE(sqlc.narg(basis_unit_id), (SELECT unit_id FROM inventory.unit WHERE name = 'gram' LIMIT 1)),
        $4
    )
    RETURNING food_id, nutrient_id, amount, basis_quantity, basis_unit_id
)
SELECT ins.food_id, nt.nutrient_id, nt.name, nt.unit, ins.amount, ins.basis_quantity, ins.basis_unit_id
FROM ins
JOIN inventory.nutrient_type nt ON ins.nutrient_id = nt.nutrient_id;

-- name: DeleteFoodNutrient :exec
DELETE FROM inventory.food_nutrient
WHERE food_id = $1 AND nutrient_id = $2;

-- name: DeleteFoodNutrientsByItem :exec
DELETE FROM inventory.food_nutrient
WHERE food_id = $1;

-- name: CreateFoodFlavor :one
WITH ins AS (
    INSERT INTO inventory.food_flavor (food_id, flavor_id, intensity, created_by)
    VALUES ($1, $2, $3, $4)
    RETURNING food_id, flavor_id, intensity
)
SELECT ins.food_id, fp.flavor_id, fp.name, ins.intensity
FROM ins
JOIN inventory.flavor_profile fp ON ins.flavor_id = fp.flavor_id;

-- name: ListFoodFlavorsByItem :many
SELECT fp.flavor_id, fp.name, ff.intensity
FROM inventory.food_flavor ff
JOIN inventory.flavor_profile fp ON ff.flavor_id = fp.flavor_id
WHERE ff.food_id = $1
ORDER BY fp.name;

-- name: ListFoodFlavorsByItems :many
SELECT ff.food_id, fp.flavor_id, fp.name, ff.intensity
FROM inventory.food_flavor ff
JOIN inventory.flavor_profile fp ON ff.flavor_id = fp.flavor_id
WHERE ff.food_id = ANY(sqlc.arg(item_ids)::bigint[])
ORDER BY fp.name;

-- name: GetItemsByIDs :many
SELECT *
FROM inventory.item
WHERE item_id = ANY(sqlc.arg(item_ids)::bigint[]);

-- name: GetBrandsByIDs :many
SELECT *
FROM inventory.brand
WHERE brand_id = ANY(sqlc.arg(brand_ids)::bigint[]);

-- name: GetCategoriesByIDs :many
SELECT *
FROM inventory.category
WHERE category_id = ANY(sqlc.arg(category_ids)::bigint[]);

-- name: DeleteFoodFlavor :exec
DELETE FROM inventory.food_flavor
WHERE food_id = $1 AND flavor_id = $2;

-- name: UpdateBrand :one
UPDATE inventory.brand
SET name = $2
WHERE brand_id = $1
RETURNING *;

-- name: DeleteBrand :exec
DELETE FROM inventory.brand
WHERE brand_id = $1;

-- name: UpdateCategory :one
UPDATE inventory.category
SET name        = $2,
    description = $3,
    is_active   = $4,
    is_protein  = $5,
    updated_by  = $6,
    updated_at  = now()
WHERE category_id = $1
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM inventory.category
WHERE category_id = $1;

-- name: UpdateFlavorProfile :one
UPDATE inventory.flavor_profile
SET name       = $2,
    is_active  = $3,
    updated_by = $4,
    updated_at = now()
WHERE flavor_id = $1
RETURNING *;

-- name: DeleteFlavorProfile :exec
DELETE FROM inventory.flavor_profile
WHERE flavor_id = $1;

-- name: UpdateNutrientType :one
UPDATE inventory.nutrient_type
SET name = $2,
    unit = $3
WHERE nutrient_id = $1
RETURNING *;

-- name: DeleteNutrientType :exec
DELETE FROM inventory.nutrient_type
WHERE nutrient_id = $1;

-- name: CreateIngredient :one
INSERT INTO inventory.ingredient (name, category_id, default_unit_id, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetIngredientByID :one
SELECT *
FROM inventory.ingredient
WHERE ingredient_id = $1;

-- name: GetIngredientsByIDs :many
SELECT *
FROM inventory.ingredient
WHERE ingredient_id = ANY(sqlc.arg(ingredient_ids)::bigint[]);

-- name: ListIngredients :many
-- Plain alphabetical paging for internal consumers (recipe import); ranked
-- listing goes through SearchIngredients.
SELECT *
FROM inventory.ingredient
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountIngredients :one
SELECT COUNT(*)
FROM inventory.ingredient;

-- name: SearchIngredients :many
-- Engagement-ranked ingredient browse/search — same tier pattern as
-- SearchItems minus favorites (ingredients have no favorite store).
SELECT *
FROM inventory.ingredient
WHERE (sqlc.narg('search')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('search')) || '%')
ORDER BY
  CASE
    WHEN ingredient_id = ANY(sqlc.arg(personal_ids)::bigint[]) THEN 0
    WHEN ingredient_id = ANY(sqlc.arg(household_ids)::bigint[]) THEN 1
    WHEN EXISTS (
      SELECT 1 FROM unnest(sqlc.arg(search_terms)::text[]) t
      WHERE position(lower(t) in lower(name)) > 0
    ) THEN 2
    WHEN ingredient_id = ANY(sqlc.arg(global_ids)::bigint[]) THEN 3
    ELSE 4
  END,
  array_position(sqlc.arg(personal_ids)::bigint[], ingredient_id),
  array_position(sqlc.arg(household_ids)::bigint[], ingredient_id),
  array_position(sqlc.arg(global_ids)::bigint[], ingredient_id),
  name,
  ingredient_id
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchIngredients :one
SELECT COUNT(*)
FROM inventory.ingredient
WHERE (sqlc.narg('search')::text IS NULL OR lower(name) LIKE '%' || lower(sqlc.narg('search')) || '%');

-- name: MatchItemIDs :many
-- IDs of visible items whose name matches the term — feeds include_ids on
-- household-scoped queries that cannot join this schema (pantry search).
SELECT item_id
FROM inventory.item
WHERE (status = 'approved' OR submitted_by_user_id = $1)
  AND lower(name) LIKE '%' || lower($2) || '%'
LIMIT 1000;

-- name: UpdateIngredient :one
UPDATE inventory.ingredient
SET name            = $2,
    category_id     = $3,
    default_unit_id = $4,
    is_active       = $5,
    updated_by   = $6,
    updated_at   = now()
WHERE ingredient_id = $1
RETURNING *;

-- name: DeleteIngredient :exec
DELETE FROM inventory.ingredient
WHERE ingredient_id = $1;

-- name: CreateUnit :one
INSERT INTO inventory.unit (name, abbreviation, kind, to_base_factor, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUnitByID :one
SELECT *
FROM inventory.unit
WHERE unit_id = $1;

-- name: GetUnitByName :one
SELECT *
FROM inventory.unit
WHERE lower(name) = lower($1) OR lower(abbreviation) = lower($1);

-- name: GetUnitsByIDs :many
SELECT *
FROM inventory.unit
WHERE unit_id = ANY(sqlc.arg(unit_ids)::bigint[]);

-- name: ListUnits :many
SELECT *
FROM inventory.unit
ORDER BY kind, name;
