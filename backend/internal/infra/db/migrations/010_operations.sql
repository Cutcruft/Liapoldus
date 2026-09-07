-- Round 10 (R6): ui-runtime operation/endpoint descriptors move from static
-- client-side builtins into the backend database. Two tables share the same
-- shape: a row with site_id IS NULL is a GLOBAL SYSTEM descriptor (seeded from
-- the platform builtin set below) and is read-only through the admin API; a
-- row with a site_id is admin-editable. The runtime contract serves system+
-- site rows; the client keeps the builtin list only as a boot fallback.

CREATE TABLE IF NOT EXISTS operations (
  id          TEXT NOT NULL,
  site_id     TEXT,
  system      BOOLEAN NOT NULL DEFAULT FALSE,
  provider    TEXT NOT NULL DEFAULT 'liapoldus.builtin',
  type_op     TEXT NOT NULL DEFAULT 'query',
  method      TEXT NOT NULL DEFAULT 'GET',
  path        TEXT NOT NULL DEFAULT '',
  cache       TEXT NOT NULL DEFAULT 'disabled',
  ttl         INTEGER,
  scope       TEXT NOT NULL DEFAULT 'public',
  result_type TEXT NOT NULL DEFAULT '',
  params      JSONB NOT NULL DEFAULT '{}',
  poll        JSONB NOT NULL DEFAULT '{}',
  subscribe   JSONB NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, site_id)
);
-- system rows have site_id NULL; Postgres treats NULLs as distinct in a PK,
-- so a partial unique index is required to enforce single-system-row-per-id.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_operations_global ON operations (id) WHERE site_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_operations_site ON operations (site_id);

CREATE TABLE IF NOT EXISTS endpoints (
  id           TEXT NOT NULL,
  site_id      TEXT,
  system       BOOLEAN NOT NULL DEFAULT FALSE,
  method       TEXT NOT NULL DEFAULT 'POST',
  path         TEXT NOT NULL DEFAULT '',
  operation_id TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, site_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_endpoints_global ON endpoints (id) WHERE site_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_endpoints_site ON endpoints (site_id);

-- Seed the platform builtin operation descriptors (mirrors
-- ui-runtime/src/core/builtin/descriptors.ts).
INSERT INTO operations
  (id, site_id, system, provider, type_op, method, path, cache, ttl, scope, result_type, params, created_at, updated_at)
VALUES
  ('content.get',  NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/api/contents/{contentId}',             'immutable', NULL, 'public', 'content',    '{"in":"query","fields":{"contentId":{"required":true},"locale":{"required":false}}}', now(), now()),
  ('content.list', NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/api/sites/{siteId}/contents',          'ttl',       120,  'public', 'content[]',  '{"in":"query","fields":{"siteId":{"required":true},"collectionId":{"required":false},"locale":{"required":false}}}', now(), now()),
  ('content.batch',NULL, TRUE, 'liapoldus.builtin', 'query',    'POST', '/api/sites/{siteId}/contents/batch',    'immutable', NULL, 'public', 'content[]',  '{"in":"body"}', now(), now()),
  ('asset.get',    NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/api/assets/{assetId}',                 'immutable', NULL, 'public', 'asset',      '{"in":"query","fields":{"assetId":{"required":true}}}', now(), now()),
  ('asset.list',   NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/api/sites/{siteId}/assets',            'immutable', NULL, 'public', 'asset[]',    '{"in":"query","fields":{"siteId":{"required":true}}}', now(), now()),
  ('asset.batch',  NULL, TRUE, 'liapoldus.builtin', 'query',    'POST', '/api/sites/{siteId}/assets/batch',      'immutable', NULL, 'public', 'assets',     '{"in":"body"}', now(), now()),
  ('form.get',     NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/api/forms/{formId}',                   'ttl',       300,  'public', 'form',       '{"in":"query","fields":{"formId":{"required":true}}}', now(), now()),
  ('form.submit',  NULL, TRUE, 'liapoldus.builtin', 'mutation', 'POST', '/api/forms/{formId}/submissions',       'disabled',  NULL, 'server', '',           '{"in":"path","fields":{"formId":{"required":true}}}', now(), now()),
  ('tree.get',     NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/runtime/tree',                         'immutable', NULL, 'public', 'tree',       '{"in":"query","fields":{"routeId":{"required":false},"locale":{"required":false}}}', now(), now()),
  ('tokens.get',   NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/runtime/tokens',                       'immutable', NULL, 'public', 'tokens',     '{"in":"query","fields":{"themeId":{"required":false}}}', now(), now()),
  ('routes.get',   NULL, TRUE, 'liapoldus.builtin', 'query',    'GET',  '/runtime/routes',                       'immutable', NULL, 'public', 'routes[]',   '{"in":"query"}', now(), now())
ON CONFLICT DO NOTHING;

-- Seed the platform builtin endpoint descriptors.
INSERT INTO endpoints
  (id, site_id, system, method, path, operation_id, created_at, updated_at)
VALUES
  ('form.submit', NULL, TRUE, 'POST', '/api/forms/{formId}/submissions', 'form.submit', now(), now())
ON CONFLICT DO NOTHING;