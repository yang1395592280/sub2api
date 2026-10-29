package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

// Start drains persisted jobs, including queued work left by a restart. Running
// work is never replayed: its acceptance at the upstream may be ambiguous.
func (s *PelicanTestService) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.wg.Add(4)
		for i := 0; i < 3; i++ {
			go s.worker(ctx)
		}
		go s.scheduleLoop(ctx)
	})
}

func (s *PelicanTestService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
	s.wg.Wait()
}

func (s *PelicanTestService) worker(ctx context.Context) {
	defer s.wg.Done()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		test, err := s.repo.ClaimNext(claimCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("pelican.claim_failed", "error", err)
		}
		if test != nil && err == nil {
			s.runOne(ctx, *test)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (s *PelicanTestService) scheduleLoop(ctx context.Context) {
	defer s.wg.Done()
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	for {
		cycleCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := s.repo.ExpireStale(cycleCtx)
		if err == nil {
			err = s.repo.EnqueueDueGroups(cycleCtx)
		}
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("pelican.schedule_failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (s *PelicanTestService) GetGroupConfig(ctx context.Context, groupID int64) (*GroupPelicanConfig, error) {
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.Platform != PlatformOpenAI {
		return nil, fmt.Errorf("%w: only OpenAI groups support scheduled pelican tests", ErrInvalidPelicanBatch)
	}
	return s.repo.GetGroupConfig(ctx, groupID)
}

func (s *PelicanTestService) SaveGroupConfig(ctx context.Context, config GroupPelicanConfig) (*GroupPelicanConfig, error) {
	if _, err := s.GetGroupConfig(ctx, config.GroupID); err != nil {
		return nil, err
	}
	config.ModelID = strings.TrimSpace(config.ModelID)
	config.Prompt = strings.TrimSpace(config.Prompt)
	if config.Prompt == "" {
		config.Prompt = DefaultPelicanTestPrompt
	}
	if len(config.ModelID) > 255 || utf8.RuneCountInString(config.Prompt) > PelicanTestMaxPrompt ||
		(config.Enabled && config.ModelID == "") || (config.IntervalMinutes != 0 && config.IntervalMinutes != 30) {
		return nil, fmt.Errorf("%w: a model and a prompt of at most 4000 characters are required; interval must be 30 minutes", ErrInvalidPelicanBatch)
	}
	if config.Enabled && (isImageGenerationModel(config.ModelID) || isOpenAIImageModel(config.ModelID) || isGrokVideoGenerationModel(config.ModelID)) {
		return nil, fmt.Errorf("%w: select a text generation model", ErrInvalidPelicanBatch)
	}
	if err := s.repo.SaveGroupConfig(ctx, config); err != nil {
		return nil, err
	}
	return s.repo.GetGroupConfig(ctx, config.GroupID)
}

func (s *PelicanTestService) ListHistory(ctx context.Context, accountID int64, page, pageSize int) (*PelicanTestHistory, error) {
	if accountID <= 0 || page < 1 || page > 1000000 || pageSize < 1 || pageSize > 50 {
		return nil, fmt.Errorf("%w: invalid account or pagination", ErrInvalidPelicanBatch)
	}
	return s.repo.ListHistory(ctx, accountID, page, pageSize)
}
