-- name: CreateRecipe :one
INSERT INTO recipe.recipe (name, description, servings, prep_time_minutes, cook_time_minutes, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetRecipeByID :one
SELECT *
FROM recipe.recipe
WHERE recipe_id = $1;

-- name: ListRecipes :many
SELECT *
FROM recipe.recipe
WHERE is_active = $1
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: CountRecipes :one
SELECT COUNT(*)
FROM recipe.recipe
WHERE is_active = $1;

-- name: GetRecipesByIDs :many
SELECT *
FROM recipe.recipe
WHERE recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[]);

-- name: UpdateRecipe :execrows
UPDATE recipe.recipe
SET name              = $2,
    description       = $3,
    servings          = $4,
    prep_time_minutes = $5,
    cook_time_minutes = $6,
    is_active         = $7,
    updated_by        = $8,
    updated_at        = now()
WHERE recipe_id = $1;

-- name: DeleteRecipe :exec
DELETE FROM recipe.recipe
WHERE recipe_id = $1;

-- name: AddRecipeItem :exec
INSERT INTO recipe.recipe_item (recipe_id, item_id, ingredient_id, quantity, unit_id, section_name, display_order, notes, is_optional)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListRecipeItems :many
SELECT *
FROM recipe.recipe_item
WHERE recipe_id = $1
ORDER BY display_order, recipe_item_id;

-- name: ListRecipeItemsByRecipes :many
SELECT *
FROM recipe.recipe_item
WHERE recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[])
ORDER BY recipe_id, display_order, recipe_item_id;

-- name: DeleteRecipeItems :exec
DELETE FROM recipe.recipe_item
WHERE recipe_id = $1;

-- name: RemoveRecipeItem :exec
DELETE FROM recipe.recipe_item
WHERE recipe_item_id = $1;

-- name: AddRecipeStep :one
INSERT INTO recipe.recipe_step (recipe_id, step_number, instruction, duration_minutes, step_type, is_passive, depends_on_step_number, appliance, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListRecipeSteps :many
SELECT *
FROM recipe.recipe_step
WHERE recipe_id = $1
ORDER BY step_number;

-- name: ListRecipeStepsByRecipes :many
SELECT *
FROM recipe.recipe_step
WHERE recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[])
ORDER BY step_number;

-- name: UpdateRecipeStep :execrows
-- Timing columns are written only by the create/replace-children path
-- (AddRecipeStep); this partial update preserves them.
UPDATE recipe.recipe_step
SET step_number = $2,
    instruction = $3,
    updated_by  = $4,
    updated_at  = now()
WHERE step_id = $1;

-- name: DeleteRecipeSteps :exec
DELETE FROM recipe.recipe_step
WHERE recipe_id = $1;

-- name: DeleteRecipeStep :exec
DELETE FROM recipe.recipe_step
WHERE step_id = $1;

-- name: UpsertRecipeRating :one
INSERT INTO recipe.recipe_rating (user_id, recipe_id, rating, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, recipe_id)
    DO UPDATE SET
        rating     = EXCLUDED.rating,
        updated_by = EXCLUDED.updated_by,
        updated_at = now()
RETURNING *;

-- name: GetRecipeRating :one
SELECT *
FROM recipe.recipe_rating
WHERE user_id = $1 AND recipe_id = $2;

-- name: ListRecipeRatings :many
SELECT *
FROM recipe.recipe_rating
WHERE user_id = $1 AND recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[]);

-- name: ListRecipeRatingSummaries :many
SELECT recipe_id,
       AVG(rating)::float8 AS average_rating,
       COUNT(*)            AS rating_count
FROM recipe.recipe_rating
WHERE recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[])
GROUP BY recipe_id;

-- name: ListRecipeRatingsAtLeast :many
-- One user's recipe ratings at or above a threshold. Recency scoring joins
-- this to mealplan data in the BFF; SQL never crosses schemas.
SELECT recipe_id, rating
FROM recipe.recipe_rating
WHERE user_id = $1 AND rating >= $2;

-- ---------- recipe categories (0035) ----------

