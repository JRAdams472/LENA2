CREATE TABLE recipe.recipe (
    recipe_id         BIGSERIAL PRIMARY KEY,
    name              VARCHAR(200) NOT NULL UNIQUE,
    description       VARCHAR(500),
    servings          INTEGER,
    prep_time_minutes INTEGER,
    cook_time_minutes INTEGER,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        VARCHAR(100) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        VARCHAR(100),
    updated_at        TIMESTAMPTZ
);

CREATE TABLE recipe.recipe_item (
    recipe_id     BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    item_id       BIGINT NOT NULL REFERENCES inventory.item(item_id),
    quantity      NUMERIC(10,4) NOT NULL,
    unit          VARCHAR(20) NOT NULL,
    notes         VARCHAR(500),
    is_optional   BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (recipe_id, item_id)
);

CREATE TABLE recipe.recipe_step (
    step_id       BIGSERIAL PRIMARY KEY,
    recipe_id     BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    step_number   INTEGER NOT NULL,
    instruction   VARCHAR(2000) NOT NULL,
    created_by    VARCHAR(100) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    VARCHAR(100),
    updated_at    TIMESTAMPTZ,
    UNIQUE (recipe_id, step_number)
);
