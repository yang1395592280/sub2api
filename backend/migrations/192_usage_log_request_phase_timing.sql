-- Split the pre-forward portion of OpenAI TTFT into actionable request phases.
-- Nullable columns keep historical rows distinguishable from measured zeroes.
-- PostgreSQL adds nullable columns without rewriting the usage_logs table.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS body_read_ms INTEGER,
    ADD COLUMN IF NOT EXISTS preprocess_ms INTEGER,
    ADD COLUMN IF NOT EXISTS user_queue_ms INTEGER;
