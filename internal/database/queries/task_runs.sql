-- name: InsertTaskRun :one
INSERT INTO task_runs (task_id, started_at, status)
VALUES ($1, NOW(), 'running')
RETURNING *;

-- name: CompleteTaskRun :exec
UPDATE task_runs
SET status = $2, completed_at = NOW(), error_message = $3
WHERE id = $1;
