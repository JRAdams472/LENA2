-- 0053_household_member.up.sql — LEN-26 P1: many-to-many household
-- membership. household.household_member becomes the source of truth for
-- who belongs where; identity.users.household_id/household_role remain as
-- the ACTIVE household pointer (synced by the service layer on every
-- membership change), so existing household-scoped queries keep working.

CREATE TABLE household.household_member (
    household_id BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    role         VARCHAR(20) NOT NULL CHECK (role IN ('owner','admin','member')),
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ,
    PRIMARY KEY (household_id, user_id)
);

-- Membership lookup by user (active-household resolution, "my households").
CREATE INDEX idx_household_member_user
    ON household.household_member (user_id);

-- Backfill: every user's current household/role becomes a membership row.
INSERT INTO household.household_member (household_id, user_id, role, created_by)
SELECT u.household_id, u.user_id, u.household_role, left(u.email, 100)
FROM identity.users u
WHERE u.household_id IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON household.household_member TO lena_app;
