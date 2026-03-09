-- name: GetCompletedTaskRuns :many
SELECT * FROM task_runs
WHERE status = 'completed' AND task_id = $1;
