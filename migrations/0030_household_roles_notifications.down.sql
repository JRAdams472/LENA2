-- 0030_household_roles_notifications.down.sql — drops the phase-5
-- additions. Notification history is discarded; household_role and name
-- revert to the phase-2 schema.

REVOKE SELECT, INSERT, UPDATE, DELETE ON household.notifications FROM lena_app;
REVOKE USAGE ON SEQUENCE household.notifications_notification_id_seq FROM lena_app;

DROP TABLE household.notifications;

ALTER TABLE identity.users DROP COLUMN household_role;

ALTER TABLE household.households
    DROP COLUMN name,
    DROP COLUMN updated_by;
