-- 0036_notification_manager.down.sql

ALTER TABLE inventory.category
    DROP COLUMN is_protein;

DROP TABLE IF EXISTS userprefs.notification_pref;

DROP INDEX IF EXISTS household.idx_notifications_dedup_key;


ALTER TABLE household.notifications
    DROP CONSTRAINT notifications_kind_fkey,
    DROP COLUMN title,
    DROP COLUMN body,
    DROP COLUMN recipe_id,
    DROP COLUMN item_id,
    DROP COLUMN dedup_key;

ALTER TABLE household.notifications
    ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('invite_received','invite_accepted','invite_declined','invite_cancelled',
                    'member_joined','member_left','member_removed','role_changed','household_renamed',
                    'event_created','event_updated','event_deleted'));

DROP TABLE IF EXISTS household.notification_type;
