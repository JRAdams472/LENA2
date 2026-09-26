ALTER TABLE household.notifications DROP COLUMN food_event_id;
ALTER TABLE household.notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE household.notifications
    ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('invite_received','invite_accepted','invite_declined','invite_cancelled',
                    'member_joined','member_left','member_removed','role_changed','household_renamed'));

ALTER TABLE recipe.recipe_step
    DROP CONSTRAINT fk_step_depends,
    DROP COLUMN duration_minutes,
    DROP COLUMN step_type,
    DROP COLUMN is_passive,
    DROP COLUMN depends_on_step_number,
    DROP COLUMN appliance;

DROP TABLE event.event_recipe;
DROP TABLE event.food_event;
DROP SCHEMA event;
