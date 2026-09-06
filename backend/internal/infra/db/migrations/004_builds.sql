-- Round 4: build service (Этап 3). Builds are the result of compiling a
-- snapshot for an environment; statuses queue → building → ready | failed.

CREATE TABLE IF NOT EXISTS builds (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    snapshot_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    status TEXT NOT NULL,
    log JSONB NOT NULL DEFAULT '[]'::jsonb,
    artifact_dir TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    UNIQUE (site_id, environment, snapshot_id, status)
);

CREATE INDEX IF NOT EXISTS builds_site_env_snapshot_idx
    ON builds (site_id, environment, snapshot_id);