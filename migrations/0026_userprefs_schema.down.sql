ALTER TABLE userprefs.user_item SET SCHEMA inventory;
ALTER TABLE userprefs.user_bottle SET SCHEMA wine;
ALTER TABLE userprefs.user_recipe_preference SET SCHEMA recipe;

DROP SCHEMA userprefs;
