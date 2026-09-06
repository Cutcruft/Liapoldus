-- Round 6 (Этап 4b, слайс 6): per-site dependency allowlist. An empty
-- allowlist for a site authorizes everything (backward compatible); once a
-- site has at least one entry, every resolved package of that site — top-level
-- and transitive — must match one of the entries or lock creation fails
-- (ErrDepNotAllowed, 422). Entries are npm names ("lodash"), scoped packages
-- ("@acme/core") or scope wildcards ("@acme/*", "*").

CREATE TABLE IF NOT EXISTS site_dependency_allowlist (
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    entry TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_id, entry)
);