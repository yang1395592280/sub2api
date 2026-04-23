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

	if _, err := s.gameService.EnsureCurrentRound(ctx, now); err != nil {
		log.Printf("[SizeBetRuntime] ensure current round failed: %v", err)
	}

	rounds, err := s.gameService.repo.ListRoundsDueForSettlement(ctx, now, sizeBetRuntimeSettlementBatch)
	if err != nil {
		log.Printf("[SizeBetRuntime] list due rounds failed: %v", err)
		return
	}
	for _, round := range rounds {
		input := buildSizeBetSettlementInput(&round, now)
		if err := s.gameService.SettleRound(ctx, input); err != nil {
			log.Printf("[SizeBetRuntime] settle round %d failed: %v", round.ID, err)
		}
	}

	if _, err := s.gameService.EnsureCurrentRound(ctx, now); err != nil {
		log.Printf("[SizeBetRuntime] post-settlement ensure current round failed: %v", err)
	}
}

func buildSizeBetSettlementInput(round *SizeBetRound, settledAt time.Time) SettleRoundInput {
	seed := round.ServerSeed
	if seed == "" {
		seed = fmt.Sprintf("%d:%d:settle", round.RoundNo, settledAt.UnixNano())
	}
	number, direction := chooseSizeBetResult(seed, round)
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

func chooseSizeBetResult(seed string, round *SizeBetRound) (int, SizeBetDirection) {
	hash := sha256.Sum256([]byte(seed))
	roll := float64(binary.BigEndian.Uint64(hash[0:8])%1000000) / 10000
	switch {
	case roll < round.ProbSmall:
		return 1 + int(hash[8]%5), SizeBetDirectionSmall
	case roll < round.ProbSmall+round.ProbMid:
		return 6, SizeBetDirectionMid
	default:
		return 7 + int(hash[9]%5), SizeBetDirectionBig
	}
}
