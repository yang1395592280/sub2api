INSERT INTO settings (key, value, updated_at)
VALUES
    ('game_center_enabled', 'true', NOW()),
    ('checkin_enabled', 'true', NOW()),
    ('checkin_min_reward', '2', NOW()),
    ('checkin_max_reward', '20', NOW()),
    ('checkin_distribution_enabled', 'false', NOW()),
    ('checkin_distribution_config', '[]', NOW()),
    ('checkin_lucky_bonus_enabled', 'true', NOW()),
    ('checkin_lucky_bonus_success_rate', '50', NOW()),
    ('lucky_wheel_enabled', 'true', NOW()),
    ('lucky_wheel_daily_spin_limit', '5', NOW()),
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
    ]$$, NOW()),
    ('lucky_wheel_rules_markdown', $$1. 每位用户每天默认可转动 5 次，次数由后台可配置。
2. 转盘结果按后台配置的概率随机抽取，奖励、惩罚和“谢谢惠顾”都会直接结算到游戏积分。
3. 若当前积分低于奖池中的最大惩罚值，则无法参与本次转盘，避免积分扣成负数。
4. 概率、奖池与每日次数均以管理员配置为准，最终结果以系统记录为准。$$, NOW()),
    ('size_bet_enabled', 'true', NOW()),
    ('size_bet_round_duration_seconds', '60', NOW()),
    ('size_bet_bet_close_offset_seconds', '50', NOW()),
    ('size_bet_allowed_stakes', '[2,5,10,20]', NOW()),
    ('size_bet_custom_stake_min', '1', NOW()),
    ('size_bet_custom_stake_max', '9999', NOW()),
    ('size_bet_prob_small', '45', NOW()),
    ('size_bet_prob_mid', '10', NOW()),
    ('size_bet_prob_big', '45', NOW()),
    ('size_bet_odds_small', '2', NOW()),
    ('size_bet_odds_mid', '10', NOW()),
    ('size_bet_odds_big', '2', NOW()),
    ('size_bet_rules_markdown', $$## 大小中竞猜规则

- 每局持续 60 秒，前 50 秒可下注，后 10 秒封盘等待开奖。
- 开奖数字范围为 1 到 11：1-5 为小，6 为中，7-11 为大。
- 每个账号每期只能下注 1 次，且每次只能选择一个方向。
- 默认可选下注积分为 2、5、10、20。
- 默认赔率为：小 2 倍、中 10 倍、大 2 倍。
- 若系统异常导致本期作废，平台将按规则原路退回积分。$$, NOW())
ON CONFLICT (key) DO NOTHING;
