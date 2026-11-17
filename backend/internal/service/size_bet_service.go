package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	SizeBetGameKey = "size_bet"

	SizeBetDirectionSmall SizeBetDirection = "small"
	SizeBetDirectionMid   SizeBetDirection = "mid"
	SizeBetDirectionBig   SizeBetDirection = "big"

	SizeBetRoundStatusOpen    SizeBetRoundStatus = "open"
	SizeBetRoundStatusSettled SizeBetRoundStatus = "settled"

	SizeBetStatusPlaced   SizeBetStatus = "placed"
	SizeBetStatusWon      SizeBetStatus = "won"
	SizeBetStatusLost     SizeBetStatus = "lost"
	SizeBetStatusRefunded SizeBetStatus = "refunded"
)

var (
	ErrSizeBetRoundNotFound       = infraerrors.NotFound("SIZE_BET_ROUND_NOT_FOUND", "size bet round not found")
	ErrSizeBetRoundAlreadySettled = infraerrors.Conflict(
		"SIZE_BET_ROUND_ALREADY_SETTLED",
		"size bet round already settled",
	)
	ErrSizeBetClosed             = infraerrors.Conflict("SIZE_BET_CLOSED", "size bet round is closed")
	ErrSizeBetDuplicateBet       = infraerrors.Conflict("SIZE_BET_DUPLICATE_BET", "size bet already exists for this round or idempotency key")
	ErrSizeBetSettlementConflict = infraerrors.Conflict(
		"SIZE_BET_SETTLEMENT_CONFLICT",
		"size bet settlement state changed",
	)
	ErrSizeBetInvalidStake     = infraerrors.BadRequest("SIZE_BET_INVALID_STAKE", "stake amount is not allowed")
	ErrSizeBetInvalidDirection = infraerrors.BadRequest(
		"SIZE_BET_INVALID_DIRECTION",
		"size bet direction is invalid",
	)
	ErrSizeBetInvalidResult = infraerrors.BadRequest(
		"SIZE_BET_INVALID_RESULT",
		"result number and direction are inconsistent",
	)
	ErrSizeBetInsufficientPoints = infraerrors.BadRequest(
		"SIZE_BET_INSUFFICIENT_POINTS",
		"insufficient points",
	)
)

type SizeBetDirection string
type SizeBetRoundStatus string
type SizeBetStatus string

type SizeBetRound struct {
	ID              int64
	GameKey         string
	RoundNo         int64
	Status          SizeBetRoundStatus
	StartsAt        time.Time
	BetClosesAt     time.Time
	SettlesAt       time.Time
	ProbSmall       float64
	ProbMid         float64
	ProbBig         float64
	OddsSmall       float64
	OddsMid         float64
	OddsBig         float64
	AllowedStakes   []int
	ResultNumber    *int
	ResultDirection SizeBetDirection
	ServerSeedHash  string
	ServerSeed      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SizeBet struct {
	ID              int64
	RoundID         int64
	UserID          int64
	Direction       SizeBetDirection
	StakeAmount     float64
	PayoutAmount    float64
	NetResultAmount float64
	Status          SizeBetStatus
	IdempotencyKey  string
	PlacedAt        time.Time
	SettledAt       *time.Time
}

type SizeBetLedgerEntry struct {
	ID            int64
	UserID        int64
	GameKey       string
	RoundID       *int64
	BetID         *int64
	EntryType     string
	Direction     string
	StakeAmount   float64
	DeltaAmount   float64
	BalanceBefore float64
	BalanceAfter  float64
	Reason        string
	CreatedAt     time.Time
}

type PlaceSizeBetRequest struct {
	UserID         int64
	RoundID        int64
	Direction      SizeBetDirection
	StakeAmount    float64
	IdempotencyKey string
}

type SettleRoundInput struct {
	RoundID         int64
	ResultNumber    int
	ResultDirection SizeBetDirection
	OddsSmall       float64
	OddsMid         float64
	OddsBig         float64
	SettledAt       time.Time
	ServerSeed      string
}

type SizeBetRepository interface {
	GetRoundByID(ctx context.Context, roundID int64) (*SizeBetRound, error)
	GetRoundByTime(ctx context.Context, now time.Time) (*SizeBetRound, error)
	CreateRound(ctx context.Context, round *SizeBetRound) (*SizeBetRound, error)
	CreateBetAndDebit(ctx context.Context, bet *SizeBet, entry *SizeBetLedgerEntry) error
	ApplySettlement(ctx context.Context, input SettleRoundInput) ([]SizeBet, error)
	RefreshLeaderboardSnapshots(ctx context.Context, settledRoundID int64) error
	GetBetByRoundAndUser(ctx context.Context, roundID, userID int64) (*SizeBet, error)
	ListRecentRounds(ctx context.Context, limit int) ([]SizeBetRound, error)
	ListUserHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]SizeBetUserHistoryItem, *pagination.PaginationResult, error)
	ListLeaderboard(ctx context.Context, scopeType, scopeKey string, limit int) ([]SizeBetLeaderboardEntry, time.Time, error)
	ListAdminRounds(ctx context.Context, params pagination.PaginationParams) ([]SizeBetRound, *pagination.PaginationResult, error)
	ListAdminBets(ctx context.Context, params pagination.PaginationParams, filter SizeBetAdminBetFilter) ([]SizeBetAdminBet, *pagination.PaginationResult, error)
	ListAdminLedger(ctx context.Context, params pagination.PaginationParams, filter SizeBetAdminLedgerFilter) ([]SizeBetLedgerEntry, *pagination.PaginationResult, error)
	RefundRound(ctx context.Context, roundID int64, refundedAt time.Time) ([]SizeBet, error)
	ListRoundsDueForSettlement(ctx context.Context, now time.Time, limit int) ([]SizeBetRound, error)
	GetStatsOverview(ctx context.Context, date string) (*SizeBetStatsOverview, error)
	ListStatsUsers(ctx context.Context, date string, params pagination.PaginationParams) ([]SizeBetStatsUserItem, *pagination.PaginationResult, error)
}

