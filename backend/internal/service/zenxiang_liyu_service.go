package service

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	zenxiangLiyuProbabilityEpsilon       = 0.000001
	zenxiangLiyuFreePlayUsageThreshold   = 5.0
	zenxiangLiyuFreePlayThresholdEpsilon = 0.000001
	defaultZenxiangLiyuTicketThreshold   = 5.0
	defaultZenxiangLiyuDailyTicketLimit  = 3
	defaultZenxiangLiyuLuckyProbability  = 50.0
	ZenxiangLiyuTicketCapacity           = 5
	ZenxiangLiyuTicketRetentionDays      = 2
)

var (
	ErrZenxiangLiyuDisabled                = errors.New("zenxiang liyu is disabled")
	ErrZenxiangLiyuUnauthorized            = errors.New("zenxiang liyu unauthorized")
	ErrZenxiangLiyuInvalidSettings         = errors.New("zenxiang liyu invalid settings")
	ErrZenxiangLiyuInvalidProbabilityTotal = errors.New("zenxiang liyu invalid probability total")
	ErrZenxiangLiyuInsufficientBalance     = errors.New("zenxiang liyu insufficient balance")
	ErrZenxiangLiyuDailyLimitReached       = errors.New("zenxiang liyu daily limit reached")
	ErrZenxiangLiyuNoTicket                = errors.New("zenxiang liyu no ticket")
	ErrZenxiangLiyuRequestIDRequired       = errors.New("zenxiang liyu request id required")
	ErrZenxiangLiyuLuckyCoinDisabled       = errors.New("zenxiang liyu lucky coin disabled")
	ErrZenxiangLiyuLuckyCoinAlreadyPlayed  = errors.New("zenxiang liyu lucky coin already played")
	ErrZenxiangLiyuLuckyCoinUnavailable    = errors.New("zenxiang liyu lucky coin unavailable")
)

type ZenxiangLiyuSettings struct {
	GlobalEnabled        bool    `json:"global_enabled"`
	TicketAmount         float64 `json:"ticket_amount"`
	MinimumBalance       float64 `json:"minimum_balance"`
	DailyPlayLimit       int     `json:"daily_play_limit"`
	TicketUsageThreshold float64 `json:"ticket_usage_threshold"`
	DailyTicketLimit     int     `json:"daily_ticket_limit"`
	UnitSalePrice        float64 `json:"unit_sale_price"`
	UnitCostPrice        float64 `json:"unit_cost_price"`
	LuckyCoinEnabled     bool    `json:"lucky_coin_enabled"`
	LuckyCoinProbability float64 `json:"lucky_coin_double_probability"`
}

type ZenxiangLiyuSettingsUpdate struct {
	GlobalEnabled        bool    `json:"global_enabled"`
	TicketAmount         float64 `json:"ticket_amount"`
	MinimumBalance       float64 `json:"minimum_balance"`
	DailyPlayLimit       int     `json:"daily_play_limit"`
	TicketUsageThreshold float64 `json:"ticket_usage_threshold"`
	DailyTicketLimit     int     `json:"daily_ticket_limit"`
	UnitSalePrice        float64 `json:"unit_sale_price"`
	UnitCostPrice        float64 `json:"unit_cost_price"`
	LuckyCoinEnabled     bool    `json:"lucky_coin_enabled"`
	LuckyCoinProbability float64 `json:"lucky_coin_double_probability"`
}

func (u ZenxiangLiyuSettingsUpdate) Settings() ZenxiangLiyuSettings {
	settings := ZenxiangLiyuSettings(u)
	if settings.TicketUsageThreshold == 0 {
		settings.TicketUsageThreshold = defaultZenxiangLiyuTicketThreshold
	}
	if settings.DailyTicketLimit == 0 {
		settings.DailyTicketLimit = defaultZenxiangLiyuDailyTicketLimit
	}
	if settings.UnitSalePrice == 0 {
		settings.UnitSalePrice = 0.1
	}
	if settings.UnitCostPrice == 0 {
		settings.UnitCostPrice = 0.05
	}
	return settings
}

type ZenxiangLiyuPrize struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	RewardAmount float64 `json:"reward_amount"`
	Probability  float64 `json:"probability"`
	Enabled      bool    `json:"enabled"`
	SortOrder    int     `json:"sort_order"`
}

type ZenxiangLiyuPrizeUpdate struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	RewardAmount float64 `json:"reward_amount"`
	Probability  float64 `json:"probability"`
	Enabled      bool    `json:"enabled"`
	SortOrder    int     `json:"sort_order"`
}

func (u ZenxiangLiyuPrizeUpdate) Prize() ZenxiangLiyuPrize {
	return ZenxiangLiyuPrize(u)
}

