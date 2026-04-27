PRAGMA user_version = 1;
CREATE TABLE IF NOT EXISTS logs (
  id          INTEGER PRIMARY KEY NOT NULL,
  level       INTEGER NOT NULL, -- log level as used in slog: -4 for debug, 0 for info, 4 for warn and 8 for error
  project     TEXT,
  delivery_id TEXT,
  pipe_id     TEXT,
  message     TEXT NOT NULL,
  data        BLOB, -- additional data passed as logs attr (besides project, deliveryId and pipeId)
  ts          INTEGER DEFAULT (strftime('%s', 'now')) NOT NULL
);
CREATE INDEX IF NOT EXISTS logs_ts_id_idx          ON logs (             ts DESC, id DESC);
CREATE INDEX IF NOT EXISTS logs_project_ts_idx     ON logs (project,     ts DESC, id DESC) WHERE project     IS NOT NULL;
CREATE INDEX IF NOT EXISTS logs_delivery_id_ts_idx ON logs (delivery_id, ts DESC, id DESC) WHERE delivery_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS logs_pipe_id_ts_idx     ON logs (pipe_id,     ts DESC, id DESC) WHERE pipe_id     IS NOT NULL;
