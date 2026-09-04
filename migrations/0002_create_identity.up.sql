CREATE TABLE identity.users (
    user_id             BIGSERIAL PRIMARY KEY,
    provider            VARCHAR(50) NOT NULL DEFAULT 'google',
    external_subject    VARCHAR(255) NOT NULL,
    email               VARCHAR(320) NOT NULL,
    display_name        VARCHAR(200),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at       TIMESTAMPTZ,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ,
    UNIQUE (provider, external_subject)
);

CREATE INDEX idx_users_email ON identity.users (email);
