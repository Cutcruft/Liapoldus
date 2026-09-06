-- Slice 3: per-site design token set (colors, typography, spacing, …).
--
-- One row per site, JSONB holding the whole token set. The editor always
-- replaces the entire set, so there is no finer-grained storage needed.

CREATE TABLE IF NOT EXISTS site_tokens (
    site_id    TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    tokens     JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_id)
);