INSERT INTO webhooks (id, name, url, type)
VALUES
  (1, 'example webhook', 'https://discord.com/api/webhooks/1234567890/randomchars', 'item');

INSERT INTO proxy_lists (id, name) VALUES (1, 'example');
INSERT INTO proxies (id, proxy_list_id, host, port, username, password)
VALUES
  (1, 1, 'localhost', '8080', NULL, NULL),
  (2, 1, '127.0.0.1', '80', 'admin', 'password');

INSERT INTO tasks (id, platform, task_type, url, webhook_id, proxy_list_id, delay, enabled)
VALUES
  (1, 'shopify', 'search', 'https://store.obeygiant.com', 1, 1, 5000, true);
