-- 0028_household.up.sql — household sharing foundation (additive only).
--
-- Creates the household schema, invite tracking, and per-user
-- household_id/is_searchable columns. The shared-table switchover
-- (household_id on userprefs stock tables, mealplan, grocery; favorite
-- extraction; user_id drops; table renames) lands in the next migration
-- alongside the code that writes household_id, so nothing added here can
-- go stale while user_id remains the write key.

CREATE SCHEMA household;

CREATE TABLE household.households (
    household_id BIGSERIAL PRIMARY KEY,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ
);

CREATE TABLE household.invites (
    invite_id    BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    to_user_id   BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    household_id BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','accepted','declined','cancelled')),
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ
);

CREATE INDEX idx_invites_to_status   ON household.invites (to_user_id, status);
CREATE INDEX idx_invites_from_status ON household.invites (from_user_id, status);

-- At most one pending invite between a pair; concluded invites never block
-- a later re-invite.
CREATE UNIQUE INDEX idx_invites_pending_unique
    ON household.invites (from_user_id, to_user_id)
    WHERE status = 'pending';

ALTER TABLE identity.users
    ADD COLUMN household_id  BIGINT REFERENCES household.households(household_id),
    ADD COLUMN is_searchable BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_users_household ON identity.users (household_id);

-- Backfill one single-person household per existing user. household_id is
-- set equal to user_id so the mapping is deterministic; the sequence is
-- then advanced past the backfilled ids.
INSERT INTO household.households (household_id, created_by)
SELECT user_id, left(email, 100)
FROM identity.users
ORDER BY user_id;

SELECT setval('household.households_household_id_seq',
              (SELECT COALESCE(MAX(household_id), 1) FROM household.households));

UPDATE identity.users SET household_id = user_id;
