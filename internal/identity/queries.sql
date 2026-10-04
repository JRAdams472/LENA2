-- name: GetUserByID :one
SELECT *
FROM identity.users
WHERE user_id = $1;

-- name: GetUserByProviderSubject :one
SELECT *
FROM identity.users
WHERE provider = $1
  AND external_subject = $2;

-- name: GetUserByLogin :one
-- Resolves a provider identity to its user via the login mapping table —
-- covers both primary logins and explicitly linked ones.
SELECT u.*
FROM identity.users AS u
JOIN identity.user_login AS l ON l.user_id = u.user_id
WHERE l.provider = $1
  AND l.external_subject = $2;

-- name: UpsertLogin :one
-- First sign-in creates the mapping; subsequent sign-ins refresh the
-- cached provider claims and the sign-in timestamp.
INSERT INTO identity.user_login (
    user_id,
    provider,
    external_subject,
    email,
    display_name,
    last_login_at
)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (provider, external_subject)
    DO UPDATE SET
        email = CASE WHEN EXCLUDED.email = '' THEN identity.user_login.email ELSE EXCLUDED.email END,
        display_name = EXCLUDED.display_name,
        last_login_at = now()
RETURNING *;

-- name: InsertLogin :one
-- Link flow: strict insert — a conflict means the provider identity is
-- already bound (to this or another user) and surfaces as ErrConflict.
INSERT INTO identity.user_login (
    user_id,
    provider,
    external_subject,
    email,
    display_name,
    last_login_at
)
VALUES ($1, $2, $3, $4, $5, now())
RETURNING *;

-- name: ListLoginsByUser :many
SELECT *
FROM identity.user_login
WHERE user_id = $1
ORDER BY created_at;

-- name: CountLoginsByUser :one
SELECT count(*)
FROM identity.user_login
WHERE user_id = $1;

-- name: DeleteLoginsByProvider :execrows
DELETE FROM identity.user_login
WHERE user_id = $1
  AND provider = $2;

-- name: TouchUserLogin :execrows
-- Linked-login sign-in: bump the user row's sign-in timestamp without
-- overwriting its primary email/display_name.
UPDATE identity.users
SET last_login_at = now()
WHERE user_id = $1;

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

-- name: SetUserRole :execrows
UPDATE identity.users
SET role       = $2,
    updated_at = now()
WHERE user_id = $1;

-- name: UpdateUser :execrows
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

-- name: SetUserActive :execrows
UPDATE identity.users
SET is_active  = $2,
    updated_by = $3,
    updated_at = now()
WHERE user_id = $1;

-- name: UpdateUserProfile :execrows
UPDATE identity.users
SET first_name   = $2,
    last_name    = $3,
    backup_email = $4,
    birthdate    = $5,
    updated_by   = $6,
    updated_at   = now()
WHERE user_id = $1;

-- name: SetUserHousehold :execrows
-- Conditional update: the expected-household guard turns a concurrent
-- accept/leave race into a zero-row conflict instead of a lost update.
-- household_role is set atomically with the move: 'owner' for fresh
-- default households, 'member' when joining via invite accept.
UPDATE identity.users
SET household_id   = sqlc.arg(household_id),
    household_role = sqlc.arg(household_role),
    updated_at     = now()
WHERE user_id = sqlc.arg(user_id)
  AND household_id IS NOT DISTINCT FROM sqlc.narg(expected_household_id);

-- name: SetUserHouseholdRole :execrows
-- Role change within the same household; the expected-household guard
-- keeps a stale actor from re-adding a role after the user moved.
UPDATE identity.users
SET household_role = sqlc.arg(household_role),
    updated_by     = sqlc.arg(by),
    updated_at     = now()
WHERE user_id = sqlc.arg(user_id)
  AND household_id = sqlc.arg(household_id);

-- name: CountUsersByHousehold :one
SELECT count(*)
FROM identity.users
WHERE household_id = $1;

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

-- name: RevokeUserSessions :exec
-- Deactivation kills every live refresh-token family for the user so the
-- session cookie path dies immediately (access tokens still expire on
-- their own TTL).
UPDATE identity.session
SET revoked_at = now()
WHERE user_id = $1
  AND revoked_at IS NULL;
