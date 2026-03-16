CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_task_runs_latest
  ON task_runs (task_id, completed_at DESC NULLS LAST)
  WHERE status IN ('completed', 'error');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_items_task_active
  ON items (task_id)
  WHERE delisted = false;
