ALTER TABLE task_runs
  ADD COLUMN status_code smallint,
  ADD COLUMN response_time_ms integer,
  ADD COLUMN cache_status varchar(20);
