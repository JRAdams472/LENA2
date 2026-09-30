-- 0038_user_birthdate.down.sql

ALTER TABLE identity.users
    DROP COLUMN birthdate;
