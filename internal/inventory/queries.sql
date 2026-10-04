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

-- name: SetBrandStatus :execrows
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

-- name: SetItemStatus :execrows
UPDATE inventory.item
SET status              = $2,
    approved_by_user_id = $3,
    approved_at         = $4,
    updated_by          = $5,
    updated_at          = now()
WHERE item_id = $1;

-- name: UpdateItem :execrows
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

-- name: FindIngredientByNormalizedName :one
-- Free-create dedupe: matches the normalized unique index expression so
-- "Carrots", " carrots  ", and "CARROTS" all resolve to the same row.
SELECT *
FROM inventory.ingredient
WHERE lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) =
      lower(btrim(regexp_replace($1, '\s+', ' ', 'g')));

-- name: GetItemIngredient :one
-- Effective ingredient inputs for a branded item: the household's
-- override wins over the catalog-level link. Both NULL when unlinked.
SELECT o.ingredient_id AS override_ingredient_id, i.ingredient_id
FROM inventory.item i
LEFT JOIN userprefs.household_item_ingredient o
       ON o.item_id = i.item_id
      AND o.household_id = $2
WHERE i.item_id = $1;

-- name: GetItemIngredientsForItems :many
-- Batch form of GetItemIngredient: one row per requested item carrying the
-- override and catalog ingredient (override wins; both NULL when unlinked).
SELECT i.item_id,
       o.ingredient_id AS override_ingredient_id,
       i.ingredient_id
FROM inventory.item i
LEFT JOIN userprefs.household_item_ingredient o
       ON o.item_id = i.item_id
      AND o.household_id = sqlc.arg(household_id)
WHERE i.item_id = ANY(sqlc.arg(item_ids)::bigint[]);

-- name: ListUsualItemsForIngredients :many
-- Batch "usual brand" lookup for grocery-line preloads — one row per
-- (ingredient, household) pair recorded by brand-picked check-offs.
SELECT *
FROM userprefs.household_ingredient_item
WHERE household_id = $1
  AND ingredient_id = ANY(sqlc.arg(ingredient_ids)::bigint[]);

-- name: ListItemsForIngredient :many
-- Every item resolving to the ingredient under the same resolution order:
-- catalog link plus overrides (an override can point an otherwise-linked
-- item at this ingredient too).
SELECT i.*
FROM inventory.item i
LEFT JOIN userprefs.household_item_ingredient o
       ON o.item_id = i.item_id
      AND o.household_id = $2
WHERE COALESCE(o.ingredient_id, i.ingredient_id) = $1
  AND (i.status = 'approved' OR i.submitted_by_user_id = $3)
ORDER BY i.name;

-- name: SetItemIngredient :execrows
-- Writes the catalog-level item -> ingredient link.
UPDATE inventory.item
SET ingredient_id = $2,
    updated_by    = $3,
    updated_at    = now()
WHERE item_id = $1;

-- name: UpsertItemIngredientOverride :exec
-- Household-level remap: "this product is a different ingredient for us."
INSERT INTO userprefs.household_item_ingredient (household_id, item_id, ingredient_id, created_by, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $4, now())
ON CONFLICT (household_id, item_id)
DO UPDATE SET ingredient_id = EXCLUDED.ingredient_id,
              updated_by    = EXCLUDED.updated_by,
              updated_at    = now();

-- name: DeleteItemIngredientOverride :execrows
DELETE FROM userprefs.household_item_ingredient
WHERE household_id = $1 AND item_id = $2;

-- name: GetUsualItemForIngredient :one
SELECT *
FROM userprefs.household_ingredient_item
WHERE household_id = $1 AND ingredient_id = $2;

-- name: UpsertUsualItemForIngredient :one
-- "Usual brand" record — updated each time a check-off credits stock so
-- repeat purchases stop prompting.
INSERT INTO userprefs.household_ingredient_item (household_id, ingredient_id, item_id, last_used_at, created_by, updated_by, updated_at)
VALUES ($1, $2, $3, now(), $4, $4, now())
ON CONFLICT (household_id, ingredient_id)
DO UPDATE SET item_id      = EXCLUDED.item_id,
              last_used_at = now(),
              updated_by   = EXCLUDED.updated_by,
              updated_at   = now()
RETURNING *;

