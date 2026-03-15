INSERT INTO task_subscriptions (task_id, channel_id, mode)
SELECT id, channel_id, 'all' FROM tasks WHERE channel_id != '';

ALTER TABLE tasks DROP COLUMN channel_id;
