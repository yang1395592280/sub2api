CREATE TABLE IF NOT EXISTS zenxiang_liyu_settings (
    id BIGSERIAL PRIMARY KEY,
    global_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ticket_amount NUMERIC(20,8) NOT NULL DEFAULT 2,
    minimum_balance NUMERIC(20,8) NOT NULL DEFAULT 10,
    daily_play_limit INTEGER NOT NULL DEFAULT 5,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_settings_singleton CHECK (id = 1),
    CONSTRAINT zenxiang_liyu_settings_positive_ticket CHECK (ticket_amount > 0),
    CONSTRAINT zenxiang_liyu_settings_non_negative_minimum CHECK (minimum_balance >= 0),
    CONSTRAINT zenxiang_liyu_settings_positive_daily_limit CHECK (daily_play_limit > 0)
);

INSERT INTO zenxiang_liyu_settings (id, global_enabled, ticket_amount, minimum_balance, daily_play_limit)
VALUES (1, FALSE, 2, 10, 5)
ON CONFLICT (id) DO NOTHING;

SELECT setval(
    'zenxiang_liyu_settings_id_seq',
    (SELECT MAX(id) FROM zenxiang_liyu_settings),
    TRUE
);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_prizes (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    reward_amount NUMERIC(20,8) NOT NULL,
    probability NUMERIC(12,8) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_prizes_non_negative_reward CHECK (reward_amount >= 0),
    CONSTRAINT zenxiang_liyu_prizes_probability_range CHECK (probability >= 0 AND probability <= 100)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_prizes_enabled_sort
    ON zenxiang_liyu_prizes (enabled, sort_order, id);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_user_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    granted_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_user_grants_user_unique UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_user_grants_enabled
    ON zenxiang_liyu_user_grants (enabled);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_records (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    play_date DATE NOT NULL,
    ticket_amount NUMERIC(20,8) NOT NULL,
    reward_amount NUMERIC(20,8) NOT NULL,
    user_net_amount NUMERIC(20,8) NOT NULL,
    system_revenue NUMERIC(20,8) NOT NULL,
    system_expense NUMERIC(20,8) NOT NULL,
    system_profit NUMERIC(20,8) NOT NULL,
    prize_id BIGINT NULL REFERENCES zenxiang_liyu_prizes(id) ON DELETE SET NULL,
    prize_name_snapshot VARCHAR(100) NOT NULL,
    probability_snapshot NUMERIC(12,8) NOT NULL,
    config_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after_ticket NUMERIC(20,8) NOT NULL,
    balance_after_reward NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_records_request_unique UNIQUE (request_id),
    CONSTRAINT zenxiang_liyu_records_non_negative_ticket CHECK (ticket_amount > 0),
    CONSTRAINT zenxiang_liyu_records_non_negative_reward CHECK (reward_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_records_user_date
    ON zenxiang_liyu_records (user_id, play_date);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_records_play_date
    ON zenxiang_liyu_records (play_date);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_records_prize
    ON zenxiang_liyu_records (prize_id);
