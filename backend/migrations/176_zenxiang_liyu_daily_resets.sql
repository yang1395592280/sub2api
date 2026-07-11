CREATE TABLE IF NOT EXISTS zenxiang_liyu_daily_resets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    play_date DATE NOT NULL,
    reset_count INTEGER NOT NULL DEFAULT 0,
    reset_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_daily_resets_user_date_unique UNIQUE (user_id, play_date),
    CONSTRAINT zenxiang_liyu_daily_resets_non_negative_count CHECK (reset_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_daily_resets_date
    ON zenxiang_liyu_daily_resets (play_date);
