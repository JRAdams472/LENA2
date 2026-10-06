-- 0050_push_notifications.up.sql — LEN-17 P1: mobile push delivery channel.
-- Push is a second delivery channel beside the in-app feed, not a new
-- notification type: writers fan out feed rows (Allowed gate) and
-- push_delivery outbox rows (PushAllowed gate) from the same kind.

-- Per-category push opt-in, independent of the feed 'enabled' flag: a
-- member can want a category pushed without the feed copy (or vice versa).
-- The '_all' row doubles as a master push opt-in; muted_until suppresses
-- both channels regardless of the independent enable flags.
ALTER TABLE userprefs.notification_pref
    ADD COLUMN push_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- Registered devices per user. The token is an opaque provider credential
-- (FCM today); 'android' ships first and 'ios'/'web' keep the door open for
-- APNs-via-FCM and web push without a schema change. UNIQUE(token) plus an
-- upsert on register implements "same install, latest login owns it".
CREATE TABLE identity.device_token (
    device_token_id BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    platform        VARCHAR(10) NOT NULL CHECK (platform IN ('android','ios','web')),
    token           VARCHAR(512) NOT NULL UNIQUE,
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ
);
CREATE INDEX idx_device_token_user ON identity.device_token (user_id);

-- Durable delivery outbox. Rows are written in the same transaction as the
-- event that produced them (or alongside the sweep's feed insert) and are
-- deliberately denormalized: push is allowed to fire without a feed row
-- (independent channels), so it cannot FK to household.notifications.
-- The worker claims due rows, fans out to the user's tokens, and records
-- attempts/next_attempt_at for exponential backoff; 'failed' is terminal
-- after the retry budget is spent.
CREATE TABLE household.push_delivery (
    push_delivery_id BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    kind             VARCHAR(30) NOT NULL REFERENCES household.notification_type(kind),
    household_id     BIGINT REFERENCES household.households(household_id) ON DELETE SET NULL,
    actor_user_id    BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    invite_id        BIGINT REFERENCES household.invites(invite_id) ON DELETE SET NULL,
    food_event_id    BIGINT REFERENCES event.food_event(food_event_id) ON DELETE SET NULL,
    recipe_id        BIGINT REFERENCES recipe.recipe(recipe_id) ON DELETE SET NULL,
    item_id          BIGINT REFERENCES inventory.item(item_id) ON DELETE SET NULL,
    title            VARCHAR(200),
    body             VARCHAR(500),
    status           VARCHAR(10) NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','sending','sent','failed')),
    attempts         INTEGER NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at          TIMESTAMPTZ,
    last_error       VARCHAR(500),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ
);
CREATE INDEX idx_push_delivery_pending
    ON household.push_delivery (next_attempt_at)
    WHERE status = 'pending';
CREATE INDEX idx_push_delivery_user ON household.push_delivery (user_id);

-- Default privileges already cover new tables in these schemas for the
-- migration runner, but grant explicitly for parity with 0049.
GRANT SELECT, INSERT, UPDATE, DELETE ON
    identity.device_token, household.push_delivery
    TO lena_app;
GRANT USAGE ON SEQUENCE
    identity.device_token_device_token_id_seq,
    household.push_delivery_push_delivery_id_seq
    TO lena_app;