type ZenxiangLiyuStatus struct {
	Visible                bool                `json:"visible"`
	CanPlay                bool                `json:"can_play"`
	Reason                 string              `json:"reason,omitempty"`
	Balance                float64             `json:"balance"`
	TicketAmount           float64             `json:"ticket_amount"`
	EffectiveTicketAmount  float64             `json:"effective_ticket_amount"`
	MinimumBalance         float64             `json:"minimum_balance"`
	DailyPlayLimit         int                 `json:"daily_play_limit"`
	TodayPlayCount         int                 `json:"today_play_count"`
	RemainingPlays         int                 `json:"remaining_plays"`
	TodayUsageAmount       float64             `json:"today_usage_amount"`
	FreePlayUsageThreshold float64             `json:"free_play_usage_threshold"`
	FreePlayAvailable      bool                `json:"free_play_available"`
	FreePlayUsed           bool                `json:"free_play_used"`
	TicketUsageThreshold   float64             `json:"ticket_usage_threshold"`
	DailyTicketLimit       int                 `json:"daily_ticket_limit"`
	TicketCapacity         int                 `json:"ticket_capacity"`
	TicketRetentionDays    int                 `json:"ticket_retention_days"`
	TicketsAvailable       int                 `json:"tickets_available"`
	LuckyCoinEnabled       bool                `json:"lucky_coin_enabled"`
	LuckyCoinProbability   float64             `json:"lucky_coin_double_probability"`
	TodayTicketsEarned     int                 `json:"today_tickets_earned"`
	TodayTicketsFromUsage  int                 `json:"today_tickets_from_usage"`
	TodayTicketsGranted    int                 `json:"today_tickets_granted"`
	TodayTicketsUsed       int                 `json:"today_tickets_used"`
	TodayTicketsAvailable  int                 `json:"today_tickets_available"`
	NextTicketUsageTarget  float64             `json:"next_ticket_usage_target"`
	NextTicketUsageMissing float64             `json:"next_ticket_usage_missing"`
	Prizes                 []ZenxiangLiyuPrize `json:"prizes"`
}

type ZenxiangLiyuPlayCommand struct {
	UserID    int64
	RequestID string
	PlayDate  time.Time
	Roll      float64
}

type ZenxiangLiyuPlayResult struct {
	ID                 int64     `json:"id"`
	Applied            bool      `json:"applied"`
	RequestID          string    `json:"request_id"`
	PrizeID            int64     `json:"prize_id"`
	PrizeName          string    `json:"prize_name"`
	RewardAmount       float64   `json:"reward_amount"`
	TicketAmount       float64   `json:"ticket_amount"`
	FreePlay           bool      `json:"free_play"`
	UserNetAmount      float64   `json:"user_net_amount"`
	BalanceBefore      float64   `json:"balance_before"`
	BalanceAfterTicket float64   `json:"balance_after_ticket"`
	BalanceAfterReward float64   `json:"balance_after_reward"`
	PlayedAt           time.Time `json:"played_at"`
	LuckyCoinAvailable bool      `json:"lucky_coin_available"`
	LuckyCoinPlayed    bool      `json:"lucky_coin_played"`
}

type ZenxiangLiyuLuckyCoinCommand struct {
	UserID   int64
	RecordID int64
	Roll     float64
}

type ZenxiangLiyuLuckyCoinResult struct {
	RecordID           int64     `json:"record_id"`
	Outcome            string    `json:"outcome"`
	OriginalReward     float64   `json:"original_reward"`
	AdjustmentAmount   float64   `json:"adjustment_amount"`
	BalanceAfter       float64   `json:"balance_after"`
	DoubleProbability  float64   `json:"double_probability"`
	PlayedAt           time.Time `json:"played_at"`
	LuckyCoinAvailable bool      `json:"lucky_coin_available"`
}

type ZenxiangLiyuSimulationRequest struct {
	UserCount      int                 `json:"user_count"`
	PlaysPerUser   int                 `json:"plays_per_user"`
	InitialBalance float64             `json:"initial_balance"`
	TicketAmount   float64             `json:"ticket_amount"`
	MinimumBalance float64             `json:"minimum_balance"`
	DailyPlayLimit int                 `json:"daily_play_limit"`
	Prizes         []ZenxiangLiyuPrize `json:"prizes"`
}

type ZenxiangLiyuSimulationPrizeHit struct {
	PrizeID    int64   `json:"prize_id"`
	PrizeName  string  `json:"prize_name"`
	HitCount   int     `json:"hit_count"`
	ActualRate float64 `json:"actual_rate"`
}

type ZenxiangLiyuSimulationResult struct {
	TotalPlays      int                              `json:"total_plays"`
	TotalRevenue    float64                          `json:"total_revenue"`
	TotalExpense    float64                          `json:"total_expense"`
	NetProfit       float64                          `json:"net_profit"`
	ProfitRate      float64                          `json:"profit_rate"`
	ProfitableUsers int                              `json:"profitable_users"`
	LosingUsers     int                              `json:"losing_users"`
	BreakEvenUsers  int                              `json:"break_even_users"`
	PrizeHits       []ZenxiangLiyuSimulationPrizeHit `json:"prize_hits"`
}

type ZenxiangLiyuRecommendationRequest struct {
	TargetProfitRate float64             `json:"target_profit_rate"`
	TicketAmount     float64             `json:"ticket_amount"`
	Prizes           []ZenxiangLiyuPrize `json:"prizes"`
}

type ZenxiangLiyuRecommendationPlan struct {
	Prizes           []ZenxiangLiyuPrize `json:"prizes"`
	ProbabilityTotal float64             `json:"probability_total"`
	TheoryExpense    float64             `json:"theory_expense"`
	TheoryProfit     float64             `json:"theory_profit"`
	TheoryProfitRate float64             `json:"theory_profit_rate"`
}

type ZenxiangLiyuRecommendationResult struct {
	TargetExpense float64                          `json:"target_expense"`
	Plans         []ZenxiangLiyuRecommendationPlan `json:"plans"`
}

