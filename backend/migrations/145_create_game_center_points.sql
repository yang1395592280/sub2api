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
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_game_points_ledger_user_created_at
    ON game_points_ledger (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_game_points_ledger_game_created_at
    ON game_points_ledger (related_game_key, created_at DESC);

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

CREATE INDEX IF NOT EXISTS idx_game_points_claims_user_claim_date
    ON game_points_claims (user_id, claim_date DESC);

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
