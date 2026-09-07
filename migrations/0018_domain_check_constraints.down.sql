ALTER TABLE wine.bottle_grape_variety
    DROP CONSTRAINT bottle_grape_variety_percentage;

ALTER TABLE wine.bottle
    DROP CONSTRAINT bottle_acidity,
    DROP CONSTRAINT bottle_tannin_level,
    DROP CONSTRAINT bottle_body,
    DROP CONSTRAINT bottle_sweetness;

ALTER TABLE grocery.grocery_list_item
    DROP CONSTRAINT grocery_list_item_quantity,
    DROP CONSTRAINT grocery_list_item_identity;

ALTER TABLE recipe.recipe_item
    DROP CONSTRAINT recipe_item_quantity;

ALTER TABLE recipe.recipe
    DROP CONSTRAINT recipe_servings;

ALTER TABLE mealplan.meal_slot_item
    DROP CONSTRAINT meal_slot_item_quantity;

ALTER TABLE mealplan.meal_slot
    DROP CONSTRAINT meal_slot_day_of_week,
    DROP CONSTRAINT meal_slot_servings;

ALTER TABLE mealplan.meal_plan
    DROP CONSTRAINT meal_plan_week_start_dow;
