CREATE TABLE webhooks (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  url TEXT NOT NULL UNIQUE,
  type VARCHAR(20) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE proxy_lists (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE proxies (
  id SERIAL PRIMARY KEY,
  proxy_list_id INT NOT NULL REFERENCES proxy_lists(id) ON DELETE CASCADE,
  host TEXT NOT NULL,
  port TEXT NOT NULL,
  username TEXT,
  password TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_proxies_proxy_list_id ON proxies(proxy_list_id);

CREATE TABLE discord_users (
  id SERIAL PRIMARY KEY,
  discord_user_id VARCHAR(20) NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE tasks (
  id SERIAL PRIMARY KEY,
  platform VARCHAR(50) NOT NULL,
  task_type VARCHAR(20) NOT NULL,
  url TEXT NOT NULL,
  webhook_id INT REFERENCES webhooks(id),
  proxy_list_id INT REFERENCES proxy_lists(id),
  delay INT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_webhook_id ON tasks(webhook_id);
CREATE INDEX idx_tasks_proxy_list_id ON tasks(proxy_list_id);

CREATE TABLE task_runs (
  id SERIAL PRIMARY KEY,
  task_id INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  started_at TIMESTAMP NOT NULL,
  completed_at TIMESTAMP,
  status VARCHAR(20) NOT NULL,
  error_message TEXT
);

CREATE INDEX idx_task_runs_task_id ON task_runs(task_id);

CREATE TABLE items (
  id SERIAL PRIMARY KEY,
  task_id INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  platform VARCHAR(50) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  data JSONB NOT NULL DEFAULT '{}',
  UNIQUE(task_id, url)
);

CREATE INDEX idx_items_task_id ON items(task_id);

CREATE TABLE item_events (
  id SERIAL PRIMARY KEY,
  item_id INT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  previous_state JSONB NOT NULL,
  new_state JSONB NOT NULL,
  webhook_sent BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_item_events_item_id ON item_events(item_id);
