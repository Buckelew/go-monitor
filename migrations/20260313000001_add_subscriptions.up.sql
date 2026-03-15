CREATE TABLE task_subscriptions (
  id          SERIAL PRIMARY KEY,
  task_id     INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  channel_id  TEXT NOT NULL,
  mode        VARCHAR(20) NOT NULL CHECK (mode IN ('all', 'new', 'restock')),
  item_id     INT REFERENCES items(id) ON DELETE CASCADE,
  created_at  TIMESTAMP NOT NULL DEFAULT NOW(),

  CONSTRAINT chk_mode_item CHECK (
    (mode IN ('all', 'new') AND item_id IS NULL) OR
    (mode = 'restock' AND item_id IS NOT NULL)
  )
);

-- Store-level: one subscription per (task, channel) regardless of all vs new
CREATE UNIQUE INDEX uq_sub_store ON task_subscriptions (task_id, channel_id)
  WHERE mode IN ('all', 'new');
-- Product-level: one subscription per (task, channel, item)
CREATE UNIQUE INDEX uq_sub_product ON task_subscriptions (task_id, channel_id, item_id)
  WHERE mode = 'restock';

CREATE INDEX idx_task_subscriptions_task_id ON task_subscriptions (task_id);
