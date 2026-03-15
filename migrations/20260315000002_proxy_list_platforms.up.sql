CREATE TABLE proxy_list_platforms (
  proxy_list_id INT NOT NULL REFERENCES proxy_lists(id) ON DELETE CASCADE,
  platform VARCHAR(50) NOT NULL,
  PRIMARY KEY (proxy_list_id, platform)
);

-- Migrate existing data
INSERT INTO proxy_list_platforms (proxy_list_id, platform)
SELECT id, platform FROM proxy_lists WHERE platform IS NOT NULL AND platform != '';

ALTER TABLE proxy_lists DROP COLUMN platform;
