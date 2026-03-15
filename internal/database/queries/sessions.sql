-- name: CreateSession :exec
INSERT INTO sessions (id, discord_user_id, expires_at)
VALUES ($1, $2, $3);

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();
