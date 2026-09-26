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

-- name: RenameHousehold :one
UPDATE household.households
SET name       = $2,
    updated_by = $3,
    updated_at = now()
WHERE household_id = $1
RETURNING *;

-- name: GetHouseholdByIDForUpdate :one
-- Row lock: serialize member-count checks and membership transitions for
-- concurrent accept/leave/remove operations against the same household.
SELECT *
FROM household.households
WHERE household_id = $1
FOR UPDATE;

-- name: CreateNotification :one
INSERT INTO household.notifications
    (user_id, household_id, kind, actor_user_id, invite_id, food_event_id)
VALUES ($1, sqlc.narg(household_id), sqlc.arg(kind), sqlc.narg(actor_user_id), sqlc.narg(invite_id), sqlc.narg(food_event_id))
RETURNING *;

-- name: PruneReadNotifications :exec
-- Retention: keep at most the 100 most recent read notifications per user;
-- called inside the same transaction as CreateNotification.
DELETE FROM household.notifications n
WHERE n.user_id = $1
  AND n.read_at IS NOT NULL
  AND n.notification_id NOT IN (
      SELECT k.notification_id
      FROM household.notifications k
      WHERE k.user_id = $1
        AND k.read_at IS NOT NULL
      ORDER BY k.created_at DESC
      LIMIT 100
  );

-- name: ListNotificationsForUser :many
SELECT *
FROM household.notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountUnreadNotifications :one
SELECT count(*)
FROM household.notifications
WHERE user_id = $1
  AND read_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
UPDATE household.notifications
SET read_at = now()
WHERE user_id = $1
  AND read_at IS NULL;

-- name: CancelPendingInvitesFrom :many
-- Leave/remove cleanup: pending invites a departing member sent for that
-- household are cancelled so they can no longer be accepted against a
-- household the sender no longer belongs to.
UPDATE household.invites
SET status     = 'cancelled',
    updated_by = $3,
    updated_at = now()
WHERE from_user_id = $1
  AND household_id = $2
  AND status = 'pending'
RETURNING *;
