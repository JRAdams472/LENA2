-- 0054_user_approval_status.up.sql — Admin approval barrier (LEN-127)
--
-- New sign-ins must be reviewed by an administrator before they reach
-- household data. approval_status is deliberately separate from is_active:
-- is_active keeps its banned/disabled semantics (audit history), while
-- approval_status tracks the pending -> approved|denied lifecycle.
-- Existing rows default to 'approved' so every current account is unaffected.
--
-- The auth-path upsert inserts 'pending' explicitly; direct inserts (seed
-- data, tests, backfills) fall through to the 'approved' default.

ALTER TABLE identity.users
    ADD COLUMN approval_status VARCHAR(20) NOT NULL DEFAULT 'approved'
        CHECK (approval_status IN ('pending', 'approved', 'denied'));

-- Pending requests are a small, hot admin-lookup set; a partial index keeps
-- the pending-requests query off the full table scan.
CREATE INDEX idx_users_approval_pending
    ON identity.users (created_at)
    WHERE approval_status = 'pending';
