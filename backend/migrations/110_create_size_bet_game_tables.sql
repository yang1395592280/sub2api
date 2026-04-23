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

CREATE TABLE IF NOT EXISTS game_bets (
    id BIGSERIAL PRIMARY KEY,
    round_id BIGINT NOT NULL REFERENCES game_rounds(id) ON DELETE CASCADE,
    -- Intentionally no FK to users: the app hard-deletes users and audit history must survive.
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

CREATE TABLE IF NOT EXISTS game_wallet_ledger (
    id BIGSERIAL PRIMARY KEY,
    -- Intentionally no FK to users: the app hard-deletes users and audit history must survive.
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

CREATE TABLE IF NOT EXISTS game_rank_snapshots (
    id BIGSERIAL PRIMARY KEY,
    scope_type VARCHAR(32) NOT NULL,
    scope_key VARCHAR(64) NOT NULL,
    -- Intentionally no FK to users: the app hard-deletes users and audit history must survive.
    user_id BIGINT NOT NULL,
    net_profit DECIMAL(20,8) NOT NULL DEFAULT 0,
    win_count BIGINT NOT NULL DEFAULT 0,
    bet_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_rank_snapshots_scope_user ON game_rank_snapshots(scope_type, scope_key, user_id);
CREATE INDEX IF NOT EXISTS idx_game_rank_snapshots_scope_profit ON game_rank_snapshots(scope_type, scope_key, net_profit DESC);
