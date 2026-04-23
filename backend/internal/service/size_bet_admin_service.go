package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSizeBetInvalidRoundDuration = infraerrors.BadRequest(
		"SIZE_BET_INVALID_ROUND_DURATION",
		"round duration must be greater than 0",
	)
	ErrSizeBetInvalidBetCloseOffset = infraerrors.BadRequest(
		"SIZE_BET_INVALID_BET_CLOSE_OFFSET",
		"bet close offset must be between 0 and round duration",
	)
	ErrSizeBetInvalidAllowedStakes = infraerrors.BadRequest(
		"SIZE_BET_INVALID_ALLOWED_STAKES",
		"allowed stakes must contain unique positive integers",
	)
	ErrSizeBetInvalidProbabilities = infraerrors.BadRequest(
		"SIZE_BET_INVALID_PROBABILITIES",
		"probabilities must be non-negative and sum to 100",
	)
	ErrSizeBetInvalidOdds = infraerrors.BadRequest(
		"SIZE_BET_INVALID_ODDS",
		"odds must be greater than 0",
	)
	ErrSizeBetRulesMarkdownRequired = infraerrors.BadRequest(
		"SIZE_BET_RULES_MARKDOWN_REQUIRED",
		"rules markdown is required",
	)
)

type SizeBetAdminService struct {
	settingRepo SettingRepository
}

func NewSizeBetAdminService(settingRepo SettingRepository) *SizeBetAdminService {
	return &SizeBetAdminService{settingRepo: settingRepo}
}

func (s *SizeBetAdminService) GetSettings(ctx context.Context) (*SizeBetSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeySizeBetEnabled,
		SettingKeySizeBetRoundDurationSeconds,
		SettingKeySizeBetBetCloseOffsetSeconds,
		SettingKeySizeBetAllowedStakes,
		SettingKeySizeBetProbabilities,
		SettingKeySizeBetOdds,
		SettingKeySizeBetRulesMarkdown,
	})
	if err != nil {
		return nil, err
	}
	return parseSizeBetSettings(values), nil
}

func (s *SizeBetAdminService) UpdateSettings(ctx context.Context, req UpdateSizeBetSettingsRequest) error {
	if err := validateSizeBetSettings(req); err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeySizeBetEnabled:               strconv.FormatBool(req.Enabled),
		SettingKeySizeBetRoundDurationSeconds:  strconv.Itoa(req.RoundDurationSeconds),
		SettingKeySizeBetBetCloseOffsetSeconds: strconv.Itoa(req.BetCloseOffsetSeconds),
		SettingKeySizeBetAllowedStakes:         mustJSON(req.AllowedStakes),
		SettingKeySizeBetProbabilities:         mustJSON(req.Probabilities),
		SettingKeySizeBetOdds:                  mustJSON(req.Odds),
		SettingKeySizeBetRulesMarkdown:         req.RulesMarkdown,
	})
}

func parseSizeBetSettings(values map[string]string) *SizeBetSettings {
	defaultProbabilities := defaultSizeBetProbabilities()
	defaultOdds := defaultSizeBetOdds()

	settings := &SizeBetSettings{
		Enabled:               defaultSizeBetEnabled,
		RoundDurationSeconds:  defaultSizeBetRoundDurationSeconds,
		BetCloseOffsetSeconds: defaultSizeBetBetCloseOffsetSeconds,
		AllowedStakes:         defaultSizeBetAllowedStakes(),
		ProbSmall:             defaultProbabilities.Small,
		ProbMid:               defaultProbabilities.Mid,
		ProbBig:               defaultProbabilities.Big,
		OddsSmall:             defaultOdds.Small,
		OddsMid:               defaultOdds.Mid,
		OddsBig:               defaultOdds.Big,
		RulesMarkdown:         defaultSizeBetRulesMarkdown,
	}

	if raw := strings.TrimSpace(values[SettingKeySizeBetEnabled]); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			settings.Enabled = enabled
		}
	}

	roundDuration := settings.RoundDurationSeconds
	if raw := strings.TrimSpace(values[SettingKeySizeBetRoundDurationSeconds]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			roundDuration = parsed
		}
	}

	betCloseOffset := defaultSizeBetCloseOffsetForRoundDuration(roundDuration)
	if raw := strings.TrimSpace(values[SettingKeySizeBetBetCloseOffsetSeconds]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed < roundDuration {
			betCloseOffset = parsed
		}
	}

	settings.RoundDurationSeconds = roundDuration
	settings.BetCloseOffsetSeconds = betCloseOffset
	settings.AllowedStakes = parseSizeBetAllowedStakes(values[SettingKeySizeBetAllowedStakes])

	probabilities, ok := parseSizeBetProbabilityConfig(values[SettingKeySizeBetProbabilities])
	if ok {
		settings.ProbSmall = probabilities.Small
		settings.ProbMid = probabilities.Mid
		settings.ProbBig = probabilities.Big
	}

	odds, ok := parseSizeBetOddsConfig(values[SettingKeySizeBetOdds])
	if ok {
		settings.OddsSmall = odds.Small
		settings.OddsMid = odds.Mid
		settings.OddsBig = odds.Big
	}

	if raw := values[SettingKeySizeBetRulesMarkdown]; strings.TrimSpace(raw) != "" {
		settings.RulesMarkdown = raw
	}

	return settings
}

