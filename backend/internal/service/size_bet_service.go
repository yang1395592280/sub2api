package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
	round, settings, err := s.loadBettableRound(ctx, req.RoundID)
	if err != nil {
		return nil, err
	}
	if s.now().After(round.BetClosesAt) {
		return nil, ErrSizeBetClosed
	}
	if !settings.IsStakeAllowed(req.StakeAmount) {
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
	current, err := s.repo.GetRoundByTime(ctx, now)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return current, nil
	}
	settings, err := s.adminService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateRound(ctx, BuildNextRound(now, settings))
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
	return false
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
	closeOffset := settings.BetCloseOffsetSeconds
	if closeOffset < 0 || closeOffset >= sec {
		closeOffset = defaultSizeBetCloseOffsetForRoundDuration(sec)
	}
	startUnix := now.Unix() / int64(sec) * int64(sec)
	startsAt := time.Unix(startUnix, 0).In(now.Location())
	roundNo := startUnix / int64(sec)
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
	}
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
