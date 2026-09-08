-- Phase 33: user profile fields (first/last name, backup email) for
-- self-service account settings. CHECK treats NULL as satisfied, so the
-- columns keep "not set" semantics; only present values are bounded.

ALTER TABLE identity.users
    ADD COLUMN first_name VARCHAR(100),
    ADD COLUMN last_name VARCHAR(100),
    ADD COLUMN backup_email VARCHAR(320)
        CONSTRAINT users_backup_email CHECK (backup_email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$');
