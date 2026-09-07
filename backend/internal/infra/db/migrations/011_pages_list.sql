-- R7a: pages/page_versions move from a JSONB component tree (`root`) to a
-- linear element list (`list`). Old tree data is dropped (no converters), per
-- the redesign reset decision. Statements are idempotent because migrations
-- re-run on every startup.
ALTER TABLE pages DROP COLUMN IF EXISTS root;
ALTER TABLE pages ADD COLUMN IF NOT EXISTS list JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE page_versions DROP COLUMN IF EXISTS root;
ALTER TABLE page_versions ADD COLUMN IF NOT EXISTS list JSONB NOT NULL DEFAULT '[]'::jsonb;
