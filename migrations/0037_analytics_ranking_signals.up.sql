-- 0037_analytics_ranking_signals.up.sql
-- Decayed engagement scores for analytics-driven ranking, plus grocery
-- check-off ordering for the future store-routing feature.

-- Decayed selection scores, rebuilt by the analytics decay job from
-- analytics.interaction_event. scope_type 'user'/'household' carry the
-- user_id/household_id in scope_id; 'global' rows use scope_id 0.
-- Selection-intent events only (selections, menu adds, ratings, creates,
-- pantry/grocery activity) — views and searches feed their own tiers.
CREATE TABLE analytics.selection_score (
    entity_type      VARCHAR(20)   NOT NULL,
    entity_id        BIGINT        NOT NULL,
    scope_type       VARCHAR(10)   NOT NULL CHECK (scope_type IN ('user', 'household', 'global')),
    scope_id         BIGINT        NOT NULL DEFAULT 0,
    score            NUMERIC(14,4) NOT NULL DEFAULT 0,
    event_count      BIGINT        NOT NULL DEFAULT 0,
    last_selected_at TIMESTAMPTZ,
    computed_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id, scope_type, scope_id)
);

-- Ranking lookups: top entities for one scope + type, best score first.
CREATE INDEX idx_selection_score_rank ON analytics.selection_score (scope_type, scope_id, entity_type, score DESC);

-- Decay job reads the whole log; engagement lookups filter user + event.
CREATE INDEX idx_interaction_event_user_event ON analytics.interaction_event (user_id, event_type, created_at);
CREATE INDEX idx_interaction_event_entity_event ON analytics.interaction_event (entity_type, event_type, created_at);

-- Grocery check-off order: checked_at is wall-clock, checked_seq is a
-- per-list monotonically increasing position assigned on check and cleared
-- on uncheck. groundwork for the V2 store-routing feature.
ALTER TABLE grocery.grocery_list_item
    ADD COLUMN checked_at TIMESTAMPTZ,
    ADD COLUMN checked_seq INTEGER;
