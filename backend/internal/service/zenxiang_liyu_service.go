package service

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

const zenxiangLiyuProbabilityEpsilon = 0.000001

var (
	ErrZenxiangLiyuDisabled                = errors.New("zenxiang liyu is disabled")
	ErrZenxiangLiyuUnauthorized            = errors.New("zenxiang liyu unauthorized")
	ErrZenxiangLiyuInvalidSettings         = errors.New("zenxiang liyu invalid settings")
	ErrZenxiangLiyuInvalidProbabilityTotal = errors.New("zenxiang liyu invalid probability total")
	ErrZenxiangLiyuInsufficientBalance     = errors.New("zenxiang liyu insufficient balance")
	ErrZenxiangLiyuDailyLimitReached       = errors.New("zenxiang liyu daily limit reached")
	ErrZenxiangLiyuRequestIDRequired       = errors.New("zenxiang liyu request id required")
)

type ZenxiangLiyuSettings struct {
	GlobalEnabled  bool    `json:"global_enabled"`
	TicketAmount   float64 `json:"ticket_amount"`
	MinimumBalance float64 `json:"minimum_balance"`
	DailyPlayLimit int     `json:"daily_play_limit"`
}

type ZenxiangLiyuSettingsUpdate struct {
	GlobalEnabled  bool    `json:"global_enabled"`
	TicketAmount   float64 `json:"ticket_amount"`
	MinimumBalance float64 `json:"minimum_balance"`
	DailyPlayLimit int     `json:"daily_play_limit"`
}

func (u ZenxiangLiyuSettingsUpdate) Settings() ZenxiangLiyuSettings {
	return ZenxiangLiyuSettings(u)
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
	Visible        bool                `json:"visible"`
	CanPlay        bool                `json:"can_play"`
	Reason         string              `json:"reason,omitempty"`
	TicketAmount   float64             `json:"ticket_amount"`
	MinimumBalance float64             `json:"minimum_balance"`
	DailyPlayLimit int                 `json:"daily_play_limit"`
	TodayPlayCount int                 `json:"today_play_count"`
	RemainingPlays int                 `json:"remaining_plays"`
	Prizes         []ZenxiangLiyuPrize `json:"prizes"`
}

type ZenxiangLiyuPlayCommand struct {
	UserID         int64
	RequestID      string
	PlayDate       time.Time
	Settings       ZenxiangLiyuSettings
	Prize          ZenxiangLiyuPrize
	ConfigSnapshot map[string]any
}

type ZenxiangLiyuPlayResult struct {
	Applied            bool      `json:"applied"`
	RequestID          string    `json:"request_id"`
	PrizeID            int64     `json:"prize_id"`
	PrizeName          string    `json:"prize_name"`
	RewardAmount       float64   `json:"reward_amount"`
	TicketAmount       float64   `json:"ticket_amount"`
	UserNetAmount      float64   `json:"user_net_amount"`
	BalanceBefore      float64   `json:"balance_before"`
	BalanceAfterTicket float64   `json:"balance_after_ticket"`
	BalanceAfterReward float64   `json:"balance_after_reward"`
	PlayedAt           time.Time `json:"played_at"`
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

// ZenxiangLiyuRepository isolates service policy from the storage and transaction implementation.
type ZenxiangLiyuRepository interface {
	GetSettings(ctx context.Context) (*ZenxiangLiyuSettings, error)
	UpdateSettings(ctx context.Context, settings ZenxiangLiyuSettings) (*ZenxiangLiyuSettings, error)
	ListPrizes(ctx context.Context) ([]ZenxiangLiyuPrize, error)
	SavePrize(ctx context.Context, prize ZenxiangLiyuPrize) (*ZenxiangLiyuPrize, error)
	DeletePrize(ctx context.Context, id int64) error
	IsUserGranted(ctx context.Context, userID int64) (bool, error)
	CountUserPlaysOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error)
	Play(ctx context.Context, cmd ZenxiangLiyuPlayCommand) (*ZenxiangLiyuPlayResult, error)
}

type ZenxiangLiyuService struct {
	repo  ZenxiangLiyuRepository
	clock func() time.Time
	rng   *rand.Rand
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
		if strings.TrimSpace(prize.Name) == "" || prize.RewardAmount < 0 || prize.Probability < 0 || prize.Probability > 100 {
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
	status := &ZenxiangLiyuStatus{TicketAmount: settings.TicketAmount, MinimumBalance: settings.MinimumBalance, DailyPlayLimit: settings.DailyPlayLimit, Prizes: prizes}
	if !settings.GlobalEnabled {
		status.Reason = ErrZenxiangLiyuDisabled.Error()
		return status, nil
	}
	granted, err := s.repo.IsUserGranted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !granted {
		status.Reason = ErrZenxiangLiyuUnauthorized.Error()
		return status, nil
	}
	status.Visible = true
	status.TodayPlayCount, err = s.repo.CountUserPlaysOnDate(ctx, userID, s.playDate())
	if err != nil {
		return nil, err
	}
	status.RemainingPlays = max(0, settings.DailyPlayLimit-status.TodayPlayCount)
	status.CanPlay = status.RemainingPlays > 0
	if !status.CanPlay {
		status.Reason = ErrZenxiangLiyuDailyLimitReached.Error()
	}
	return status, nil
}

func (s *ZenxiangLiyuService) Play(ctx context.Context, userID int64, requestID string) (*ZenxiangLiyuPlayResult, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, ErrZenxiangLiyuRequestIDRequired
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.GlobalEnabled {
		return nil, ErrZenxiangLiyuDisabled
	}
	granted, err := s.repo.IsUserGranted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !granted {
		return nil, ErrZenxiangLiyuUnauthorized
	}
	prizes, err := s.ListPrizes(ctx)
	if err != nil {
		return nil, err
	}
	prize, err := PickZenxiangLiyuPrize(prizes, s.rng.Float64()*100)
	if err != nil {
		return nil, err
	}
	return s.repo.Play(ctx, ZenxiangLiyuPlayCommand{UserID: userID, RequestID: requestID, PlayDate: s.playDate(), Settings: *settings, Prize: *prize, ConfigSnapshot: map[string]any{"settings": settings, "prize": prize}})
}

func (s *ZenxiangLiyuService) GetSettings(ctx context.Context) (*ZenxiangLiyuSettings, error) {
	if s.repo == nil {
		return nil, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.GetSettings(ctx)
}

func (s *ZenxiangLiyuService) UpdateSettings(ctx context.Context, req ZenxiangLiyuSettingsUpdate) (*ZenxiangLiyuSettings, error) {
	settings := req.Settings()
	if settings.TicketAmount <= 0 || settings.MinimumBalance < 0 || settings.DailyPlayLimit <= 0 {
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

func (s *ZenxiangLiyuService) DeletePrize(ctx context.Context, id int64) error {
	if id <= 0 || s.repo == nil {
		return ErrZenxiangLiyuInvalidSettings
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
			prize, err := PickZenxiangLiyuPrize(req.Prizes, s.rng.Float64()*100)
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

func (s *ZenxiangLiyuService) playDate() time.Time {
	now := s.clock().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
