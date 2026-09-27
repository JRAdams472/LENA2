-- 0034_idempotency.up.sql — request dedup store for idempotent mutations.
--
-- Stripe-style idempotency keys: the client sends Idempotency-Key, the BFF
-- stores (user_id, key) → request_hash → serialized response, and a retry
-- replays the stored response instead of re-executing. Keys with an "auto:"
-- prefix are server-synthesized for the payload-hash fallback — identical
-- mutation payloads seen inside the fallback window are deduped even when
-- the client sent no key.
--
-- In-flight rows (status 'in_progress') fence concurrent same-key requests;
-- rows that never complete are reclaimed by the BFF once they age past the
-- in-flight TTL, and 'completed' rows are swept after expires_at.

CREATE SCHEMA platform;

CREATE TABLE platform.idempotency_key (
    user_id         BIGINT       NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    key             TEXT         NOT NULL,
    request_hash    BYTEA        NOT NULL,
    status          TEXT         NOT NULL CHECK (status IN ('in_progress', 'completed')),
    response        JSONB,
    response_status INT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (user_id, key)
);

-- Fallback lookup: latest completed auto row for an identical payload.
CREATE INDEX idx_idem_hash ON platform.idempotency_key (user_id, request_hash, created_at DESC);
-- Expiry sweep.
CREATE INDEX idx_idem_expiry ON platform.idempotency_key (expires_at);

-- lena_app grants for the new schema — mirrors 0031/0032/0033. The default
-- privileges cover tables created later inside this schema; the explicit
-- grants keep this file correct even if those defaults change.
GRANT USAGE ON SCHEMA platform TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA platform TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA platform TO lena_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA platform
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO lena_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA platform
    GRANT USAGE ON SEQUENCES TO lena_app;
