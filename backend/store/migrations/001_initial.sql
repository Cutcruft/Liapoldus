CREATE TABLE IF NOT EXISTS sites (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS pages (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    root JSONB NOT NULL,
    current_version INTEGER NOT NULL CHECK (current_version > 0),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (site_id, slug)
);

CREATE TABLE IF NOT EXISTS page_versions (
    id TEXT PRIMARY KEY,
    page_id TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    number INTEGER NOT NULL CHECK (number > 0),
    root JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (page_id, number)
);

CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshot_pages (
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    page_id TEXT NOT NULL REFERENCES pages(id) ON DELETE RESTRICT,
    version_id TEXT NOT NULL REFERENCES page_versions(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version > 0),
    PRIMARY KEY (snapshot_id, page_id)
);

CREATE INDEX IF NOT EXISTS pages_site_id_idx ON pages(site_id);
CREATE INDEX IF NOT EXISTS page_versions_page_id_idx ON page_versions(page_id);
CREATE INDEX IF NOT EXISTS snapshots_site_id_idx ON snapshots(site_id);
