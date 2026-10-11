-- 0054_user_approval_status.down.sql — drop the approval barrier column.

DROP INDEX IF EXISTS identity.idx_users_approval_pending;

ALTER TABLE identity.users
    DROP COLUMN approval_status;
