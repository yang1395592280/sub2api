package service

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrLuckyWheelClosed = infraerrors.Conflict(
		"LUCKY_WHEEL_CLOSED",
		"lucky wheel is disabled",
	)
	ErrLuckyWheelDailyLimitReached = infraerrors.Conflict(
		"LUCKY_WHEEL_DAILY_LIMIT_REACHED",
		"daily spin limit reached",
	)
	ErrLuckyWheelInsufficientPoints = infraerrors.BadRequest(
		"LUCKY_WHEEL_INSUFFICIENT_POINTS",
		"insufficient points for lucky wheel penalty risk",
	)
)

type LuckyWheelSpinRecord struct {
	ID           int64               `json:"id"`
	UserID       int64               `json:"user_id"`
	Email        string              `json:"email,omitempty"`
	Username     string              `json:"username,omitempty"`
	SpinDate     string              `json:"spin_date"`
	SpinIndex    int                 `json:"spin_index"`
	PrizeKey     string              `json:"prize_key"`
	PrizeLabel   string              `json:"prize_label"`
	PrizeType    LuckyWheelPrizeType `json:"prize_type"`
	DeltaPoints  int64               `json:"delta_points"`
	PointsBefore int64               `json:"points_before"`
	PointsAfter  int64               `json:"points_after"`
	Probability  float64             `json:"probability"`
	CreatedAt    time.Time           `json:"created_at"`
}

type LuckyWheelLeaderboardItem struct {
	Rank           int    `json:"rank"`
	UserID         int64  `json:"user_id"`
	Email          string `json:"email"`
	Username       string `json:"username"`
	Points         int64  `json:"points"`
	NetPoints      int64  `json:"net_points"`
	SpinCount      int64  `json:"spin_count"`
	BestDelta      int64  `json:"best_delta"`
	BestPrizeLabel string `json:"best_prize_label"`
}

type LuckyWheelLeaderboardView struct {
	Date  string                      `json:"date"`
	Items []LuckyWheelLeaderboardItem `json:"items"`
}

type LuckyWheelOverview struct {
	Enabled             bool                        `json:"enabled"`
	ServerTime          time.Time                   `json:"server_time"`
	Points              int64                       `json:"points"`
	DailySpinLimit      int                         `json:"daily_spin_limit"`
	SpinsUsedToday      int                         `json:"spins_used_today"`
	SpinsRemainingToday int                         `json:"spins_remaining_today"`
	MinPointsRequired   int64                       `json:"min_points_required"`
	Prizes              []LuckyWheelPrizeConfig     `json:"prizes"`
	RulesMarkdown       string                      `json:"rules_markdown"`
	Leaderboard         []LuckyWheelLeaderboardItem `json:"leaderboard"`
	RecentHistory       []LuckyWheelSpinRecord      `json:"recent_history"`
}

type LuckyWheelSpinResult struct {
	Record              *LuckyWheelSpinRecord `json:"record"`
	SpinsUsedToday      int                   `json:"spins_used_today"`
	SpinsRemainingToday int                   `json:"spins_remaining_today"`
}

type LuckyWheelApplySpinInput struct {
	UserID      int64
	SpinDate    string
	DailyLimit  int
	Prize       LuckyWheelPrizeConfig
	TriggeredAt time.Time
}

type LuckyWheelAdminSpinFilter struct {
	UserID    *int64
	StartTime *time.Time
	EndTime   *time.Time
}

type LuckyWheelRepository interface {
	GetUserAssets(ctx context.Context, userID int64) (*GameCenterAssets, error)
	CountUserSpinsOnDate(ctx context.Context, userID int64, spinDate string) (int, error)
	ApplySpin(ctx context.Context, input LuckyWheelApplySpinInput) (*LuckyWheelSpinRecord, error)
	ListUserSpins(ctx context.Context, userID int64, params pagination.PaginationParams) ([]LuckyWheelSpinRecord, *pagination.PaginationResult, error)
	ListAdminSpins(ctx context.Context, params pagination.PaginationParams, filter LuckyWheelAdminSpinFilter) ([]LuckyWheelSpinRecord, *pagination.PaginationResult, error)
	ListLeaderboard(ctx context.Context, spinDate string, limit int) ([]LuckyWheelLeaderboardItem, error)
}

type LuckyWheelService struct {
	repo             LuckyWheelRepository
	adminService     *LuckyWheelAdminService
	now              func() time.Time
	randomPercentage func() float64
}

func NewLuckyWheelService(repo LuckyWheelRepository, settingRepo SettingRepository) *LuckyWheelService {
	return &LuckyWheelService{
		repo:             repo,
		adminService:     NewLuckyWheelAdminService(settingRepo),
		now:              time.Now,
		randomPercentage: defaultLuckyWheelRandomPercentage,
	}
}

