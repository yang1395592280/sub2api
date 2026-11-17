package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"
)

const sizeBetRuntimeSettlementBatch = 8

type SizeBetRuntimeService struct {
	gameService *SizeBetService
	interval    time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

func NewSizeBetRuntimeService(gameService *SizeBetService, interval time.Duration) *SizeBetRuntimeService {
	return &SizeBetRuntimeService{
		gameService: gameService,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

func (s *SizeBetRuntimeService) Start() {
	if s == nil || s.gameService == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *SizeBetRuntimeService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *SizeBetRuntimeService) runOnce() {
	if s == nil || s.gameService == nil || s.gameService.repo == nil {
		return
	}
	now := time.Now()
	if s.gameService.now != nil {
		now = s.gameService.now()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	settings, err := s.gameService.adminService.GetSettings(ctx)
	if err != nil {
		log.Printf("[SizeBetRuntime] load settings failed: %v", err)
		return
	}
	if !settings.Enabled {
		return
	}

	if _, err := s.gameService.EnsureCurrentRound(ctx, now); err != nil {
		log.Printf("[SizeBetRuntime] ensure current round failed: %v", err)
	}

	rounds, err := s.gameService.repo.ListRoundsDueForSettlement(ctx, now, sizeBetRuntimeSettlementBatch)
	if err != nil {
		log.Printf("[SizeBetRuntime] list due rounds failed: %v", err)
		return
	}
	for _, round := range rounds {
		recentRounds, err := s.gameService.repo.ListRecentRounds(ctx, 5)
		if err != nil {
			log.Printf("[SizeBetRuntime] list recent rounds failed: %v", err)
			recentRounds = nil
		}
		input := buildSizeBetSettlementInput(&round, now, recentRounds)
		if err := s.gameService.SettleRound(ctx, input); err != nil {
			log.Printf("[SizeBetRuntime] settle round %d failed: %v", round.ID, err)
		}
	}

	if _, err := s.gameService.EnsureCurrentRound(ctx, now); err != nil {
		log.Printf("[SizeBetRuntime] post-settlement ensure current round failed: %v", err)
	}
}

func buildSizeBetSettlementInput(round *SizeBetRound, settledAt time.Time, recentRounds []SizeBetRound) SettleRoundInput {
	seed := round.ServerSeed
	if seed == "" {
		seed = fmt.Sprintf("%d:%d:settle", round.RoundNo, settledAt.UnixNano())
	}
	number, direction := chooseSizeBetResult(seed, round, recentRounds)
	return SettleRoundInput{
		RoundID:         round.ID,
		ResultNumber:    number,
		ResultDirection: direction,
		OddsSmall:       round.OddsSmall,
		OddsMid:         round.OddsMid,
		OddsBig:         round.OddsBig,
		SettledAt:       settledAt,
		ServerSeed:      seed,
	}
}

func chooseSizeBetResult(seed string, round *SizeBetRound, recentRounds []SizeBetRound) (int, SizeBetDirection) {
	hash := sha256.Sum256([]byte(seed))
	smallWeight, midWeight, bigWeight := sizeBetSmoothedWeights(round, recentRounds)
	total := smallWeight + midWeight + bigWeight
	if total <= 0 {
		smallWeight, midWeight, bigWeight = round.ProbSmall, round.ProbMid, round.ProbBig
		total = smallWeight + midWeight + bigWeight
	}
	roll := (float64(binary.BigEndian.Uint64(hash[0:8])%1000000) / 1000000) * total
	switch {
	case roll < smallWeight:
		return 1 + int(hash[8]%5), SizeBetDirectionSmall
	case roll < smallWeight+midWeight:
		return 6, SizeBetDirectionMid
	default:
		return 7 + int(hash[9]%5), SizeBetDirectionBig
	}
}

func sizeBetSmoothedWeights(round *SizeBetRound, recentRounds []SizeBetRound) (float64, float64, float64) {
	smallWeight := round.ProbSmall
	midWeight := round.ProbMid
	bigWeight := round.ProbBig
	if len(recentRounds) < 2 {
		return smallWeight, midWeight, bigWeight
	}

	streakDirection := recentRounds[0].ResultDirection
	if streakDirection == "" {
		return smallWeight, midWeight, bigWeight
	}
	streak := 0
	for _, item := range recentRounds {
		if item.ResultDirection != streakDirection {
			break
		}
		streak++
	}
	if streak < 2 {
		return smallWeight, midWeight, bigWeight
	}

	penalty := 0.35
	if streak >= 3 {
		penalty = 0.6
	}
	if streakDirection == SizeBetDirectionMid {
		penalty *= 0.5
	}

	switch streakDirection {
	case SizeBetDirectionSmall:
		shift := smallWeight * penalty
		smallWeight -= shift
		midWeight += shift * 0.2
		bigWeight += shift * 0.8
	case SizeBetDirectionMid:
		shift := midWeight * penalty
		midWeight -= shift
		smallWeight += shift * 0.5
		bigWeight += shift * 0.5
	case SizeBetDirectionBig:
		shift := bigWeight * penalty
		bigWeight -= shift
		midWeight += shift * 0.2
		smallWeight += shift * 0.8
	}

	return smallWeight, midWeight, bigWeight
}
