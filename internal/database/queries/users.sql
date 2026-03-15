-- name: UpsertDiscordUser :one
INSERT INTO discord_users (discord_user_id, username, avatar, access_token, refresh_token, token_expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (discord_user_id)
DO UPDATE SET
  username = EXCLUDED.username,
  avatar = EXCLUDED.avatar,
  access_token = EXCLUDED.access_token,
  refresh_token = EXCLUDED.refresh_token,
  token_expires_at = EXCLUDED.token_expires_at
RETURNING *;

-- name: GetDiscordUserByID :one
SELECT * FROM discord_users WHERE id = $1;

-- name: GetDiscordUserByDiscordID :one
SELECT * FROM discord_users WHERE discord_user_id = $1;