-- name: CreateCategoryGroup :one
INSERT INTO recipe.category_group (name, exclusive, display_order, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateCategoryGroup :one
UPDATE recipe.category_group
SET name          = $2,
    exclusive     = $3,
    display_order = $4,
    updated_by    = $5,
    updated_at    = now()
WHERE category_group_id = $1
RETURNING *;

-- name: DeleteCategoryGroup :exec
DELETE FROM recipe.category_group
WHERE category_group_id = $1;

-- name: GetCategoryGroupByID :one
SELECT *
FROM recipe.category_group
WHERE category_group_id = $1;

-- name: ListCategoryGroups :many
SELECT *
FROM recipe.category_group
ORDER BY display_order, name;

-- name: CountCategoriesInGroup :one
SELECT COUNT(*)
FROM recipe.category
WHERE category_group_id = $1;

-- name: CreateCategory :one
INSERT INTO recipe.category (category_group_id, name, created_by, updated_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateCategory :one
UPDATE recipe.category
SET name       = $2,
    updated_by = $3,
    updated_at = now()
WHERE category_id = $1
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM recipe.category
WHERE category_id = $1;

-- name: GetCategoryByID :one
SELECT *
FROM recipe.category
WHERE category_id = $1;

-- name: ListCategoriesByGroup :many
SELECT *
FROM recipe.category
WHERE category_group_id = $1
ORDER BY name;

-- name: ListCategoriesByIDs :many
-- Categories with their group's exclusivity/name, for assignment-time
-- validation (a recipe may hold at most one category per exclusive group).
SELECT c.*, g.name AS group_name, g.exclusive AS group_exclusive, g.display_order AS group_display_order
FROM recipe.category c
JOIN recipe.category_group g ON g.category_group_id = c.category_group_id
WHERE c.category_id = ANY(sqlc.arg(category_ids)::bigint[]);

-- name: ClearRecipeCategories :exec
DELETE FROM recipe.recipe_category
WHERE recipe_id = $1;

-- name: AddRecipeCategories :exec
INSERT INTO recipe.recipe_category (recipe_id, category_id, assigned_by)
SELECT $1, unnest($2::bigint[]), $3
ON CONFLICT DO NOTHING;

-- name: ListCategoriesForRecipes :many
-- Batch child preload: every category each recipe carries, with group
-- metadata so resolvers never query per-row.
SELECT rc.recipe_id, c.category_id, c.name, c.category_group_id,
       g.name AS group_name, g.exclusive AS group_exclusive, g.display_order AS group_display_order
FROM recipe.recipe_category rc
JOIN recipe.category c ON c.category_id = rc.category_id
JOIN recipe.category_group g ON g.category_group_id = c.category_group_id
WHERE rc.recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[])
ORDER BY g.display_order, c.name;

-- name: SearchRecipes :many
-- Filtered + engagement-ranked recipe listing. Ranking tiers come from
-- engagement ID arrays computed by the BFF (analytics/userprefs live in
-- other schemas — SQL never crosses schemas):
--   0 favorite, 1 course-boost (meal-type match), 2 used (household
--   menus), 3 viewed, 4 searched, 5 rest.
-- The used/viewed arrays arrive pre-sorted by signal strength so
-- array_position doubles as the in-tier tiebreaker.
SELECT r.*
FROM recipe.recipe r
WHERE r.is_active = $1
  AND (sqlc.arg(search)::text IS NULL OR lower(r.name) LIKE '%' || lower(sqlc.arg(search)) || '%')
  AND (
    sqlc.arg(category_ids)::bigint[] IS NULL
    OR (
      SELECT COUNT(DISTINCT c.category_group_id)
      FROM recipe.recipe_category rc
      JOIN recipe.category c ON c.category_id = rc.category_id
      WHERE rc.recipe_id = r.recipe_id AND c.category_id = ANY(sqlc.arg(category_ids)::bigint[])
    ) = (
      SELECT COUNT(DISTINCT category_group_id)
      FROM recipe.category
      WHERE category_id = ANY(sqlc.arg(category_ids)::bigint[])
    )
  )
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR r.recipe_id = ANY(sqlc.arg(include_ids)::bigint[]))
  AND (sqlc.arg(exclude_ids)::bigint[] IS NULL OR NOT (r.recipe_id = ANY(sqlc.arg(exclude_ids)::bigint[])))