type SizeBetService struct {
	repo                 SizeBetRepository
	adminService         *SizeBetAdminService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCache         BillingCache
	now                  func() time.Time
}

func NewSizeBetService(
	repo SizeBetRepository,
	settingRepo SettingRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCache BillingCache,
) *SizeBetService {
	return &SizeBetService{
		repo:                 repo,
		adminService:         NewSizeBetAdminService(settingRepo),
		authCacheInvalidator: authCacheInvalidator,
		billingCache:         billingCache,
		now:                  time.Now,
	}
}

func (s *SizeBetService) PlaceBet(ctx context.Context, req PlaceSizeBetRequest) (*SizeBet, error) {
	if !req.Direction.IsValid() {
		return nil, ErrSizeBetInvalidDirection
	}
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, ErrSizeBetClosed
	}
	round, snapshot, err := s.loadBettableRound(ctx, req.RoundID)
	if err != nil {
		return nil, err
	}
	if s.now().After(round.BetClosesAt) {
		return nil, ErrSizeBetClosed
	}
	if !snapshot.IsStakeAllowed(req.StakeAmount) {
		return nil, ErrSizeBetInvalidStake
	}

	bet := &SizeBet{
		RoundID:        req.RoundID,
		UserID:         req.UserID,
		Direction:      req.Direction,
		StakeAmount:    req.StakeAmount,
		Status:         SizeBetStatusPlaced,
		IdempotencyKey: req.IdempotencyKey,
	}
	if err := s.repo.CreateBetAndDebit(ctx, bet, NewBetDebitLedger(round, bet)); err != nil {
		return nil, err
	}
	s.invalidateCaches(ctx, req.UserID)
	return bet, nil
}

func (s *SizeBetService) SettleRound(ctx context.Context, input SettleRoundInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	round, err := s.repo.GetRoundByID(ctx, input.RoundID)
	if err != nil {
		return err
	}
	if round == nil {
		return ErrSizeBetRoundNotFound
	}
	if round.Status == SizeBetRoundStatusSettled {
		if round.ResultNumber == nil {
			return ErrSizeBetRoundAlreadySettled
		}
		if !round.MatchesSettlement(input) {
			return ErrSizeBetSettlementConflict
		}
		return s.repo.RefreshLeaderboardSnapshots(ctx, input.RoundID)
	}
	input = input.withDefaults(round, s.now)

	bets, err := s.repo.ApplySettlement(ctx, input)
	if err != nil {
		return err
	}
	for _, userID := range uniqueSizeBetUserIDs(bets) {
		s.invalidateCaches(ctx, userID)
	}
	return s.repo.RefreshLeaderboardSnapshots(ctx, input.RoundID)
}

