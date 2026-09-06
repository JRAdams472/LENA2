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
INSERT INTO recipe.recipe_item (recipe_id, item_id, ingredient_id, quantity, unit, notes, is_optional)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListRecipeItems :many
SELECT *
FROM recipe.recipe_item
WHERE recipe_id = $1
ORDER BY item_id;

-- name: ListRecipeItemsByRecipes :many
SELECT *
FROM recipe.recipe_item
WHERE recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[])
ORDER BY item_id;

-- name: DeleteRecipeItems :exec
DELETE FROM recipe.recipe_item
WHERE recipe_id = $1;

-- name: RemoveRecipeItem :exec
DELETE FROM recipe.recipe_item
WHERE recipe_id = $1 AND item_id = $2;

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