-- name: MergeIngredientRefsItem :exec
-- Repoint every ingredient reference onto the merge target. For tables
-- where two rows would collapse to the same natural key, only rows that
-- do not conflict are repointed; the conflicting leftovers are deleted by
-- the paired DeleteIngredientRefs* query.
UPDATE inventory.item
SET ingredient_id = $2,
    updated_at    = now()
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsOverride :exec
UPDATE userprefs.household_item_ingredient
SET ingredient_id = $2,
    updated_at    = now()
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsUsual :exec
UPDATE userprefs.household_ingredient_item u
SET ingredient_id = $2,
    updated_at    = now()
WHERE u.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM userprefs.household_ingredient_item t
      WHERE t.household_id = u.household_id AND t.ingredient_id = $2
  );

-- name: DeleteIngredientRefsUsual :exec
DELETE FROM userprefs.household_ingredient_item
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsRecipeItem :exec
UPDATE recipe.recipe_item r
SET ingredient_id = $2
WHERE r.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM recipe.recipe_item t
      WHERE t.recipe_id = r.recipe_id AND t.ingredient_id = $2
  );

-- name: DemoteIngredientRefsRecipeItem :exec
-- Conflicting rows keep their branded hint: drop the ingredient ref but
-- leave item_id, so "Green Giant corn" survives as a preferred brand even
-- after a merge collapses the generic identity.
UPDATE recipe.recipe_item
SET ingredient_id = NULL
WHERE ingredient_id = $1 AND item_id IS NOT NULL;

-- name: DeleteIngredientRefsRecipeItem :exec
-- Whatever remains references the source with no item anchor — a true
-- duplicate of the target's row.
DELETE FROM recipe.recipe_item
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsMealSlot :exec
UPDATE mealplan.meal_slot_item m
SET ingredient_id = $2
WHERE m.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM mealplan.meal_slot_item t
      WHERE t.slot_id = m.slot_id AND t.ingredient_id = $2
  );

-- name: DemoteIngredientRefsMealSlot :exec
UPDATE mealplan.meal_slot_item
SET ingredient_id = NULL
WHERE ingredient_id = $1 AND item_id IS NOT NULL;

-- name: DeleteIngredientRefsMealSlot :exec
DELETE FROM mealplan.meal_slot_item
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsEventRecipeItem :exec
UPDATE event.event_recipe_item e
SET ingredient_id = $2
WHERE e.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM event.event_recipe_item t
      WHERE t.event_recipe_id = e.event_recipe_id AND t.ingredient_id = $2
  );

-- name: DemoteIngredientRefsEventRecipeItem :exec
UPDATE event.event_recipe_item
SET ingredient_id = NULL
WHERE ingredient_id = $1 AND item_id IS NOT NULL;

-- name: DeleteIngredientRefsEventRecipeItem :exec
DELETE FROM event.event_recipe_item
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsGroceryLine :exec
UPDATE grocery.grocery_list_item g
SET ingredient_id = $2
WHERE g.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM grocery.grocery_list_item t
      WHERE t.grocery_list_id = g.grocery_list_id AND t.ingredient_id = $2
  );

-- name: DemoteIngredientRefsGroceryLine :exec
UPDATE grocery.grocery_list_item
SET ingredient_id = NULL
WHERE ingredient_id = $1 AND (item_id IS NOT NULL OR manual_item_name IS NOT NULL);

-- name: DeleteIngredientRefsGroceryLine :exec
DELETE FROM grocery.grocery_list_item
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsAisle :exec
UPDATE grocery.aisle_assignment a
SET ingredient_id = $2,
    updated_at    = now()
WHERE a.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM grocery.aisle_assignment t
      WHERE t.store_id = a.store_id AND t.ingredient_id = $2
  );

-- name: DeleteIngredientRefsAisle :exec
DELETE FROM grocery.aisle_assignment
WHERE ingredient_id = $1;

-- name: MergeIngredientRefsRoute :exec
UPDATE grocery.item_route r
SET ingredient_id = $2,
    updated_at    = now()
WHERE r.ingredient_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM grocery.item_route t
      WHERE t.household_id = r.household_id
        AND t.store_id = r.store_id
        AND t.ingredient_id = $2
  );

-- name: DeleteIngredientRefsRoute :exec
DELETE FROM grocery.item_route
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

-- ---------- allergen registry + entity flags ----------

-- name: ListAllergens :many
SELECT *
FROM inventory.allergen
ORDER BY name;

