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
        -- An empty email claim must never blank out a stored address.
        email = CASE WHEN EXCLUDED.email = '' THEN identity.users.email ELSE EXCLUDED.email END,
        display_name = EXCLUDED.display_name,
        last_login_at = now(),
        updated_by = EXCLUDED.updated_by,
        updated_at = now()
RETURNING *;

-- name: ConditionalSetUserRole :execrows
UPDATE identity.users AS u
SET role       = $2,
    updated_at = now()
WHERE u.user_id = $1
  AND ($2 = 'admin' OR (
    SELECT count(*) FROM identity.users AS other
    WHERE other.role = 'admin'
      AND other.is_active
      AND other.user_id <> u.user_id
  ) >= 1);

-- name: SetUserRole :exec
UPDATE identity.users
SET role       = $2,
    updated_at = now()
WHERE user_id = $1;

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

-- name: CountUsers :one
SELECT count(*)
FROM identity.users;

-- name: CountActiveAdmins :one
SELECT count(*)
FROM identity.users
WHERE role = 'admin'
  AND is_active;

-- name: ConditionalSetUserActive :execrows
UPDATE identity.users AS u
SET is_active  = $2,
    updated_by = $3,
    updated_at = now()
WHERE u.user_id = $1
  AND ($2 = true OR (
    SELECT count(*) FROM identity.users AS other
    WHERE other.role = 'admin'
      AND other.is_active
      AND other.user_id <> u.user_id
  ) >= 1);

-- name: SetUserActive :exec
UPDATE identity.users
SET is_active  = $2,
    updated_by = $3,
    updated_at = now()
WHERE user_id = $1;

-- name: UpdateUserProfile :exec
UPDATE identity.users
SET first_name   = $2,
    last_name    = $3,
    backup_email = $4,
    updated_by   = $5,
    updated_at   = now()
WHERE user_id = $1;

-- name: SetUserHousehold :execrows
-- Conditional update: the expected-household guard turns a concurrent
-- accept/leave race into a zero-row conflict instead of a lost update.
UPDATE identity.users
SET household_id = sqlc.arg(household_id),
    updated_at   = now()
WHERE user_id = sqlc.arg(user_id)
  AND household_id IS NOT DISTINCT FROM sqlc.narg(expected_household_id);

-- name: SetUserSearchable :execrows
UPDATE identity.users
SET is_searchable = $2,
    updated_by    = $3,
    updated_at    = now()
WHERE user_id = $1;

-- name: ListUsersByHousehold :many
SELECT *
FROM identity.users
WHERE household_id = $1
ORDER BY created_at;

-- name: ListUsersByIDs :many
SELECT *
FROM identity.users
WHERE user_id = ANY($1::bigint[]);

-- name: SearchUsers :many
-- Household-invite candidate search: opt-in, active users only, caller and
-- the caller's household members excluded. pattern is a pre-escaped LIKE
-- pattern built by the service.
SELECT *
FROM identity.users
WHERE is_active
  AND is_searchable
  AND user_id <> sqlc.arg(exclude_user_id)
  AND (sqlc.narg(exclude_household_id)::bigint IS NULL
       OR household_id IS DISTINCT FROM sqlc.narg(exclude_household_id)::bigint)
  AND (
         display_name ILIKE sqlc.arg(pattern)
      OR first_name   ILIKE sqlc.arg(pattern)
      OR last_name    ILIKE sqlc.arg(pattern)
      OR email        ILIKE sqlc.arg(pattern)
  )
ORDER BY display_name NULLS LAST, email
LIMIT sqlc.arg(row_limit);