type ZenxiangLiyuProfitPreviewRequest struct {
	ConsumptionAmount    float64             `json:"consumption_amount"`
	TicketUsageThreshold float64             `json:"ticket_usage_threshold"`
	DailyTicketLimit     int                 `json:"daily_ticket_limit"`
	UnitSalePrice        float64             `json:"unit_sale_price"`
	UnitCostPrice        float64             `json:"unit_cost_price"`
	Prizes               []ZenxiangLiyuPrize `json:"prizes"`
}

type ZenxiangLiyuProfitPreviewResult struct {
	ExpectedRewardPerTicket float64 `json:"expected_reward_per_ticket"`
	ExpectedTickets         int     `json:"expected_tickets"`
	ExpectedRewardTotal     float64 `json:"expected_reward_total"`
	GrossProfitBeforeReward float64 `json:"gross_profit_before_reward"`
	GrossProfitAfterReward  float64 `json:"gross_profit_after_reward"`
	GrossProfitRateBefore   float64 `json:"gross_profit_rate_before"`
	GrossProfitRateAfter    float64 `json:"gross_profit_rate_after"`
	RewardRate              float64 `json:"reward_rate"`
}

type ZenxiangLiyuRecord struct {
	ID                  int64     `json:"id"`
	RequestID           string    `json:"request_id"`
	TicketAmount        float64   `json:"ticket_amount"`
	RewardAmount        float64   `json:"reward_amount"`
	UserNetAmount       float64   `json:"user_net_amount"`
	LuckyCoinPlayed     bool      `json:"lucky_coin_played"`
	LuckyCoinOutcome    string    `json:"lucky_coin_outcome,omitempty"`
	LuckyCoinAdjustment float64   `json:"lucky_coin_adjustment"`
	BalanceAfterLucky   *float64  `json:"balance_after_lucky,omitempty"`
	PrizeID             *int64    `json:"prize_id,omitempty"`
	PrizeName           string    `json:"prize_name"`
	Probability         float64   `json:"probability"`
	PlayedAt            time.Time `json:"played_at"`
}

type ZenxiangLiyuDailySummary struct {
	PlayDate      time.Time `json:"play_date"`
	PlayCount     int       `json:"play_count"`
	TicketAmount  float64   `json:"ticket_amount"`
	RewardAmount  float64   `json:"reward_amount"`
	UserNetAmount float64   `json:"user_net_amount"`
}

