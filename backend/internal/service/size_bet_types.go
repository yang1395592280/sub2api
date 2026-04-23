package service

const (
	defaultSizeBetEnabled               = true
	defaultSizeBetRoundDurationSeconds  = 60
	defaultSizeBetBetCloseOffsetSeconds = 50
)

const defaultSizeBetRulesMarkdown = `## 大小中竞猜规则

- 每局持续 60 秒，前 50 秒可下注，后 10 秒封盘等待开奖。
- 开奖数字范围为 1 到 11：1-5 为小，6 为中，7-11 为大。
- 每个账号每期只能下注 1 次，且每次只能选择一个方向。
- 默认可选下注额度为 2、5、10、20。
- 默认赔率为：小 2.0x、中 10.0x、大 2.0x。
- 若系统异常导致本期作废，平台将按规则退款。`

type SizeBetProbabilityConfig struct {
	Small float64 `json:"small"`
	Mid   float64 `json:"mid"`
	Big   float64 `json:"big"`
}

type SizeBetOddsConfig struct {
	Small float64 `json:"small"`
	Mid   float64 `json:"mid"`
	Big   float64 `json:"big"`
}

type UpdateSizeBetSettingsRequest struct {
	Enabled               bool                     `json:"enabled"`
	RoundDurationSeconds  int                      `json:"round_duration_seconds"`
	BetCloseOffsetSeconds int                      `json:"bet_close_offset_seconds"`
	AllowedStakes         []int                    `json:"allowed_stakes"`
	Probabilities         SizeBetProbabilityConfig `json:"probabilities"`
	Odds                  SizeBetOddsConfig        `json:"odds"`
	RulesMarkdown         string                   `json:"rules_markdown"`
}

type SizeBetSettings struct {
	Enabled               bool    `json:"enabled"`
	RoundDurationSeconds  int     `json:"round_duration_seconds"`
	BetCloseOffsetSeconds int     `json:"bet_close_offset_seconds"`
	AllowedStakes         []int   `json:"allowed_stakes"`
	ProbSmall             float64 `json:"prob_small"`
	ProbMid               float64 `json:"prob_mid"`
	ProbBig               float64 `json:"prob_big"`
	OddsSmall             float64 `json:"odds_small"`
	OddsMid               float64 `json:"odds_mid"`
	OddsBig               float64 `json:"odds_big"`
	RulesMarkdown         string  `json:"rules_markdown"`
}

func defaultSizeBetAllowedStakes() []int {
	return []int{2, 5, 10, 20}
}

func defaultSizeBetProbabilities() SizeBetProbabilityConfig {
	return SizeBetProbabilityConfig{
		Small: 45,
		Mid:   10,
		Big:   45,
	}
}

func defaultSizeBetOdds() SizeBetOddsConfig {
	return SizeBetOddsConfig{
		Small: 2,
		Mid:   10,
		Big:   2,
	}
}
