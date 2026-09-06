-- name: InsertInteractionEvent :exec
INSERT INTO analytics.interaction_event (
    user_id, event_type, entity_type, entity_id, search_term, weight, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, now());

-- name: UpsertUserSelectionCount :exec
INSERT INTO analytics.user_selection_count (
    entity_type, entity_id, user_id, select_count, last_selected_at
)
VALUES ($1, $2, $3, 1, now())
ON CONFLICT (entity_type, entity_id, user_id)
    DO UPDATE SET
        select_count = analytics.user_selection_count.select_count + 1,
        last_selected_at = now();

-- name: UpsertGlobalSelectionCount :exec
INSERT INTO analytics.global_selection_count (
    entity_type, entity_id, select_count, last_selected_at
)
VALUES ($1, $2, 1, now())
ON CONFLICT (entity_type, entity_id)
    DO UPDATE SET
        select_count = analytics.global_selection_count.select_count + 1,
        last_selected_at = now();

-- name: GetUserSelectionCounts :many
SELECT entity_type, entity_id, user_id, select_count, last_selected_at
FROM analytics.user_selection_count
WHERE entity_type = $1 AND user_id = $2 AND entity_id = ANY(sqlc.arg(entity_ids)::bigint[]);

-- name: GetGlobalSelectionCounts :many
SELECT entity_type, entity_id, select_count, last_selected_at
FROM analytics.global_selection_count
WHERE entity_type = $1 AND entity_id = ANY(sqlc.arg(entity_ids)::bigint[]);

-- name: TopUserSelections :many
SELECT entity_type, entity_id, user_id, select_count, last_selected_at
FROM analytics.user_selection_count
WHERE entity_type = $1 AND user_id = $2
ORDER BY select_count DESC, last_selected_at DESC NULLS LAST
LIMIT $3;

-- name: TopGlobalSelections :many
SELECT entity_type, entity_id, select_count, last_selected_at
FROM analytics.global_selection_count
WHERE entity_type = $1
ORDER BY select_count DESC, last_selected_at DESC NULLS LAST
LIMIT $2;