func (s *LuckyWheelService) GetOverview(ctx context.Context, userID int64) (*LuckyWheelOverview, error) {
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	assets, err := s.repo.GetUserAssets(ctx, userID)
	if err != nil {
		return nil, err
	}
	spinDate := s.currentSpinDate()
	used, err := s.repo.CountUserSpinsOnDate(ctx, userID, spinDate)
	if err != nil {
		return nil, err
	}
	history, _, err := s.repo.ListUserSpins(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 10})
	if err != nil {
		return nil, err
	}
	leaderboard, err := s.repo.ListLeaderboard(ctx, spinDate, 10)
	if err != nil {
		return nil, err
	}
	return &LuckyWheelOverview{
		Enabled:             settings.Enabled,
		ServerTime:          s.now(),
		Points:              assets.Points,
		DailySpinLimit:      settings.DailySpinLimit,
		SpinsUsedToday:      used,
		SpinsRemainingToday: countLuckyWheelRemaining(settings.DailySpinLimit, used),
		MinPointsRequired:   maxLuckyWheelPenaltyAbs(settings.Prizes),
		Prizes:              cloneLuckyWheelPrizes(settings.Prizes),
		RulesMarkdown:       settings.RulesMarkdown,
		Leaderboard:         leaderboard,
		RecentHistory:       history,
	}, nil
}

func (s *LuckyWheelService) Spin(ctx context.Context, userID int64) (*LuckyWheelSpinResult, error) {
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, ErrLuckyWheelClosed
	}
	assets, err := s.repo.GetUserAssets(ctx, userID)
	if err != nil {
		return nil, err
	}
	if assets.Points < maxLuckyWheelPenaltyAbs(settings.Prizes) {
		return nil, ErrLuckyWheelInsufficientPoints
	}
	prize, err := s.drawPrize(settings.Prizes)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.ApplySpin(ctx, LuckyWheelApplySpinInput{
		UserID:      userID,
		SpinDate:    s.currentSpinDate(),
		DailyLimit:  settings.DailySpinLimit,
		Prize:       *prize,
		TriggeredAt: s.now(),
	})
	if err != nil {
		return nil, err
	}
	return &LuckyWheelSpinResult{
		Record:              record,
		SpinsUsedToday:      record.SpinIndex,
		SpinsRemainingToday: countLuckyWheelRemaining(settings.DailySpinLimit, record.SpinIndex),
	}, nil
}

func (s *LuckyWheelService) GetHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	return s.repo.ListUserSpins(ctx, userID, params)
}

func (s *LuckyWheelService) GetLeaderboard(ctx context.Context) (*LuckyWheelLeaderboardView, error) {
	spinDate := s.currentSpinDate()
	items, err := s.repo.ListLeaderboard(ctx, spinDate, 20)
	if err != nil {
		return nil, err
	}
	return &LuckyWheelLeaderboardView{
		Date:  spinDate,
		Items: items,
	}, nil
}

func (s *LuckyWheelService) GetSettings(ctx context.Context) (*LuckyWheelSettings, error) {
	return s.adminService.GetSettings(ctx)
}

func (s *LuckyWheelService) UpdateSettings(ctx context.Context, req UpdateLuckyWheelSettingsRequest) error {
	return s.adminService.UpdateSettings(ctx, req)
}

func (s *LuckyWheelService) ListAdminSpins(ctx context.Context, params pagination.PaginationParams, filter LuckyWheelAdminSpinFilter) ([]LuckyWheelSpinRecord, *pagination.PaginationResult, error) {
	return s.repo.ListAdminSpins(ctx, params, filter)
}

func (s *LuckyWheelService) currentSpinDate() string {
	return s.now().Format("2006-01-02")
}

func (s *LuckyWheelService) drawPrize(prizes []LuckyWheelPrizeConfig) (*LuckyWheelPrizeConfig, error) {
	if !isValidLuckyWheelPrizes(prizes) {
		return nil, ErrLuckyWheelInvalidPrizeConfig
	}
	target := s.randomPercentage()
	if target < 0 {
		target = 0
	}
	if target >= 100 {
		target = math.Nextafter(100, 0)
	}
	var cumulative float64
	for index := range prizes {
		cumulative += prizes[index].Probability
		if target < cumulative || index == len(prizes)-1 {
			prize := prizes[index]
			return &prize, nil
		}
	}
	return nil, ErrLuckyWheelInvalidPrizeConfig
}

func countLuckyWheelRemaining(limit, used int) int {
	if limit <= used {
		return 0
	}
	return limit - used
}

func maxLuckyWheelPenaltyAbs(prizes []LuckyWheelPrizeConfig) int64 {
	var required int64
	for _, prize := range prizes {
		if prize.DeltaPoints < 0 && -prize.DeltaPoints > required {
			required = -prize.DeltaPoints
		}
	}
	return required
}

func defaultLuckyWheelRandomPercentage() float64 {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		now := time.Now().UnixNano() % 1_000_000
		if now < 0 {
			now = -now
		}
		return float64(now) / 10_000
	}
	return float64(value.Int64()) / 10_000
}
