DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_servers;

ALTER TABLE discord_users
  DROP COLUMN IF EXISTS username,
  DROP COLUMN IF EXISTS avatar,
  DROP COLUMN IF EXISTS access_token,
  DROP COLUMN IF EXISTS refresh_token,
  DROP COLUMN IF EXISTS token_expires_at;
