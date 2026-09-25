-- 0030_household_roles_notifications.up.sql — household names, member
-- roles, and in-app notifications (household phase 5).
--
-- Adds: a display name on households, an owner/admin/member role on each
-- user's household membership, and a notifications table written inside
-- the same transactions as the operations that produce them (invite,
-- accept, decline, cancel, join, leave, remove, role change, rename).

ALTER TABLE household.households
    ADD COLUMN name       VARCHAR(100),
    ADD COLUMN updated_by VARCHAR(100);

ALTER TABLE identity.users
    ADD COLUMN household_role VARCHAR(20) NOT NULL DEFAULT 'member'
        CHECK (household_role IN ('owner','admin','member'));

-- The earliest member of each existing household becomes its owner. That
-- matches the default-household convention (sole member == owner) and is
-- deterministic for households that already merged; a wrong pick is
-- recoverable via ownership transfer.
UPDATE identity.users u
SET household_role = 'owner'
WHERE u.household_id IS NOT NULL
  AND u.user_id = (
      SELECT MIN(user_id) FROM identity.users
      WHERE household_id = u.household_id
  );

CREATE TABLE household.notifications (
    notification_id BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    household_id   BIGINT REFERENCES household.households(household_id) ON DELETE SET NULL,
    kind           VARCHAR(30) NOT NULL CHECK (kind IN (
                       'invite_received','invite_accepted','invite_declined','invite_cancelled',
                       'member_joined','member_left','member_removed','role_changed','household_renamed')),
    actor_user_id  BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    invite_id      BIGINT REFERENCES household.invites(invite_id) ON DELETE SET NULL,
    read_at        TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The hot path is "unread count for user" — a partial index keeps it cheap.
CREATE INDEX idx_notifications_user_unread
    ON household.notifications (user_id)
    WHERE read_at IS NULL;

CREATE INDEX idx_notifications_user_created
    ON household.notifications (user_id, created_at DESC);

-- lena_app writes notifications at request time (inside resolver
-- transactions) and reads them for the badge/feed. 0029's default
-- privileges already cover new household tables created by the migrate
-- role; the explicit grants keep this file self-contained and correct
-- even if those defaults are ever changed.
GRANT SELECT, INSERT, UPDATE, DELETE ON household.notifications TO lena_app;
GRANT USAGE ON SEQUENCE household.notifications_notification_id_seq TO lena_app;
