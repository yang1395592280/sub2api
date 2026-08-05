ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS registration_ip VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS risk_status VARCHAR(32) NOT NULL DEFAULT 'clear',
    ADD COLUMN IF NOT EXISTS risk_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_user_affiliates_registration_ip
    ON user_affiliates (registration_ip)
    WHERE registration_ip <> '';

CREATE TABLE IF NOT EXISTS affiliate_ticket_campaign_events (
    id BIGSERIAL PRIMARY KEY,
    event_key VARCHAR(160) NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    play_date DATE NOT NULL,
    amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ticket_count INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'granted',
    risk_reason TEXT NOT NULL DEFAULT '',
    inviter_ip VARCHAR(64) NOT NULL DEFAULT '',
    invitee_ip VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT affiliate_ticket_campaign_events_key_unique UNIQUE (event_key),
    CONSTRAINT affiliate_ticket_campaign_events_type_check CHECK (event_type IN ('invite_register', 'invite_recharge')),
    CONSTRAINT affiliate_ticket_campaign_events_status_check CHECK (status IN ('granted', 'blocked', 'frozen', 'skipped')),
    CONSTRAINT affiliate_ticket_campaign_events_non_negative CHECK (amount >= 0 AND ticket_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_ticket_campaign_events_inviter_date
    ON affiliate_ticket_campaign_events (inviter_id, play_date, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_ticket_campaign_events_risk
    ON affiliate_ticket_campaign_events (status, created_at DESC)
    WHERE status IN ('blocked', 'frozen');

CREATE TABLE IF NOT EXISTS affiliate_ticket_campaign_daily (
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    play_date DATE NOT NULL,
    registered_count INTEGER NOT NULL DEFAULT 0,
    recharge_count INTEGER NOT NULL DEFAULT 0,
    ticket_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (inviter_id, play_date),
    CONSTRAINT affiliate_ticket_campaign_daily_non_negative CHECK (
        registered_count >= 0 AND recharge_count >= 0 AND ticket_count >= 0
    )
);

CREATE TABLE IF NOT EXISTS affiliate_ticket_campaign_batches (
    id BIGSERIAL PRIMARY KEY,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id BIGINT NOT NULL REFERENCES affiliate_ticket_campaign_events(id) ON DELETE CASCADE,
    granted_count INTEGER NOT NULL,
    remaining_count INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT affiliate_ticket_campaign_batches_event_unique UNIQUE (event_id),
    CONSTRAINT affiliate_ticket_campaign_batches_count_check CHECK (
        granted_count > 0 AND remaining_count >= 0 AND remaining_count <= granted_count
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_ticket_campaign_batches_available
    ON affiliate_ticket_campaign_batches (inviter_id, expires_at, id)
    WHERE remaining_count > 0;
