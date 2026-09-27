-- 0032_event_recipe_steps.up.sql — per-slot recipe step snapshots.
--
-- An event recipe slot owns a COPY of its recipe's steps (plus any
-- hand-entered steps on free-form slots). Edits made in the event context
-- write only to this snapshot — the original recipe.recipe_step rows are
-- never touched, and a later recipe edit or deletion does not change a
-- plan that was already laid out. The master timeline schedules the
-- snapshot, not the live recipe.

CREATE TABLE event.event_recipe_step (
    event_recipe_step_id    BIGSERIAL PRIMARY KEY,
    event_recipe_id         BIGINT NOT NULL REFERENCES event.event_recipe(event_recipe_id) ON DELETE CASCADE,
    step_number             INTEGER NOT NULL,
    instruction             TEXT NOT NULL,
    duration_minutes        INTEGER CHECK (duration_minutes >= 0),
    step_type               VARCHAR(20),
    is_passive              BOOLEAN NOT NULL DEFAULT FALSE,
    depends_on_step_number  INTEGER,
    appliance               VARCHAR(50),
    created_by              VARCHAR(100) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by              VARCHAR(100),
    updated_at              TIMESTAMPTZ,
    UNIQUE (event_recipe_id, step_number)
);

-- depends_on references (event_recipe_id, step_number) — the snapshot's
-- own numbering, same pattern as fk_step_depends on recipe.recipe_step.
-- Snapshot edits are in-place or delete-all/reinsert, both of which keep
-- the pair valid; NO ACTION checks at end of statement.
ALTER TABLE event.event_recipe_step
    ADD CONSTRAINT fk_event_step_depends
    FOREIGN KEY (event_recipe_id, depends_on_step_number)
    REFERENCES event.event_recipe_step (event_recipe_id, step_number)
    ON UPDATE CASCADE;

CREATE INDEX idx_event_recipe_step_recipe ON event.event_recipe_step (event_recipe_id);

-- Backfill: every existing slot with a linked recipe gets a snapshot of
-- that recipe's current steps so the timeline keeps working for events
-- created before this table existed. Free-form slots start empty.
INSERT INTO event.event_recipe_step
    (event_recipe_id, step_number, instruction, duration_minutes,
     step_type, is_passive, depends_on_step_number, appliance,
     created_by, updated_by)
SELECT er.event_recipe_id,
       rs.step_number,
       rs.instruction,
       rs.duration_minutes,
       rs.step_type,
       rs.is_passive,
       rs.depends_on_step_number,
       rs.appliance,
       er.created_by,
       er.created_by
FROM event.event_recipe er
JOIN recipe.recipe_step rs ON rs.recipe_id = er.recipe_id
WHERE er.recipe_id IS NOT NULL;

-- lena_app grants for the new table — mirrors the 0031 block.
GRANT SELECT, INSERT, UPDATE, DELETE ON event.event_recipe_step TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA event TO lena_app;
