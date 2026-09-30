-- name: GetSessionByRefreshHash :one
SELECT *
FROM identity.session
WHERE refresh_hash = $1;

-- name: CreateSession :one
-- The family root's family_id equals its own session_id, so draw the
-- sequence value first and insert both columns explicitly. (An
-- INSERT-then-UPDATE CTE cannot see the row it just inserted.)
WITH seq AS (SELECT nextval('identity.session_session_id_seq') AS id)
INSERT INTO identity.session (session_id, user_id, family_id, refresh_hash, device, expires_at)
SELECT seq.id, $1, seq.id, $2, $3, $4 FROM seq
RETURNING *;

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
