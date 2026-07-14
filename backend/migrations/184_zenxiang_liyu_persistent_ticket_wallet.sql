ALTER TABLE zenxiang_liyu_settings
    ADD COLUMN IF NOT EXISTS ticket_carryover_started_on DATE NOT NULL
        DEFAULT ((NOW() AT TIME ZONE 'Asia/Shanghai')::date);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_ticket_wallets (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_ticket_wallets_balance_range CHECK (balance >= 0 AND balance <= 5)
);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_ticket_usage_credits (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    usage_date DATE NOT NULL,
    ticket_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, usage_date),
    CONSTRAINT zenxiang_liyu_ticket_usage_credits_non_negative CHECK (ticket_count >= 0)
);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_ticket_batches (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type VARCHAR(32) NOT NULL,
    source_key VARCHAR(128) NOT NULL,
    granted_count INTEGER NOT NULL,
    remaining_count INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_ticket_batches_source_unique UNIQUE (user_id, source_type, source_key),
    CONSTRAINT zenxiang_liyu_ticket_batches_count_range CHECK (
        granted_count >= 0 AND remaining_count >= 0 AND remaining_count <= granted_count
    )
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_ticket_batches_available
    ON zenxiang_liyu_ticket_batches (user_id, expires_at, id)
    WHERE remaining_count > 0;

WITH current_settings AS (
    SELECT ticket_usage_threshold, daily_ticket_limit
    FROM zenxiang_liyu_settings
    WHERE id = 1
)
INSERT INTO zenxiang_liyu_ticket_usage_credits (user_id, usage_date, ticket_count)
SELECT logs.user_id,
       (logs.created_at AT TIME ZONE 'Asia/Shanghai')::date,
       LEAST(
           FLOOR(SUM(logs.actual_cost) / settings.ticket_usage_threshold)::integer,
           settings.daily_ticket_limit
       )
FROM usage_logs logs
CROSS JOIN current_settings settings
WHERE (logs.created_at AT TIME ZONE 'Asia/Shanghai')::date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
GROUP BY logs.user_id, (logs.created_at AT TIME ZONE 'Asia/Shanghai')::date,
         settings.ticket_usage_threshold, settings.daily_ticket_limit
ON CONFLICT (user_id, usage_date) DO NOTHING;

WITH activity_users AS (
    SELECT user_id
    FROM zenxiang_liyu_ticket_usage_credits
    WHERE usage_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
    UNION
    SELECT user_id
    FROM zenxiang_liyu_ticket_gifts
    WHERE play_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
    UNION
    SELECT user_id
    FROM zenxiang_liyu_records
    WHERE play_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
),
earned AS (
    SELECT user_id, SUM(ticket_count)::integer AS ticket_count
    FROM zenxiang_liyu_ticket_usage_credits
    WHERE usage_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
    GROUP BY user_id
),
gifted AS (
    SELECT user_id, SUM(ticket_count)::integer AS ticket_count
    FROM zenxiang_liyu_ticket_gifts
    WHERE play_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
    GROUP BY user_id
),
played AS (
    SELECT user_id, COUNT(*)::integer AS ticket_count
    FROM zenxiang_liyu_records
    WHERE play_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
    GROUP BY user_id
),
resets AS (
    SELECT user_id, SUM(reset_count)::integer AS ticket_count
    FROM zenxiang_liyu_daily_resets
    WHERE play_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date
    GROUP BY user_id
)
INSERT INTO zenxiang_liyu_ticket_wallets (user_id, balance)
SELECT users.user_id,
       LEAST(5, GREATEST(
           0,
           COALESCE(earned.ticket_count, 0) + COALESCE(gifted.ticket_count, 0)
               - COALESCE(played.ticket_count, 0) + COALESCE(resets.ticket_count, 0)
       ))
FROM activity_users users
LEFT JOIN earned ON earned.user_id = users.user_id
LEFT JOIN gifted ON gifted.user_id = users.user_id
LEFT JOIN played ON played.user_id = users.user_id
LEFT JOIN resets ON resets.user_id = users.user_id
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO zenxiang_liyu_ticket_batches (
    user_id, source_type, source_key, granted_count, remaining_count, expires_at
)
SELECT user_id,
       'migration',
       ticket_carryover_started_on::text,
       balance,
       balance,
       ((ticket_carryover_started_on + 2)::timestamp AT TIME ZONE 'Asia/Shanghai')
FROM zenxiang_liyu_ticket_wallets
CROSS JOIN (
    SELECT ticket_carryover_started_on
    FROM zenxiang_liyu_settings
    WHERE id = 1
) settings
WHERE balance > 0
ON CONFLICT (user_id, source_type, source_key) DO NOTHING;
