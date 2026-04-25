package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrGameCenterClaimDisabled                 = infraerrors.Forbidden("GAME_CENTER_CLAIM_DISABLED", "game center claim is disabled")
	ErrGameCenterClaimBatchNotFound            = infraerrors.NotFound("GAME_CENTER_CLAIM_BATCH_NOT_FOUND", "claim batch not found")
	ErrGameCenterCatalogNotFound               = infraerrors.NotFound("GAME_CENTER_CATALOG_NOT_FOUND", "game catalog not found")
	ErrGameCenterClaimAlreadyClaimed           = infraerrors.Conflict("GAME_CENTER_CLAIM_ALREADY_CLAIMED", "claim batch already claimed")
	ErrGameCenterClaimNotReady                 = infraerrors.Conflict("GAME_CENTER_CLAIM_NOT_READY", "claim batch is not ready")
	ErrGameCenterExchangeBalanceToPointsClosed = infraerrors.Forbidden("GAME_CENTER_EXCHANGE_B2P_DISABLED", "balance to points exchange is disabled")
	ErrGameCenterExchangePointsToBalanceClosed = infraerrors.Forbidden("GAME_CENTER_EXCHANGE_P2B_DISABLED", "points to balance exchange is disabled")
	ErrGameCenterExchangeAmountTooSmall        = infraerrors.BadRequest("GAME_CENTER_EXCHANGE_AMOUNT_TOO_SMALL", "exchange amount is below the minimum")
	ErrGameCenterInsufficientBalance           = infraerrors.BadRequest("GAME_CENTER_INSUFFICIENT_BALANCE", "insufficient balance for exchange")
	ErrGameCenterInsufficientPoints            = infraerrors.BadRequest("GAME_CENTER_INSUFFICIENT_POINTS", "insufficient points for exchange")
	ErrGameCenterInvalidExchangeRate           = infraerrors.BadRequest("GAME_CENTER_INVALID_EXCHANGE_RATE", "exchange rate is invalid")
)

const (
	GameCenterExchangeDirectionBalanceToPoints = "balance_to_points"
	GameCenterExchangeDirectionPointsToBalance = "points_to_balance"
)

