CREATE SCHEMA IF NOT EXISTS analytics;

CREATE TABLE analytics.interaction_event (
    event_id        BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NULL REFERENCES identity.users(user_id) ON DELETE SET NULL,
    event_type      VARCHAR(40) NOT NULL,
    entity_type     VARCHAR(20) NULL,
    entity_id       BIGINT NULL,
    search_term     VARCHAR(500) NULL,
    weight          SMALLINT NOT NULL DEFAULT 1,
    metadata        JSONB NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_interaction_event_event_type_created_at ON analytics.interaction_event (event_type, created_at);
CREATE INDEX idx_interaction_event_entity_type_entity_id ON analytics.interaction_event (entity_type, entity_id);
CREATE INDEX idx_interaction_event_user_id_created_at ON analytics.interaction_event (user_id, created_at);

CREATE TABLE analytics.user_selection_count (
    entity_type     VARCHAR(20) NOT NULL,
    entity_id       BIGINT NOT NULL,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    select_count    BIGINT NOT NULL DEFAULT 0,
    last_selected_at TIMESTAMPTZ,
    PRIMARY KEY (entity_type, entity_id, user_id)
);

CREATE INDEX idx_user_selection_count_user_last ON analytics.user_selection_count (user_id, last_selected_at DESC NULLS LAST);

CREATE TABLE analytics.global_selection_count (
    entity_type     VARCHAR(20) NOT NULL,
    entity_id       BIGINT NOT NULL,
    select_count    BIGINT NOT NULL DEFAULT 0,
    last_selected_at TIMESTAMPTZ,
    PRIMARY KEY (entity_type, entity_id)
);

CREATE INDEX idx_global_selection_count_count ON analytics.global_selection_count (entity_type, select_count DESC);
