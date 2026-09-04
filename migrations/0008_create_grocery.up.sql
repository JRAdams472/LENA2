CREATE TABLE grocery.grocery_list (
    grocery_list_id BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    meal_plan_id    BIGINT REFERENCES mealplan.meal_plan(meal_plan_id),
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

CREATE TABLE grocery.grocery_list_item (
    grocery_list_item_id BIGSERIAL PRIMARY KEY,
    grocery_list_id      BIGINT NOT NULL REFERENCES grocery.grocery_list(grocery_list_id) ON DELETE CASCADE,
    item_id              BIGINT REFERENCES inventory.item(item_id),
    manual_item_name     VARCHAR(200),
    quantity_needed      NUMERIC(10,4) NOT NULL,
    unit_of_measure      VARCHAR(20),
    source               VARCHAR(50) NOT NULL,
    is_checked           BOOLEAN NOT NULL DEFAULT FALSE,
    created_by           VARCHAR(100) NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           VARCHAR(100),
    updated_at           TIMESTAMPTZ
);
