-- name: GetCompletedTaskRuns :many
SELECT * FROM task_runs
WHERE status = 'completed' AND task_id = $1;

-- name: GetRecentTaskRuns :many
SELECT
  tr.id, tr.started_at, tr.completed_at, tr.status, tr.error_message,
  t.platform, t.url, tr.task_id,
  EXTRACT(EPOCH FROM (tr.completed_at - tr.started_at))::float AS duration_seconds,
  tr.status_code, tr.response_time_ms, tr.cache_status
FROM task_runs tr
JOIN tasks t ON t.id = tr.task_id
WHERE tr.status IN ('completed', 'error')
  AND (sqlc.narg('platform')::text IS NULL OR t.platform = sqlc.narg('platform'))
  AND (sqlc.narg('filter_status')::text IS NULL OR tr.status = sqlc.narg('filter_status'))
  AND (sqlc.narg('task_id')::int IS NULL OR tr.task_id = sqlc.narg('task_id'))
  AND (sqlc.narg('cursor')::timestamptz IS NULL OR tr.completed_at < sqlc.narg('cursor'))
ORDER BY tr.completed_at DESC NULLS LAST
LIMIT sqlc.arg('page_limit');
