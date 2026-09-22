-- Give userprefs a schema of its own: the module owns per-user rows that
-- were scattered across inventory, wine and recipe. Cross-schema foreign
-- keys keep working unchanged.
CREATE SCHEMA userprefs;

ALTER TABLE inventory.user_item SET SCHEMA userprefs;
ALTER TABLE wine.user_bottle SET SCHEMA userprefs;
ALTER TABLE recipe.user_recipe_preference SET SCHEMA userprefs;
