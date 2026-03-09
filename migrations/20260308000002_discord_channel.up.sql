ALTER TABLE tasks DROP CONSTRAINT tasks_webhook_id_fkey;
ALTER TABLE tasks DROP COLUMN webhook_id;
ALTER TABLE tasks ADD COLUMN channel_id TEXT NOT NULL DEFAULT '';
DROP TABLE webhooks;
