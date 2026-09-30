-- 0038_user_birthdate.up.sql
-- Optional birthdate on the user profile. The sommelier/cocktail AI
-- suggestions are age-gated server-side: the resolver requires a stored
-- birthdate showing the caller is 21+ before alcohol recommendations run.

ALTER TABLE identity.users
    ADD COLUMN birthdate DATE;
