CREATE TABLE mealplan.meal_plan (
    meal_plan_id        BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    name                VARCHAR(200) NOT NULL,
    week_start_date     DATE NOT NULL,
    week_start_day_of_week SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ
);

CREATE INDEX idx_meal_plan_user_week ON mealplan.meal_plan (user_id, week_start_date);

CREATE TABLE mealplan.meal_slot (
    slot_id       BIGSERIAL PRIMARY KEY,
    meal_plan_id  BIGINT NOT NULL REFERENCES mealplan.meal_plan(meal_plan_id) ON DELETE CASCADE,
    day_of_week   SMALLINT NOT NULL,
    meal_type     VARCHAR(50) NOT NULL,
    recipe_id     BIGINT REFERENCES recipe.recipe(recipe_id),
    servings      INTEGER,
    replacement_note VARCHAR(500),
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    UNIQUE (meal_plan_id, day_of_week, meal_type)
);

CREATE TABLE mealplan.meal_slot_item (
    slot_item_id  BIGSERIAL PRIMARY KEY,
    slot_id       BIGINT NOT NULL REFERENCES mealplan.meal_slot(slot_id) ON DELETE CASCADE,
    item_id       BIGINT REFERENCES inventory.item(item_id),
    quantity      NUMERIC(10,4) NOT NULL,
    unit          VARCHAR(20) NOT NULL,
    is_from_recipe BOOLEAN NOT NULL DEFAULT FALSE,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ
);
