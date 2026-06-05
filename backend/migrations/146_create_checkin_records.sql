CREATE TABLE IF NOT EXISTS checkin_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    reward_points BIGINT NOT NULL,
    base_reward_points BIGINT NOT NULL DEFAULT 0,
    bonus_status VARCHAR(16) NOT NULL DEFAULT 'none',
    bonus_delta_points BIGINT NOT NULL DEFAULT 0,
    user_timezone VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bonus_played_at TIMESTAMPTZ NULL,
    UNIQUE (user_id, checkin_date)
);

CREATE INDEX IF NOT EXISTS idx_checkin_records_user_checkin_date
    ON checkin_records (user_id, checkin_date DESC);

CREATE INDEX IF NOT EXISTS idx_checkin_records_checkin_date
    ON checkin_records (checkin_date DESC);
