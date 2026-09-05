-- Round 3: component definitions registry (R5 metadata side; source lives in git).

CREATE TABLE IF NOT EXISTS component_definitions (
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'component',
    schema JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    current_sha TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_id, id)
);