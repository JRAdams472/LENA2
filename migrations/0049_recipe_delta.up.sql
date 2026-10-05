-- LEN-25: household-scoped recipe deltas — structured per-line/per-step
-- tweaks layered on the shared canonical recipe, no forking required.
-- One delta per (recipe, household). base_updated_at snapshots the
-- canonical recipe's updated_at at the last delta write/ack so reads can
-- flag "the base recipe changed" without storing a diff.
CREATE TABLE recipe.recipe_delta (
    recipe_delta_id  BIGSERIAL PRIMARY KEY,
    recipe_id        BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    household_id     BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    base_updated_at  TIMESTAMPTZ NOT NULL,
    created_by       VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       VARCHAR(100),
    updated_at       TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_recipe_delta_recipe_household
    ON recipe.recipe_delta (recipe_id, household_id);
CREATE INDEX idx_recipe_delta_household ON recipe.recipe_delta (household_id);

-- Per-line changes. recipe_item_id anchors a change to a base line and is
-- NULL for added lines; canonical edits delete+re-insert recipe items, so
-- anchors break on purpose — SET NULL keeps the row and the apply engine
-- counts it as orphaned rather than silently dropping it.
CREATE TABLE recipe.recipe_delta_item (
    delta_item_id    BIGSERIAL PRIMARY KEY,
    recipe_delta_id  BIGINT NOT NULL REFERENCES recipe.recipe_delta(recipe_delta_id) ON DELETE CASCADE,
    recipe_item_id   BIGINT REFERENCES recipe.recipe_item(recipe_item_id) ON DELETE SET NULL,
    kind             VARCHAR(10) NOT NULL CHECK (kind IN ('substitute','adjust','remove','add')),
    -- Substitute fully replaces the line's refs (exactly one of the two).
    -- Adjust treats NULL columns as "keep the base value"; '' clears
    -- free-text fields.
    item_id          BIGINT REFERENCES inventory.item(item_id) ON DELETE SET NULL,
    ingredient_id    BIGINT REFERENCES inventory.ingredient(ingredient_id) ON DELETE SET NULL,
    quantity         NUMERIC(10,4),
    unit_id          BIGINT REFERENCES inventory.unit(unit_id),
    section_name     VARCHAR(200),
    display_order    INTEGER,
    notes            VARCHAR(500),
    is_optional      BOOLEAN,
    created_by       VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       VARCHAR(100),
    updated_at       TIMESTAMPTZ,
    CONSTRAINT delta_item_kind_refs CHECK (
        (kind = 'add' AND recipe_item_id IS NULL AND (item_id IS NOT NULL OR ingredient_id IS NOT NULL)) OR
        (kind = 'substitute' AND recipe_item_id IS NOT NULL AND
            (item_id IS NOT NULL) <> (ingredient_id IS NOT NULL)) OR
        (kind = 'adjust' AND recipe_item_id IS NOT NULL) OR
        (kind = 'remove' AND recipe_item_id IS NOT NULL) OR
        (recipe_item_id IS NULL AND kind <> 'add')  -- orphaned row after base edit
    )
);
-- One change per base line per delta.
CREATE UNIQUE INDEX uq_delta_item_line
    ON recipe.recipe_delta_item (recipe_delta_id, recipe_item_id)
    WHERE recipe_item_id IS NOT NULL;
CREATE INDEX idx_delta_item_delta ON recipe.recipe_delta_item (recipe_delta_id);

CREATE TABLE recipe.recipe_delta_step (
    delta_step_id    BIGSERIAL PRIMARY KEY,
    recipe_delta_id  BIGINT NOT NULL REFERENCES recipe.recipe_delta(recipe_delta_id) ON DELETE CASCADE,
    step_id          BIGINT REFERENCES recipe.recipe_step(step_id) ON DELETE SET NULL,
    kind             VARCHAR(10) NOT NULL CHECK (kind IN ('replace','remove','add')),
    step_number      INTEGER,                 -- insert position for adds; keep NULL otherwise
    instruction      VARCHAR(2000),
    duration_minutes INTEGER,
    step_type        VARCHAR(50),
    is_passive       BOOLEAN,
    depends_on_step_number INTEGER,
    appliance        VARCHAR(50),
    created_by       VARCHAR(100) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       VARCHAR(100),
    updated_at       TIMESTAMPTZ,
    CONSTRAINT delta_step_kind_refs CHECK (
        (kind = 'add' AND step_id IS NULL AND step_number IS NOT NULL AND instruction IS NOT NULL) OR
        (kind = 'replace' AND step_id IS NOT NULL) OR
        (kind = 'remove' AND step_id IS NOT NULL) OR
        (step_id IS NULL AND kind <> 'add')  -- orphaned row after base edit
    )
);
CREATE UNIQUE INDEX uq_delta_step_step
    ON recipe.recipe_delta_step (recipe_delta_id, step_id)
    WHERE step_id IS NOT NULL;
CREATE INDEX idx_delta_step_delta ON recipe.recipe_delta_step (recipe_delta_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON
    recipe.recipe_delta, recipe.recipe_delta_item, recipe.recipe_delta_step
    TO lena_app;
GRANT USAGE ON SEQUENCE
    recipe.recipe_delta_recipe_delta_id_seq,
    recipe.recipe_delta_item_delta_item_id_seq,
    recipe.recipe_delta_step_delta_step_id_seq
    TO lena_app;
