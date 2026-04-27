ALTER TABLE anthropic_auto_inspect_batches
    ADD COLUMN IF NOT EXISTS skip_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_anthropic_auto_inspect_batches_status_created_at
    ON anthropic_auto_inspect_batches (status, created_at DESC);
