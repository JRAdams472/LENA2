-- 0056_mealplan_single_active.up.sql — enforce one active meal plan per
-- household (LEN-79 / M-12).
--
-- is_active was a dead flag: CreateMealPlan always inserted true and no
-- code path ever deactivated a plan, so every household accumulated rows
-- that all read "Active" in the clients. Keep the most recently created
-- active plan per household and clear the rest; the service layer now
-- maintains the invariant going forward.

UPDATE mealplan.meal_plan
SET is_active = false
WHERE is_active
  AND meal_plan_id NOT IN (
    SELECT DISTINCT ON (household_id) meal_plan_id
    FROM mealplan.meal_plan
    WHERE is_active
    ORDER BY household_id, created_at DESC, meal_plan_id DESC
  );
