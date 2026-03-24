-- name: InsertCompletedTaskRun :exec
INSERT INTO task_runs (task_id, started_at, completed_at, status, error_message, status_code, response_time_ms, cache_status)
VALUES ($1, $2, NOW(), $3, $4, $5, $6, $7);

-- name: DeleteTaskRunsBefore :execrows
DELETE FROM task_runs
WHERE id IN (
  SELECT tr.id FROM task_runs tr WHERE tr.completed_at < $1 LIMIT $2
);
