-- 0057_disclaimer_acceptance.up.sql — record when a user accepted the
-- medical/allergen disclaimer (LEN-31).
--
-- Acceptance is required before a member can record allergy or dietary
-- records (setMyAllergy on=true) so every allergy-bearing write is backed
-- by a stored consent timestamp.

ALTER TABLE identity.users
    ADD COLUMN disclaimer_accepted_at TIMESTAMPTZ;
