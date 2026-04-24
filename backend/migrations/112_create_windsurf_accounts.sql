CREATE TABLE IF NOT EXISTS windsurf_accounts (
    id BIGSERIAL PRIMARY KEY,
    account VARCHAR(255) NOT NULL UNIQUE,
    password_encrypted TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    maintained_by BIGINT NOT NULL,
    maintained_at TIMESTAMPTZ NOT NULL,
    status_updated_by BIGINT NULL,
    status_updated_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_windsurf_accounts_enabled ON windsurf_accounts (enabled);
CREATE INDEX IF NOT EXISTS idx_windsurf_accounts_maintained_at ON windsurf_accounts (maintained_at DESC);
