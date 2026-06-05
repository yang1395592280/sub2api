package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type GameCenterAssets struct {
	Points int64
}

type GameCatalog struct {
	GameKey            string `json:"game_key"`
	Name               string `json:"name"`
	Subtitle           string `json:"subtitle"`
	CoverImage         string `json:"cover_image"`
	Description        string `json:"description"`
	Enabled            bool   `json:"enabled"`
	SortOrder          int    `json:"sort_order"`
	DefaultOpenMode    string `json:"default_open_mode"`
	SupportsEmbed      bool   `json:"supports_embed"`
	SupportsStandalone bool   `json:"supports_standalone"`
}

type GamePointsLedgerItem struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id,omitempty"`
	Email       string    `json:"email,omitempty"`
	Username    string    `json:"username,omitempty"`
	EntryType   string    `json:"entry_type"`
	DeltaPoints int64     `json:"delta_points"`
	PointsAfter int64     `json:"points_after"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

type GamePointsLedgerFilter struct {
	UserID    *int64
	StartTime *time.Time
	EndTime   *time.Time
}

type GamePointsLeaderboardItem struct {
	Rank     int    `json:"rank"`
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Points   int64  `json:"points"`
}

type GameCenterOverview struct {
	Enabled      bool                   `json:"enabled"`
	Points       int64                  `json:"points"`
	Checkin      *CheckinStatus         `json:"checkin,omitempty"`
	Catalogs     []GameCatalog          `json:"catalogs"`
	RecentLedger []GamePointsLedgerItem `json:"recent_ledger"`
}

type ClaimPointsInput struct {
	UserID       int64
	BatchKey     string
	ClaimDate    string
	PointsAmount int64
	ClaimedAt    time.Time
}

type GameCenterAdminLedgerItem struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Email          string    `json:"email"`
	Username       string    `json:"username"`
	EntryType      string    `json:"entry_type"`
	DeltaPoints    int64     `json:"delta_points"`
	PointsBefore   int64     `json:"points_before"`
	PointsAfter    int64     `json:"points_after"`
	Reason         string    `json:"reason"`
	RelatedGameKey string    `json:"related_game_key"`
	CreatedAt      time.Time `json:"created_at"`
}

type GameCenterClaimRecord struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	ClaimDate    string    `json:"claim_date"`
	BatchKey     string    `json:"batch_key"`
	PointsAmount int64     `json:"points_amount"`
	ClaimedAt    time.Time `json:"claimed_at"`
}

type AdminAdjustPointsInput struct {
	UserID      int64
	DeltaPoints int64
	Reason      string
}

var (
	ErrGameCenterCatalogNotFound     = infraerrors.NotFound("GAME_CENTER_CATALOG_NOT_FOUND", "game catalog not found")
	ErrGameCenterInsufficientPoints  = infraerrors.BadRequest("GAME_CENTER_INSUFFICIENT_POINTS", "insufficient points")
	ErrGameCenterClaimAlreadyClaimed = infraerrors.Conflict("GAME_CENTER_CLAIM_ALREADY_CLAIMED", "claim batch already claimed")
)

type GameCenterRepository interface {
	GetUserAssets(ctx context.Context, userID int64) (*GameCenterAssets, error)
	ListCatalogs(ctx context.Context) ([]GameCatalog, error)
	UpdateCatalog(ctx context.Context, gameKey string, req UpdateGameCatalogRequest) error
	ListLedger(ctx context.Context, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GamePointsLedgerItem, *pagination.PaginationResult, error)
	ListPointsLeaderboard(ctx context.Context, params pagination.PaginationParams) ([]GamePointsLeaderboardItem, *pagination.PaginationResult, error)
	ListAdminLedger(ctx context.Context, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GameCenterAdminLedgerItem, *pagination.PaginationResult, error)
	ListClaimRecords(ctx context.Context, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GameCenterClaimRecord, *pagination.PaginationResult, error)
	AdjustPoints(ctx context.Context, input AdminAdjustPointsInput) error
}

type GameCenterService struct {
	repo           GameCenterRepository
	settingRepo    SettingRepository
	checkinService *CheckinService
	now            func() time.Time
}

func NewGameCenterService(repo GameCenterRepository, settingRepo SettingRepository, checkinService *CheckinService) *GameCenterService {
	return &GameCenterService{
		repo:           repo,
		settingRepo:    settingRepo,
		checkinService: checkinService,
		now:            time.Now,
	}
}

func (s *GameCenterService) GetOverview(ctx context.Context, userID int64, params pagination.PaginationParams, userTZ string) (*GameCenterOverview, error) {
	assets, err := s.repo.GetUserAssets(ctx, userID)
	if err != nil {
		return nil, err
	}
	catalogs, err := s.repo.ListCatalogs(ctx)
	if err != nil {
		return nil, err
	}
	ledger, _, err := s.GetLedger(ctx, userID, params, GamePointsLedgerFilter{})
	if err != nil {
		return nil, err
	}

	var checkinStatus *CheckinStatus
	if s.checkinService != nil {
		checkinStatus, err = s.checkinService.GetStatus(ctx, userID, "", userTZ)
		if err != nil {
			return nil, err
		}
	}

	return &GameCenterOverview{
		Enabled:      readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterEnabled, false),
		Points:       assets.Points,
		Checkin:      checkinStatus,
		Catalogs:     catalogs,
		RecentLedger: ledger,
	}, nil
}

func (s *GameCenterService) GetCatalog(ctx context.Context) ([]GameCatalog, error) {
	return s.repo.ListCatalogs(ctx)
}

func (s *GameCenterService) UpdateCatalog(ctx context.Context, gameKey string, req UpdateGameCatalogRequest) error {
	return s.repo.UpdateCatalog(ctx, gameKey, req)
}

func (s *GameCenterService) GetLedger(ctx context.Context, userID int64, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GamePointsLedgerItem, *pagination.PaginationResult, error) {
	filter.UserID = &userID
	return s.repo.ListLedger(ctx, params, filter)
}

func (s *GameCenterService) GetUserLedger(ctx context.Context, userID int64, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GamePointsLedgerItem, *pagination.PaginationResult, error) {
	filter.UserID = &userID
	return s.repo.ListLedger(ctx, params, filter)
}

func (s *GameCenterService) GetPointsLeaderboard(ctx context.Context, params pagination.PaginationParams) ([]GamePointsLeaderboardItem, *pagination.PaginationResult, error) {
	return s.repo.ListPointsLeaderboard(ctx, params)
}

func (s *GameCenterService) GetAdminLedger(ctx context.Context, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	return s.repo.ListAdminLedger(ctx, params, filter)
}

func (s *GameCenterService) GetClaimRecords(ctx context.Context, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GameCenterClaimRecord, *pagination.PaginationResult, error) {
	return s.repo.ListClaimRecords(ctx, params, filter)
}

func (s GameCenterService) AdjustPoints(ctx context.Context, input AdminAdjustPointsInput) error {
	if input.DeltaPoints == 0 {
		return infraerrors.BadRequest("GAME_CENTER_ADJUST_ZERO", "adjust points delta cannot be zero")
	}
	input.Reason = strings.TrimSpace(input.Reason)
	return s.repo.AdjustPoints(ctx, input)
}

func (s *GameCenterService) Validate() error {
	if s.repo == nil {
		return errors.New("game center repo is required")
	}
	if s.settingRepo == nil {
		return errors.New("game center setting repo is required")
	}
	return nil
}

type UpdateGameCatalogRequest struct {
	Enabled            bool   `json:"enabled"`
	SortOrder          int    `json:"sort_order"`
	DefaultOpenMode    string `json:"default_open_mode"`
	SupportsEmbed      bool   `json:"supports_embed"`
	SupportsStandalone bool   `json:"supports_standalone"`
}

func readBoolSetting(ctx context.Context, repo SettingRepository, key string, fallback bool) bool {
	raw, err := repo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	return strings.TrimSpace(raw) == "true"
}

func readInt64Setting(ctx context.Context, repo SettingRepository, key string, fallback int64) int64 {
	raw, err := repo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	value, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if parseErr != nil {
		return fallback
	}
	return value
}
