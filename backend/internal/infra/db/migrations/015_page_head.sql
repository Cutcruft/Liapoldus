-- R10 (P0-2): per-page presentation — optional layout override and page head
-- (spec §2.4). Both pages and page_versions carry the fields so a snapshot
-- pins the exact presentation as written, and a snapshot rollback restores
-- layout + head together with the list (same semantics as `list`). Empty
-- layoutSectionId means "fall back to the site default"; head defaults to an
-- empty document that materializes purely from SiteSettings.
ALTER TABLE pages ADD COLUMN IF NOT EXISTS layout_section_id TEXT NOT NULL DEFAULT '';
ALTER TABLE pages ADD COLUMN IF NOT EXISTS head JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE page_versions ADD COLUMN IF NOT EXISTS layout_section_id TEXT NOT NULL DEFAULT '';
ALTER TABLE page_versions ADD COLUMN IF NOT EXISTS head JSONB NOT NULL DEFAULT '{}'::jsonb;