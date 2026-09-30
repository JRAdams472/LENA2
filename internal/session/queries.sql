-- name: GetSessionByRefreshHash :one
SELECT *
FROM identity.session
WHERE refresh_hash = $1;

-- name: CreateSession :one
WITH ins AS (
    INSERT INTO identity.session (user_id, family_id, refresh_hash, device, expires_at)
    VALUES ($1, 0, $2, $3, $4)
    RETURNING session_id
)
UPDATE identity.session AS s
SET family_id = ins.session_id
FROM ins
WHERE s.session_id = ins.session_id
RETURNING s.*;

-- name: GetSessionByIDForUpdate :one
SELECT *
FROM identity.session
WHERE session_id = $1
FOR UPDATE;

-- name: InsertRotatedSession :one
INSERT INTO identity.session (user_id, family_id, refresh_hash, device, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: MarkSessionReplaced :execrows
UPDATE identity.session
SET revoked_at  = now(),
    replaced_by = $2
WHERE session_id = $1
  AND revoked_at IS NULL;

-- name: RevokeSession :execrows
UPDATE identity.session
SET revoked_at = now()
WHERE session_id = $1
  AND revoked_at IS NULL;

-- name: RevokeSessionFamily :exec
UPDATE identity.session
SET revoked_at = now()
WHERE family_id = $1
  AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :execrows
DELETE FROM identity.session
WHERE expires_at < $1
   OR revoked_at < $2;
