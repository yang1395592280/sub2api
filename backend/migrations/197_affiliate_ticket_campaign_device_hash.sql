ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS campaign_device_hash VARCHAR(128) NOT NULL DEFAULT '';
