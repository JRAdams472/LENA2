ALTER TABLE grocery.grocery_list
    DROP CONSTRAINT IF EXISTS grocery_list_meal_plan_id_fkey,
    ADD CONSTRAINT grocery_list_meal_plan_id_fkey
        FOREIGN KEY (meal_plan_id) REFERENCES mealplan.meal_plan(meal_plan_id);
