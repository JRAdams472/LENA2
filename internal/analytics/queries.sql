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

-- name: UpsertRecipeRecommendation :exec
INSERT INTO analytics.recipe_recommendation (user_id, recipe_id, reason, score)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, recipe_id, reason)
    DO UPDATE SET
        score        = EXCLUDED.score,
        generated_at = now();

-- name: ListRecipeRecommendations :many
SELECT recommendation_id, user_id, recipe_id, reason, score, generated_at
FROM analytics.recipe_recommendation
WHERE user_id = $1 AND reason = $2
ORDER BY score DESC
LIMIT $3;

-- name: IngredientOverlapScores :many
-- For a newly created recipe ($1), compute each user's best Jaccard
-- similarity between the new recipe's item set and the item sets of the
-- recipes in that user's meal-plan history (|intersection| / |union|).
WITH new_items AS (
    SELECT item_id
    FROM recipe.recipe_item
    WHERE recipe_id = sqlc.arg(recipe_id)::bigint
),
hist AS (
    SELECT DISTINCT mp.user_id, ms.recipe_id
    FROM mealplan.meal_slot ms
    JOIN mealplan.meal_plan mp ON mp.meal_plan_id = ms.meal_plan_id
    WHERE ms.recipe_id IS NOT NULL AND ms.recipe_id <> sqlc.arg(recipe_id)::bigint
),
scored AS (
    SELECT h.user_id, MAX(s.score) AS score
    FROM hist h
    CROSS JOIN LATERAL (
        SELECT
            COUNT(*) FILTER (WHERE n.item_id IS NOT NULL)::numeric
            / NULLIF(
                (SELECT COUNT(*) FROM new_items) + COUNT(*)
                - COUNT(*) FILTER (WHERE n.item_id IS NOT NULL),
                0
            ) AS score
        FROM recipe.recipe_item ri
        LEFT JOIN new_items n ON n.item_id = ri.item_id
        WHERE ri.recipe_id = h.recipe_id
    ) s
    GROUP BY h.user_id
)
SELECT user_id, score::float8 AS score
FROM scored
WHERE score >= sqlc.arg(min_score)::numeric;
