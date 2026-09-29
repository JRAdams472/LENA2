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

-- ---------- decayed selection scores (analytics ranking) ----------

-- name: ClearSelectionScores :exec
-- Full rebuild strategy: the decay job clears and repopulates in one tx so
-- scores always reflect a consistent decay epoch.
DELETE FROM analytics.selection_score;

-- name: RebuildUserSelectionScores :exec
-- score = SUM(weight * 2^(-age_days/half_life)); selection-intent events
-- only — *_viewed / *_searched feed their own ranking tiers.
INSERT INTO analytics.selection_score (
    entity_type, entity_id, scope_type, scope_id, score, event_count, last_selected_at
)
SELECT entity_type, entity_id, 'user', user_id,
       SUM(weight * exp(-ln(2) * EXTRACT(EPOCH FROM (now() - created_at)) / 86400.0
           / sqlc.arg(half_life_days)::float8)),
       COUNT(*), MAX(created_at)
FROM analytics.interaction_event
WHERE entity_type IS NOT NULL AND entity_id IS NOT NULL AND user_id IS NOT NULL
  AND event_type NOT LIKE '%\_viewed' ESCAPE '\'
  AND event_type NOT LIKE '%\_searched' ESCAPE '\'
GROUP BY entity_type, entity_id, user_id;

-- name: RebuildHouseholdSelectionScores :exec
-- Household scope = aggregate of every member's selection events (ADR-001
-- read-model join through identity.users).
INSERT INTO analytics.selection_score (
    entity_type, entity_id, scope_type, scope_id, score, event_count, last_selected_at
)
SELECT e.entity_type, e.entity_id, 'household', u.household_id,
       SUM(e.weight * exp(-ln(2) * EXTRACT(EPOCH FROM (now() - e.created_at)) / 86400.0
           / sqlc.arg(half_life_days)::float8)),
       COUNT(*), MAX(e.created_at)
FROM analytics.interaction_event e
JOIN identity.users u ON u.user_id = e.user_id
WHERE e.entity_type IS NOT NULL AND e.entity_id IS NOT NULL
  AND u.household_id IS NOT NULL
  AND e.event_type NOT LIKE '%\_viewed' ESCAPE '\'
  AND e.event_type NOT LIKE '%\_searched' ESCAPE '\'
GROUP BY e.entity_type, e.entity_id, u.household_id;

-- name: RebuildGlobalSelectionScores :exec
INSERT INTO analytics.selection_score (
    entity_type, entity_id, scope_type, scope_id, score, event_count, last_selected_at
)
SELECT entity_type, entity_id, 'global', 0,
       SUM(weight * exp(-ln(2) * EXTRACT(EPOCH FROM (now() - created_at)) / 86400.0
           / sqlc.arg(half_life_days)::float8)),
       COUNT(*), MAX(created_at)
FROM analytics.interaction_event
WHERE entity_type IS NOT NULL AND entity_id IS NOT NULL
  AND event_type NOT LIKE '%\_viewed' ESCAPE '\'
  AND event_type NOT LIKE '%\_searched' ESCAPE '\'
GROUP BY entity_type, entity_id;

-- name: TopSelectionScores :many
-- Highest-scoring entities for one scope — drives the used/household/
-- popular ranking tiers. scope_id is the user_id or household_id; pass 0
-- for 'global'.
SELECT entity_id, score::float8 AS score, event_count, last_selected_at
FROM analytics.selection_score
WHERE scope_type = $1 AND scope_id = $2 AND entity_type = $3
ORDER BY score DESC, last_selected_at DESC NULLS LAST
LIMIT $4;

-- ---------- generic per-entity engagement inputs ----------

-- name: UserViewedEntityIDs :many
-- Entities of any type the caller has viewed, most-viewed first.
-- event_type is derived as '<entity_type>_viewed'.
SELECT entity_id, COUNT(*) AS hits
FROM analytics.interaction_event
WHERE event_type = sqlc.arg(entity_type)::text || '_viewed'
  AND entity_type = sqlc.arg(entity_type)::text
  AND user_id = sqlc.arg(user_id)::bigint
GROUP BY entity_id
ORDER BY hits DESC, MAX(created_at) DESC
LIMIT sqlc.arg('limit')::int;

-- name: UserEntitySearchTerms :many
-- Distinct terms the caller has searched for a given entity type.
SELECT DISTINCT search_term
FROM analytics.interaction_event
WHERE event_type = sqlc.arg(entity_type)::text || '_searched'
  AND user_id = sqlc.arg(user_id)::bigint
  AND search_term IS NOT NULL
  AND search_term <> '';
