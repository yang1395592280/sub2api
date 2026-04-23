package service

import "time"

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

type SizeBetPhase string

const (
	SizeBetPhaseBetting     SizeBetPhase = "betting"
	SizeBetPhaseClosed      SizeBetPhase = "closed"
	SizeBetPhaseMaintenance SizeBetPhase = "maintenance"
)

type SizeBetCurrentRound struct {
	ID                  int64              `json:"id"`
	RoundNo             int64              `json:"round_no"`
	Status              SizeBetRoundStatus `json:"status"`
	StartsAt            time.Time          `json:"starts_at"`
	BetClosesAt         time.Time          `json:"bet_closes_at"`
	SettlesAt           time.Time          `json:"settles_at"`
	ProbSmall           float64            `json:"prob_small"`
	ProbMid             float64            `json:"prob_mid"`
	ProbBig             float64            `json:"prob_big"`
	OddsSmall           float64            `json:"odds_small"`
	OddsMid             float64            `json:"odds_mid"`
	OddsBig             float64            `json:"odds_big"`
	AllowedStakes       []int              `json:"allowed_stakes"`
	ServerSeedHash      string             `json:"server_seed_hash"`
	CountdownSeconds    int                `json:"countdown_seconds"`
	BetCountdownSeconds int                `json:"bet_countdown_seconds"`
}

type SizeBetCurrentRoundView struct {
	Enabled       bool                 `json:"enabled"`
	Phase         SizeBetPhase         `json:"phase"`
	ServerTime    time.Time            `json:"server_time"`
	Round         *SizeBetCurrentRound `json:"round,omitempty"`
	MyBet         *SizeBet             `json:"my_bet,omitempty"`
	PreviousRound *SizeBetRound        `json:"previous_round,omitempty"`
}

type SizeBetUserHistoryItem struct {
	BetID           int64            `json:"bet_id"`
	RoundID         int64            `json:"round_id"`
	RoundNo         int64            `json:"round_no"`
	Direction       SizeBetDirection `json:"direction"`
	StakeAmount     float64          `json:"stake_amount"`
	PayoutAmount    float64          `json:"payout_amount"`
	NetResultAmount float64          `json:"net_result_amount"`
	Status          SizeBetStatus    `json:"status"`
	IdempotencyKey  string           `json:"idempotency_key"`
	PlacedAt        time.Time        `json:"placed_at"`
	SettledAt       *time.Time       `json:"settled_at,omitempty"`
	ResultNumber    *int             `json:"result_number,omitempty"`
	ResultDirection SizeBetDirection `json:"result_direction,omitempty"`
	RoundStartsAt   time.Time        `json:"round_starts_at"`
	RoundSettlesAt  time.Time        `json:"round_settles_at"`
}

type SizeBetLeaderboardEntry struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	Username  string  `json:"username"`
	NetProfit float64 `json:"net_profit"`
	WinCount  int64   `json:"win_count"`
	BetCount  int64   `json:"bet_count"`
	HitRate   float64 `json:"hit_rate"`
}

type SizeBetLeaderboardView struct {
	Scope       string                    `json:"scope"`
	ScopeKey    string                    `json:"scope_key"`
	RefreshedAt *time.Time                `json:"refreshed_at,omitempty"`
	Items       []SizeBetLeaderboardEntry `json:"items"`
}

type SizeBetRulesView struct {
	Enabled               bool                     `json:"enabled"`
	RoundDurationSeconds  int                      `json:"round_duration_seconds"`
	BetCloseOffsetSeconds int                      `json:"bet_close_offset_seconds"`
	AllowedStakes         []int                    `json:"allowed_stakes"`
	Probabilities         SizeBetProbabilityConfig `json:"probabilities"`
	Odds                  SizeBetOddsConfig        `json:"odds"`
	RulesMarkdown         string                   `json:"rules_markdown"`
}

type SizeBetAdminBet struct {
	ID              int64            `json:"id"`
	RoundID         int64            `json:"round_id"`
	RoundNo         int64            `json:"round_no"`
	UserID          int64            `json:"user_id"`
	Username        string           `json:"username"`
	Direction       SizeBetDirection `json:"direction"`
	StakeAmount     float64          `json:"stake_amount"`
	PayoutAmount    float64          `json:"payout_amount"`
	NetResultAmount float64          `json:"net_result_amount"`
	Status          SizeBetStatus    `json:"status"`
	IdempotencyKey  string           `json:"idempotency_key"`
	PlacedAt        time.Time        `json:"placed_at"`
	SettledAt       *time.Time       `json:"settled_at,omitempty"`
}

type SizeBetAdminBetFilter struct {
	RoundID *int64 `json:"round_id,omitempty"`
	UserID  *int64 `json:"user_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

type SizeBetAdminLedgerFilter struct {
	RoundID   *int64 `json:"round_id,omitempty"`
	UserID    *int64 `json:"user_id,omitempty"`
	EntryType string `json:"entry_type,omitempty"`
}

type SizeBetRefundResult struct {
	RoundID       int64     `json:"round_id"`
	RefundedCount int       `json:"refunded_count"`
	RefundedAt    time.Time `json:"refunded_at"`
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
