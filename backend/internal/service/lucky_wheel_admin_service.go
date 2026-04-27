package service

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	LuckyWheelGameKey            = "lucky_wheel"
	defaultLuckyWheelEnabled     = true
	defaultLuckyWheelDailySpins  = 5
	defaultLuckyWheelMaxDailyCap = 20
)

var (
	ErrLuckyWheelInvalidDailySpinLimit = infraerrors.BadRequest(
		"LUCKY_WHEEL_INVALID_DAILY_SPIN_LIMIT",
		"daily spin limit must be between 1 and 20",
	)
	ErrLuckyWheelInvalidPrizeConfig = infraerrors.BadRequest(
		"LUCKY_WHEEL_INVALID_PRIZE_CONFIG",
		"prize config must contain reward, penalty, thanks and sum to 100",
	)
	ErrLuckyWheelRulesMarkdownRequired = infraerrors.BadRequest(
		"LUCKY_WHEEL_RULES_MARKDOWN_REQUIRED",
		"rules markdown is required",
	)
)

type LuckyWheelPrizeType string

const (
	LuckyWheelPrizeReward  LuckyWheelPrizeType = "reward"
	LuckyWheelPrizePenalty LuckyWheelPrizeType = "penalty"
	LuckyWheelPrizeThanks  LuckyWheelPrizeType = "thanks"
)

type LuckyWheelPrizeConfig struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Type        LuckyWheelPrizeType `json:"type"`
	DeltaPoints int64               `json:"delta_points"`
	Probability float64             `json:"probability"`
}

type LuckyWheelSettings struct {
	Enabled        bool                    `json:"enabled"`
	DailySpinLimit int                     `json:"daily_spin_limit"`
	Prizes         []LuckyWheelPrizeConfig `json:"prizes"`
	RulesMarkdown  string                  `json:"rules_markdown"`
}

type UpdateLuckyWheelSettingsRequest = LuckyWheelSettings

type LuckyWheelAdminService struct {
	settingRepo SettingRepository
}

func NewLuckyWheelAdminService(settingRepo SettingRepository) *LuckyWheelAdminService {
	return &LuckyWheelAdminService{settingRepo: settingRepo}
}

func (s *LuckyWheelAdminService) GetSettings(ctx context.Context) (*LuckyWheelSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyLuckyWheelEnabled,
		SettingKeyLuckyWheelDailySpinLimit,
		SettingKeyLuckyWheelPrizes,
		SettingKeyLuckyWheelRulesMarkdown,
	})
	if err != nil {
		return nil, err
	}
	return parseLuckyWheelSettings(values), nil
}

func (s *LuckyWheelAdminService) UpdateSettings(ctx context.Context, req UpdateLuckyWheelSettingsRequest) error {
	if err := validateLuckyWheelSettings(req); err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyLuckyWheelEnabled:        strconv.FormatBool(req.Enabled),
		SettingKeyLuckyWheelDailySpinLimit: strconv.Itoa(req.DailySpinLimit),
		SettingKeyLuckyWheelPrizes:         mustJSON(req.Prizes),
		SettingKeyLuckyWheelRulesMarkdown:  req.RulesMarkdown,
	})
}

func parseLuckyWheelSettings(values map[string]string) *LuckyWheelSettings {
	settings := &LuckyWheelSettings{
		Enabled:        defaultLuckyWheelEnabled,
		DailySpinLimit: defaultLuckyWheelDailySpins,
		Prizes:         defaultLuckyWheelPrizes(),
		RulesMarkdown:  defaultLuckyWheelRulesMarkdown,
	}

	if raw := strings.TrimSpace(values[SettingKeyLuckyWheelEnabled]); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			settings.Enabled = enabled
		}
	}
	if raw := strings.TrimSpace(values[SettingKeyLuckyWheelDailySpinLimit]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= defaultLuckyWheelMaxDailyCap {
			settings.DailySpinLimit = parsed
		}
	}
	if prizes, ok := parseLuckyWheelPrizes(values[SettingKeyLuckyWheelPrizes]); ok {
		settings.Prizes = prizes
	}
	if raw := strings.TrimSpace(values[SettingKeyLuckyWheelRulesMarkdown]); raw != "" {
		settings.RulesMarkdown = raw
	}
	return settings
}

