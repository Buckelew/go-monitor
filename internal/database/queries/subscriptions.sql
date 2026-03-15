-- name: GetSubscriptionsByTask :many
SELECT * FROM task_subscriptions WHERE task_id = $1;

-- name: CreateStoreSubscription :one
INSERT INTO task_subscriptions (task_id, channel_id, mode, guild_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (task_id, channel_id) WHERE mode IN ('all', 'new')
DO UPDATE SET mode = EXCLUDED.mode, guild_id = EXCLUDED.guild_id
RETURNING *;

-- name: CreateProductSubscription :one
INSERT INTO task_subscriptions (task_id, channel_id, mode, item_id, guild_id)
VALUES ($1, $2, 'restock', $3, $4)
ON CONFLICT (task_id, channel_id, item_id) WHERE mode = 'restock'
DO NOTHING
RETURNING *;

-- name: DeleteSubscription :exec
DELETE FROM task_subscriptions WHERE id = $1;

-- name: GetSubscriptionsByChannel :many
SELECT * FROM task_subscriptions WHERE channel_id = $1;

-- name: GetSubscriptionsByGuildIDs :many
SELECT * FROM task_subscriptions WHERE guild_id = ANY($1::text[]);
