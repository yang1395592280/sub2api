-- Existing balance and frozen balances retain their original amounts.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS checkin_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS liyu_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS wallet_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    event_type VARCHAR(40) NOT NULL,
    operation_id VARCHAR(200) NOT NULL,
    api_key_id BIGINT NOT NULL DEFAULT 0,
    paid_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    checkin_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    liyu_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    gift_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    paid_after NUMERIC(20,8) NOT NULL,
    checkin_after NUMERIC(20,8) NOT NULL,
    liyu_after NUMERIC(20,8) NOT NULL,
    gift_after NUMERIC(20,8) NOT NULL,
    actor_id BIGINT,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, event_type, operation_id, api_key_id)
);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_time ON wallet_transactions(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_usage ON wallet_transactions(operation_id, api_key_id) WHERE event_type = 'usage';

CREATE TABLE IF NOT EXISTS wallet_image_holds (
    batch_id VARCHAR(200) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    paid NUMERIC(20,8) NOT NULL,
    checkin NUMERIC(20,8) NOT NULL,
    liyu NUMERIC(20,8) NOT NULL,
    gift NUMERIC(20,8) NOT NULL,
    settled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
