-- Round 9 (R5): the component registry becomes the source of truth for the
-- editor's source buffer; git keeps the per-version durable history. Old rows
-- get an empty source (nothing pre-existing was migrated before R5 shipped).

ALTER TABLE component_definitions ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';