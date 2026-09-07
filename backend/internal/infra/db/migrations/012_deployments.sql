-- R9: deployment pins (spec §2.6). A deployment is the site + environment →
-- snapshotId pin; one row per (site, environment), rewritten on release and
-- on rollback. FK RESTRICTs snapshot deletion while the snapshot is pinned.

CREATE TABLE IF NOT EXISTS deployments (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    environment TEXT NOT NULL,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (site_id, environment)
);

CREATE INDEX IF NOT EXISTS deployments_site_idx
    ON deployments (site_id);