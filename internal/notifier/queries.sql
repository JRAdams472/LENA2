-- name: ListActiveNotificationTypes :many
SELECT kind, category, label, description
FROM household.notification_type
WHERE is_active
ORDER BY category, kind;

-- name: GetNotificationTypeCategory :one
SELECT category
FROM household.notification_type
WHERE kind = $1 AND is_active;

-- name: ListNotificationPrefs :many
SELECT *
FROM userprefs.notification_pref
WHERE user_id = $1;

-- name: UpsertNotificationPref :exec
INSERT INTO userprefs.notification_pref (user_id, category, enabled, muted_until, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (user_id, category)
    DO UPDATE SET enabled = EXCLUDED.enabled,
                  muted_until = EXCLUDED.muted_until,
                  updated_at = now();

-- name: ListMealSlotsWithRecipes :many
-- Every recipe-backed slot on each household's active plans. The sweep
-- computes slot dates in Go so the query stays a simple join.
SELECT s.slot_id, s.day_of_week, s.meal_type, s.servings,
       p.week_start_date, p.week_start_day_of_week,
       p.household_id,
       r.recipe_id, r.name AS recipe_name, r.servings AS recipe_servings
FROM mealplan.meal_slot s
JOIN mealplan.meal_plan p ON p.meal_plan_id = s.meal_plan_id
JOIN recipe.recipe r ON r.recipe_id = s.recipe_id
WHERE p.is_active
  AND s.recipe_id IS NOT NULL;

-- name: ListProteinItemsForRecipes :many
-- Ingredient rows whose item's inventory category is flagged protein, with
-- the unit's weight conversion factor (NULL unit_id or a non-weight kind
-- means the quantity can't contribute to the lbs total — Go skips it).
SELECT ri.recipe_id, ri.quantity, i.name AS item_name,
       un.kind AS unit_kind, un.to_base_factor
FROM recipe.recipe_item ri
JOIN inventory.item i ON i.item_id = ri.item_id
JOIN inventory.category c ON c.category_id = i.category_id
                          AND c.is_protein
LEFT JOIN inventory.unit un ON un.unit_id = ri.unit_id
WHERE ri.recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[]);

-- name: ListLongStepsForRecipes :many
-- Steps of a full day or longer are "advance prep" for notification purposes.
SELECT recipe_id, MAX(duration_minutes)::int AS max_duration_minutes
FROM recipe.recipe_step
WHERE recipe_id = ANY(sqlc.arg(recipe_ids)::bigint[])
  AND duration_minutes >= 1440
GROUP BY recipe_id;

-- name: ListExpiringHouseholdItems :many
SELECT hi.household_item_id, hi.household_id, hi.item_id,
       i.name AS item_name, hi.expires_at
FROM userprefs.household_item hi
JOIN inventory.item i ON i.item_id = hi.item_id
WHERE hi.expires_at IS NOT NULL
  AND hi.expires_at > $1
  AND hi.expires_at <= $2;

-- name: ListMemberIDsForHouseholds :many
SELECT user_id, household_id
FROM identity.users
WHERE household_id = ANY(sqlc.arg(household_ids)::bigint[]);

-- name: InsertRecipeReminderNotification :execresult
-- dedup_key makes every scheduled insert naturally idempotent. NULL keys never
-- conflict, so event-driven rows are unaffected.
INSERT INTO household.notifications
    (user_id, household_id, kind, title, body, recipe_id, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (dedup_key) DO NOTHING;

-- name: InsertItemReminderNotification :execresult
INSERT INTO household.notifications
    (user_id, household_id, kind, title, body, item_id, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (dedup_key) DO NOTHING;

-- name: PruneReadNotifications :exec
DELETE FROM household.notifications n
WHERE n.user_id = $1
  AND n.read_at IS NOT NULL
  AND n.notification_id NOT IN (
      SELECT k.notification_id
      FROM household.notifications k
      WHERE k.user_id = $1
        AND k.read_at IS NOT NULL
      ORDER BY k.created_at DESC
      LIMIT 100
  );
