-- R10 component hierarchy: only sections are page-level templates; each
-- section explicitly allowlists the primitive components it may import.
ALTER TABLE component_definitions ADD COLUMN IF NOT EXISTS is_section BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE component_definitions ADD COLUMN IF NOT EXISTS allowed_primitive_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE component_definitions ADD COLUMN IF NOT EXISTS accepts_page_content BOOLEAN NOT NULL DEFAULT FALSE;
