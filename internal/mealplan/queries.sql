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
SELECT ms.*
FROM mealplan.meal_slot ms
JOIN mealplan.meal_plan mp ON ms.meal_plan_id = mp.meal_plan_id
WHERE ms.slot_id = $1 AND mp.user_id = $2;

-- name: ListMealSlotsForPlan :many
SELECT ms.*
FROM mealplan.meal_slot ms
JOIN mealplan.meal_plan mp ON ms.meal_plan_id = mp.meal_plan_id
WHERE ms.meal_plan_id = $1 AND mp.user_id = $2
ORDER BY ms.day_of_week, ms.meal_type;

-- name: ListMealSlotsByPlans :many
SELECT ms.*
FROM mealplan.meal_slot ms
JOIN mealplan.meal_plan mp ON ms.meal_plan_id = mp.meal_plan_id
WHERE ms.meal_plan_id = ANY(sqlc.arg(meal_plan_ids)::bigint[]) AND mp.user_id = sqlc.arg(user_id)
ORDER BY ms.day_of_week, ms.meal_type;

-- name: UpdateMealSlot :exec
UPDATE mealplan.meal_slot ms
SET day_of_week      = $3,
    meal_type        = $4,
    recipe_id        = $5,
    servings         = $6,
    replacement_note = $7,
    updated_by       = $8,
    updated_at       = now()
FROM mealplan.meal_plan mp
WHERE ms.meal_plan_id = mp.meal_plan_id AND ms.slot_id = $1 AND mp.user_id = $2;

-- name: DeleteMealSlot :exec
DELETE FROM mealplan.meal_slot ms
USING mealplan.meal_plan mp
WHERE ms.meal_plan_id = mp.meal_plan_id AND ms.slot_id = $1 AND mp.user_id = $2;

-- name: AddMealSlotItem :one
INSERT INTO mealplan.meal_slot_item (slot_id, item_id, ingredient_id, quantity, unit_id, is_from_recipe, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListMealSlotItems :many
SELECT msi.*
FROM mealplan.meal_slot_item msi
JOIN mealplan.meal_slot ms ON msi.slot_id = ms.slot_id
JOIN mealplan.meal_plan mp ON ms.meal_plan_id = mp.meal_plan_id
WHERE msi.slot_id = $1 AND mp.user_id = $2
ORDER BY msi.slot_item_id;

-- name: ListMealSlotItemsByPlan :many
SELECT msi.*
FROM mealplan.meal_slot_item msi
JOIN mealplan.meal_slot ms ON msi.slot_id = ms.slot_id
JOIN mealplan.meal_plan mp ON ms.meal_plan_id = mp.meal_plan_id
WHERE ms.meal_plan_id = $1 AND mp.user_id = $2
ORDER BY msi.slot_item_id;

-- name: ListMealSlotItemsByPlans :many
SELECT msi.*
FROM mealplan.meal_slot_item msi
JOIN mealplan.meal_slot ms ON msi.slot_id = ms.slot_id
JOIN mealplan.meal_plan mp ON ms.meal_plan_id = mp.meal_plan_id
WHERE ms.meal_plan_id = ANY(sqlc.arg(meal_plan_ids)::bigint[]) AND mp.user_id = sqlc.arg(user_id)
ORDER BY msi.slot_item_id;

-- name: DeleteMealSlotItem :exec
DELETE FROM mealplan.meal_slot_item msi
USING mealplan.meal_slot ms, mealplan.meal_plan mp
WHERE msi.slot_id = ms.slot_id AND ms.meal_plan_id = mp.meal_plan_id
  AND msi.slot_item_id = $1 AND mp.user_id = $2;
