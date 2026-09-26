-- 0031_event.up.sql — food events (parties/gatherings): a household-scoped
-- event groups recipes with absolute serve times; recipe steps gain timing
-- metadata for the future master-timeline engine.
--
-- Household scope matches mealplan/grocery/pantry post-0029: household_id
-- is the ownership key, all members can read and write. Event mutations
-- notify other members via household.notifications (new kind values below).

CREATE SCHEMA event;

CREATE TABLE event.food_event (
    food_event_id            BIGSERIAL PRIMARY KEY,
    household_id             BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    name                     VARCHAR(200) NOT NULL,
    event_date               DATE NOT NULL,
    slot_granularity_minutes SMALLINT NOT NULL DEFAULT 15
                             CHECK (slot_granularity_minutes IN (15, 30)),
    is_active                BOOLEAN NOT NULL DEFAULT TRUE,
    created_by               VARCHAR(100) NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by               VARCHAR(100),
    updated_at               TIMESTAMPTZ
);

CREATE INDEX idx_food_event_household_date
    ON event.food_event (household_id, event_date);

CREATE TABLE event.event_recipe (
    event_recipe_id BIGSERIAL PRIMARY KEY,
    food_event_id   BIGINT NOT NULL REFERENCES event.food_event(food_event_id) ON DELETE CASCADE,
    -- Nullable so a deleted recipe leaves the slot (and its target time)
    -- behind rather than cascading the whole assignment away.
    recipe_id       BIGINT REFERENCES recipe.recipe(recipe_id) ON DELETE SET NULL,
    meal_type       VARCHAR(50) NOT NULL,
    target_time     TIMESTAMPTZ NOT NULL,
    servings        INTEGER,
    notes           VARCHAR(500),
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);

CREATE INDEX idx_event_recipe_event ON event.event_recipe (food_event_id);

-- Per-step timing metadata for the future timeline engine (all additive /
-- non-breaking): how long a step takes, what kind of work it is, whether it
-- needs the cook's attention (passive steps like resting or baking can
-- overlap other steps), which earlier step it depends on, and which
-- appliance it occupies (for resource-contention scheduling).
ALTER TABLE recipe.recipe_step
    ADD COLUMN duration_minutes       INTEGER CHECK (duration_minutes >= 0),
    ADD COLUMN step_type              VARCHAR(20),
    ADD COLUMN is_passive             BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN depends_on_step_number INTEGER,
    ADD COLUMN appliance              VARCHAR(50);

-- depends_on references (recipe_id, step_number), not step_id: recipe
-- updates delete and re-insert all steps, so step_id is ephemeral. The pair
-- is already UNIQUE (0005). NO ACTION checks at end of statement, so the
-- bulk delete/reinsert path is unaffected; deleting a single depended-on
-- step is correctly blocked.
ALTER TABLE recipe.recipe_step
    ADD CONSTRAINT fk_step_depends
    FOREIGN KEY (recipe_id, depends_on_step_number)
    REFERENCES recipe.recipe_step (recipe_id, step_number)
    ON UPDATE CASCADE;

-- Event notification kinds + deep-link target. The CHECK is replaced
-- atomically so there is no window where new or old kinds fail.
ALTER TABLE household.notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE household.notifications
    ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('invite_received','invite_accepted','invite_declined','invite_cancelled',
                    'member_joined','member_left','member_removed','role_changed','household_renamed',
                    'event_created','event_updated','event_deleted'));
ALTER TABLE household.notifications
    ADD COLUMN food_event_id BIGINT REFERENCES event.food_event(food_event_id) ON DELETE SET NULL;

-- lena_app grants for the new schema — mirrors 0028/0029. The default
-- privileges cover tables created later inside this schema; the explicit
-- grants keep this file correct even if those defaults change.
GRANT USAGE ON SCHEMA event TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA event TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA event TO lena_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA event
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO lena_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA event
    GRANT USAGE ON SEQUENCES TO lena_app;
