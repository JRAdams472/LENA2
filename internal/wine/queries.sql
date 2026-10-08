-- name: CreateCountry :one
INSERT INTO wine.country (name, iso_code, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCountryByID :one
SELECT *
FROM wine.country
WHERE country_id = $1;

-- name: ListCountries :many
SELECT *
FROM wine.country
ORDER BY name;

-- name: CreateRegion :one
INSERT INTO wine.region (country_id, name, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRegionByID :one
SELECT *
FROM wine.region
WHERE region_id = $1;

-- name: ListRegions :many
SELECT *
FROM wine.region
WHERE country_id = $1
ORDER BY name;

-- name: CreateType :one
INSERT INTO wine.type (name, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTypeByID :one
SELECT *
FROM wine.type
WHERE type_id = $1;

-- name: GetVintageByID :one
SELECT *
FROM wine.vintage
WHERE vintage_id = $1;

-- name: GetGrapeVarietyByID :one
SELECT *
FROM wine.grape_variety
WHERE grape_variety_id = $1;

-- name: ListTypes :many
SELECT *
FROM wine.type
ORDER BY name;

-- name: CreateBottle :one
INSERT INTO wine.bottle (
    type_id, country_id, region_id, vintage_year, vineyard, abv,
    acidity, tannin_level, body, sweetness, oak_integration, bottle_size,
    created_by, updated_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetBottleByID :one
SELECT *
FROM wine.bottle
WHERE bottle_id = $1;

-- name: ListBottles :many
-- Plain insertion-order paging for internal consumers; ranked listing goes
-- through SearchBottles.
SELECT *
FROM wine.bottle
ORDER BY bottle_id DESC
LIMIT $1 OFFSET $2;

-- name: CountBottles :one
SELECT COUNT(*)
FROM wine.bottle;

-- name: SearchBottles :many
-- Engagement-ranked bottle browse/search. The term and prior-search-term
-- tiers match a haystack of vineyard + type/country/region names (all
-- same-schema joins). Optional structured filters (country, region, type,
-- vintage, favorites) narrow the catalog before ranking. Tiers from
-- BFF-computed ID arrays:
--   0 favorite, 1 personal-used, 2 household-used, 3 prior-search-term
--   match, 4 global-popular, 5 rest.
SELECT b.*
FROM wine.bottle b
JOIN wine.type t ON t.type_id = b.type_id
JOIN wine.country c ON c.country_id = b.country_id
JOIN wine.region rg ON rg.region_id = b.region_id
WHERE (
    sqlc.narg('search')::text IS NULL
    OR position(lower(sqlc.narg('search')) in lower(
      coalesce(b.vineyard, '') || ' ' || t.name || ' ' || c.name || ' ' || rg.name
    )) > 0
  )
  AND (
    sqlc.narg('country_id')::bigint IS NULL
    OR b.country_id = sqlc.narg('country_id')::bigint
  )
  AND (
    sqlc.narg('region_id')::bigint IS NULL
    OR b.region_id = sqlc.narg('region_id')::bigint
  )
  AND (
    sqlc.narg('type_id')::bigint IS NULL
    OR b.type_id = sqlc.narg('type_id')::bigint
  )
  AND (
    sqlc.narg('vintage_year')::int IS NULL
    OR b.vintage_year = sqlc.narg('vintage_year')::int
  )
  AND (
    sqlc.narg('favorites_only')::bool IS DISTINCT FROM TRUE
    OR b.bottle_id = ANY(sqlc.arg(favorite_ids)::bigint[])
  )
ORDER BY
  CASE
    WHEN b.bottle_id = ANY(sqlc.arg(favorite_ids)::bigint[]) THEN 0
    WHEN b.bottle_id = ANY(sqlc.arg(personal_ids)::bigint[]) THEN 1
    WHEN b.bottle_id = ANY(sqlc.arg(household_ids)::bigint[]) THEN 2
    WHEN EXISTS (
      SELECT 1 FROM unnest(sqlc.arg(search_terms)::text[]) term
      WHERE position(lower(term) in lower(
        coalesce(b.vineyard, '') || ' ' || t.name || ' ' || c.name || ' ' || rg.name
      )) > 0
    ) THEN 3
    WHEN b.bottle_id = ANY(sqlc.arg(global_ids)::bigint[]) THEN 4
    ELSE 5
  END,
  array_position(sqlc.arg(favorite_ids)::bigint[], b.bottle_id),
  array_position(sqlc.arg(personal_ids)::bigint[], b.bottle_id),
  array_position(sqlc.arg(household_ids)::bigint[], b.bottle_id),
  array_position(sqlc.arg(global_ids)::bigint[], b.bottle_id),
  lower(coalesce(b.vineyard, '')),
  b.vintage_year,
  b.bottle_id
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchBottles :one
-- Must apply the exact same predicates as SearchBottles so pageInfo.totalCount
-- agrees with the returned rows.
SELECT COUNT(*)
FROM wine.bottle b
JOIN wine.type t ON t.type_id = b.type_id
JOIN wine.country c ON c.country_id = b.country_id
JOIN wine.region rg ON rg.region_id = b.region_id
WHERE (
    sqlc.narg('search')::text IS NULL
    OR position(lower(sqlc.narg('search')) in lower(
      coalesce(b.vineyard, '') || ' ' || t.name || ' ' || c.name || ' ' || rg.name
    )) > 0
  )
  AND (
    sqlc.narg('country_id')::bigint IS NULL
    OR b.country_id = sqlc.narg('country_id')::bigint
  )
  AND (
    sqlc.narg('region_id')::bigint IS NULL
    OR b.region_id = sqlc.narg('region_id')::bigint
  )
  AND (
    sqlc.narg('type_id')::bigint IS NULL
    OR b.type_id = sqlc.narg('type_id')::bigint
  )
  AND (
    sqlc.narg('vintage_year')::int IS NULL
    OR b.vintage_year = sqlc.narg('vintage_year')::int
  )
  AND (
    sqlc.narg('favorites_only')::bool IS DISTINCT FROM TRUE
    OR b.bottle_id = ANY(sqlc.arg(favorite_ids)::bigint[])
  );

-- name: MatchBottleIDs :many
-- IDs of bottles matching the term — feeds include_ids on household-scoped
-- queries that cannot join this schema (cellar search).
SELECT b.bottle_id
FROM wine.bottle b
JOIN wine.type t ON t.type_id = b.type_id
JOIN wine.country c ON c.country_id = b.country_id
JOIN wine.region rg ON rg.region_id = b.region_id
WHERE position(lower($1) in lower(
    coalesce(b.vineyard, '') || ' ' || t.name || ' ' || c.name || ' ' || rg.name
  )) > 0
LIMIT 1000;

-- name: UpdateBottle :execrows
UPDATE wine.bottle
SET type_id         = $2,
    country_id      = $3,
    region_id       = $4,
    vintage_year    = $5,
    vineyard        = $6,
    abv             = $7,
    acidity         = $8,
    tannin_level    = $9,
    body            = $10,
    sweetness       = $11,
    oak_integration = $12,
    bottle_size     = $13,
    updated_by      = $14,
    updated_at      = now()
WHERE bottle_id = $1;

-- name: DeleteBottle :exec
DELETE FROM wine.bottle
WHERE bottle_id = $1;

-- name: CreateVintage :one
INSERT INTO wine.vintage (year, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListVintages :many
SELECT *
FROM wine.vintage
ORDER BY year DESC;

-- name: CreateGrapeVariety :one
INSERT INTO wine.grape_variety (name, description, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGrapeVarieties :many
SELECT *
FROM wine.grape_variety
ORDER BY name;

-- name: CreateBottleGrapeVariety :one
INSERT INTO wine.bottle_grape_variety (bottle_id, grape_variety_id, percentage, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListBottleGrapeVarieties :many
SELECT gv.grape_variety_id, gv.name, bgv.percentage
FROM wine.bottle_grape_variety bgv
JOIN wine.grape_variety gv ON bgv.grape_variety_id = gv.grape_variety_id
WHERE bgv.bottle_id = $1
ORDER BY gv.name;

-- name: ListBottleGrapeVarietiesByBottles :many
SELECT bgv.bottle_id, gv.grape_variety_id, gv.name, bgv.percentage
FROM wine.bottle_grape_variety bgv
JOIN wine.grape_variety gv ON bgv.grape_variety_id = gv.grape_variety_id
WHERE bgv.bottle_id = ANY(sqlc.arg(bottle_ids)::bigint[])
ORDER BY gv.name;

-- name: GetBottlesByIDs :many
SELECT *
FROM wine.bottle
WHERE bottle_id = ANY(sqlc.arg(bottle_ids)::bigint[]);

-- name: DeleteBottleGrapeVariety :exec
DELETE FROM wine.bottle_grape_variety
WHERE bottle_id = $1 AND grape_variety_id = $2;

-- name: ListWineFlavorProfiles :many
SELECT *
FROM wine.flavor_profile
ORDER BY name;

-- name: GetWineFlavorProfileByID :one
SELECT *
FROM wine.flavor_profile
WHERE flavor_profile_id = $1;

-- name: CreateWineFlavorProfile :one
INSERT INTO wine.flavor_profile (name, description, is_active, created_by, updated_by)
VALUES ($1, $2, true, $3, $3)
RETURNING *;

-- name: UpdateWineFlavorProfile :one
UPDATE wine.flavor_profile
SET name        = $2,
    description = $3,
    is_active   = $4,
    updated_by  = $5,
    updated_at  = now()
WHERE flavor_profile_id = $1
RETURNING *;

-- name: DeleteWineFlavorProfile :exec
DELETE FROM wine.flavor_profile
WHERE flavor_profile_id = $1;

-- name: CreateBottleFlavorProfile :one
INSERT INTO wine.bottle_flavor_profile (bottle_id, flavor_profile_id, intensity, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListBottleFlavorProfiles :many
SELECT fp.flavor_profile_id, fp.name, bfp.intensity
FROM wine.bottle_flavor_profile bfp
JOIN wine.flavor_profile fp ON bfp.flavor_profile_id = fp.flavor_profile_id
WHERE bfp.bottle_id = $1
ORDER BY fp.name;

-- name: ListBottleFlavorProfilesByBottles :many
SELECT bfp.bottle_id, fp.flavor_profile_id, fp.name, bfp.intensity
FROM wine.bottle_flavor_profile bfp
JOIN wine.flavor_profile fp ON bfp.flavor_profile_id = fp.flavor_profile_id
WHERE bfp.bottle_id = ANY(sqlc.arg(bottle_ids)::bigint[])
ORDER BY fp.name;

-- name: DeleteBottleFlavorProfile :exec
DELETE FROM wine.bottle_flavor_profile
WHERE bottle_id = $1 AND flavor_profile_id = $2;

-- name: UpdateCountry :one
UPDATE wine.country
SET name        = $2,
    iso_code    = $3,
    description = $4,
    is_active   = $5,
    updated_by  = $6,
    updated_at  = now()
WHERE country_id = $1
RETURNING *;

-- name: DeleteCountry :exec
DELETE FROM wine.country
WHERE country_id = $1;

-- name: UpdateRegion :one
UPDATE wine.region
SET country_id  = $2,
    name        = $3,
    description = $4,
    is_active   = $5,
    updated_by  = $6,
    updated_at  = now()
WHERE region_id = $1
RETURNING *;

-- name: DeleteRegion :exec
DELETE FROM wine.region
WHERE region_id = $1;

-- name: UpdateType :one
UPDATE wine.type
SET name        = $2,
    description = $3,
    is_active   = $4,
    updated_by  = $5,
    updated_at  = now()
WHERE type_id = $1
RETURNING *;

-- name: DeleteType :exec
DELETE FROM wine.type
WHERE type_id = $1;

-- name: UpdateVintage :one
UPDATE wine.vintage
SET year        = $2,
    description = $3,
    is_active   = $4,
    updated_by  = $5,
    updated_at  = now()
WHERE vintage_id = $1
RETURNING *;

-- name: DeleteVintage :exec
DELETE FROM wine.vintage
WHERE vintage_id = $1;

-- name: UpdateGrapeVariety :one
UPDATE wine.grape_variety
SET name        = $2,
    description = $3,
    is_active   = $4,
    updated_by  = $5,
    updated_at  = now()
WHERE grape_variety_id = $1
RETURNING *;

-- name: DeleteGrapeVariety :exec
DELETE FROM wine.grape_variety
WHERE grape_variety_id = $1;
