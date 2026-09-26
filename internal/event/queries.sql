-- name: CreateFoodEvent :one
INSERT INTO event.food_event (household_id, name, event_date, slot_granularity_minutes, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetFoodEventByID :one
SELECT *
FROM event.food_event
WHERE food_event_id = $1 AND household_id = $2;

-- name: ListFoodEvents :many
SELECT *
FROM event.food_event
WHERE household_id = $1
ORDER BY event_date DESC, food_event_id DESC
LIMIT $2 OFFSET $3;

-- name: CountFoodEvents :one
SELECT COUNT(*)
FROM event.food_event
WHERE household_id = $1;

-- name: UpdateFoodEvent :exec
UPDATE event.food_event
SET name                     = $3,
    event_date               = $4,
    slot_granularity_minutes = $5,
    is_active                = $6,
    updated_by               = $7,
    updated_at               = now()
WHERE food_event_id = $1 AND household_id = $2;

-- name: DeleteFoodEvent :exec
DELETE FROM event.food_event
WHERE food_event_id = $1 AND household_id = $2;

-- name: ReassignFoodEventsToHousehold :exec
-- Invite-accept merge: repoint all of the source household's events. Zero
-- rows is not an error (the source may have had none).
UPDATE event.food_event
SET household_id = sqlc.arg(to_household_id),
    updated_by   = sqlc.arg(updated_by)::varchar,
    updated_at   = now()
WHERE household_id = sqlc.arg(from_household_id);

-- name: AddEventRecipe :one
INSERT INTO event.event_recipe (food_event_id, recipe_id, meal_type, target_time, servings, notes, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetEventRecipeByID :one
SELECT er.*
FROM event.event_recipe er
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE er.event_recipe_id = $1 AND fe.household_id = $2;

-- name: ListEventRecipesForEvent :many
SELECT er.*
FROM event.event_recipe er
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE er.food_event_id = $1 AND fe.household_id = $2
ORDER BY er.target_time, er.event_recipe_id;

-- name: ListEventRecipesByEvents :many
SELECT er.*
FROM event.event_recipe er
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE er.food_event_id = ANY(sqlc.arg(food_event_ids)::bigint[]) AND fe.household_id = sqlc.arg(household_id)
ORDER BY er.target_time, er.event_recipe_id;

-- name: UpdateEventRecipe :exec
UPDATE event.event_recipe er
SET recipe_id   = $3,
    meal_type   = $4,
    target_time = $5,
    servings    = $6,
    notes       = $7,
    updated_by  = $8,
    updated_at  = now()
FROM event.food_event fe
WHERE er.food_event_id = fe.food_event_id AND er.event_recipe_id = $1 AND fe.household_id = $2;

-- name: DeleteEventRecipe :exec
DELETE FROM event.event_recipe er
USING event.food_event fe
WHERE er.food_event_id = fe.food_event_id AND er.event_recipe_id = $1 AND fe.household_id = $2;