func (s *SizeBetService) EnsureCurrentRound(ctx context.Context, now time.Time) (*SizeBetRound, error) {
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, nil
	}
	if sizeBetInPreparationWindow(now, settings) {
		return nil, nil
	}

	current, err := s.repo.GetRoundByTime(ctx, now)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return current, nil
	}
	return s.repo.CreateRound(ctx, BuildNextRound(now, settings))
}

func (s *SizeBetService) GetCurrentRoundView(ctx context.Context, userID int64, now time.Time) (*SizeBetCurrentRoundView, error) {
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	recentRounds, err := s.repo.ListRecentRounds(ctx, 1)
	if err != nil {
		return nil, err
	}

	view := &SizeBetCurrentRoundView{
		Enabled:    settings.Enabled,
		Phase:      SizeBetPhaseMaintenance,
		ServerTime: now,
	}
	if len(recentRounds) > 0 {
		view.PreviousRound = &recentRounds[0]
	}
	if !settings.Enabled {
		return view, nil
	}

	current, err := s.EnsureCurrentRound(ctx, now)
	if err != nil {
		return nil, err
	}

	var myBet *SizeBet
	if current != nil {
		myBet, err = s.repo.GetBetByRoundAndUser(ctx, current.ID, userID)
		if err != nil {
			return nil, err
		}
	}

	view.Phase = sizeBetPhaseForRound(settings.Enabled, current, now)
	view.MyBet = myBet
	if current != nil {
		view.Round = current.ToCurrentView(now)
	}
	return view, nil
}

func (s *SizeBetService) GetHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]SizeBetUserHistoryItem, *pagination.PaginationResult, error) {
	return s.repo.ListUserHistory(ctx, userID, normalizeSizeBetPagination(params))
}

func (s *SizeBetService) ListRecentRounds(ctx context.Context, limit int) ([]SizeBetRound, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListRecentRounds(ctx, limit)
}

func (s *SizeBetService) GetLeaderboard(ctx context.Context, scope string, now time.Time) (*SizeBetLeaderboardView, error) {
	normalizedScope := normalizeSizeBetLeaderboardScope(scope)
	scopeKey := sizeBetLeaderboardScopeKey(normalizedScope, now)
	items, refreshedAt, err := s.repo.ListLeaderboard(ctx, normalizedScope, scopeKey, 20)
	if err != nil {
		return nil, err
	}

	for i := range items {
		items[i].Rank = i + 1
		items[i].Username = maskSizeBetUsername(items[i].Username, items[i].UserID)
		if items[i].BetCount > 0 {
			items[i].HitRate = float64(items[i].WinCount) / float64(items[i].BetCount)
		}
	}

	view := &SizeBetLeaderboardView{
		Scope:    normalizedScope,
		ScopeKey: scopeKey,
		Items:    items,
	}
	if !refreshedAt.IsZero() {
		view.RefreshedAt = &refreshedAt
	}
	return view, nil
}

func (s *SizeBetService) GetRules(ctx context.Context, now time.Time) (*SizeBetRulesView, error) {
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	current, err := s.repo.GetRoundByTime(ctx, now)
	if err != nil {
		return nil, err
	}
	if current != nil {
		settings.AllowedStakes = append([]int(nil), current.AllowedStakes...)
		settings.ProbSmall = current.ProbSmall
		settings.ProbMid = current.ProbMid
		settings.ProbBig = current.ProbBig
		settings.OddsSmall = current.OddsSmall
		settings.OddsMid = current.OddsMid
		settings.OddsBig = current.OddsBig
		settings.RoundDurationSeconds = int(current.SettlesAt.Sub(current.StartsAt).Seconds())
		settings.BetCloseOffsetSeconds = int(current.BetClosesAt.Sub(current.StartsAt).Seconds())
	}

	return &SizeBetRulesView{
		Enabled:               settings.Enabled,
		RoundDurationSeconds:  settings.RoundDurationSeconds,
		BetCloseOffsetSeconds: settings.BetCloseOffsetSeconds,
		AllowedStakes:         append([]int(nil), settings.AllowedStakes...),
		CustomStakeMin:        settings.CustomStakeMin,
		CustomStakeMax:        settings.CustomStakeMax,
		Probabilities: SizeBetProbabilityConfig{
			Small: settings.ProbSmall,
			Mid:   settings.ProbMid,
			Big:   settings.ProbBig,
		},
		Odds: SizeBetOddsConfig{
			Small: settings.OddsSmall,
			Mid:   settings.OddsMid,
			Big:   settings.OddsBig,
		},
		RulesMarkdown: settings.RulesMarkdown,
	}, nil
}

