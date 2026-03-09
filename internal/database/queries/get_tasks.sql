-- name: GetTask :one
SELECT * FROM tasks
WHERE id = $1 LIMIT 1;

-- name: GetTasks :many
SELECT * FROM tasks
ORDER BY created_at DESC;

-- name: CreateTask :one
INSERT INTO tasks (
  platform, task_type, url, delay
) VALUES (
  $1, $2, $3, $4
)
RETURNING *;
