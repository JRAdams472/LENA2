-- name: Claim :execrows
INSERT INTO platform.idempotency_key (user_id, key, request_hash, status, expires_at)
VALUES ($1, $2, $3, 'in_progress', $4)
ON CONFLICT (user_id, key) DO UPDATE
    SET request_hash    = EXCLUDED.request_hash,
        status          = 'in_progress',
        response        = NULL,
        response_status = NULL,
        created_at      = now(),
        completed_at    = NULL,
        expires_at      = EXCLUDED.expires_at
    WHERE platform.idempotency_key.expires_at < now();

-- name: Get :one
SELECT *
FROM platform.idempotency_key
WHERE user_id = $1 AND key = $2;

-- name: Complete :execrows
UPDATE platform.idempotency_key
SET status          = 'completed',
    response        = $3,
    response_status = $4,
    completed_at    = now(),
    expires_at      = $5
WHERE user_id = $1 AND key = $2 AND status = 'in_progress';

-- name: DeleteExpired :exec
DELETE FROM platform.idempotency_key
WHERE status = 'completed' AND expires_at < now();

-- name: ReclaimStale :exec
-- expires_at on an in_progress row is the in-flight deadline set at claim
-- time (now + InFlightTTL), so this is the single reclaim boundary.
DELETE FROM platform.idempotency_key
WHERE status = 'in_progress' AND expires_at < now();

-- name: Abandon :exec
DELETE FROM platform.idempotency_key
WHERE user_id = $1 AND key = $2 AND status = 'in_progress';
