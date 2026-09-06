-- Round 5 (Этап 4b), slice 7: dependency cache limits and LRU eviction.
--
-- Per-site dependency tarball cache limits (shared on-disk cache, <data>/deps).
-- A site without a row has no limit; the effective limit of the shared cache
-- is the maximum over all configured sites. The dep_tarball_access table
-- records the last access time of each cached tarball (name, version) so the
-- eviction pass can remove the least recently used entries first.

CREATE TABLE IF NOT EXISTS site_cache_config (
    site_id        TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    max_deps_bytes BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_id)
);

CREATE TABLE IF NOT EXISTS dep_tarball_access (
    name        TEXT NOT NULL,
    version     TEXT NOT NULL,
    last_access TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (name, version)
);
