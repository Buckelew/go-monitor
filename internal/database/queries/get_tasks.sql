-- name: GetTask :one
SELECT * FROM tasks
WHERE id = $1 LIMIT 1;

-- name: GetTasks :many
SELECT * FROM tasks
ORDER BY created_at DESC;

-- name: CreateTask :one
INSERT INTO tasks (
  platform, task_type, url, delay, proxy_list_id
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpdateTaskEnabled :exec
UPDATE tasks SET enabled = $2, updated_at = NOW() WHERE id = $1;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = $1;

-- name: GetTasksWithStats :many
SELECT
  t.id, t.platform, t.url, t.delay, t.enabled, t.created_at,
  COALESCE(lr.status, '') AS last_run_status,
  lr.completed_at AS last_run_at,
  COALESCE(sc.sub_count, 0)::int AS sub_count,
  COALESCE(ic.item_count, 0)::int AS item_count
FROM tasks t
LEFT JOIN LATERAL (
  SELECT status, completed_at
  FROM task_runs WHERE task_id = t.id AND status IN ('completed', 'error')
  ORDER BY completed_at DESC NULLS LAST LIMIT 1
) lr ON true
LEFT JOIN (
  SELECT task_id, COUNT(*) AS sub_count FROM task_subscriptions GROUP BY task_id
) sc ON sc.task_id = t.id
LEFT JOIN (
  SELECT task_id, COUNT(*) AS item_count FROM items WHERE delisted = false GROUP BY task_id
) ic ON ic.task_id = t.id
WHERE (sqlc.narg('platform')::text IS NULL OR t.platform = sqlc.narg('platform'))
  AND (sqlc.narg('url_search')::text IS NULL OR t.url ILIKE '%' || sqlc.narg('url_search') || '%')
ORDER BY t.created_at DESC;

-- name: GetTaskByURL :one
SELECT * FROM tasks WHERE url = $1;

-- name: CountTasksByPlatform :one
SELECT COUNT(*) FROM tasks WHERE platform = $1 AND enabled = true;
