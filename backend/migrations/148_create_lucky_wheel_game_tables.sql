CREATE TABLE IF NOT EXISTS game_lucky_wheel_spins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    spin_date DATE NOT NULL,
    spin_index INT NOT NULL,
    prize_key VARCHAR(64) NOT NULL,
    prize_label VARCHAR(128) NOT NULL DEFAULT '',
    prize_type VARCHAR(32) NOT NULL,
    delta_points BIGINT NOT NULL DEFAULT 0,
    points_before BIGINT NOT NULL DEFAULT 0,
    points_after BIGINT NOT NULL DEFAULT 0,
    probability DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_lucky_wheel_spins_user_date_index
    ON game_lucky_wheel_spins (user_id, spin_date, spin_index);

CREATE INDEX IF NOT EXISTS idx_game_lucky_wheel_spins_spin_date_created_at
    ON game_lucky_wheel_spins (spin_date, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_game_lucky_wheel_spins_user_created_at
    ON game_lucky_wheel_spins (user_id, created_at DESC);

INSERT INTO game_catalogs (
    game_key,
    name,
    subtitle,
    description,
    sort_order,
    default_open_mode
) VALUES
    (
        'size_bet',
        '猜大小',
        '经典快节奏积分竞猜',
        '每局独立结算的纯积分猜大小玩法。',
        20,
        'dual'
    ),
    (
        'lucky_wheel',
        '幸运转盘',
        '每日限次抽取积分奖励或惩罚',
        '每日限次的幸运转盘，结果仅结算到游戏积分。',
        10,
        'dual'
    )
ON CONFLICT (game_key) DO NOTHING;