func (s *SizeBetService) ListRounds(ctx context.Context, params pagination.PaginationParams) ([]SizeBetRound, *pagination.PaginationResult, error) {
	return s.repo.ListAdminRounds(ctx, normalizeSizeBetPagination(params))
}

func (s *SizeBetService) ListBets(ctx context.Context, params pagination.PaginationParams, filter SizeBetAdminBetFilter) ([]SizeBetAdminBet, *pagination.PaginationResult, error) {
	return s.repo.ListAdminBets(ctx, normalizeSizeBetPagination(params), filter)
}

func (s *SizeBetService) ListLedger(ctx context.Context, params pagination.PaginationParams, filter SizeBetAdminLedgerFilter) ([]SizeBetLedgerEntry, *pagination.PaginationResult, error) {
	return s.repo.ListAdminLedger(ctx, normalizeSizeBetPagination(params), filter)
}

func (s *SizeBetService) RefundRound(ctx context.Context, roundID int64, refundedAt time.Time) (*SizeBetRefundResult, error) {
	round, err := s.repo.GetRoundByID(ctx, roundID)
	if err != nil {
		return nil, err
	}
	if round == nil {
		return nil, ErrSizeBetRoundNotFound
	}
	if round.Status == SizeBetRoundStatusSettled && round.ResultNumber != nil {
		return nil, ErrSizeBetRoundAlreadySettled
	}
	if refundedAt.IsZero() {
		refundedAt = s.now()
	}

	refundedBets, err := s.repo.RefundRound(ctx, roundID, refundedAt)
	if err != nil {
		return nil, err
	}
	for _, userID := range uniqueSizeBetUserIDs(refundedBets) {
		s.invalidateCaches(ctx, userID)
	}
	if err := s.repo.RefreshLeaderboardSnapshots(ctx, roundID); err != nil {
		return nil, err
	}
	return &SizeBetRefundResult{
		RoundID:       roundID,
		RefundedCount: len(refundedBets),
		RefundedAt:    refundedAt,
	}, nil
}

func (s *SizeBetService) GetStatsOverview(ctx context.Context, date string) (*SizeBetStatsOverview, error) {
	return s.repo.GetStatsOverview(ctx, date)
}

func (s *SizeBetService) ListStatsUsers(ctx context.Context, date string, params pagination.PaginationParams) ([]SizeBetStatsUserItem, *pagination.PaginationResult, error) {
	return s.repo.ListStatsUsers(ctx, date, normalizeSizeBetPagination(params))
}

func (s *SizeBetService) loadBettableRound(ctx context.Context, roundID int64) (*SizeBetRound, *SizeBetSettings, error) {
	round, err := s.repo.GetRoundByID(ctx, roundID)
	if err != nil {
		return nil, nil, err
	}
	if round == nil {
		return nil, nil, ErrSizeBetRoundNotFound
	}
	if round.Status != SizeBetRoundStatusOpen {
		return nil, nil, ErrSizeBetClosed
	}
	return round, round.SnapshotSettings(), nil
}

func (s *SizeBetService) invalidateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCache == nil {
		return
	}
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.billingCache.InvalidateUserBalance(cacheCtx, userID)
	}()
}

func (d SizeBetDirection) IsValid() bool {
	return d == SizeBetDirectionSmall || d == SizeBetDirectionMid || d == SizeBetDirectionBig
}

func (s *SizeBetSettings) IsStakeAllowed(amount float64) bool {
	for _, stake := range s.AllowedStakes {
		if math.Abs(amount-float64(stake)) <= 0.0001 {
			return true
		}
	}
	if amount <= 0 || math.Abs(amount-math.Round(amount)) > 0.0001 {
		return false
	}
	intAmount := int(math.Round(amount))
	if s.CustomStakeMin > 0 && intAmount < s.CustomStakeMin {
		return false
	}
	if s.CustomStakeMax > 0 && intAmount > s.CustomStakeMax {
		return false
	}
	return true
}