ORDER BY
  CASE
    WHEN sqlc.arg(favorite_ids)::bigint[] IS NOT NULL AND r.recipe_id = ANY(sqlc.arg(favorite_ids)::bigint[]) THEN 0
    WHEN sqlc.narg(boost_category_id)::bigint IS NOT NULL AND EXISTS (
      SELECT 1 FROM recipe.recipe_category rc
      WHERE rc.recipe_id = r.recipe_id
        AND rc.category_id = sqlc.narg(boost_category_id)::bigint
    ) THEN 1
    WHEN r.recipe_id = ANY(sqlc.arg(used_ids)::bigint[]) THEN 2
    WHEN r.recipe_id = ANY(sqlc.arg(viewed_ids)::bigint[]) THEN 3
    WHEN EXISTS (
      SELECT 1 FROM unnest(sqlc.arg(search_terms)::text[]) t
      WHERE position(lower(t) in lower(r.name)) > 0
    ) THEN 4
    ELSE 5
  END,
  array_position(sqlc.arg(used_ids)::bigint[], r.recipe_id),
  array_position(sqlc.arg(viewed_ids)::bigint[], r.recipe_id),
  r.name
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchRecipes :one
SELECT COUNT(*)
FROM recipe.recipe r
WHERE r.is_active = $1
  AND (sqlc.arg(search)::text IS NULL OR lower(r.name) LIKE '%' || lower(sqlc.arg(search)) || '%')
  AND (
    sqlc.arg(category_ids)::bigint[] IS NULL
    OR (
      SELECT COUNT(DISTINCT c.category_group_id)
      FROM recipe.recipe_category rc
      JOIN recipe.category c ON c.category_id = rc.category_id
      WHERE rc.recipe_id = r.recipe_id AND c.category_id = ANY(sqlc.arg(category_ids)::bigint[])
    ) = (
      SELECT COUNT(DISTINCT category_group_id)
      FROM recipe.category
      WHERE category_id = ANY(sqlc.arg(category_ids)::bigint[])
    )
  )
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR r.recipe_id = ANY(sqlc.arg(include_ids)::bigint[]))
  AND (sqlc.arg(exclude_ids)::bigint[] IS NULL OR NOT (r.recipe_id = ANY(sqlc.arg(exclude_ids)::bigint[])));

-- name: SearchRecipesSemantic :many
-- Vector-similarity recipe listing. Only embedded rows participate (the
-- backfill sweep fills the rest). Ranking blends cosine distance with a
-- small additive engagement bump — favorites 1.0, household-used 0.6,
-- viewed 0.3, scaled by semanticEngagementBoost — so a mediocre-vector
-- favorite can't swamp a strong match. The same category/include/exclude
-- filters as SearchRecipes apply; there is no name-LIKE filter since the
-- query text becomes the vector.
SELECT r.*, (r.embedding <=> sqlc.arg(query_vec)::text::vector)::float8 AS distance
FROM recipe.recipe r
WHERE r.is_active = $1
  AND r.embedding IS NOT NULL
  AND (
    sqlc.arg(category_ids)::bigint[] IS NULL
    OR (
      SELECT COUNT(DISTINCT c.category_group_id)
      FROM recipe.recipe_category rc
      JOIN recipe.category c ON c.category_id = rc.category_id
      WHERE rc.recipe_id = r.recipe_id AND c.category_id = ANY(sqlc.arg(category_ids)::bigint[])
    ) = (
      SELECT COUNT(DISTINCT category_group_id)
      FROM recipe.category
      WHERE category_id = ANY(sqlc.arg(category_ids)::bigint[])
    )
  )
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR r.recipe_id = ANY(sqlc.arg(include_ids)::bigint[]))
  AND (sqlc.arg(exclude_ids)::bigint[] IS NULL OR NOT (r.recipe_id = ANY(sqlc.arg(exclude_ids)::bigint[])))
ORDER BY
  (r.embedding <=> sqlc.arg(query_vec)::text::vector)
    - (0.15 * CASE
        WHEN sqlc.arg(favorite_ids)::bigint[] IS NOT NULL AND r.recipe_id = ANY(sqlc.arg(favorite_ids)::bigint[]) THEN 1.0
        WHEN r.recipe_id = ANY(sqlc.arg(used_ids)::bigint[]) THEN 0.6
        WHEN r.recipe_id = ANY(sqlc.arg(viewed_ids)::bigint[]) THEN 0.3
        ELSE 0
      END),
  r.name
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSearchRecipesSemantic :one
-- The un-paged match count for the same semantic-mode filters (no
-- engagement args — they only affect ordering).
SELECT COUNT(*)
FROM recipe.recipe r
WHERE r.is_active = $1
  AND r.embedding IS NOT NULL
  AND (
    sqlc.arg(category_ids)::bigint[] IS NULL
    OR (
      SELECT COUNT(DISTINCT c.category_group_id)
      FROM recipe.recipe_category rc
      JOIN recipe.category c ON c.category_id = rc.category_id
      WHERE rc.recipe_id = r.recipe_id AND c.category_id = ANY(sqlc.arg(category_ids)::bigint[])
    ) = (
      SELECT COUNT(DISTINCT category_group_id)
      FROM recipe.category
      WHERE category_id = ANY(sqlc.arg(category_ids)::bigint[])
    )
  )
  AND (sqlc.arg(include_ids)::bigint[] IS NULL OR r.recipe_id = ANY(sqlc.arg(include_ids)::bigint[]))
  AND (sqlc.arg(exclude_ids)::bigint[] IS NULL OR NOT (r.recipe_id = ANY(sqlc.arg(exclude_ids)::bigint[])));

