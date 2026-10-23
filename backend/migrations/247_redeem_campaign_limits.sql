-- Welfare limits count successful redemptions across all codes in a campaign.
CREATE TABLE IF NOT EXISTS redeem_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    per_user_limit INTEGER NOT NULL DEFAULT 1 CHECK (per_user_limit BETWEEN 1 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS campaign_id BIGINT REFERENCES redeem_campaigns(id);

CREATE INDEX IF NOT EXISTS idx_redeem_codes_campaign_id ON redeem_codes (campaign_id);

-- Independent of redeem_codes so deleting a used code cannot reset eligibility.
CREATE TABLE IF NOT EXISTS redeem_campaign_claims (
    campaign_id BIGINT NOT NULL REFERENCES redeem_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redeemed_count INTEGER NOT NULL CHECK (redeemed_count > 0),
    PRIMARY KEY (campaign_id, user_id)
);
