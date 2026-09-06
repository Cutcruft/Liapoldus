-- Slice 1: git-backed snapshots.
-- snapshots gain the git commit sha that produced them.
ALTER TABLE snapshots ADD COLUMN IF NOT EXISTS git_sha TEXT;

-- snapshot_pages are informational bookkeeping: the canonical content of a
-- snapshot lives in the site git tree (snapshots.git_sha). Site-wide
-- publish/restore replace the page set in the registry, so page deletion must
-- cascade through snapshot_pages instead of being blocked.
ALTER TABLE snapshot_pages DROP CONSTRAINT IF EXISTS snapshot_pages_page_id_fkey;
ALTER TABLE snapshot_pages DROP CONSTRAINT IF EXISTS snapshot_pages_version_id_fkey;
ALTER TABLE snapshot_pages
    ADD CONSTRAINT snapshot_pages_page_id_fkey
    FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE;
ALTER TABLE snapshot_pages
    ADD CONSTRAINT snapshot_pages_version_id_fkey
    FOREIGN KEY (version_id) REFERENCES page_versions(id) ON DELETE CASCADE;