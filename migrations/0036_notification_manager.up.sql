-- 0036_notification_manager.up.sql — Notification Manager MVP (notifications-p1)
--
-- Adds: a notification_type registry (kind -> category/label, replacing the
-- kind CHECK constraint with a foreign key so new types are a seed row rather
-- than a constraint rewrite), free-text title/body plus recipe/item deep-link
-- columns and a dedup key for scheduler-generated reminders, per-user
-- notification preferences (per-category opt-out + mute-until, '_all' row =
-- global mute), and a protein flag on inventory categories used by the
-- defrost-reminder sweep.

CREATE TABLE household.notification_type (
    kind        VARCHAR(30) PRIMARY KEY,
    category    VARCHAR(30) NOT NULL,
    label       VARCHAR(100) NOT NULL,
    description VARCHAR(300),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO household.notification_type (kind, category, label, description) VALUES
    ('invite_received',    'household',      'Invite received',            'Someone invited you to their household'),
    ('invite_accepted',    'household',      'Invite accepted',            'Someone accepted your household invite'),
    ('invite_declined',    'household',      'Invite declined',            'Someone declined your household invite'),
    ('invite_cancelled',   'household',      'Invite cancelled',           'A household invite was cancelled'),
    ('member_joined',      'household',      'Member joined',              'Someone joined your household'),
    ('member_left',        'household',      'Member left',                'Someone left your household'),
    ('member_removed',     'household',      'Member removed',             'You were removed from a household'),
    ('role_changed',       'household',      'Role changed',               'A household role was changed'),
    ('household_renamed',  'household',      'Household renamed',          'Your household was renamed'),
    ('event_created',      'events',         'Event created',              'A food event was created'),
    ('event_updated',      'events',         'Event updated',              'A food event was updated'),
    ('event_deleted',      'events',         'Event deleted',              'A food event was deleted'),
    ('protein_defrost',    'meal_reminders', 'Defrost reminder',           'Time to take protein out of the freezer for an upcoming meal'),
    ('meal_prep_advance',  'meal_reminders', 'Advance prep reminder',      'A recipe on the meal plan has a step that needs to start days ahead'),
    ('item_expiring',      'expiry',         'Item expiring soon',         'A pantry item is approaching its expiry date');

-- The registry becomes the source of truth for valid kinds; the CHECK is
-- dropped atomically with the FK add so no window exists where either old or
-- new kinds fail. Seeded rows cover every value already stored.
ALTER TABLE household.notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE household.notifications
    ADD CONSTRAINT notifications_kind_fkey
    FOREIGN KEY (kind) REFERENCES household.notification_type(kind);

-- Reminder payloads: feed text computed server-side (title/body) plus
-- deep-link targets matching the existing invite/event column style.
ALTER TABLE household.notifications
    ADD COLUMN title      VARCHAR(200),
    ADD COLUMN body       VARCHAR(500),
    ADD COLUMN recipe_id  BIGINT REFERENCES recipe.recipe(recipe_id) ON DELETE SET NULL,
    ADD COLUMN item_id    BIGINT REFERENCES inventory.item(item_id) ON DELETE SET NULL,
    ADD COLUMN dedup_key  VARCHAR(200);

-- Scheduler inserts carry a dedup key so a repeated sweep can never create
-- the same reminder twice. A plain unique index suffices: NULL keys are
-- distinct from each other in Postgres, so event-driven rows never conflict.
CREATE UNIQUE INDEX idx_notifications_dedup_key
    ON household.notifications (dedup_key);

-- Per-user notification preferences. One row per (user, category) is
-- materialized lazily on first change; a missing row means defaults (enabled,
-- not muted). The '_all' pseudo-category implements the global mute.
CREATE TABLE userprefs.notification_pref (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    category    VARCHAR(30) NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    muted_until TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, category)
);

-- Defrost sweep signal: which inventory categories count as "protein".
ALTER TABLE inventory.category
    ADD COLUMN is_protein BOOLEAN NOT NULL DEFAULT FALSE;

-- Flag the obvious protein categories so the defrost sweep works out of the
-- box; admins can toggle more via the category admin.
UPDATE inventory.category SET is_protein = TRUE
WHERE name IN ('Meat', 'Poultry', 'Fish', 'Seafood', 'Beef', 'Pork', 'Chicken');

GRANT SELECT, INSERT, UPDATE, DELETE ON household.notification_type TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON userprefs.notification_pref TO lena_app;
