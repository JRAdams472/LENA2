CREATE TABLE recipe.recipe_rating (
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    recipe_id       BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    rating          SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ,
    PRIMARY KEY (user_id, recipe_id)
);
