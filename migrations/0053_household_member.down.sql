-- 0053_household_member.down.sql — drop the membership table. The users
-- row's household_id/household_role columns were never removed, so
-- membership state survives the rollback.

DROP TABLE household.household_member;
