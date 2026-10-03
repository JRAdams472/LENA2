-- 0044_searchable_optin.down.sql
-- Restores the opt-out default. Cannot restore which users had
-- is_searchable=true before 0044 ran — that state is intentionally lost.

ALTER TABLE identity.users
    ALTER COLUMN is_searchable SET DEFAULT TRUE;
