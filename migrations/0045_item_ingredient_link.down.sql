-- 0045_item_ingredient_link.down.sql
-- Reverses 0045. Restoring NOT NULL on item_id only succeeds while every
-- row still carries item_id — run this before ingredient-only rows exist.

ALTER TABLE mealplan.meal_slot_item DROP CONSTRAINT meal_slot_item_one_ref;
ALTER TABLE event.event_recipe_item DROP CONSTRAINT event_recipe_item_one_ref;
ALTER TABLE event.event_recipe_item ALTER COLUMN item_id SET NOT NULL;
ALTER TABLE recipe.recipe_item DROP CONSTRAINT recipe_item_one_ref;
ALTER TABLE recipe.recipe_item ALTER COLUMN item_id SET NOT NULL;

DROP TABLE userprefs.household_ingredient_item;
DROP TABLE userprefs.household_item_ingredient;

DROP INDEX idx_item_ingredient;
ALTER TABLE inventory.item DROP COLUMN ingredient_id;
