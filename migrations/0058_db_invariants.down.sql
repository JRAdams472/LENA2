-- 0058_db_invariants.down.sql — drop the new constraints and indexes.
-- The negative-quantity clamp is not reversible (rows were already
-- normalized), so it is intentionally a no-op.

ALTER TABLE userprefs.household_item
    DROP CONSTRAINT IF EXISTS household_item_current_qty_nonnegative,
    DROP CONSTRAINT IF EXISTS household_item_min_qty_nonnegative;

ALTER TABLE userprefs.household_bottle
    DROP CONSTRAINT IF EXISTS household_bottle_quantity_nonnegative;

DROP INDEX IF EXISTS mealplan.meal_plan_one_active_per_household;
DROP INDEX IF EXISTS grocery.idx_grocery_list_household;
DROP INDEX IF EXISTS grocery.idx_grocery_list_item_list;
DROP INDEX IF EXISTS mealplan.idx_meal_slot_plan;
DROP INDEX IF EXISTS mealplan.idx_meal_slot_item_slot;
DROP INDEX IF EXISTS mealplan.idx_meal_slot_recipe;
