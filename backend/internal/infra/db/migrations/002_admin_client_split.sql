-- Round 2: admin/client split. Sites get locales + hosts; add contents,
-- assets, routes, forms, submissions.

ALTER TABLE sites
    ADD COLUMN IF NOT EXISTS default_locale TEXT NOT NULL DEFAULT 'ru',
    ADD COLUMN IF NOT EXISTS hosts TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS contents (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL,
    key TEXT NOT NULL,
    fields JSONB NOT NULL DEFAULT '{}'::jsonb,
    translations JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (site_id, collection_id, key)
);

CREATE TABLE IF NOT EXISTS assets (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    mime TEXT NOT NULL,
    size BIGINT NOT NULL,
    etag TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS routes (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    matcher TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    action JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS forms (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    definition JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS submissions (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    form_id TEXT NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS contents_site_collection_idx ON contents(site_id, collection_id);
CREATE INDEX IF NOT EXISTS assets_site_id_idx ON assets(site_id);
CREATE INDEX IF NOT EXISTS routes_site_id_idx ON routes(site_id);
CREATE INDEX IF NOT EXISTS forms_site_id_idx ON forms(site_id);
CREATE INDEX IF NOT EXISTS submissions_form_id_idx ON submissions(form_id);