-- 0042_recipe_embedding.down.sql
-- Drops the embedding columns and index. The `vector` extension is left
-- installed: other databases in the cluster may share it, and re-running up
-- is idempotent.
DROP INDEX IF EXISTS recipe.recipe_embedding_hnsw;
ALTER TABLE recipe.recipe
    DROP COLUMN IF EXISTS embedding,
    DROP COLUMN IF EXISTS embedding_model,
    DROP COLUMN IF EXISTS embedding_at;
