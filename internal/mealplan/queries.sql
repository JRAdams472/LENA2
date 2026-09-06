-- name: CreateMealPlan :one
INSERT INTO mealplan.meal_plan (user_id, name, week_start_date, week_start_day_of_week, is_active, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetMealPlanByID :one
SELECT *
FROM mealplan.meal_plan
WHERE meal_plan_id = $1 AND user_id = $2;

-- name: ListMealPlans :many
SELECT *
FROM mealplan.meal_plan
WHERE user_id = $1
ORDER BY week_start_date DESC
LIMIT $2 OFFSET $3;

-- name: CountMealPlans :one
SELECT COUNT(*)
FROM mealplan.meal_plan
WHERE user_id = $1;

-- name: UpdateMealPlan :exec
UPDATE mealplan.meal_plan
SET name                = $3,
    week_start_date     = $4,
    week_start_day_of_week = $5,
    is_active           = $6,
    updated_by          = $7,
    updated_at          = now()
WHERE meal_plan_id = $1 AND user_id = $2;

-- name: DeleteMealPlan :exec
DELETE FROM mealplan.meal_plan
WHERE meal_plan_id = $1 AND user_id = $2;

-- name: AddMealSlot :one
INSERT INTO mealplan.meal_slot (meal_plan_id, day_of_week, meal_type, recipe_id, servings, replacement_note, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMealSlotByID :one
SELECT *
FROM mealplan.meal_slot
WHERE slot_id = $1;

-- name: ListMealSlotsForPlan :many
SELECT *
FROM mealplan.meal_slot
WHERE meal_plan_id = $1
ORDER BY day_of_week, meal_type;

-- name: ListMealSlotsByPlans :many
SELECT *
FROM mealplan.meal_slot
WHERE meal_plan_id = ANY(sqlc.arg(meal_plan_ids)::bigint[])
ORDER BY day_of_week, meal_type;

-- name: UpdateMealSlot :exec
UPDATE mealplan.meal_slot
SET day_of_week      = $2,
    meal_type        = $3,
    recipe_id        = $4,
    servings         = $5,
    replacement_note = $6,
    updated_by       = $7,
    updated_at       = now()
WHERE slot_id = $1;

-- name: DeleteMealSlot :exec
DELETE FROM mealplan.meal_slot
WHERE slot_id = $1;

-- name: AddMealSlotItem :one
INSERT INTO mealplan.meal_slot_item (slot_id, item_id, ingredient_id, quantity, unit, is_from_recipe, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListMealSlotItems :many
SELECT *
FROM mealplan.meal_slot_item
WHERE slot_id = $1
ORDER BY slot_item_id;

-- name: ListMealSlotItemsByPlan :many
SELECT msi.*
FROM mealplan.meal_slot_item msi
JOIN mealplan.meal_slot ms ON msi.slot_id = ms.slot_id
WHERE ms.meal_plan_id = $1
ORDER BY msi.slot_item_id;

-- name: ListMealSlotItemsByPlans :many
SELECT msi.*
FROM mealplan.meal_slot_item msi
JOIN mealplan.meal_slot ms ON msi.slot_id = ms.slot_id
WHERE ms.meal_plan_id = ANY(sqlc.arg(meal_plan_ids)::bigint[])
ORDER BY msi.slot_item_id;

-- name: DeleteMealSlotItem :exec
DELETE FROM mealplan.meal_slot_item
WHERE slot_item_id = $1;