type GameCenterAssets struct {
	Balance float64
	Points  int64
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
	EntryType   string    `json:"entry_type"`
	DeltaPoints int64     `json:"delta_points"`
	PointsAfter int64     `json:"points_after"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

type GameCenterClaimBatchConfig struct {
	BatchKey     string `json:"batch_key"`
	Name         string `json:"name"`
	ClaimTime    string `json:"claim_time"`
	PointsAmount int64  `json:"points_amount"`
	Enabled      bool   `json:"enabled"`
}

type GameCenterClaimView struct {
	BatchKey     string `json:"batch_key"`
	Name         string `json:"name"`
	ClaimTime    string `json:"claim_time"`
	PointsAmount int64  `json:"points_amount"`
	Status       string `json:"status"`
}

type GameCenterExchangeView struct {
	BalanceToPointsEnabled bool    `json:"balance_to_points_enabled"`
	PointsToBalanceEnabled bool    `json:"points_to_balance_enabled"`
	BalanceToPointsRate    float64 `json:"balance_to_points_rate"`
	PointsToBalanceRate    float64 `json:"points_to_balance_rate"`
}

type GameCenterExchangeSettings struct {
	BalanceToPointsEnabled bool    `json:"balance_to_points_enabled"`
	PointsToBalanceEnabled bool    `json:"points_to_balance_enabled"`
	BalanceToPointsRate    float64 `json:"balance_to_points_rate"`
	PointsToBalanceRate    float64 `json:"points_to_balance_rate"`
	MinBalanceAmount       float64 `json:"min_balance_amount"`
	MinPointsAmount        int64   `json:"min_points_amount"`
}

type GameCenterAdminSettings struct {
	GameCenterEnabled bool                         `json:"game_center_enabled"`
	ClaimEnabled      bool                         `json:"claim_enabled"`
	ClaimSchedule     []GameCenterClaimBatchConfig `json:"claim_schedule"`
	Exchange          GameCenterExchangeSettings   `json:"exchange"`
}

type UpdateGameCatalogRequest struct {
	Enabled            bool   `json:"enabled"`
	SortOrder          int    `json:"sort_order"`
	DefaultOpenMode    string `json:"default_open_mode"`
	SupportsEmbed      bool   `json:"supports_embed"`
	SupportsStandalone bool   `json:"supports_standalone"`
}

type GameCenterOverview struct {
	Points       int64                  `json:"points"`
	ClaimBatches []GameCenterClaimView  `json:"claim_batches"`
	Exchange     GameCenterExchangeView `json:"exchange"`
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

type ExchangeBalanceToPointsInput struct {
	UserID       int64
	Amount       float64
	Rate         float64
	TargetPoints int64
}

type ExchangePointsToBalanceInput struct {
	UserID       int64
	Points       int64
	Rate         float64
	TargetAmount float64
}

type GameCenterExchangeResult struct {
	Direction    string  `json:"direction"`
	SourceAmount float64 `json:"source_amount,omitempty"`
	SourcePoints int64   `json:"source_points,omitempty"`
	TargetAmount float64 `json:"target_amount,omitempty"`
	TargetPoints int64   `json:"target_points,omitempty"`
	Rate         float64 `json:"rate"`
}

type GameCenterAdminLedgerItem struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
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
	ClaimDate    string    `json:"claim_date"`
	BatchKey     string    `json:"batch_key"`
	PointsAmount int64     `json:"points_amount"`
	ClaimedAt    time.Time `json:"claimed_at"`
}

type GameCenterExchangeRecord struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Direction    string    `json:"direction"`
	SourceAmount float64   `json:"source_amount"`
	SourcePoints int64     `json:"source_points"`
	TargetAmount float64   `json:"target_amount"`
	TargetPoints int64     `json:"target_points"`
	Rate         float64   `json:"rate"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminAdjustPointsInput struct {
	UserID      int64
	DeltaPoints int64
	Reason      string
}

type GameCenterRepository interface {
	GetUserAssets(ctx context.Context, userID int64) (*GameCenterAssets, error)
	ClaimPoints(ctx context.Context, input ClaimPointsInput) error
	ExchangeBalanceToPoints(ctx context.Context, input ExchangeBalanceToPointsInput) (*GameCenterExchangeResult, error)
	ExchangePointsToBalance(ctx context.Context, input ExchangePointsToBalanceInput) (*GameCenterExchangeResult, error)
	ListCatalogs(ctx context.Context) ([]GameCatalog, error)
	UpdateCatalog(ctx context.Context, gameKey string, req UpdateGameCatalogRequest) error
	ListClaimedBatchKeys(ctx context.Context, userID int64, claimDate string) (map[string]struct{}, error)
	ListLedger(ctx context.Context, userID int64, params pagination.PaginationParams) ([]GamePointsLedgerItem, *pagination.PaginationResult, error)
	ListAdminLedger(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]GameCenterAdminLedgerItem, *pagination.PaginationResult, error)
	ListClaimRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]GameCenterClaimRecord, *pagination.PaginationResult, error)
	ListExchangeRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]GameCenterExchangeRecord, *pagination.PaginationResult, error)
	AdjustPoints(ctx context.Context, input AdminAdjustPointsInput) error
}

type GameCenterService struct {
	repo        GameCenterRepository
	settingRepo SettingRepository
	now         func() time.Time
}

func NewGameCenterService(repo GameCenterRepository, settingRepo SettingRepository) *GameCenterService {
	return &GameCenterService{
		repo:        repo,
		settingRepo: settingRepo,
		now:         time.Now,
	}
}

func (s *GameCenterService) GetOverview(ctx context.Context, userID int64, params pagination.PaginationParams) (*GameCenterOverview, error) {
	assets, err := s.repo.GetUserAssets(ctx, userID)
	if err != nil {
		return nil, err
	}
	catalogs, err := s.repo.ListCatalogs(ctx)
	if err != nil {
		return nil, err
	}
	ledger, _, err := s.repo.ListLedger(ctx, userID, params)
	if err != nil {
		return nil, err
	}
	claimedBatchKeys, err := s.repo.ListClaimedBatchKeys(ctx, userID, s.now().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return &GameCenterOverview{
		Points:       assets.Points,
		ClaimBatches: buildClaimViews(s.loadClaimSchedule(ctx), s.now(), claimedBatchKeys),
		Exchange:     s.loadExchangeView(ctx),
		Catalogs:     catalogs,
		RecentLedger: ledger,
	}, nil
}

func (s *GameCenterService) ClaimPoints(ctx context.Context, userID int64, batchKey string) error {
	if !readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterClaimEnabled, false) {
		return ErrGameCenterClaimDisabled
	}
	now := s.now()
	for _, batch := range s.loadClaimSchedule(ctx) {
		if batch.BatchKey != batchKey {
			continue
		}
		if !batch.Enabled {
			return ErrGameCenterClaimBatchNotFound
		}
		readyAt, err := claimReadyAt(now, batch.ClaimTime)
		if err != nil {
			return err
		}
		if now.Before(readyAt) {
			return ErrGameCenterClaimNotReady
		}
		return s.repo.ClaimPoints(ctx, ClaimPointsInput{
			UserID:       userID,
			BatchKey:     batch.BatchKey,
			ClaimDate:    now.Format("2006-01-02"),
			PointsAmount: batch.PointsAmount,
			ClaimedAt:    now,
		})
	}
	return ErrGameCenterClaimBatchNotFound
}

