-- name: GetItemByTaskAndURL :one
SELECT * FROM items
WHERE task_id = $1 AND url = $2
LIMIT 1;

-- name: GetItemSummariesByTask :many
SELECT id, url, delisted, in_stock FROM items
WHERE task_id = $1;

-- name: GetActiveItemsByTask :many
SELECT * FROM items
WHERE task_id = $1 AND delisted = false;

-- name: GetActiveItemURLsByTask :many
SELECT id, url FROM items
WHERE task_id = $1 AND delisted = false;

-- name: GetItemDataByID :one
SELECT data FROM items
WHERE id = $1;

-- name: UpdateItemData :exec
UPDATE items
SET data = $2
WHERE id = $1;

-- name: MarkItemDelisted :exec
UPDATE items
SET delisted = true
WHERE id = $1;

-- name: UndelistItem :exec
UPDATE items
SET delisted = false
WHERE id = $1;

-- name: UpdateItemInStock :exec
UPDATE items
SET in_stock = $2
WHERE id = $1;

-- name: InsertItemEvent :one
INSERT INTO item_events (item_id, previous_state, new_state)
VALUES ($1, $2, $3)
RETURNING *;
