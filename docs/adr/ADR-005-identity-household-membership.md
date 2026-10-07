# ADR-005: identity reads household.household_member

## Status

Accepted (LEN-26 Phase 1)

## Context

LEN-26 adds `household.household_member` as the source of truth for
who-belongs-where, while `identity.users.household_id`/`household_role`
become the user's *active* household pointer. Three identity queries are
membership-scoped and must read the membership table:

- `ListUsersByHousehold` / `CountUsersByHousehold` — member lists and the
  invite-accept cap must follow membership, not the active pointer (a
  member's pointer may sit on another of their households).
- `SearchUsers` — invite-candidate exclusion must reject users who belong
  to the caller's household regardless of where their active pointer sits.
- `SetActiveHousehold` — moving the pointer must prove membership in the
  same statement (an `EXISTS` guard) and sync `household_role` from the
  membership row, or the pointer and role columns can diverge.

The alternatives were worse:

- **Compose at the service layer** (`household.ListMembersByHousehold` +
  `identity.ListUsersByIDs`) — two round-trips per member read, and the
  `SearchUsers` exclusion + `SetActiveHousehold` guard cannot be expressed
  as composition: they need the membership predicate inside the SQL or the
  operation loses its atomicity.
- **Move the member queries into the household package** — they return
  `identity.users` rows; household would then read the identity schema,
  trading one exception for another and leaving `SetActiveHousehold`
  (which writes `identity.users`) in a package that does not own it.

## Decision

`internal/identity` may **read** `household.household_member` — the table
stays owned and written by `internal/household` (`JoinHousehold`,
`RemoveMembership`, `UpsertMembership`); identity never writes it. The
exception is recorded in the schema-guard allow-list.

The scope is deliberately narrow: only the membership predicate/join and
the `SetActiveHousehold` `EXISTS`/`SELECT` subqueries cross the boundary.
`identity` still owns every write to `identity.users`.

## Consequences

- `allowedSchemas["identity"]` in
  `internal/testutil/schema_guard_test.go` gains `household`, with a
  comment pointing here.
- Membership *writes* stay in `internal/household`; orchestrations that
  move a user's active pointer (auth bootstrap, invite accept, leave,
  remove) mirror the membership change in the same ambient transaction.
