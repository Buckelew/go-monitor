ALTER TABLE discord_users ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user';
UPDATE discord_users SET role = 'admin' WHERE discord_user_id = '272274142818992130';
