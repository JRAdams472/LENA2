-- 0058_db_invariants.up.sql — LEN-170: quantity CHECK constraints, a
-- DB-enforced single-active-meal-plan index, and missing FK indexes on
-- hot paths.

-- ---------- non-negative pantry/cellar quantities (H13) ----------
--
-- The atomic adjust path already clamps at 0, but the absolute-set upsert
-- path accepted negatives — a negative current_qty *increased* generated
-- grocery lines and permanently tripped low-stock. Clamp any rows that
-- predate the guard before the CHECKs land, or the ALTER would fail.

UPDATE userprefs.household_item SET current_qty = 0 WHERE current_qty < 0;
UPDATE userprefs.household_item SET min_qty = 0 WHERE min_qty < 0;
UPDATE userprefs.household_bottle SET quantity = 0 WHERE quantity < 0;

ALTER TABLE userprefs.household_item
    ADD CONSTRAINT household_item_current_qty_nonnegative CHECK (current_qty >= 0),
    ADD CONSTRAINT household_item_min_qty_nonnegative CHECK (min_qty >= 0);

ALTER TABLE userprefs.household_bottle
    ADD CONSTRAINT household_bottle_quantity_nonnegative CHECK (quantity >= 0);

-- ---------- one active meal plan per household (M3) ----------
--
-- 0056 backfilled deduped actives and the service deactivates siblings,
-- but under READ COMMITTED two concurrent activations could both commit
-- is_active=true. Dedupe any races that slipped in since 0056 (keep the
-- newest), then add the partial unique index so the loser fails at
-- commit instead of silently producing two actives.

UPDATE mealplan.meal_plan
SET is_active = false
WHERE is_active
  AND meal_plan_id NOT IN (
    SELECT DISTINCT ON (household_id) meal_plan_id
    FROM mealplan.meal_plan
    WHERE is_active
    ORDER BY household_id, created_at DESC, meal_plan_id DESC
  );

CREATE UNIQUE INDEX meal_plan_one_active_per_household
    ON mealplan.meal_plan (household_id)
    WHERE is_active;

-- ---------- missing FK indexes (M16) ----------
--
-- Cascade deletes and household-scoped list views scan sequentially
-- without these. IF NOT EXISTS keeps the migration idempotent against
-- hand-created indexes.

CREATE INDEX IF NOT EXISTS idx_grocery_list_household
    ON grocery.grocery_list (household_id);
CREATE INDEX IF NOT EXISTS idx_grocery_list_item_list
    ON grocery.grocery_list_item (grocery_list_id);
CREATE INDEX IF NOT EXISTS idx_meal_slot_plan
    ON mealplan.meal_slot (meal_plan_id);
CREATE INDEX IF NOT EXISTS idx_meal_slot_item_slot
    ON mealplan.meal_slot_item (slot_id);
CREATE INDEX IF NOT EXISTS idx_meal_slot_recipe
    ON mealplan.meal_slot (recipe_id);