func (s *GameCenterService) ExchangeBalanceToPoints(ctx context.Context, userID int64, amount float64) (*GameCenterExchangeResult, error) {
	if !readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeBalanceToPointsEnabled, false) {
		return nil, ErrGameCenterExchangeBalanceToPointsClosed
	}
	rate := readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeBalanceToPointsRate, 0)
	if rate <= 0 {
		return nil, ErrGameCenterInvalidExchangeRate
	}
	minAmount := readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeMinBalanceAmount, 0)
	if amount < minAmount {
		return nil, ErrGameCenterExchangeAmountTooSmall
	}
	assets, err := s.repo.GetUserAssets(ctx, userID)
	if err != nil {
		return nil, err
	}
	if assets.Balance < amount {
		return nil, ErrGameCenterInsufficientBalance
	}
	targetPoints := int64(math.Round(amount * rate))
	return s.repo.ExchangeBalanceToPoints(ctx, ExchangeBalanceToPointsInput{
		UserID:       userID,
		Amount:       amount,
		Rate:         rate,
		TargetPoints: targetPoints,
	})
}

func (s *GameCenterService) ExchangePointsToBalance(ctx context.Context, userID int64, points int64) (*GameCenterExchangeResult, error) {
	if !readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangePointsToBalanceEnabled, false) {
		return nil, ErrGameCenterExchangePointsToBalanceClosed
	}
	rate := readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangePointsToBalanceRate, 0)
	if rate <= 0 {
		return nil, ErrGameCenterInvalidExchangeRate
	}
	minPoints := readInt64Setting(ctx, s.settingRepo, SettingKeyGameCenterExchangeMinPointsAmount, 0)
	if points < minPoints {
		return nil, ErrGameCenterExchangeAmountTooSmall
	}
	assets, err := s.repo.GetUserAssets(ctx, userID)
	if err != nil {
		return nil, err
	}
	if assets.Points < points {
		return nil, ErrGameCenterInsufficientPoints
	}
	targetAmount := float64(points) / rate
	return s.repo.ExchangePointsToBalance(ctx, ExchangePointsToBalanceInput{
		UserID:       userID,
		Points:       points,
		Rate:         rate,
		TargetAmount: targetAmount,
	})
}

func (s *GameCenterService) GetCatalog(ctx context.Context) ([]GameCatalog, error) {
	return s.repo.ListCatalogs(ctx)
}

func (s *GameCenterService) UpdateCatalog(ctx context.Context, gameKey string, req UpdateGameCatalogRequest) error {
	return s.repo.UpdateCatalog(ctx, gameKey, req)
}

func (s *GameCenterService) GetLedger(ctx context.Context, userID int64, params pagination.PaginationParams) ([]GamePointsLedgerItem, *pagination.PaginationResult, error) {
	return s.repo.ListLedger(ctx, userID, params)
}

func (s *GameCenterService) GetAdminLedger(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	return s.repo.ListAdminLedger(ctx, params, userID)
}

func (s *GameCenterService) GetClaimRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]GameCenterClaimRecord, *pagination.PaginationResult, error) {
	return s.repo.ListClaimRecords(ctx, params, userID)
}

func (s *GameCenterService) GetExchangeRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]GameCenterExchangeRecord, *pagination.PaginationResult, error) {
	return s.repo.ListExchangeRecords(ctx, params, userID)
}

func (s *GameCenterService) AdjustPoints(ctx context.Context, input AdminAdjustPointsInput) error {
	if input.DeltaPoints == 0 {
		return infraerrors.BadRequest("GAME_CENTER_ADJUST_ZERO", "adjust points delta cannot be zero")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return infraerrors.BadRequest("GAME_CENTER_ADJUST_REASON_REQUIRED", "adjust points reason is required")
	}
	return s.repo.AdjustPoints(ctx, input)
}

func (s *GameCenterService) GetAdminSettings(ctx context.Context) (*GameCenterAdminSettings, error) {
	return &GameCenterAdminSettings{
		GameCenterEnabled: readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterEnabled, false),
		ClaimEnabled:      readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterClaimEnabled, false),
		ClaimSchedule:     s.loadClaimSchedule(ctx),
		Exchange: GameCenterExchangeSettings{
			BalanceToPointsEnabled: readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeBalanceToPointsEnabled, false),
			PointsToBalanceEnabled: readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangePointsToBalanceEnabled, false),
			BalanceToPointsRate:    readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeBalanceToPointsRate, 0),
			PointsToBalanceRate:    readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangePointsToBalanceRate, 0),
			MinBalanceAmount:       readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeMinBalanceAmount, 0),
			MinPointsAmount:        readInt64Setting(ctx, s.settingRepo, SettingKeyGameCenterExchangeMinPointsAmount, 0),
		},
	}, nil
}

