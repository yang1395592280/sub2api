CREATE TABLE IF NOT EXISTS openai_scheduler_daily_stats (
    stat_date DATE NOT NULL,
    group_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    select_count BIGINT NOT NULL DEFAULT 0,
    last_selected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (stat_date, group_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_openai_scheduler_daily_stats_group_date
    ON openai_scheduler_daily_stats (group_id, stat_date);
