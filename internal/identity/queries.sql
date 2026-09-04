-- name: GetUserByID :one
SELECT *
FROM identity.users
WHERE user_id = $1;

-- name: GetUserByProviderSubject :one
SELECT *
FROM identity.users
WHERE provider = $1
  AND external_subject = $2;

-- name: UpsertUser :one
INSERT INTO identity.users (
    provider,
    external_subject,
    email,
    display_name,
    last_login_at,
    created_by,
    updated_by
)
VALUES ($1, $2, $3, $4, now(), $5, $6)
ON CONFLICT (provider, external_subject)
    DO UPDATE SET
        email = EXCLUDED.email,
        display_name = EXCLUDED.display_name,
        last_login_at = now(),
        updated_by = EXCLUDED.updated_by,
        updated_at = now()
RETURNING *;

-- name: UpdateUser :exec
UPDATE identity.users
SET email        = $2,
    display_name = $3,
    is_active    = $4,
    updated_by   = $5,
    updated_at   = now()
WHERE user_id = $1;

-- name: ListUsers :many
SELECT *
FROM identity.users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
