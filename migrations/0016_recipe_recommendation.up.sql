CREATE TABLE analytics.recipe_recommendation (
    recommendation_id BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    recipe_id       BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    reason          VARCHAR(30) NOT NULL,
    score           NUMERIC(10,4) NOT NULL,
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, recipe_id, reason)
);

CREATE INDEX idx_recipe_recommendation_user_score ON analytics.recipe_recommendation (user_id, score DESC);
