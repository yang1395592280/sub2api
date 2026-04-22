ALTER TABLE checkin_records
    ADD COLUMN IF NOT EXISTS base_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_status VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS bonus_delta_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_played_at TIMESTAMPTZ NULL;

UPDATE checkin_records
SET base_reward_amount = reward_amount
WHERE base_reward_amount = 0;
