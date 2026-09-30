-- 0040_identity_user_login.up.sql
-- Many provider logins per user: the mapping table for explicit account
-- linking. users.provider/external_subject remain the *primary* login;
-- this table additionally holds every login (backfilled) so resolution
-- has a single lookup point.

CREATE TABLE identity.user_login (
    user_login_id    BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    provider         VARCHAR(50)  NOT NULL,
    external_subject VARCHAR(255) NOT NULL,
    email            VARCHAR(320) NOT NULL DEFAULT '',
    display_name     VARCHAR(200),
    last_login_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, external_subject)
);

CREATE INDEX idx_user_login_user ON identity.user_login (user_id);

INSERT INTO identity.user_login (user_id, provider, external_subject, email, display_name, last_login_at)
SELECT user_id, provider, external_subject, email, display_name, last_login_at
FROM identity.users;
