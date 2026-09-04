CREATE TABLE inventory.user_item (
    user_item_id    BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    item_id         BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    current_qty     NUMERIC(10,2) NOT NULL DEFAULT 0,
    min_qty         NUMERIC(10,2),
    purchase_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    notes           VARCHAR(500),
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ,
    UNIQUE (user_id, item_id)
);

CREATE INDEX idx_user_item_item_id ON inventory.user_item (item_id);

CREATE TABLE wine.user_bottle (
    user_bottle_id  BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    bottle_id       BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    bottle_number   INTEGER,
    quantity        INTEGER NOT NULL DEFAULT 1,
    purchase_at     TIMESTAMPTZ,
    purchase_price  NUMERIC(10,2),
    storage_temp    NUMERIC(5,1),
    location        VARCHAR(100),
    notes           VARCHAR(500),
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

CREATE TABLE recipe.user_recipe_preference (
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    recipe_id       BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ,
    PRIMARY KEY (user_id, recipe_id)
);