-- name: SetRecipeEmbedding :exec
-- Stores an embedding for semantic search. The vector arrives as a text
-- literal and is cast server-side so generated code stays dependency-free.
UPDATE recipe.recipe
SET embedding = sqlc.arg(embedding)::text::vector,
    embedding_model = sqlc.arg(embedding_model),
    embedding_at = now()
WHERE recipe_id = sqlc.arg(recipe_id);

-- name: ListEmbeddingCandidates :many
-- Active recipes whose embedding is missing or was built by another model —
-- the backfill sweep's work set.
SELECT recipe_id
FROM recipe.recipe
WHERE is_active
  AND (embedding IS NULL OR embedding_model <> sqlc.arg(model)::text)
ORDER BY recipe_id
LIMIT sqlc.arg('limit')::int;

-- name: ClearRecipeEmbedding :exec
-- Drops a recipe's embedding (stale-failure bookkeeping; the sweep will
-- retry on its next pass since embedding becomes NULL).
UPDATE recipe.recipe
SET embedding = NULL,
    embedding_model = NULL,
    embedding_at = NULL
WHERE recipe_id = sqlc.arg(recipe_id);

-- name: GetRecipeDelta :one
-- The household's delta for a recipe, or no row.
SELECT *
FROM recipe.recipe_delta
WHERE recipe_id = $1 AND household_id = $2;

-- name: ListRecipeDeltas :many
-- The household's deltas across a set of recipes — the batch load behind
-- loadRecipeChildren/planRecipes.
SELECT *
FROM recipe.recipe_delta
WHERE household_id = $1 AND recipe_id = ANY($2::bigint[]);

-- name: UpsertRecipeDelta :one
-- Creates the delta row on first change or touches updated_* on later
-- writes. base_updated_at snapshots the canonical recipe's current
-- updated_at (created_at when never edited) so stale detection compares
-- against a real version marker.
INSERT INTO recipe.recipe_delta (recipe_id, household_id, base_updated_at, created_by, updated_by)
SELECT $1, $2,
       COALESCE(r.updated_at, r.created_at),
       $3, $3
FROM recipe.recipe r
WHERE r.recipe_id = $1
ON CONFLICT (recipe_id, household_id)
DO UPDATE SET updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: AcknowledgeRecipeDelta :execrows
-- Marks the delta as seen against the recipe's current version — clears
-- the stale flag until the next canonical edit.
UPDATE recipe.recipe_delta
SET base_updated_at = (SELECT COALESCE(r.updated_at, r.created_at) FROM recipe.recipe r WHERE r.recipe_id = $1),
    updated_by      = $3,
    updated_at      = now()
WHERE recipe_id = $1 AND household_id = $2;

-- name: DeleteRecipeDelta :execrows
DELETE FROM recipe.recipe_delta
WHERE recipe_id = $1 AND household_id = $2;

-- name: ReplaceDeltaItems :exec
-- Whole-delta replace: clears the delta's item rows before the caller
-- re-inserts the desired set in the same transaction.
DELETE FROM recipe.recipe_delta_item
WHERE recipe_delta_id = $1;

-- name: AddDeltaItem :one
INSERT INTO recipe.recipe_delta_item
    (recipe_delta_id, recipe_item_id, kind, item_id, ingredient_id,
     quantity, unit_id, section_name, display_order, notes, is_optional,
     created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
RETURNING *;

-- name: ListDeltaItemsByDeltas :many
SELECT *
FROM recipe.recipe_delta_item
WHERE recipe_delta_id = ANY($1::bigint[])
ORDER BY recipe_delta_id, delta_item_id;

-- name: ReplaceDeltaSteps :exec
DELETE FROM recipe.recipe_delta_step
WHERE recipe_delta_id = $1;

-- name: AddDeltaStep :one
INSERT INTO recipe.recipe_delta_step
    (recipe_delta_id, step_id, kind, step_number, instruction,
     duration_minutes, step_type, is_passive, depends_on_step_number,
     appliance, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
RETURNING *;

-- name: ListDeltaStepsByDeltas :many
SELECT *
FROM recipe.recipe_delta_step
WHERE recipe_delta_id = ANY($1::bigint[])
ORDER BY recipe_delta_id, delta_step_id;