func (r *SizeBetRound) SnapshotSettings() *SizeBetSettings {
	return &SizeBetSettings{
		AllowedStakes: append([]int(nil), r.AllowedStakes...),
		ProbSmall:     r.ProbSmall,
		ProbMid:       r.ProbMid,
		ProbBig:       r.ProbBig,
		OddsSmall:     r.OddsSmall,
		OddsMid:       r.OddsMid,
		OddsBig:       r.OddsBig,
	}
}

func (r *SizeBetRound) MatchesSettlement(input SettleRoundInput) bool {
	if r == nil || r.ResultNumber == nil {
		return false
	}
	if *r.ResultNumber != input.ResultNumber || r.ResultDirection != input.ResultDirection {
		return false
	}
	if input.ServerSeed != "" && r.ServerSeed != "" && r.ServerSeed != input.ServerSeed {
		return false
	}
	return true
}

func (in SettleRoundInput) withDefaults(round *SizeBetRound, now func() time.Time) SettleRoundInput {
	if in.SettledAt.IsZero() {
		in.SettledAt = now()
	}
	if round != nil {
		if in.OddsSmall <= 0 {
			in.OddsSmall = round.OddsSmall
		}
		if in.OddsMid <= 0 {
			in.OddsMid = round.OddsMid
		}
		if in.OddsBig <= 0 {
			in.OddsBig = round.OddsBig
		}
	}
	return in
}

func (in SettleRoundInput) Validate() error {
	if !in.ResultDirection.IsValid() {
		return ErrSizeBetInvalidDirection
	}
	expected, ok := SizeBetDirectionForNumber(in.ResultNumber)
	if !ok || expected != in.ResultDirection {
		return ErrSizeBetInvalidResult
	}
	return nil
}

func (in SettleRoundInput) OddsFor(direction SizeBetDirection) float64 {
	switch direction {
	case SizeBetDirectionSmall:
		return in.OddsSmall
	case SizeBetDirectionMid:
		return in.OddsMid
	default:
		return in.OddsBig
	}
}

func SizeBetDirectionForNumber(number int) (SizeBetDirection, bool) {
	switch {
	case number >= 1 && number <= 5:
		return SizeBetDirectionSmall, true
	case number == 6:
		return SizeBetDirectionMid, true
	case number >= 7 && number <= 11:
		return SizeBetDirectionBig, true
	default:
		return "", false
	}
}

func BuildNextRound(now time.Time, settings *SizeBetSettings) *SizeBetRound {
	sec := settings.RoundDurationSeconds
	if sec <= 0 {
		sec = defaultSizeBetRoundDurationSeconds
	}
	cycle := sec + defaultSizeBetPreparationSeconds
	closeOffset := settings.BetCloseOffsetSeconds
	if closeOffset < 0 || closeOffset >= sec {
		closeOffset = defaultSizeBetCloseOffsetForRoundDuration(sec)
	}
	startUnix := now.Unix() / int64(cycle) * int64(cycle)
	startsAt := time.Unix(startUnix, 0).In(now.Location())
	roundNo := startUnix / int64(cycle)
	seed := fmt.Sprintf("%d:%d", roundNo, now.UnixNano())
	hash := sha256.Sum256([]byte(seed))
	return &SizeBetRound{
		GameKey:        SizeBetGameKey,
		RoundNo:        roundNo,
		Status:         SizeBetRoundStatusOpen,
		StartsAt:       startsAt,
		BetClosesAt:    startsAt.Add(time.Duration(closeOffset) * time.Second),
		SettlesAt:      startsAt.Add(time.Duration(sec) * time.Second),
		ProbSmall:      settings.ProbSmall,
		ProbMid:        settings.ProbMid,
		ProbBig:        settings.ProbBig,
		OddsSmall:      settings.OddsSmall,
		OddsMid:        settings.OddsMid,
		OddsBig:        settings.OddsBig,
		AllowedStakes:  append([]int(nil), settings.AllowedStakes...),
		ServerSeedHash: hex.EncodeToString(hash[:]),
		ServerSeed:     seed,
	}
}

