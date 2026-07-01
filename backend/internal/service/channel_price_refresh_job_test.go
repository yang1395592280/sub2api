package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type channelPriceRefreshAccountRepoStub struct {
	accounts []Account
}

func (s *channelPriceRefreshAccountRepoStub) ListActive(ctx context.Context) ([]Account, error) {
	return append([]Account(nil), s.accounts...), nil
}

type upstreamBalanceRefresherStub struct {
	mu           sync.Mutex
	calls        []int64
	errors       map[int64]error
	inFlight     int
	maxInFlight  int
	sleepPerCall time.Duration
}

func (s *upstreamBalanceRefresherStub) Refresh(ctx context.Context, accountID int64) (*Account, error) {
	s.mu.Lock()
	s.calls = append(s.calls, accountID)
	s.inFlight++
	if s.inFlight > s.maxInFlight {
		s.maxInFlight = s.inFlight
	}
	s.mu.Unlock()

	if s.sleepPerCall > 0 {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.inFlight--
			s.mu.Unlock()
			return nil, ctx.Err()
		case <-time.After(s.sleepPerCall):
		}
	}

	s.mu.Lock()
	s.inFlight--
	err := s.errors[accountID]
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &Account{ID: accountID}, nil
}

func (s *upstreamBalanceRefresherStub) callIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int64(nil), s.calls...)
}

func (s *upstreamBalanceRefresherStub) observedMaxInFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxInFlight
}

func TestChannelPriceRefreshJob_RunOnceRefreshesEligibleAccountsAndKeepsGoingAfterFailure(t *testing.T) {
	repo := &channelPriceRefreshAccountRepoStub{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey},
		{ID: 3, Platform: PlatformGemini, Type: AccountTypeAPIKey},
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}}
	refresher := &upstreamBalanceRefresherStub{
		errors: map[int64]error{2: errors.New("upstream failed")},
	}
	job := NewChannelPriceRefreshJob(repo, refresher, nil, &config.Config{
		ChannelPriceRefresh: config.ChannelPriceRefreshConfig{
			Enabled:        true,
			Concurrency:    2,
			TimeoutSeconds: 1,
		},
	})

	result := job.RunOnce(context.Background())

	if result.Attempted != 3 || result.Success != 2 || result.Failed != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	assertSameInt64Set(t, []int64{1, 2, 5}, refresher.callIDs())
}

func TestChannelPriceRefreshJob_DisabledDoesNothing(t *testing.T) {
	repo := &channelPriceRefreshAccountRepoStub{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}}
	refresher := &upstreamBalanceRefresherStub{}
	job := NewChannelPriceRefreshJob(repo, refresher, nil, &config.Config{
		ChannelPriceRefresh: config.ChannelPriceRefreshConfig{
			Enabled: false,
		},
	})

	result := job.RunOnce(context.Background())

	if result.Attempted != 0 || result.Success != 0 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := refresher.callIDs(); len(got) != 0 {
		t.Fatalf("expected no refresh calls, got %v", got)
	}
}

func TestChannelPriceRefreshJob_UsesConcurrencyLimit(t *testing.T) {
	repo := &channelPriceRefreshAccountRepoStub{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	}}
	refresher := &upstreamBalanceRefresherStub{sleepPerCall: 20 * time.Millisecond}
	job := NewChannelPriceRefreshJob(repo, refresher, nil, &config.Config{
		ChannelPriceRefresh: config.ChannelPriceRefreshConfig{
			Enabled:        true,
			Concurrency:    2,
			TimeoutSeconds: 1,
		},
	})

	result := job.RunOnce(context.Background())

	if result.Attempted != 4 || result.Success != 4 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := refresher.observedMaxInFlight(); got > 2 {
		t.Fatalf("expected max in-flight <= 2, got %d", got)
	}
}

func assertSameInt64Set(t *testing.T, want, got []int64) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("expected calls %v, got %v", want, got)
	}
	counts := make(map[int64]int, len(want))
	for _, id := range want {
		counts[id]++
	}
	for _, id := range got {
		counts[id]--
	}
	for id, count := range counts {
		if count != 0 {
			t.Fatalf("expected calls %v, got %v (id %d count delta %d)", want, got, id, count)
		}
	}
}
