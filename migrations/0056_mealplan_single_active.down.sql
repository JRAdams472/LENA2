-- 0056_mealplan_single_active.down.sql — not reversible: the backfill
-- discarded which plans were previously active. Re-activating every plan
-- would recreate the bug, so the down migration is intentionally a no-op.

SELECT 1;