func parseLuckyWheelPrizes(raw string) ([]LuckyWheelPrizeConfig, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	var prizes []LuckyWheelPrizeConfig
	if err := json.Unmarshal([]byte(raw), &prizes); err != nil || !isValidLuckyWheelPrizes(prizes) {
		return nil, false
	}
	return cloneLuckyWheelPrizes(prizes), true
}

func validateLuckyWheelSettings(req UpdateLuckyWheelSettingsRequest) error {
	if req.DailySpinLimit < 1 || req.DailySpinLimit > defaultLuckyWheelMaxDailyCap {
		return ErrLuckyWheelInvalidDailySpinLimit
	}
	if !isValidLuckyWheelPrizes(req.Prizes) {
		return ErrLuckyWheelInvalidPrizeConfig
	}
	if strings.TrimSpace(req.RulesMarkdown) == "" {
		return ErrLuckyWheelRulesMarkdownRequired
	}
	return nil
}

func isValidLuckyWheelPrizes(prizes []LuckyWheelPrizeConfig) bool {
	if len(prizes) < 4 {
		return false
	}

	var (
		sum        float64
		hasReward  bool
		hasPenalty bool
		hasThanks  bool
	)
	seen := make(map[string]struct{}, len(prizes))
	for _, prize := range prizes {
		key := strings.TrimSpace(prize.Key)
		label := strings.TrimSpace(prize.Label)
		if key == "" || label == "" {
			return false
		}
		if _, ok := seen[key]; ok {
			return false
		}
		seen[key] = struct{}{}
		if prize.Probability <= 0 {
			return false
		}
		sum += prize.Probability
		switch prize.Type {
		case LuckyWheelPrizeReward:
			if prize.DeltaPoints <= 0 {
				return false
			}
			hasReward = true
		case LuckyWheelPrizePenalty:
			if prize.DeltaPoints >= 0 {
				return false
			}
			hasPenalty = true
		case LuckyWheelPrizeThanks:
			if prize.DeltaPoints != 0 {
				return false
			}
			hasThanks = true
		default:
			return false
		}
	}
	return hasReward && hasPenalty && hasThanks && math.Abs(sum-100) <= 0.0001
}

func cloneLuckyWheelPrizes(prizes []LuckyWheelPrizeConfig) []LuckyWheelPrizeConfig {
	items := make([]LuckyWheelPrizeConfig, len(prizes))
	copy(items, prizes)
	return items
}

func defaultLuckyWheelPrizes() []LuckyWheelPrizeConfig {
	return []LuckyWheelPrizeConfig{
		{Key: "jackpot_888", Label: "超级大奖 +888", Type: LuckyWheelPrizeReward, DeltaPoints: 888, Probability: 0.5},
		{Key: "bonus_288", Label: "好运爆发 +288", Type: LuckyWheelPrizeReward, DeltaPoints: 288, Probability: 2},
		{Key: "bonus_128", Label: "幸运加倍 +128", Type: LuckyWheelPrizeReward, DeltaPoints: 128, Probability: 6},
		{Key: "bonus_66", Label: "小赚一笔 +66", Type: LuckyWheelPrizeReward, DeltaPoints: 66, Probability: 10},
		{Key: "bonus_18", Label: "安慰奖励 +18", Type: LuckyWheelPrizeReward, DeltaPoints: 18, Probability: 14},
		{Key: "thanks", Label: "谢谢惠顾", Type: LuckyWheelPrizeThanks, DeltaPoints: 0, Probability: 25},
		{Key: "penalty_18", Label: "手滑一下 -18", Type: LuckyWheelPrizePenalty, DeltaPoints: -18, Probability: 18},
		{Key: "penalty_66", Label: "运气欠佳 -66", Type: LuckyWheelPrizePenalty, DeltaPoints: -66, Probability: 14},
		{Key: "penalty_128", Label: "倒霉暴击 -128", Type: LuckyWheelPrizePenalty, DeltaPoints: -128, Probability: 10.5},
	}
}

const defaultLuckyWheelRulesMarkdown = `1. 每位用户每天默认可转动 5 次，次数由后台可配置。
2. 转盘结果按后台配置的概率随机抽取，奖励、惩罚和“谢谢惠顾”都会直接结算到游戏中心积分。
3. 若当前积分低于奖池中的最大惩罚值，则无法参与本次转盘，以避免积分被扣成负数。
4. 概率、奖池与每日次数均以管理员配置为准，最终结果以系统记录为准。`
