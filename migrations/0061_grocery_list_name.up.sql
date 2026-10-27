-- Optional member-facing label for a grocery list (LEN-180 web UX):
-- generated lists default to NULL so clients can fall back to the source
-- plan's name or the generated timestamp. Nullable — no backfill needed.
ALTER TABLE grocery.grocery_list
    ADD COLUMN name VARCHAR(200);
