-- Round 5: dependency service (Этап 4b). Site-declared npm dependencies
-- (top-level specs) plus the frozen lock stored on each snapshot, and the
-- immutable cache of registry package metadata. Raw tarball blobs live on
-- disk under <data>/deps (internal/infra/deps/store), not in the database.

ALTER TABLE snapshots ADD COLUMN IF NOT EXISTS deps_lock JSONB NOT NULL DEFAULT '{"deps":[]}'::jsonb;

CREATE TABLE IF NOT EXISTS site_dependencies (
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    spec TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_id, name)
);

-- Immutable registry metadata cache: one entry per (name, version).
CREATE TABLE IF NOT EXISTS dep_packages (
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    integrity TEXT NOT NULL DEFAULT '',
    tarball_url TEXT NOT NULL DEFAULT '',
    dependencies JSONB NOT NULL DEFAULT '{}'::jsonb,
    fetched_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (name, version)
);