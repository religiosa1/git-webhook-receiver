CREATE TABLE IF NOT EXISTS pipelines (
  id          INTEGER PRIMARY KEY NOT NULL,
  pipe_id     TEXT NOT NULL,
  project     TEXT NOT NULL,
  delivery_id TEXT NOT NULL,
  hash        TEXT,
  config      BLOB NOT NULL,
  error       TEXT,
  output      BLOB,
  created_at  INTEGER DEFAULT (unixepoch('subsec') * 1000) NOT NULL,
  ended_at    INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS ix_unique_pipelines_pipeId on pipelines(pipe_id);
CREATE INDEX IF NOT EXISTS ix_pipelines_created  ON pipelines (             created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS ix_pipelines_project  ON pipelines (project,     created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS ix_pipelines_delivery ON pipelines (delivery_id, created_at DESC, id DESC);
