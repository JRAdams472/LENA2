-- Give userprefs a schema of its own: the module owns per-user rows that
-- were scattered across inventory, wine and recipe. Cross-schema foreign
-- keys keep working unchanged.
CREATE SCHEMA userprefs;

ALTER TABLE inventory.user_item SET SCHEMA userprefs;
ALTER TABLE wine.user_bottle SET SCHEMA userprefs;
ALTER TABLE recipe.user_recipe_preference SET SCHEMA userprefs;

-- lena_app (migration 0024) only has privileges on the schemas named
-- there; the moved tables keep their table-level grants but the role
-- still needs USAGE on the new schema, plus defaults for future tables.
GRANT USAGE ON SCHEMA userprefs TO lena_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA userprefs TO lena_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA userprefs TO lena_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA userprefs
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO lena_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA userprefs
    GRANT USAGE ON SEQUENCES TO lena_app;
