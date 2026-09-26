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

-- name: ListEventRecipeStepsForEvents :many
-- Batch: steps for every slot of the given events, owned by the household.
SELECT ers.*
FROM event.event_recipe_step ers
JOIN event.event_recipe er ON ers.event_recipe_id = er.event_recipe_id
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE er.food_event_id = ANY(sqlc.arg(food_event_ids)::bigint[]) AND fe.household_id = sqlc.arg(household_id)
ORDER BY ers.event_recipe_id, ers.step_number;

-- name: ListEventRecipeSteps :many
SELECT ers.*
FROM event.event_recipe_step ers
JOIN event.event_recipe er ON ers.event_recipe_id = er.event_recipe_id
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE ers.event_recipe_id = $1 AND fe.household_id = $2
ORDER BY ers.step_number;

-- name: GetEventRecipeStepByID :one
SELECT ers.*
FROM event.event_recipe_step ers
JOIN event.event_recipe er ON ers.event_recipe_id = er.event_recipe_id
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE ers.event_recipe_step_id = $1 AND fe.household_id = $2;

-- name: DeleteEventRecipeSteps :exec
-- Clears a slot's snapshot before a re-materialization replace.
DELETE FROM event.event_recipe_step ers
USING event.event_recipe er, event.food_event fe
WHERE ers.event_recipe_id = er.event_recipe_id
  AND er.food_event_id = fe.food_event_id
  AND ers.event_recipe_id = $1 AND fe.household_id = $2;

-- name: AddEventRecipeStep :one
INSERT INTO event.event_recipe_step
    (event_recipe_id, step_number, instruction, duration_minutes, step_type,
     is_passive, depends_on_step_number, appliance, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateEventRecipeStep :exec
UPDATE event.event_recipe_step ers
SET instruction            = $3,
    duration_minutes       = $4,
    step_type              = $5,
    is_passive             = $6,
    depends_on_step_number = $7,
    appliance              = $8,
    updated_by             = $9,
    updated_at             = now()
FROM event.event_recipe er, event.food_event fe
WHERE ers.event_recipe_id = er.event_recipe_id
  AND er.food_event_id = fe.food_event_id
  AND ers.event_recipe_step_id = $1 AND fe.household_id = $2;

-- name: DeleteEventRecipeStep :exec
DELETE FROM event.event_recipe_step ers
USING event.event_recipe er, event.food_event fe
WHERE ers.event_recipe_id = er.event_recipe_id
  AND er.food_event_id = fe.food_event_id
  AND ers.event_recipe_step_id = $1 AND fe.household_id = $2;

-- name: SetEventRecipeBaseServings :exec
-- Freezes the linked recipe's servings as the scaling denominator.
UPDATE event.event_recipe er
SET base_servings = $3, updated_by = $4, updated_at = now()
FROM event.food_event fe
WHERE er.food_event_id = fe.food_event_id AND er.event_recipe_id = $1 AND fe.household_id = $2;

-- name: ListEventRecipeItemsForEvents :many
-- Batch: items for every slot of the given events, owned by the household.
SELECT eri.*
FROM event.event_recipe_item eri
JOIN event.event_recipe er ON eri.event_recipe_id = er.event_recipe_id
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE er.food_event_id = ANY(sqlc.arg(food_event_ids)::bigint[]) AND fe.household_id = sqlc.arg(household_id)
ORDER BY eri.event_recipe_id, eri.display_order, eri.event_recipe_item_id;

-- name: ListEventRecipeItems :many
SELECT eri.*
FROM event.event_recipe_item eri
JOIN event.event_recipe er ON eri.event_recipe_id = er.event_recipe_id
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE eri.event_recipe_id = $1 AND fe.household_id = $2
ORDER BY eri.display_order, eri.event_recipe_item_id;

-- name: GetEventRecipeItemByID :one
SELECT eri.*
FROM event.event_recipe_item eri
JOIN event.event_recipe er ON eri.event_recipe_id = er.event_recipe_id
JOIN event.food_event fe ON er.food_event_id = fe.food_event_id
WHERE eri.event_recipe_item_id = $1 AND fe.household_id = $2;

-- name: DeleteEventRecipeItems :exec
-- Clears a slot's item snapshot before a re-materialization replace.
DELETE FROM event.event_recipe_item eri
USING event.event_recipe er, event.food_event fe
WHERE eri.event_recipe_id = er.event_recipe_id
  AND er.food_event_id = fe.food_event_id
  AND eri.event_recipe_id = $1 AND fe.household_id = $2;

-- name: AddEventRecipeItem :one
INSERT INTO event.event_recipe_item
    (event_recipe_id, item_id, ingredient_id, quantity, unit_id,
     section_name, display_order, notes, is_optional, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateEventRecipeItem :exec
UPDATE event.event_recipe_item eri
SET item_id       = $3,
    ingredient_id = $4,
    quantity      = $5,
    unit_id       = $6,
    section_name  = $7,
    display_order = $8,
    notes         = $9,
    is_optional   = $10,
    updated_by    = $11,
    updated_at    = now()
FROM event.event_recipe er, event.food_event fe
WHERE eri.event_recipe_id = er.event_recipe_id
  AND er.food_event_id = fe.food_event_id
  AND eri.event_recipe_item_id = $1 AND fe.household_id = $2;

-- name: DeleteEventRecipeItem :exec
DELETE FROM event.event_recipe_item eri
USING event.event_recipe er, event.food_event fe
WHERE eri.event_recipe_id = er.event_recipe_id
  AND er.food_event_id = fe.food_event_id
  AND eri.event_recipe_item_id = $1 AND fe.household_id = $2;
