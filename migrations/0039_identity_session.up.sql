-- 0039_identity_session.up.sql
-- Server-side sessions behind the refresh-token flow. createSession
-- validates an OIDC credential once, then issues a LENA-signed access
-- token plus an opaque refresh token; each refresh rotates into a new
-- row in the same family, and replaying a rotated token revokes the
-- family (theft detection).

CREATE TABLE identity.session (
    session_id    BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    family_id     BIGINT NOT NULL,
    refresh_hash  BYTEA NOT NULL,
    device        VARCHAR(200),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,
    replaced_by   BIGINT,
    UNIQUE (refresh_hash)
);

CREATE INDEX idx_session_user ON identity.session (user_id);
CREATE INDEX idx_session_family ON identity.session (family_id);
