CREATE TABLE IF NOT EXISTS checkin_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date VARCHAR(10) NOT NULL,
    reward_amount DECIMAL(20,8) NOT NULL,
    user_timezone VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_checkin_records_user_id ON checkin_records(user_id);
CREATE INDEX IF NOT EXISTS idx_checkin_records_checkin_date ON checkin_records(checkin_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_checkin_records_user_date ON checkin_records(user_id, checkin_date);

ALTER TABLE checkin_records
    ADD COLUMN IF NOT EXISTS base_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_status VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS bonus_delta_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_played_at TIMESTAMPTZ NULL;

UPDATE checkin_records
SET base_reward_amount = reward_amount
WHERE base_reward_amount = 0;

CREATE TABLE IF NOT EXISTS checkin_risk_cases (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 status VARCHAR(16) NOT NULL DEFAULT 'observing' CHECK (status IN ('observing','cleared','revoked','restored')),
 reason TEXT NOT NULL,
 evidence TEXT NOT NULL,
 confirmed_by BIGINT,
 resolved_by BIGINT,
 observe_until TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '3 days'),
 cleared_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
 decided_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_checkin_risk_active_user ON checkin_risk_cases(user_id) WHERE status IN ('observing','cleared');
CREATE INDEX IF NOT EXISTS idx_checkin_risk_due ON checkin_risk_cases(observe_until) WHERE status='observing';
CREATE TABLE IF NOT EXISTS checkin_security_events (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 event_type VARCHAR(32) NOT NULL,
 client_ip VARCHAR(64) NOT NULL DEFAULT '',
 user_agent VARCHAR(500) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_checkin_security_user_time ON checkin_security_events(user_id,created_at DESC);

CREATE TABLE IF NOT EXISTS checkin_page_tokens (
    token_hash VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '30 minutes'),
    consumed BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_checkin_page_tokens_expiry ON checkin_page_tokens(expires_at);

ALTER TABLE wallet_image_holds ADD COLUMN IF NOT EXISTS checkin_risk_case_id BIGINT REFERENCES checkin_risk_cases(id);