func (s *GameCenterService) UpdateAdminSettings(ctx context.Context, req GameCenterAdminSettings) error {
	scheduleJSON, err := json.Marshal(req.ClaimSchedule)
	if err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyGameCenterEnabled:                        strconv.FormatBool(req.GameCenterEnabled),
		SettingKeyGameCenterClaimEnabled:                   strconv.FormatBool(req.ClaimEnabled),
		SettingKeyGameCenterClaimSchedule:                  string(scheduleJSON),
		SettingKeyGameCenterExchangeBalanceToPointsEnabled: strconv.FormatBool(req.Exchange.BalanceToPointsEnabled),
		SettingKeyGameCenterExchangePointsToBalanceEnabled: strconv.FormatBool(req.Exchange.PointsToBalanceEnabled),
		SettingKeyGameCenterExchangeBalanceToPointsRate:    strconv.FormatFloat(req.Exchange.BalanceToPointsRate, 'f', -1, 64),
		SettingKeyGameCenterExchangePointsToBalanceRate:    strconv.FormatFloat(req.Exchange.PointsToBalanceRate, 'f', -1, 64),
		SettingKeyGameCenterExchangeMinBalanceAmount:       strconv.FormatFloat(req.Exchange.MinBalanceAmount, 'f', -1, 64),
		SettingKeyGameCenterExchangeMinPointsAmount:        strconv.FormatInt(req.Exchange.MinPointsAmount, 10),
	})
}

func buildClaimViews(batches []GameCenterClaimBatchConfig, now time.Time, claimedBatchKeys map[string]struct{}) []GameCenterClaimView {
	result := make([]GameCenterClaimView, 0, len(batches))
	for _, batch := range batches {
		if !batch.Enabled {
			continue
		}
		status := "claimable"
		if _, ok := claimedBatchKeys[batch.BatchKey]; ok {
			status = "claimed"
		}
		readyAt, err := claimReadyAt(now, batch.ClaimTime)
		if status != "claimed" && (err != nil || now.Before(readyAt)) {
			status = "pending"
		}
		result = append(result, GameCenterClaimView{
			BatchKey:     batch.BatchKey,
			Name:         batch.Name,
			ClaimTime:    batch.ClaimTime,
			PointsAmount: batch.PointsAmount,
			Status:       status,
		})
	}
	return result
}

func (s *GameCenterService) loadClaimSchedule(ctx context.Context) []GameCenterClaimBatchConfig {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyGameCenterClaimSchedule)
	if err != nil || strings.TrimSpace(raw) == "" {
		return []GameCenterClaimBatchConfig{}
	}
	var batches []GameCenterClaimBatchConfig
	if err := json.Unmarshal([]byte(raw), &batches); err != nil {
		return []GameCenterClaimBatchConfig{}
	}
	return batches
}

func (s *GameCenterService) loadExchangeView(ctx context.Context) GameCenterExchangeView {
	return GameCenterExchangeView{
		BalanceToPointsEnabled: readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeBalanceToPointsEnabled, false),
		PointsToBalanceEnabled: readBoolSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangePointsToBalanceEnabled, false),
		BalanceToPointsRate:    readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangeBalanceToPointsRate, 0),
		PointsToBalanceRate:    readFloatSetting(ctx, s.settingRepo, SettingKeyGameCenterExchangePointsToBalanceRate, 0),
	}
}

func claimReadyAt(now time.Time, claimTime string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(claimTime), ":")
	if len(parts) != 2 {
		return time.Time{}, infraerrors.BadRequest("GAME_CENTER_INVALID_CLAIM_TIME", "claim time is invalid")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location()), nil
}

func readBoolSetting(ctx context.Context, repo SettingRepository, key string, fallback bool) bool {
	raw, err := repo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	return strings.TrimSpace(raw) == "true"
}

func readFloatSetting(ctx context.Context, repo SettingRepository, key string, fallback float64) float64 {
	raw, err := repo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	value, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if parseErr != nil {
		return fallback
	}
	return value
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

func (s *GameCenterService) Validate() error {
	if s.repo == nil {
		return errors.New("game center repo is required")
	}
	if s.settingRepo == nil {
		return errors.New("game center setting repo is required")
	}
	return nil
}
