ALTER TABLE proxy_lists ADD COLUMN platform VARCHAR(50);

UPDATE proxy_lists SET platform = (
  SELECT platform FROM proxy_list_platforms WHERE proxy_list_id = proxy_lists.id LIMIT 1
);

DROP TABLE IF EXISTS proxy_list_platforms;
