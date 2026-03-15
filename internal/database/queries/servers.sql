-- name: CreateUserServer :one
INSERT INTO user_servers (discord_user_id, guild_id, guild_name)
VALUES ($1, $2, $3)
ON CONFLICT (discord_user_id, guild_id)
DO UPDATE SET guild_name = EXCLUDED.guild_name
RETURNING *;

-- name: GetUserServers :many
SELECT * FROM user_servers WHERE discord_user_id = $1 ORDER BY created_at DESC;

-- name: DeleteUserServer :exec
DELETE FROM user_servers WHERE id = $1 AND discord_user_id = $2;

-- name: GetUserServerByGuildID :one
SELECT * FROM user_servers WHERE discord_user_id = $1 AND guild_id = $2;
