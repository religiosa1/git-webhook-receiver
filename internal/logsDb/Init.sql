CREATE TABLE IF NOT EXISTS logs (
	id integer PRIMARY KEY NOT NULL,
  level integer NOT NULL, -- log level as used in slog: -4 for debug, 0 for info, 4 for warn and 8 for error
	project text,
	delivery_id text,
	pipe_id text,
	message text NOT NULL,
	data blob, -- additional data passed as logs attr (besides project, deliveryId and pipeId)
	ts integer DEFAULT (strftime('%s', 'now')) NOT NULL
);
CREATE INDEX IF NOT EXISTS logs_ts_id_idx ON logs (ts DESC, id);
CREATE INDEX IF NOT EXISTS logs_composite_idx ON logs (ts DESC, project, delivery_id, pipe_id, id);
CREATE INDEX IF NOT EXISTS logs_project_idx ON logs (project) WHERE project IS NOT NULL;
CREATE INDEX IF NOT EXISTS logs_delivery_id_idx ON logs (delivery_id) WHERE delivery_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS logs_pipe_id_idx ON logs (pipe_id) WHERE pipe_id IS NOT NULL;