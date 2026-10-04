-- LEN-23: AI-suggested allergen flags, held in a review queue.
-- Suggestions are proposals only — nothing reaches ingredient_allergen or
-- item_allergen until an admin accepts, which writes the flag row under
-- the reviewer's own attribution.
CREATE TABLE inventory.allergen_suggestion (
    allergen_suggestion_id BIGSERIAL PRIMARY KEY,
    recipe_id              BIGINT REFERENCES recipe.recipe(recipe_id) ON DELETE SET NULL,
    target_kind            VARCHAR(10) NOT NULL CHECK (target_kind IN ('ingredient', 'item')),
    ingredient_id          BIGINT REFERENCES inventory.ingredient(ingredient_id) ON DELETE CASCADE,
    item_id                BIGINT REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    allergen_id            BIGINT NOT NULL REFERENCES inventory.allergen(allergen_id) ON DELETE CASCADE,
    kind                   VARCHAR(20) NOT NULL CHECK (kind IN ('contains', 'may_contain')),
    rationale              TEXT,
    status                 VARCHAR(20) NOT NULL DEFAULT 'pending'
        CONSTRAINT allergen_suggestion_status CHECK (status IN ('pending', 'accepted', 'dismissed')),
    source                 VARCHAR(20) NOT NULL DEFAULT 'llm' CHECK (source IN ('llm')),
    suggested_by_user_id   BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    reviewed_by_user_id    BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    reviewed_at            TIMESTAMPTZ,
    created_by             VARCHAR(100) NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by             VARCHAR(100),
    updated_at             TIMESTAMPTZ,
    CONSTRAINT allergen_suggestion_target CHECK (
        (target_kind = 'ingredient' AND ingredient_id IS NOT NULL AND item_id IS NULL) OR
        (target_kind = 'item' AND item_id IS NOT NULL AND ingredient_id IS NULL)
    )
);

-- One open proposal per (target, allergen) — re-running the suggester on
-- the same recipe can't stack duplicates in the queue.
CREATE UNIQUE INDEX uq_allergen_suggestion_pending
    ON inventory.allergen_suggestion (target_kind, COALESCE(ingredient_id, item_id), allergen_id)
    WHERE status = 'pending';
CREATE INDEX idx_allergen_suggestion_status ON inventory.allergen_suggestion (status);
CREATE INDEX idx_allergen_suggestion_recipe ON inventory.allergen_suggestion (recipe_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.allergen_suggestion TO lena_app;
GRANT USAGE ON SEQUENCE inventory.allergen_suggestion_allergen_suggestion_id_seq TO lena_app;
