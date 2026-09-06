-- name: CreateBrand :one
INSERT INTO inventory.brand (name)
VALUES ($1)
RETURNING *;

-- name: GetBrandByID :one
SELECT *
FROM inventory.brand
WHERE brand_id = $1;

-- name: ListBrands :many
SELECT *
FROM inventory.brand
ORDER BY name;

-- name: CreateCategory :one
INSERT INTO inventory.category (name, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
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

-- name: ListCategories :many
SELECT *
FROM inventory.category
ORDER BY name;

-- name: CreateItem :one
INSERT INTO inventory.item (name, brand_id, upc12, upc14, category_id, unit, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetItemByID :one
SELECT *
FROM inventory.item
WHERE item_id = $1;

-- name: ListItems :many
SELECT *
FROM inventory.item
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountItems :one
SELECT COUNT(*)
FROM inventory.item;

-- name: UpdateItem :exec
UPDATE inventory.item
SET name        = $2,
    brand_id    = $3,
    upc12       = $4,
    upc14       = $5,
    category_id = $6,
    unit        = $7,
    updated_by  = $8,
    updated_at  = now()
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
SELECT nt.nutrient_id, nt.name, nt.unit, fn.amount
FROM inventory.food_nutrient fn
JOIN inventory.nutrient_type nt ON fn.nutrient_id = nt.nutrient_id
WHERE fn.food_id = $1
ORDER BY nt.name;

-- name: ListFoodNutrientsByItems :many
SELECT fn.food_id, nt.nutrient_id, nt.name, nt.unit, fn.amount
FROM inventory.food_nutrient fn
JOIN inventory.nutrient_type nt ON fn.nutrient_id = nt.nutrient_id
WHERE fn.food_id = ANY(sqlc.arg(item_ids)::bigint[])
ORDER BY nt.name;

-- name: CreateFoodNutrient :one
INSERT INTO inventory.food_nutrient (food_id, nutrient_id, amount, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteFoodNutrient :exec
DELETE FROM inventory.food_nutrient
WHERE food_id = $1 AND nutrient_id = $2;

-- name: CreateFoodFlavor :one
INSERT INTO inventory.food_flavor (food_id, flavor_id, intensity, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

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
RETURNING brand_id, name, created_at;

-- name: DeleteBrand :exec
DELETE FROM inventory.brand
WHERE brand_id = $1;

-- name: UpdateCategory :one
UPDATE inventory.category
SET name        = $2,
    description = $3,
    is_active   = $4,
    updated_by  = $5,
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