type ZenxiangLiyuGrant struct {
	UserID    int64     `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Enabled   bool      `json:"enabled"`
	GrantedBy *int64    `json:"granted_by,omitempty"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ZenxiangLiyuTicketGift struct {
	ID          int64     `json:"id"`
	RequestID   string    `json:"request_id"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email,omitempty"`
	PlayDate    time.Time `json:"play_date"`
	TicketCount int       `json:"ticket_count"`
	GrantedBy   *int64    `json:"granted_by,omitempty"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ZenxiangLiyuTicketGiftRequest struct {
	RequestID   string `json:"request_id"`
	UserID      int64  `json:"user_id"`
	TicketCount int    `json:"ticket_count"`
	GrantedBy   *int64 `json:"granted_by,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type ZenxiangLiyuOverviewStats struct {
	TotalPlays         int     `json:"total_plays"`
	TotalRevenue       float64 `json:"total_revenue"`
	TotalExpense       float64 `json:"total_expense"`
	NetProfit          float64 `json:"net_profit"`
	ParticipatingUsers int     `json:"participating_users"`
}

type ZenxiangLiyuUserStats struct {
	UserID        int64   `json:"user_id"`
	UserEmail     string  `json:"user_email"`
	Balance       float64 `json:"balance"`
	UsageAmount   float64 `json:"usage_amount"`
	PlayCount     int     `json:"play_count"`
	TicketAmount  float64 `json:"ticket_amount"`
	RewardAmount  float64 `json:"reward_amount"`
	UserNetAmount float64 `json:"user_net_amount"`
}

type ZenxiangLiyuPrizeStats struct {
	PrizeID      *int64  `json:"prize_id,omitempty"`
	PrizeName    string  `json:"prize_name"`
	HitCount     int     `json:"hit_count"`
	RewardAmount float64 `json:"reward_amount"`
	Probability  float64 `json:"probability"`
}

type ZenxiangLiyuPeriodStats struct {
	PeriodStart       time.Time `json:"period_start"`
	PeriodLabel       string    `json:"period_label"`
	PlayCount         int       `json:"play_count"`
	ParticipantCount  int       `json:"participant_count"`
	UsageAmount       float64   `json:"usage_amount"`
	TicketsUsed       int       `json:"tickets_used"`
	TicketAmount      float64   `json:"ticket_amount"`
	RewardAmount      float64   `json:"reward_amount"`
	AverageReward     float64   `json:"average_reward"`
	UserNetAmount     float64   `json:"user_net_amount"`
	SystemRevenue     float64   `json:"system_revenue"`
	SystemExpense     float64   `json:"system_expense"`
	SystemProfit      float64   `json:"system_profit"`
	MostHitPrizeName  string    `json:"most_hit_prize_name,omitempty"`
	MostHitPrizeCount int       `json:"most_hit_prize_count"`
}

type ZenxiangLiyuResetDailyPlayRequest struct {
	UserID  int64  `json:"user_id"`
	ResetBy *int64 `json:"reset_by,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type ZenxiangLiyuResetDailyPlayResult struct {
	UserID             int64     `json:"user_id"`
	PlayDate           time.Time `json:"play_date"`
	PreviousPlayCount  int       `json:"previous_play_count"`
	EffectivePlayCount int       `json:"effective_play_count"`
	RemainingPlays     int       `json:"remaining_plays"`
}

// ZenxiangLiyuRepository isolates service policy from the storage and transaction implementation.
type ZenxiangLiyuRepository interface {
	GetSettings(ctx context.Context) (*ZenxiangLiyuSettings, error)
	UpdateSettings(ctx context.Context, settings ZenxiangLiyuSettings) (*ZenxiangLiyuSettings, error)
	ListPrizes(ctx context.Context) ([]ZenxiangLiyuPrize, error)
	SavePrize(ctx context.Context, prize ZenxiangLiyuPrize) (*ZenxiangLiyuPrize, error)
	SavePrizes(ctx context.Context, prizes []ZenxiangLiyuPrize) ([]ZenxiangLiyuPrize, error)
	DeletePrize(ctx context.Context, id int64) error
	IsUserGranted(ctx context.Context, userID int64) (bool, error)
	GetUserBalance(ctx context.Context, userID int64) (float64, error)
	CountUserPlaysOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error)
	GetUserUsageAmountOnDate(ctx context.Context, userID int64, playDate time.Time) (float64, error)
	HasUserFreePlayOnDate(ctx context.Context, userID int64, playDate time.Time) (bool, error)
	ListUserRecords(ctx context.Context, userID int64, playDate time.Time, page, pageSize int) ([]ZenxiangLiyuRecord, int, error)
	GetUserDailySummary(ctx context.Context, userID int64, playDate time.Time) (*ZenxiangLiyuDailySummary, error)
	ListGrants(ctx context.Context, page, pageSize int) ([]ZenxiangLiyuGrant, int, error)
	SaveGrant(ctx context.Context, grant ZenxiangLiyuGrant) (*ZenxiangLiyuGrant, error)
	DeleteGrant(ctx context.Context, userID int64) error
	CountGiftedTicketsOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error)
	SyncTicketBalance(ctx context.Context, userID int64, playDate time.Time, settings ZenxiangLiyuSettings) (int, error)
	GiftTickets(ctx context.Context, gift ZenxiangLiyuTicketGift) (*ZenxiangLiyuTicketGift, error)
	GetOverviewStats(ctx context.Context) (*ZenxiangLiyuOverviewStats, error)
	ListUserStats(ctx context.Context, page, pageSize int, playDate time.Time) ([]ZenxiangLiyuUserStats, int, error)
	ListPrizeStats(ctx context.Context) ([]ZenxiangLiyuPrizeStats, error)
	ListPeriodStats(ctx context.Context, period string) ([]ZenxiangLiyuPeriodStats, error)
	ResetUserDailyPlays(ctx context.Context, userID int64, playDate time.Time, resetBy *int64, notes string) (int, error)
	Play(ctx context.Context, cmd ZenxiangLiyuPlayCommand) (*ZenxiangLiyuPlayResult, error)
	PlayLuckyCoin(ctx context.Context, cmd ZenxiangLiyuLuckyCoinCommand) (*ZenxiangLiyuLuckyCoinResult, error)
}

func (s *ZenxiangLiyuService) ListUserRecords(ctx context.Context, userID int64, page, pageSize int) ([]ZenxiangLiyuRecord, int, error) {
	return s.ListUserRecordsByDate(ctx, userID, s.playDate(), page, pageSize)
}

func (s *ZenxiangLiyuService) ListUserRecordsByDate(ctx context.Context, userID int64, playDate time.Time, page, pageSize int) ([]ZenxiangLiyuRecord, int, error) {
	if userID <= 0 || playDate.IsZero() || s.repo == nil {
		return nil, 0, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.ListUserRecords(ctx, userID, playDate, normalizedZenxiangLiyuPage(page), normalizedZenxiangLiyuPageSize(pageSize))
}

func (s *ZenxiangLiyuService) GetUserDailySummary(ctx context.Context, userID int64) (*ZenxiangLiyuDailySummary, error) {
	if userID <= 0 || s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.GetUserDailySummary(ctx, userID, s.playDate())
}

func (s *ZenxiangLiyuService) ListGrants(ctx context.Context, page, pageSize int) ([]ZenxiangLiyuGrant, int, error) {
	if s.repo == nil {
		return nil, 0, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.ListGrants(ctx, normalizedZenxiangLiyuPage(page), normalizedZenxiangLiyuPageSize(pageSize))
}

func (s *ZenxiangLiyuService) SaveGrant(ctx context.Context, grant ZenxiangLiyuGrant) (*ZenxiangLiyuGrant, error) {
	if s.repo == nil || grant.UserID <= 0 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.SaveGrant(ctx, grant)
}

func (s *ZenxiangLiyuService) DeleteGrant(ctx context.Context, userID int64) error {
	if s.repo == nil || userID <= 0 {
		return ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.DeleteGrant(ctx, userID)
}

func (s *ZenxiangLiyuService) GiftTickets(ctx context.Context, req ZenxiangLiyuTicketGiftRequest) (*ZenxiangLiyuTicketGift, error) {
	requestID := strings.TrimSpace(req.RequestID)
	if s.repo == nil || requestID == "" || len(requestID) > 128 || req.UserID <= 0 || req.TicketCount <= 0 || req.TicketCount > 1000 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.GiftTickets(ctx, ZenxiangLiyuTicketGift{
		RequestID:   requestID,
		UserID:      req.UserID,
		PlayDate:    s.playDate(),
		TicketCount: req.TicketCount,
		GrantedBy:   req.GrantedBy,
		Notes:       strings.TrimSpace(req.Notes),
	})
}

func (s *ZenxiangLiyuService) GetOverviewStats(ctx context.Context) (*ZenxiangLiyuOverviewStats, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.GetOverviewStats(ctx)
}

func (s *ZenxiangLiyuService) ListUserStats(ctx context.Context, page, pageSize int, playDate time.Time) ([]ZenxiangLiyuUserStats, int, error) {
	if s.repo == nil {
		return nil, 0, ErrZenxiangLiyuInvalidSettings
	}
	if playDate.IsZero() {
		playDate = s.playDate()
	}
	return s.repo.ListUserStats(ctx, normalizedZenxiangLiyuPage(page), normalizedZenxiangLiyuPageSize(pageSize), playDate)
}

func (s *ZenxiangLiyuService) ListPrizeStats(ctx context.Context) ([]ZenxiangLiyuPrizeStats, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.ListPrizeStats(ctx)
}

// ApplySimulation persists only a simulated prize configuration. It never touches user balances or play records.
func (s *ZenxiangLiyuService) ApplySimulation(ctx context.Context, prizes []ZenxiangLiyuPrizeUpdate) ([]ZenxiangLiyuPrize, error) {
	return s.SavePrizes(ctx, prizes)
}

func normalizedZenxiangLiyuPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}
func normalizedZenxiangLiyuPageSize(pageSize int) int {
	if pageSize < 1 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

type ZenxiangLiyuService struct {
	repo  ZenxiangLiyuRepository
	clock func() time.Time
	rng   *rand.Rand
	rngMu sync.Mutex
}

func NewZenxiangLiyuService(repo ZenxiangLiyuRepository, clock func() time.Time, rng *rand.Rand) *ZenxiangLiyuService {
	if clock == nil {
		clock = time.Now
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(clock().UnixNano()))
	}
	return &ZenxiangLiyuService{repo: repo, clock: clock, rng: rng}
}

func ValidateZenxiangLiyuPrizes(prizes []ZenxiangLiyuPrize) error {
	total := 0.0
	enabled := 0
	for _, prize := range prizes {
		if strings.TrimSpace(prize.Name) == "" ||
			math.IsNaN(prize.RewardAmount) || math.IsInf(prize.RewardAmount, 0) || prize.RewardAmount < 0 ||
			math.IsNaN(prize.Probability) || math.IsInf(prize.Probability, 0) || prize.Probability < 0 || prize.Probability > 100 {
			return ErrZenxiangLiyuInvalidSettings
		}
		if prize.Enabled {
			enabled++
			total += prize.Probability
		}
	}
	if enabled == 0 || math.Abs(total-100) > zenxiangLiyuProbabilityEpsilon {
		return ErrZenxiangLiyuInvalidProbabilityTotal
	}
	return nil
}

func PickZenxiangLiyuPrize(prizes []ZenxiangLiyuPrize, roll float64) (*ZenxiangLiyuPrize, error) {
	if err := ValidateZenxiangLiyuPrizes(prizes); err != nil {
		return nil, err
	}
	cumulative := 0.0
	for i := range prizes {
		if !prizes[i].Enabled {
			continue
		}
		cumulative += prizes[i].Probability
		if roll < cumulative || math.Abs(cumulative-100) <= zenxiangLiyuProbabilityEpsilon {
			picked := prizes[i]
			return &picked, nil
		}
	}
	return nil, ErrZenxiangLiyuInvalidProbabilityTotal
}

func (s *ZenxiangLiyuService) GetStatus(ctx context.Context, userID int64) (*ZenxiangLiyuStatus, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	prizes, err := s.ListPrizes(ctx)
	if err != nil {
		return nil, err
	}
	status := &ZenxiangLiyuStatus{
		TicketAmount:           settings.TicketAmount,
		EffectiveTicketAmount:  0,
		MinimumBalance:         settings.MinimumBalance,
		DailyPlayLimit:         settings.DailyPlayLimit,
		FreePlayUsageThreshold: zenxiangLiyuFreePlayUsageThreshold,
		TicketUsageThreshold:   settings.EffectiveTicketUsageThreshold(),
		DailyTicketLimit:       settings.EffectiveDailyTicketLimit(),
		TicketCapacity:         ZenxiangLiyuTicketCapacity,
		TicketRetentionDays:    ZenxiangLiyuTicketRetentionDays,
		LuckyCoinEnabled:       settings.LuckyCoinEnabled,
		LuckyCoinProbability:   settings.EffectiveLuckyCoinProbability(),
		Prizes:                 prizes,
	}
	if !settings.GlobalEnabled {
		granted, err := s.repo.IsUserGranted(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !granted {
			status.Reason = ErrZenxiangLiyuUnauthorized.Error()
			return status, nil
		}
	}
	status.Visible = true
	status.Balance, err = s.repo.GetUserBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	playDate := s.playDate()
	status.TodayPlayCount, err = s.repo.CountUserPlaysOnDate(ctx, userID, playDate)
	if err != nil {
		return nil, err
	}
	status.TodayUsageAmount, err = s.repo.GetUserUsageAmountOnDate(ctx, userID, playDate)
	if err != nil {
		return nil, err
	}
	status.FreePlayUsed, err = s.repo.HasUserFreePlayOnDate(ctx, userID, playDate)
	if err != nil {
		return nil, err
	}
	status.FreePlayAvailable = IsZenxiangLiyuFreePlayAvailable(status.TodayUsageAmount, status.FreePlayUsed)
	status.TodayTicketsFromUsage = CalculateZenxiangLiyuEarnedTickets(status.TodayUsageAmount, settings)
	status.TodayTicketsGranted, err = s.repo.CountGiftedTicketsOnDate(ctx, userID, playDate)
	if err != nil {
		return nil, err
	}
	status.TodayTicketsEarned = status.TodayTicketsFromUsage + status.TodayTicketsGranted
	status.TodayTicketsUsed = status.TodayPlayCount
	status.TicketsAvailable, err = s.repo.SyncTicketBalance(ctx, userID, playDate, *settings)
	if err != nil {
		return nil, err
	}
	// Keep the legacy field populated for older clients while availability now comes from the persistent wallet.
	status.TodayTicketsAvailable = status.TicketsAvailable
	status.NextTicketUsageTarget, status.NextTicketUsageMissing = CalculateZenxiangLiyuNextTicketUsage(status.TodayUsageAmount, settings)
	status.RemainingPlays = max(0, settings.DailyPlayLimit-status.TodayPlayCount)
	status.CanPlay = status.TicketsAvailable > 0
	if !status.CanPlay {
		status.Reason = ErrZenxiangLiyuNoTicket.Error()
	}
	return status, nil
}

func (s *ZenxiangLiyuService) Play(ctx context.Context, userID int64, requestID string) (*ZenxiangLiyuPlayResult, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, ErrZenxiangLiyuRequestIDRequired
	}
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.Play(ctx, ZenxiangLiyuPlayCommand{UserID: userID, RequestID: requestID, PlayDate: s.playDate(), Roll: s.randomFloat64() * 100})
}

func (s *ZenxiangLiyuService) PlayLuckyCoin(ctx context.Context, userID, recordID int64) (*ZenxiangLiyuLuckyCoinResult, error) {
	if s.repo == nil || userID <= 0 || recordID <= 0 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.PlayLuckyCoin(ctx, ZenxiangLiyuLuckyCoinCommand{UserID: userID, RecordID: recordID, Roll: s.randomFloat64() * 100})
}

func (s *ZenxiangLiyuService) ListPeriodStats(ctx context.Context, period string) ([]ZenxiangLiyuPeriodStats, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	if period != "day" && period != "week" && period != "month" {
		period = "day"
	}
	return s.repo.ListPeriodStats(ctx, period)
}

func (s *ZenxiangLiyuService) ResetUserDailyPlays(ctx context.Context, req ZenxiangLiyuResetDailyPlayRequest) (*ZenxiangLiyuResetDailyPlayResult, error) {
	if req.UserID <= 0 || s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	playDate := s.playDate()
	previous, err := s.repo.ResetUserDailyPlays(ctx, req.UserID, playDate, req.ResetBy, req.Notes)
	if err != nil {
		return nil, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &ZenxiangLiyuResetDailyPlayResult{
		UserID:             req.UserID,
		PlayDate:           playDate,
		PreviousPlayCount:  previous,
		EffectivePlayCount: 0,
		RemainingPlays:     max(0, settings.DailyPlayLimit),
	}, nil
}

func (s *ZenxiangLiyuService) GetSettings(ctx context.Context) (*ZenxiangLiyuSettings, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.GetSettings(ctx)
}

func (s *ZenxiangLiyuService) UpdateSettings(ctx context.Context, req ZenxiangLiyuSettingsUpdate) (*ZenxiangLiyuSettings, error) {
	settings := req.Settings()
	if settings.TicketAmount < 0 || settings.MinimumBalance < 0 || settings.DailyPlayLimit <= 0 ||
		settings.TicketUsageThreshold <= 0 || settings.DailyTicketLimit <= 0 ||
		settings.UnitSalePrice < 0 || settings.UnitCostPrice < 0 ||
		settings.LuckyCoinProbability < 0 || settings.LuckyCoinProbability > 100 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.UpdateSettings(ctx, settings)
}

func (s *ZenxiangLiyuService) ListPrizes(ctx context.Context) ([]ZenxiangLiyuPrize, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.ListPrizes(ctx)
}

func (s *ZenxiangLiyuService) SavePrize(ctx context.Context, req ZenxiangLiyuPrizeUpdate) (*ZenxiangLiyuPrize, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	prizes, err := s.repo.ListPrizes(ctx)
	if err != nil {
		return nil, err
	}
	prize := req.Prize()
	found := false
	for i := range prizes {
		if prizes[i].ID == prize.ID && prize.ID != 0 {
			prizes[i] = prize
			found = true
			break
		}
	}
	if !found {
		prizes = append(prizes, prize)
	}
	if err := ValidateZenxiangLiyuPrizes(prizes); err != nil {
		return nil, err
	}
	return s.repo.SavePrize(ctx, prize)
}

// SavePrizes validates and atomically replaces the complete prize configuration.
func (s *ZenxiangLiyuService) SavePrizes(ctx context.Context, req []ZenxiangLiyuPrizeUpdate) ([]ZenxiangLiyuPrize, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	prizes := make([]ZenxiangLiyuPrize, len(req))
	for i := range req {
		prizes[i] = req[i].Prize()
	}
	if err := ValidateZenxiangLiyuPrizes(prizes); err != nil {
		return nil, err
	}
	return s.repo.SavePrizes(ctx, prizes)
}

func (s *ZenxiangLiyuService) DeletePrize(ctx context.Context, id int64) error {
	if id <= 0 || s.repo == nil {
		return ErrZenxiangLiyuInvalidSettings
	}
	prizes, err := s.repo.ListPrizes(ctx)
	if err != nil {
		return err
	}
	remaining := make([]ZenxiangLiyuPrize, 0, len(prizes))
	for _, prize := range prizes {
		if prize.ID != id {
			remaining = append(remaining, prize)
		}
	}
	if err := ValidateZenxiangLiyuPrizes(remaining); err != nil {
		return err
	}
	return s.repo.DeletePrize(ctx, id)
}

func (s *ZenxiangLiyuService) Simulate(_ context.Context, req ZenxiangLiyuSimulationRequest) (*ZenxiangLiyuSimulationResult, error) {
	if req.UserCount < 0 || req.PlaysPerUser < 0 || req.InitialBalance < 0 || req.TicketAmount <= 0 || req.MinimumBalance < 0 || req.DailyPlayLimit <= 0 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	if err := ValidateZenxiangLiyuPrizes(req.Prizes); err != nil {
		return nil, err
	}
	result := &ZenxiangLiyuSimulationResult{}
	hits := make(map[int64]*ZenxiangLiyuSimulationPrizeHit)
	playsPerUser := min(req.PlaysPerUser, req.DailyPlayLimit)
	for user := 0; user < req.UserCount; user++ {
		balance := req.InitialBalance
		userNet := 0.0
		for play := 0; play < playsPerUser && balance > req.MinimumBalance; play++ {
			balance -= req.TicketAmount
			prize, err := PickZenxiangLiyuPrize(req.Prizes, s.randomFloat64()*100)
			if err != nil {
				return nil, err
			}
			balance += prize.RewardAmount
			userNet += prize.RewardAmount - req.TicketAmount
			result.TotalPlays++
			result.TotalRevenue += req.TicketAmount
			result.TotalExpense += prize.RewardAmount
			hit := hits[prize.ID]
			if hit == nil {
				hit = &ZenxiangLiyuSimulationPrizeHit{PrizeID: prize.ID, PrizeName: prize.Name}
				hits[prize.ID] = hit
			}
			hit.HitCount++
		}
		switch {
		case userNet > 0:
			result.ProfitableUsers++
		case userNet < 0:
			result.LosingUsers++
		default:
			result.BreakEvenUsers++
		}
	}
	result.NetProfit = result.TotalRevenue - result.TotalExpense
	if result.TotalRevenue > 0 {
		result.ProfitRate = result.NetProfit / result.TotalRevenue
	}
	result.PrizeHits = make([]ZenxiangLiyuSimulationPrizeHit, 0, len(hits))
	for _, hit := range hits {
		if result.TotalPlays > 0 {
			hit.ActualRate = float64(hit.HitCount) * 100 / float64(result.TotalPlays)
		}
		result.PrizeHits = append(result.PrizeHits, *hit)
	}
	sort.Slice(result.PrizeHits, func(i, j int) bool { return result.PrizeHits[i].PrizeID < result.PrizeHits[j].PrizeID })
	return result, nil
}

func (s *ZenxiangLiyuService) Recommend(_ context.Context, req ZenxiangLiyuRecommendationRequest) (*ZenxiangLiyuRecommendationResult, error) {
	if req.TicketAmount <= 0 || len(req.Prizes) == 0 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	enabled := make([]ZenxiangLiyuPrize, 0, len(req.Prizes))
	for _, prize := range req.Prizes {
		if strings.TrimSpace(prize.Name) == "" || prize.RewardAmount < 0 {
			return nil, ErrZenxiangLiyuInvalidSettings
		}
		if prize.Enabled {
			enabled = append(enabled, prize)
		}
	}
	if len(enabled) < 2 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	sort.SliceStable(enabled, func(i, j int) bool { return enabled[i].RewardAmount < enabled[j].RewardAmount })
	targetExpense := req.TicketAmount * (1 - req.TargetProfitRate)
	probabilities := make([]float64, len(enabled))
	lower, higher := -1, -1
	for i := range enabled {
		if enabled[i].RewardAmount <= targetExpense {
			lower = i
		}
		if higher == -1 && enabled[i].RewardAmount >= targetExpense {
			higher = i
		}
	}
	switch {
	case lower == -1:
		probabilities[0], probabilities[1] = 99, 1
	case higher == -1:
		probabilities[len(enabled)-1], probabilities[len(enabled)-2] = 99, 1
	case math.Abs(enabled[higher].RewardAmount-targetExpense) <= zenxiangLiyuProbabilityEpsilon:
		probabilities[higher] = 100
	case lower == higher:
		probabilities[lower] = 100
	default:
		lowerReward, higherReward := enabled[lower].RewardAmount, enabled[higher].RewardAmount
		probabilities[lower] = (higherReward - targetExpense) * 100 / (higherReward - lowerReward)
		probabilities[higher] = 100 - probabilities[lower]
	}
	planPrizes := make([]ZenxiangLiyuPrize, len(enabled))
	copy(planPrizes, enabled)
	theoryExpense := 0.0
	for i := range planPrizes {
		planPrizes[i].Probability = probabilities[i]
		theoryExpense += planPrizes[i].RewardAmount * probabilities[i] / 100
	}
	plan := ZenxiangLiyuRecommendationPlan{Prizes: planPrizes, ProbabilityTotal: 100, TheoryExpense: theoryExpense, TheoryProfit: req.TicketAmount - theoryExpense}
	plan.TheoryProfitRate = plan.TheoryProfit / req.TicketAmount
	return &ZenxiangLiyuRecommendationResult{TargetExpense: targetExpense, Plans: []ZenxiangLiyuRecommendationPlan{plan}}, nil
}

func (s *ZenxiangLiyuService) PreviewProfit(_ context.Context, req ZenxiangLiyuProfitPreviewRequest) (*ZenxiangLiyuProfitPreviewResult, error) {
	if req.ConsumptionAmount < 0 || req.TicketUsageThreshold <= 0 || req.DailyTicketLimit <= 0 ||
		req.UnitSalePrice < 0 || req.UnitCostPrice < 0 {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	if err := ValidateZenxiangLiyuPrizes(req.Prizes); err != nil {
		return nil, err
	}
	expectedRewardPerTicket := ExpectedZenxiangLiyuReward(req.Prizes)
	tickets := int(math.Floor(req.ConsumptionAmount / req.TicketUsageThreshold))
	if tickets > req.DailyTicketLimit {
		tickets = req.DailyTicketLimit
	}
	if tickets < 0 {
		tickets = 0
	}
	revenue := req.ConsumptionAmount
	cost := revenue
	if req.UnitSalePrice > 0 {
		cost = revenue * req.UnitCostPrice / req.UnitSalePrice
	}
	before := revenue - cost
	rewardTotal := expectedRewardPerTicket * float64(tickets)
	after := before - rewardTotal
	result := &ZenxiangLiyuProfitPreviewResult{
		ExpectedRewardPerTicket: expectedRewardPerTicket,
		ExpectedTickets:         tickets,
		ExpectedRewardTotal:     rewardTotal,
		GrossProfitBeforeReward: before,
		GrossProfitAfterReward:  after,
	}
	if revenue > 0 {
		result.GrossProfitRateBefore = before / revenue
		result.GrossProfitRateAfter = after / revenue
		result.RewardRate = rewardTotal / revenue
	}
	return result, nil
}

func (s *ZenxiangLiyuService) randomFloat64() float64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Float64()
}

func IsZenxiangLiyuFreePlayAvailable(todayUsageAmount float64, freePlayUsed bool) bool {
	return !freePlayUsed && todayUsageAmount > zenxiangLiyuFreePlayUsageThreshold+zenxiangLiyuFreePlayThresholdEpsilon
}

func CalculateZenxiangLiyuEarnedTickets(todayUsageAmount float64, settings *ZenxiangLiyuSettings) int {
	if settings == nil {
		return 0
	}
	threshold := settings.EffectiveTicketUsageThreshold()
	if threshold <= 0 || todayUsageAmount < threshold {
		return 0
	}
	earned := int(math.Floor(todayUsageAmount / threshold))
	limit := settings.EffectiveDailyTicketLimit()
	if earned > limit {
		earned = limit
	}
	return max(0, earned)
}

func CalculateZenxiangLiyuNextTicketUsage(todayUsageAmount float64, settings *ZenxiangLiyuSettings) (float64, float64) {
	if settings == nil {
		return 0, 0
	}
	threshold := settings.EffectiveTicketUsageThreshold()
	limit := settings.EffectiveDailyTicketLimit()
	if threshold <= 0 || limit <= 0 {
		return 0, 0
	}
	earned := CalculateZenxiangLiyuEarnedTickets(todayUsageAmount, settings)
	if earned >= limit {
		return 0, 0
	}
	target := float64(earned+1) * threshold
	return target, math.Max(0, target-todayUsageAmount)
}

func ExpectedZenxiangLiyuReward(prizes []ZenxiangLiyuPrize) float64 {
	total := 0.0
	for _, prize := range prizes {
		if !prize.Enabled {
			continue
		}
		total += prize.RewardAmount * prize.Probability / 100
	}
	return total
}

func (s ZenxiangLiyuSettings) EffectiveTicketUsageThreshold() float64 {
	if s.TicketUsageThreshold > 0 {
		return s.TicketUsageThreshold
	}
	return defaultZenxiangLiyuTicketThreshold
}

func (s ZenxiangLiyuSettings) EffectiveDailyTicketLimit() int {
	if s.DailyTicketLimit > 0 {
		return s.DailyTicketLimit
	}
	return defaultZenxiangLiyuDailyTicketLimit
}

func (s ZenxiangLiyuSettings) EffectiveLuckyCoinProbability() float64 {
	if s.LuckyCoinProbability >= 0 && s.LuckyCoinProbability <= 100 {
		return s.LuckyCoinProbability
	}
	return defaultZenxiangLiyuLuckyProbability
}

func (s *ZenxiangLiyuService) playDate() time.Time {
	now := s.clock().In(time.FixedZone("Asia/Shanghai", 8*60*60))
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
