-- LENA-034 / LENA-035: domain CHECK constraints matching the resolver and
-- service validation. CHECK treats NULL as satisfied, so nullable columns
-- keep "not set" semantics; only present values are bounded.

ALTER TABLE mealplan.meal_plan
    ADD CONSTRAINT meal_plan_week_start_dow CHECK (week_start_day_of_week BETWEEN 0 AND 6);

ALTER TABLE mealplan.meal_slot
    ADD CONSTRAINT meal_slot_day_of_week CHECK (day_of_week BETWEEN 0 AND 6),
    ADD CONSTRAINT meal_slot_servings CHECK (servings > 0);

ALTER TABLE mealplan.meal_slot_item
    ADD CONSTRAINT meal_slot_item_quantity CHECK (quantity > 0);

ALTER TABLE recipe.recipe
    ADD CONSTRAINT recipe_servings CHECK (servings > 0);

ALTER TABLE recipe.recipe_item
    ADD CONSTRAINT recipe_item_quantity CHECK (quantity > 0);

ALTER TABLE grocery.grocery_list_item
    ADD CONSTRAINT grocery_list_item_quantity CHECK (quantity_needed > 0),
    -- LENA-035: a row must identify what it refers to.
    ADD CONSTRAINT grocery_list_item_identity
        CHECK (item_id IS NOT NULL OR ingredient_id IS NOT NULL OR manual_item_name IS NOT NULL);

ALTER TABLE wine.bottle
    ADD CONSTRAINT bottle_acidity CHECK (acidity BETWEEN 1 AND 5),
    ADD CONSTRAINT bottle_tannin_level CHECK (tannin_level BETWEEN 1 AND 5),
    ADD CONSTRAINT bottle_body CHECK (body BETWEEN 1 AND 5),
    ADD CONSTRAINT bottle_sweetness CHECK (sweetness BETWEEN 1 AND 5);

ALTER TABLE wine.bottle_grape_variety
    ADD CONSTRAINT bottle_grape_variety_percentage CHECK (percentage BETWEEN 0 AND 100);
