package service

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeyCheckinEnabled               = "checkin_enabled"
	SettingKeyCheckinMinReward             = "checkin_min_reward"
	SettingKeyCheckinMaxReward             = "checkin_max_reward"
	SettingKeyCheckinDistributionEnabled   = "checkin_distribution_enabled"
	SettingKeyCheckinDistributionConfig    = "checkin_distribution_config"
	SettingKeyCheckinLuckyBonusEnabled     = "checkin_lucky_bonus_enabled"
	SettingKeyCheckinLuckyBonusSuccessRate = "checkin_lucky_bonus_success_rate"
)

type CheckinSettings struct {
	Enabled          bool                        `json:"enabled"`
	MinReward        float64                     `json:"min_reward"`
	MaxReward        float64                     `json:"max_reward"`
	BonusEnabled     bool                        `json:"bonus_enabled"`
	BonusSuccessRate float64                     `json:"bonus_success_rate"`
	Distribution     []CheckinDistributionBucket `json:"distribution"`
}

func (s *CheckinService) GetSettings(ctx context.Context) CheckinSettings {
	c := s.loadSettings(ctx)
	return CheckinSettings{Enabled: c.Enabled, MinReward: c.MinReward, MaxReward: c.MaxReward, BonusEnabled: c.LuckyBonusEnabled, BonusSuccessRate: c.LuckyBonusSuccessRate, Distribution: c.Distribution}
}

func (s *CheckinService) UpdateSettings(ctx context.Context, c CheckinSettings) error {
	for _, n := range []float64{c.MinReward, c.MaxReward, c.BonusSuccessRate} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return infraerrors.BadRequest("CHECKIN_INVALID_SETTINGS", "invalid check-in amount or probability")
		}
	}
	if c.MaxReward < c.MinReward || c.MaxReward > 10000 || c.BonusSuccessRate > 100 {
		return infraerrors.BadRequest("CHECKIN_INVALID_SETTINGS", "invalid reward range or success rate")
	}
	if len(c.Distribution) > 0 {
		if err := validateCheckinDistributionBuckets(c.Distribution); err != nil {
			return infraerrors.BadRequest("CHECKIN_INVALID_SETTINGS", err.Error())
		}
	}
	distribution, err := json.Marshal(c.Distribution)
	if err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyCheckinEnabled:               strconv.FormatBool(c.Enabled),
		SettingKeyCheckinMinReward:             strconv.FormatFloat(c.MinReward, 'f', 8, 64),
		SettingKeyCheckinMaxReward:             strconv.FormatFloat(c.MaxReward, 'f', 8, 64),
		SettingKeyCheckinLuckyBonusEnabled:     strconv.FormatBool(c.BonusEnabled),
		SettingKeyCheckinLuckyBonusSuccessRate: strconv.FormatFloat(c.BonusSuccessRate, 'f', 4, 64),
		SettingKeyCheckinDistributionEnabled:   strconv.FormatBool(len(c.Distribution) > 0),
		SettingKeyCheckinDistributionConfig:    string(distribution),
	})
}
