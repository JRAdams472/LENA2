-- 0044_searchable_optin.up.sql
-- household-invite search was documented as opt-in but shipped opt-out:
-- every account was enumerable by default. Flip the column default to
-- FALSE and mark every existing user not-searchable; anyone who wants to
-- be discoverable re-enables it via updateMyProfile { isSearchable }.

ALTER TABLE identity.users
    ALTER COLUMN is_searchable SET DEFAULT FALSE;

UPDATE identity.users SET is_searchable = FALSE;