-- name: GetAllergenByID :one
SELECT *
FROM inventory.allergen
WHERE allergen_id = $1;

-- name: GetAllergensByIDs :many
SELECT *
FROM inventory.allergen
WHERE allergen_id = ANY(sqlc.arg(allergen_ids)::bigint[]);

-- name: CreateAllergen :one
INSERT INTO inventory.allergen (name, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateAllergen :one
UPDATE inventory.allergen
SET name        = $2,
    description = $3,
    is_active   = $4,
    updated_by  = $5,
    updated_at  = now()
WHERE allergen_id = $1
RETURNING *;

-- name: ListIngredientAllergens :many
SELECT *
FROM inventory.ingredient_allergen
WHERE ingredient_id = $1;

-- name: ListIngredientAllergensByIngredients :many
SELECT *
FROM inventory.ingredient_allergen
WHERE ingredient_id = ANY(sqlc.arg(ingredient_ids)::bigint[]);

-- name: UpsertIngredientAllergen :execrows
INSERT INTO inventory.ingredient_allergen (ingredient_id, allergen_id, kind, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (ingredient_id, allergen_id)
    DO UPDATE SET
        kind       = EXCLUDED.kind,
        updated_by = EXCLUDED.updated_by,
        updated_at = now();

-- name: DeleteIngredientAllergen :execrows
DELETE FROM inventory.ingredient_allergen
WHERE ingredient_id = $1 AND allergen_id = $2;

-- name: ListItemAllergens :many
SELECT *
FROM inventory.item_allergen
WHERE item_id = $1;

-- name: ListItemAllergensByItems :many
SELECT *
FROM inventory.item_allergen
WHERE item_id = ANY(sqlc.arg(item_ids)::bigint[]);

-- name: UpsertItemAllergen :execrows
INSERT INTO inventory.item_allergen (item_id, allergen_id, kind, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (item_id, allergen_id)
    DO UPDATE SET
        kind       = EXCLUDED.kind,
        updated_by = EXCLUDED.updated_by,
        updated_at = now();

-- name: DeleteItemAllergen :execrows
DELETE FROM inventory.item_allergen
WHERE item_id = $1 AND allergen_id = $2;


-- ---------- allergen flag suggestions (LEN-23 review queue) ----------

-- name: CreateAllergenSuggestion :one
INSERT INTO inventory.allergen_suggestion
    (recipe_id, target_kind, ingredient_id, item_id, allergen_id, kind,
     rationale, suggested_by_user_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (target_kind, COALESCE(ingredient_id, item_id), allergen_id)
    WHERE status = 'pending'
    DO NOTHING
RETURNING *;

-- name: ListAllergenSuggestions :many
SELECT s.*,
       a.name  AS allergen_name,
       i.name  AS ingredient_name,
       it.name AS item_name,
       r.name  AS recipe_name
FROM inventory.allergen_suggestion s
JOIN inventory.allergen a ON a.allergen_id = s.allergen_id
LEFT JOIN inventory.ingredient i ON i.ingredient_id = s.ingredient_id
LEFT JOIN inventory.item it ON it.item_id = s.item_id
LEFT JOIN recipe.recipe r ON r.recipe_id = s.recipe_id
WHERE (sqlc.narg(status)::varchar IS NULL OR s.status = sqlc.narg(status)::varchar)
ORDER BY s.allergen_suggestion_id;

-- name: GetAllergenSuggestion :one
SELECT s.*,
       a.name  AS allergen_name,
       i.name  AS ingredient_name,
       it.name AS item_name,
       r.name  AS recipe_name
FROM inventory.allergen_suggestion s
JOIN inventory.allergen a ON a.allergen_id = s.allergen_id
LEFT JOIN inventory.ingredient i ON i.ingredient_id = s.ingredient_id
LEFT JOIN inventory.item it ON it.item_id = s.item_id
LEFT JOIN recipe.recipe r ON r.recipe_id = s.recipe_id
WHERE s.allergen_suggestion_id = $1;

-- name: SetAllergenSuggestionStatus :one
UPDATE inventory.allergen_suggestion
SET status              = $2,
    reviewed_by_user_id = $3,
    reviewed_at         = CASE WHEN $2::varchar = 'pending' THEN NULL ELSE now() END,
    updated_by          = $4,
    updated_at          = now()
WHERE allergen_suggestion_id = $1 AND status = 'pending'
RETURNING *;
