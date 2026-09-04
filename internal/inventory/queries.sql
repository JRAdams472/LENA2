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
