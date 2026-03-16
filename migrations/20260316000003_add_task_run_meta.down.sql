ALTER TABLE task_runs
  DROP COLUMN IF EXISTS status_code,
  DROP COLUMN IF EXISTS response_time_ms,
  DROP COLUMN IF EXISTS cache_status;
