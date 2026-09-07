-- R10: one optional presentation-default record per site. The JSON document
-- lets later head fields evolve without a schema migration; site_id preserves
-- isolation and cascades on site deletion.
CREATE TABLE IF NOT EXISTS site_settings (
    site_id TEXT PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    settings JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