func (r *SizeBetRound) ToCurrentView(now time.Time) *SizeBetCurrentRound {
	if r == nil {
		return nil
	}
	return &SizeBetCurrentRound{
		ID:                  r.ID,
		RoundNo:             r.RoundNo,
		Status:              r.Status,
		StartsAt:            r.StartsAt,
		BetClosesAt:         r.BetClosesAt,
		SettlesAt:           r.SettlesAt,
		ProbSmall:           r.ProbSmall,
		ProbMid:             r.ProbMid,
		ProbBig:             r.ProbBig,
		OddsSmall:           r.OddsSmall,
		OddsMid:             r.OddsMid,
		OddsBig:             r.OddsBig,
		AllowedStakes:       append([]int(nil), r.AllowedStakes...),
		ServerSeedHash:      r.ServerSeedHash,
		CountdownSeconds:    maxSizeBetSeconds(int(r.SettlesAt.Sub(now).Seconds())),
		BetCountdownSeconds: maxSizeBetSeconds(int(r.BetClosesAt.Sub(now).Seconds())),
	}
}

func sizeBetPhaseForRound(enabled bool, round *SizeBetRound, now time.Time) SizeBetPhase {
	if !enabled {
		return SizeBetPhaseMaintenance
	}
	if round == nil {
		return SizeBetPhasePreparing
	}
	if now.Before(round.BetClosesAt) || now.Equal(round.BetClosesAt) {
		return SizeBetPhaseBetting
	}
	return SizeBetPhaseClosed
}

func sizeBetInPreparationWindow(now time.Time, settings *SizeBetSettings) bool {
	roundDuration := settings.RoundDurationSeconds
	if roundDuration <= 0 {
		roundDuration = defaultSizeBetRoundDurationSeconds
	}
	cycleDuration := roundDuration + defaultSizeBetPreparationSeconds
	if cycleDuration <= 0 {
		return false
	}
	elapsed := int(now.Unix() % int64(cycleDuration))
	if elapsed < 0 {
		elapsed += cycleDuration
	}
	return elapsed >= roundDuration
}

func normalizeSizeBetPagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	params.SortOrder = params.NormalizedSortOrder(pagination.SortOrderDesc)
	return params
}

func normalizeSizeBetLeaderboardScope(scope string) string {
	switch scope {
	case "weekly":
		return "weekly"
	default:
		return "all"
	}
}

func sizeBetLeaderboardScopeKey(scope string, now time.Time) string {
	if scope != "weekly" {
		return "all"
	}
	utc := now.UTC()
	weekday := int(utc.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(weekday - 1))
	return start.Format("2006-01-02")
}

func maskSizeBetUsername(username string, userID int64) string {
	if username == "" {
		return fmt.Sprintf("user-%d", userID)
	}
	if len(username) <= 2 {
		return username
	}
	if len(username) == 3 {
		return username[:1] + "*" + username[2:]
	}
	return username[:1] + "**" + username[len(username)-1:]
}

func maxSizeBetSeconds(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func NewBetDebitLedger(round *SizeBetRound, bet *SizeBet) *SizeBetLedgerEntry {
	return &SizeBetLedgerEntry{
		UserID:      bet.UserID,
		GameKey:     SizeBetGameKey,
		RoundID:     sizeBetInt64Ptr(round.ID),
		EntryType:   "bet_debit",
		Direction:   string(bet.Direction),
		StakeAmount: bet.StakeAmount,
		DeltaAmount: -bet.StakeAmount,
		Reason:      "size bet stake debited",
	}
}

func NewBetPayoutLedger(round *SizeBetRound, bet *SizeBet, odds float64) *SizeBetLedgerEntry {
	return &SizeBetLedgerEntry{
		UserID:      bet.UserID,
		GameKey:     SizeBetGameKey,
		RoundID:     sizeBetInt64Ptr(round.ID),
		BetID:       sizeBetInt64Ptr(bet.ID),
		EntryType:   "bet_payout",
		Direction:   string(bet.Direction),
		StakeAmount: bet.StakeAmount,
		DeltaAmount: bet.StakeAmount * odds,
		Reason:      "size bet payout credited",
	}
}

func uniqueSizeBetUserIDs(bets []SizeBet) []int64 {
	seen := make(map[int64]struct{}, len(bets))
	ids := make([]int64, 0, len(bets))
	for _, bet := range bets {
		if _, ok := seen[bet.UserID]; ok {
			continue
		}
		seen[bet.UserID] = struct{}{}
		ids = append(ids, bet.UserID)
	}
	return ids
}

func sizeBetInt64Ptr(v int64) *int64 {
	return &v
}
