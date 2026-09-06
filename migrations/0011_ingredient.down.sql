ALTER TABLE grocery.grocery_list_item DROP COLUMN ingredient_id;
ALTER TABLE mealplan.meal_slot_item DROP COLUMN ingredient_id;
ALTER TABLE recipe.recipe_item DROP COLUMN ingredient_id;
DROP TABLE inventory.ingredient;
