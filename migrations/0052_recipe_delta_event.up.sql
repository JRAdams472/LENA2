-- LEN-58: append-only audit log for household recipe deltas. Each
-- set/clear/acknowledge writes one row carrying the actor and a JSONB
-- {before, after} snapshot of the change set — deltas are replaced
-- wholesale, so history is the only record of who changed what.
CREATE TABLE recipe.recipe_delta_event (
    recipe_delta_event_id BIGSERIAL PRIMARY KEY,
    recipe_id        BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    household_id     BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    -- Correlates events to a delta instance; survives the delta row's
    -- deletion so a clear stays auditable.
    recipe_delta_id  BIGINT REFERENCES recipe.recipe_delta(recipe_delta_id) ON DELETE SET NULL,
    event            VARCHAR(12) NOT NULL CHECK (event IN ('set','clear','acknowledge')),
    actor            VARCHAR(100) NOT NULL,
    -- {"before": {"items": [...], "steps": [...]} | null,
    --  "after":  {"items": [...], "steps": [...]} | null}
    detail           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_delta_event_recipe_household
    ON recipe.recipe_delta_event (recipe_id, household_id, created_at DESC);

GRANT SELECT, INSERT ON recipe.recipe_delta_event TO lena_app;
GRANT USAGE ON SEQUENCE recipe.recipe_delta_event_recipe_delta_event_id_seq TO lena_app;
