-- NOTE: This rollback is inherently lossy.
-- Tasks with only 'restock' subscriptions will get channel_id = '' (no match in the join).
-- Tasks with multiple store-level subscriptions on different channels will keep only the
-- earliest one (DISTINCT ON ... ORDER BY id). The old schema was 1:1, the new model is many:many.
ALTER TABLE tasks ADD COLUMN channel_id TEXT NOT NULL DEFAULT '';
UPDATE tasks t SET channel_id = sub.channel_id
FROM (
  SELECT DISTINCT ON (task_id) task_id, channel_id
  FROM task_subscriptions WHERE mode IN ('all', 'new') ORDER BY task_id, id
) sub WHERE t.id = sub.task_id;
