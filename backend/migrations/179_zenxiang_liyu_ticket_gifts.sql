CREATE TABLE IF NOT EXISTS zenxiang_liyu_ticket_gifts (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    play_date DATE NOT NULL,
    ticket_count INTEGER NOT NULL,
    granted_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_ticket_gifts_request_unique UNIQUE (request_id),
    CONSTRAINT zenxiang_liyu_ticket_gifts_positive_count CHECK (ticket_count > 0)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_ticket_gifts_user_date
    ON zenxiang_liyu_ticket_gifts (user_id, play_date);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_ticket_gifts_created_at
    ON zenxiang_liyu_ticket_gifts (created_at DESC);
