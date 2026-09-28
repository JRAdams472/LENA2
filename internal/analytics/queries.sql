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
-- Meal plans are household-scoped, so plan history fans out to every
-- active member of the household via identity.users (ADR-001 read-model).
WITH new_items AS (
    SELECT item_id
    FROM recipe.recipe_item
    WHERE recipe_id = sqlc.arg(recipe_id)::bigint
),
hist AS (
    SELECT DISTINCT u.user_id, ms.recipe_id
    FROM mealplan.meal_slot ms
    JOIN mealplan.meal_plan mp ON mp.meal_plan_id = ms.meal_plan_id
    JOIN identity.users u ON u.household_id = mp.household_id AND u.is_active
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

-- ---------- engagement ranking inputs (recipe categories feature) ----------

-- name: HouseholdUsedRecipeIDs :many
-- Recipes household members have put on a menu (meal-plan slot or event),
-- most-used first — drives the "used" tier of recipe search ranking.
SELECT entity_id AS recipe_id, COUNT(*) AS hits
FROM analytics.interaction_event e
JOIN identity.users u ON u.user_id = e.user_id
WHERE e.event_type IN ('menu_add', 'recipe_selected')
  AND e.entity_type = 'recipe'
  AND u.household_id = sqlc.arg(household_id)::bigint
GROUP BY entity_id
ORDER BY hits DESC, MAX(e.created_at) DESC;

-- name: UserViewedRecipeIDs :many
-- Recipes the caller has opened, most-viewed first — the "viewed" tier.
SELECT entity_id AS recipe_id, COUNT(*) AS hits
FROM analytics.interaction_event
WHERE event_type = 'recipe_viewed'
  AND entity_type = 'recipe'
  AND user_id = sqlc.arg(user_id)::bigint
GROUP BY entity_id
ORDER BY hits DESC, MAX(created_at) DESC;

-- name: UserRecipeSearchTerms :many
-- Distinct terms the caller has searched recipes for — recipes whose names
-- match these terms form the "searched but not viewed" tier.
SELECT DISTINCT search_term
FROM analytics.interaction_event
WHERE event_type = 'recipe_searched'
  AND user_id = sqlc.arg(user_id)::bigint
  AND search_term IS NOT NULL
  AND search_term <> '';
