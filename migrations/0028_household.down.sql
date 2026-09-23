-- 0028_household.down.sql — reverses the additive household foundation.
-- Rows in household.invites/households are discarded; no user data is
-- affected because shared-table columns were not yet introduced.

ALTER TABLE identity.users
    DROP COLUMN household_id,
    DROP COLUMN is_searchable;

DROP TABLE household.invites;
DROP TABLE household.households;
DROP SCHEMA household;
