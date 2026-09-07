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

-- name: UpdateRecipe :exec
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
INSERT INTO recipe.recipe_step (recipe_id, step_number, instruction, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
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

-- name: UpdateRecipeStep :exec
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

-- name: ListRatingRecencySuggestions :many
-- For one user: recipes rated at or above a threshold, scored by how long
-- it has been since each last appeared in a meal plan. Recipes never planned
-- score 1; the score clamps to [0,1] over 180 days.
WITH last_used AS (
    SELECT rr.recipe_id,
           rr.rating,
           MAX(mp.week_start_date) AS last_used
    FROM recipe.recipe_rating rr
    LEFT JOIN mealplan.meal_slot ms
           ON ms.recipe_id = rr.recipe_id
    LEFT JOIN mealplan.meal_plan mp
           ON mp.meal_plan_id = ms.meal_plan_id AND mp.user_id = rr.user_id
    WHERE rr.user_id = $1 AND rr.rating >= $2
    GROUP BY rr.recipe_id, rr.rating
)
SELECT recipe_id,
       rating,
       last_used::date AS last_used,
       GREATEST(LEAST(
           CASE
               WHEN last_used IS NULL THEN 180.0
               ELSE (CURRENT_DATE - last_used)::float8
           END / 180.0,
       1.0), 0.0)::float8 AS score
FROM last_used
ORDER BY score DESC, recipe_id ASC
LIMIT $3;
