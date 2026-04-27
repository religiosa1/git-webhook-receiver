PRAGMA user_version = 1;
CREATE TABLE IF NOT EXISTS pipeline (
  id          INTEGER PRIMARY KEY NOT NULL,
  pipe_id     TEXT NOT NULL,
  project     TEXT NOT NULL,
  delivery_id TEXT NOT NULL,
  config      BLOB NOT NULL,
  error       TEXT,
  output      BLOB,
  created_at  INTEGER DEFAULT (strftime('%s', 'now')) NOT NULL,
  ended_at    INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS ix_unique_pipeline_pipeId on pipeline(pipe_id);
CREATE INDEX IF NOT EXISTS ix_pipeline_created  ON pipeline (             created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS ix_pipeline_project  ON pipeline (project,     created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS ix_pipeline_delivery ON pipeline (delivery_id, created_at DESC, id DESC);
