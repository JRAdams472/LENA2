-- name: CreateHousehold :one
INSERT INTO household.households (created_by)
VALUES ($1)
RETURNING *;

-- name: GetHouseholdByID :one
SELECT *
FROM household.households
WHERE household_id = $1;

-- name: CreateInvite :one
INSERT INTO household.invites (from_user_id, to_user_id, household_id, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetInviteByID :one
SELECT *
FROM household.invites
WHERE invite_id = $1;

-- name: ListPendingInvitesForUser :many
SELECT *
FROM household.invites
WHERE to_user_id = $1
  AND status = 'pending'
ORDER BY created_at DESC;

-- name: ListSentInvitesForUser :many
SELECT *
FROM household.invites
WHERE from_user_id = $1
  AND status = 'pending'
ORDER BY created_at DESC;

-- name: TransitionInvite :one
-- Status-guarded: only a pending invite can transition, so a concurrent
-- accept/decline/cancel loses with zero rows instead of a lost update.
UPDATE household.invites
SET status     = $2,
    updated_by = $3,
    updated_at = now()
WHERE invite_id = $1
  AND status = 'pending'
RETURNING *;
