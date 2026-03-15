-- Extend discord_users with OAuth fields
ALTER TABLE discord_users
  ADD COLUMN username         TEXT NOT NULL DEFAULT '',
  ADD COLUMN avatar           TEXT NOT NULL DEFAULT '',
  ADD COLUMN access_token     TEXT NOT NULL DEFAULT '',
  ADD COLUMN refresh_token    TEXT NOT NULL DEFAULT '',
  ADD COLUMN token_expires_at TIMESTAMP;

-- User-registered Discord servers
CREATE TABLE user_servers (
  id              SERIAL PRIMARY KEY,
  discord_user_id INT NOT NULL REFERENCES discord_users(id) ON DELETE CASCADE,
  guild_id        TEXT NOT NULL,
  guild_name      TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE(discord_user_id, guild_id)
);

-- Server-side sessions
CREATE TABLE sessions (
  id              TEXT PRIMARY KEY,
  discord_user_id INT NOT NULL REFERENCES discord_users(id) ON DELETE CASCADE,
  expires_at      TIMESTAMP NOT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
