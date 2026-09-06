CREATE TABLE inventory.ingredient (
    ingredient_id BIGSERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL UNIQUE,
    category_id   BIGINT REFERENCES inventory.category(category_id),
    default_unit  VARCHAR(20),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ
);

ALTER TABLE recipe.recipe_item
    ADD COLUMN ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id);

ALTER TABLE mealplan.meal_slot_item
    ADD COLUMN ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id);

ALTER TABLE grocery.grocery_list_item
    ADD COLUMN ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id);
