ALTER TABLE users
ADD COLUMN IF NOT EXISTS points BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS game_points_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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

CREATE TABLE IF NOT EXISTS game_points_claims (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    claim_date DATE NOT NULL,
    batch_key VARCHAR(64) NOT NULL,
    points_amount BIGINT NOT NULL,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, claim_date, batch_key)
);

CREATE TABLE IF NOT EXISTS game_points_exchanges (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
    default_open_mode
) VALUES (
    'size_bet',
    '猜大小',
    '经典快节奏竞猜',
    '现有猜大小游戏接入游戏中心',
    'dual'
)
ON CONFLICT (game_key) DO NOTHING;
