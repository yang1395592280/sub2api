CREATE TABLE IF NOT EXISTS game_lucky_wheel_spins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
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
    ON game_lucky_wheel_spins(user_id, spin_date, spin_index);

CREATE INDEX IF NOT EXISTS idx_game_lucky_wheel_spins_spin_date_created_at
    ON game_lucky_wheel_spins(spin_date, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_game_lucky_wheel_spins_user_created_at
    ON game_lucky_wheel_spins(user_id, created_at DESC);

INSERT INTO settings (key, value)
VALUES
    ('lucky_wheel_enabled', 'true'),
    ('lucky_wheel_daily_spin_limit', '5'),
    ('lucky_wheel_prizes', $$[
        {"key":"jackpot_888","label":"超级大奖 +888","type":"reward","delta_points":888,"probability":0.5},
        {"key":"bonus_288","label":"好运爆发 +288","type":"reward","delta_points":288,"probability":2},
        {"key":"bonus_128","label":"幸运加倍 +128","type":"reward","delta_points":128,"probability":6},
        {"key":"bonus_66","label":"小赚一笔 +66","type":"reward","delta_points":66,"probability":10},
        {"key":"bonus_18","label":"安慰奖励 +18","type":"reward","delta_points":18,"probability":14},
        {"key":"thanks","label":"谢谢惠顾","type":"thanks","delta_points":0,"probability":25},
        {"key":"penalty_18","label":"手滑一下 -18","type":"penalty","delta_points":-18,"probability":18},
        {"key":"penalty_66","label":"运气欠佳 -66","type":"penalty","delta_points":-66,"probability":14},
        {"key":"penalty_128","label":"倒霉暴击 -128","type":"penalty","delta_points":-128,"probability":10.5}
    ]$$),
    ('lucky_wheel_rules_markdown', $$1. 每位用户每天默认可转动 5 次，次数由后台可配置。
2. 转盘结果按后台配置的概率随机抽取，奖励、惩罚和“谢谢惠顾”都会直接结算到游戏中心积分。
3. 若当前积分低于奖池中的最大惩罚值，则无法参与本次转盘，以避免积分被扣成负数。
4. 概率、奖池与每日次数均以管理员配置为准，最终结果以系统记录为准。$$)
ON CONFLICT (key) DO NOTHING;

INSERT INTO game_catalogs (
    game_key,
    name,
    subtitle,
    description,
    sort_order,
    default_open_mode
) VALUES (
    'lucky_wheel',
    '大转盘',
    '转动转盘赢取额度奖励，试试你的运气赢取大奖',
    '每日限次的幸运转盘，可能暴击大奖，也可能触发扣分惩罚。',
    10,
    'dual'
)
ON CONFLICT (game_key) DO NOTHING;
