INSERT INTO proxy_lists (id, name) VALUES (1, 'example');
INSERT INTO proxies (id, proxy_list_id, host, port, username, password)
VALUES
  (1, 1, 'localhost', '8080', NULL, NULL),
  (2, 1, '127.0.0.1', '80', 'admin', 'password');

INSERT INTO tasks (id, platform, task_type, url, channel_id, delay, enabled)
VALUES
  (1, 'shopify', 'search', 'https://store.obeygiant.com', '457221436432711683', 5000, true);
