-- name: UpsertItem :one
INSERT INTO items (task_id, url, platform, data)
VALUES ($1, $2, $3, $4)
ON CONFLICT (task_id, url) DO NOTHING
RETURNING *;