func validateSizeBetSettings(req UpdateSizeBetSettingsRequest) error {
	if req.RoundDurationSeconds <= 0 {
		return ErrSizeBetInvalidRoundDuration
	}
	if req.BetCloseOffsetSeconds < 0 || req.BetCloseOffsetSeconds >= req.RoundDurationSeconds {
		return ErrSizeBetInvalidBetCloseOffset
	}
	if !isValidSizeBetAllowedStakes(req.AllowedStakes) {
		return ErrSizeBetInvalidAllowedStakes
	}
	if !isValidSizeBetProbabilities(req.Probabilities) {
		return ErrSizeBetInvalidProbabilities
	}
	if !isValidSizeBetOdds(req.Odds) {
		return ErrSizeBetInvalidOdds
	}
	if strings.TrimSpace(req.RulesMarkdown) == "" {
		return ErrSizeBetRulesMarkdownRequired
	}
	return nil
}

func parseSizeBetAllowedStakes(raw string) []int {
	if strings.TrimSpace(raw) == "" {
		return defaultSizeBetAllowedStakes()
	}

	var stakes []int
	if err := json.Unmarshal([]byte(raw), &stakes); err != nil || !isValidSizeBetAllowedStakes(stakes) {
		return defaultSizeBetAllowedStakes()
	}
	return append([]int(nil), stakes...)
}

func parseSizeBetProbabilityConfig(raw string) (SizeBetProbabilityConfig, bool) {
	if strings.TrimSpace(raw) == "" {
		return SizeBetProbabilityConfig{}, false
	}

	var cfg SizeBetProbabilityConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil || !isValidSizeBetProbabilities(cfg) {
		return SizeBetProbabilityConfig{}, false
	}
	return cfg, true
}

func parseSizeBetOddsConfig(raw string) (SizeBetOddsConfig, bool) {
	if strings.TrimSpace(raw) == "" {
		return SizeBetOddsConfig{}, false
	}

	var cfg SizeBetOddsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil || !isValidSizeBetOdds(cfg) {
		return SizeBetOddsConfig{}, false
	}
	return cfg, true
}

func isValidSizeBetAllowedStakes(stakes []int) bool {
	if len(stakes) == 0 {
		return false
	}

	seen := make(map[int]struct{}, len(stakes))
	for _, stake := range stakes {
		if stake <= 0 {
			return false
		}
		if _, ok := seen[stake]; ok {
			return false
		}
		seen[stake] = struct{}{}
	}
	return true
}

func isValidSizeBetProbabilities(cfg SizeBetProbabilityConfig) bool {
	if cfg.Small < 0 || cfg.Mid < 0 || cfg.Big < 0 {
		return false
	}
	sum := cfg.Small + cfg.Mid + cfg.Big
	return math.Abs(sum-100) <= 0.0001
}

func isValidSizeBetOdds(cfg SizeBetOddsConfig) bool {
	return cfg.Small > 0 && cfg.Mid > 0 && cfg.Big > 0
}

func defaultSizeBetCloseOffsetForRoundDuration(roundDuration int) int {
	if roundDuration <= 1 {
		return 0
	}
	if defaultSizeBetBetCloseOffsetSeconds >= roundDuration {
		return roundDuration - 1
	}
	return defaultSizeBetBetCloseOffsetSeconds
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal size bet settings: %v", err))
	}
	return string(data)
}
