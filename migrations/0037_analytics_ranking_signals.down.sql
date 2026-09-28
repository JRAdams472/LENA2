-- 0037_analytics_ranking_signals.down.sql

ALTER TABLE grocery.grocery_list_item
    DROP COLUMN checked_at,
    DROP COLUMN checked_seq;

DROP INDEX IF EXISTS analytics.idx_interaction_event_entity_event;
DROP INDEX IF EXISTS analytics.idx_interaction_event_user_event;

DROP TABLE IF EXISTS analytics.selection_score;
