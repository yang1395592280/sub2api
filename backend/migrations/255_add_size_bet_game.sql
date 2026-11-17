-- 255_add_size_bet_game.sql
-- 增加用户积分字段、猜大小游戏（Size Bet）及游戏中心（Game Center）相关数据表与配置

-- 1. 用户积分字段
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS points BIGINT NOT NULL DEFAULT 0;

-- 2. 游戏轮次表
CREATE TABLE IF NOT EXISTS game_rounds (
    id BIGSERIAL PRIMARY KEY,
    game_key VARCHAR(64) NOT NULL,
    round_no BIGINT NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    bet_closes_at TIMESTAMPTZ NOT NULL,
    settles_at TIMESTAMPTZ NOT NULL,
    prob_small NUMERIC(10,4) NOT NULL,
    prob_mid NUMERIC(10,4) NOT NULL,
    prob_big NUMERIC(10,4) NOT NULL,
    odds_small NUMERIC(10,4) NOT NULL,
    odds_mid NUMERIC(10,4) NOT NULL,
    odds_big NUMERIC(10,4) NOT NULL,
    allowed_stakes_json JSONB NOT NULL,
    result_number INT,
    result_direction VARCHAR(10),
    server_seed_hash VARCHAR(128) NOT NULL,
    server_seed VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_game_rounds_game_key_status ON game_rounds(game_key, status);
CREATE INDEX IF NOT EXISTS idx_game_rounds_game_key_starts_at ON game_rounds(game_key, starts_at DESC);

-- 3. 游戏投注表
CREATE TABLE IF NOT EXISTS game_bets (
    id BIGSERIAL PRIMARY KEY,
    round_id BIGINT NOT NULL REFERENCES game_rounds(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    direction VARCHAR(10) NOT NULL,
    stake_amount DECIMAL(20,8) NOT NULL,
    payout_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    net_result_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
    placed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_bets_round_user ON game_bets(round_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_game_bets_idempotency_key ON game_bets(idempotency_key) WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_game_bets_user_id ON game_bets(user_id);
CREATE INDEX IF NOT EXISTS idx_game_bets_round_id ON game_bets(round_id);

-- 4. 游戏账本与流水
CREATE TABLE IF NOT EXISTS game_wallet_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    game_key VARCHAR(64) NOT NULL,
    round_id BIGINT REFERENCES game_rounds(id) ON DELETE SET NULL,
    bet_id BIGINT REFERENCES game_bets(id) ON DELETE SET NULL,
    entry_type VARCHAR(32) NOT NULL,
    direction VARCHAR(10),
    stake_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    delta_amount DECIMAL(20,8) NOT NULL,
    balance_before DECIMAL(20,8) NOT NULL,
    balance_after DECIMAL(20,8) NOT NULL,
    reason VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_game_wallet_ledger_user_id ON game_wallet_ledger(user_id);
CREATE INDEX IF NOT EXISTS idx_game_wallet_ledger_round_id ON game_wallet_ledger(round_id);
CREATE INDEX IF NOT EXISTS idx_game_wallet_ledger_bet_id ON game_wallet_ledger(bet_id);
CREATE INDEX IF NOT EXISTS idx_game_wallet_ledger_game_key_created_at ON game_wallet_ledger(game_key, created_at DESC);

-- 5. 游戏排行榜快照
CREATE TABLE IF NOT EXISTS game_rank_snapshots (
    id BIGSERIAL PRIMARY KEY,
    scope_type VARCHAR(32) NOT NULL,
    scope_key VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL,
    net_profit DECIMAL(20,8) NOT NULL DEFAULT 0,
    win_count BIGINT NOT NULL DEFAULT 0,
    bet_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_rank_snapshots_scope_user ON game_rank_snapshots(scope_type, scope_key, user_id);
CREATE INDEX IF NOT EXISTS idx_game_rank_snapshots_scope_profit ON game_rank_snapshots(scope_type, scope_key, net_profit DESC);

-- 6. 游戏中心积分流水
CREATE TABLE IF NOT EXISTS game_points_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    entry_type VARCHAR(64) NOT NULL,
    delta_points BIGINT NOT NULL,
    points_before BIGINT NOT NULL,
    points_after BIGINT NOT NULL,
    related_game_key VARCHAR(64) NOT NULL DEFAULT '',
    related_round_id BIGINT,
    related_bet_id BIGINT,
    related_claim_batch_key VARCHAR(64) NOT NULL DEFAULT '',
    related_exchange_id BIGINT,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_game_points_ledger_user_created_at
    ON game_points_ledger (user_id, created_at DESC);

-- 7. 游戏中心每日积分空投记录
CREATE TABLE IF NOT EXISTS game_points_claims (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    claim_date DATE NOT NULL,
    batch_key VARCHAR(64) NOT NULL,
    points_amount BIGINT NOT NULL,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, claim_date, batch_key)
);

-- 8. 积分与福利余额兑换记录
CREATE TABLE IF NOT EXISTS game_points_exchanges (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    direction VARCHAR(32) NOT NULL,
    source_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    source_points BIGINT NOT NULL DEFAULT 0,
    target_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    target_points BIGINT NOT NULL DEFAULT 0,
    rate DECIMAL(20,8) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_game_points_exchanges_user ON game_points_exchanges(user_id, created_at DESC);

-- 9. 游戏目录表
CREATE TABLE IF NOT EXISTS game_catalogs (
    id BIGSERIAL PRIMARY KEY,
    game_key VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    subtitle VARCHAR(255) NOT NULL DEFAULT '',
    cover_image TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    default_open_mode VARCHAR(16) NOT NULL DEFAULT 'dual',
    supports_embed BOOLEAN NOT NULL DEFAULT TRUE,
    supports_standalone BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO game_catalogs (
    game_key,
    name,
    subtitle,
    description,
    default_open_mode,
    enabled
) VALUES (
    'size_bet',
    '猜大小',
    '经典快节奏数字竞猜',
    '使用福利余额或每日签到空投参与猜大小游戏，每60秒一期。',
    'dual',
    TRUE
)
ON CONFLICT (game_key) DO UPDATE SET
    name = EXCLUDED.name,
    subtitle = EXCLUDED.subtitle,
    description = EXCLUDED.description;

-- 10. 初始化猜大小游戏全局配置与总开关
INSERT INTO settings (key, value, updated_at) VALUES
    ('size_bet_enabled', 'true', NOW()),
    ('size_bet_round_duration_seconds', '60', NOW()),
    ('size_bet_bet_close_offset_seconds', '50', NOW()),
    ('size_bet_allowed_stakes', '[1, 2, 5, 10, 20]', NOW()),
    ('size_bet_custom_stake_min', '1', NOW()),
    ('size_bet_custom_stake_max', '100', NOW()),
    ('size_bet_prob_small', '45.0', NOW()),
    ('size_bet_prob_mid', '10.0', NOW()),
    ('size_bet_prob_big', '45.0', NOW()),
    ('size_bet_odds_small', '2.0', NOW()),
    ('size_bet_odds_mid', '10.0', NOW()),
    ('size_bet_odds_big', '2.0', NOW()),
    ('game_center_enabled', 'true', NOW()),
    ('game_center_claim_enabled', 'true', NOW()),
    ('game_center_exchange_balance_to_points_enabled', 'true', NOW()),
    ('game_center_exchange_points_to_balance_enabled', 'true', NOW()),
    ('game_center_exchange_balance_to_points_rate', '100', NOW()),
    ('game_center_exchange_points_to_balance_rate', '0.01', NOW()),
    ('game_center_exchange_min_balance_amount', '0.01', NOW()),
    ('game_center_exchange_min_points_amount', '1', NOW())
ON CONFLICT (key) DO NOTHING;

-- 11. 适当缩小签到奖励金额（如果当前高于0.10，则平滑缩减至0.01~0.10）
UPDATE settings
SET value = '0.01', updated_at = NOW()
WHERE key = 'checkin_min_reward' AND CAST(NULLIF(value, '') AS NUMERIC) > 0.05;

UPDATE settings
SET value = '0.10', updated_at = NOW()
WHERE key = 'checkin_max_reward' AND CAST(NULLIF(value, '') AS NUMERIC) > 0.50;
