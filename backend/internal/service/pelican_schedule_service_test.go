package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type pelicanConfigGroups struct {
	GroupRepository
	group *Group
}

func (g pelicanConfigGroups) GetByIDLite(context.Context, int64) (*Group, error) { return g.group, nil }

type pelicanConfigRepo struct {
	PelicanTestRepository
	config GroupPelicanConfig
	saves  int
}

func (r *pelicanConfigRepo) GetGroupConfig(context.Context, int64) (*GroupPelicanConfig, error) {
	c := r.config
	return &c, nil
}
func (r *pelicanConfigRepo) SaveGroupConfig(_ context.Context, c GroupPelicanConfig) error {
	r.config = c
	r.saves++
	return nil
}

func TestPelicanGroupConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    GroupPelicanConfig
	}{
		{"missing model", GroupPelicanConfig{Enabled: true}},
		{"media model", GroupPelicanConfig{Enabled: true, ModelID: "gpt-image-1"}},
		{"invalid interval", GroupPelicanConfig{IntervalMinutes: 1}},
		{"long model", GroupPelicanConfig{ModelID: strings.Repeat("m", 256)}},
		{"long prompt", GroupPelicanConfig{Prompt: strings.Repeat("题", 4001)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &pelicanConfigRepo{}
			s := &PelicanTestService{repo: r, groupRepo: pelicanConfigGroups{group: &Group{Platform: PlatformOpenAI}}}
			_, err := s.SaveGroupConfig(context.Background(), tc.c)
			require.ErrorIs(t, err, ErrInvalidPelicanBatch)
			require.Zero(t, r.saves)
		})
	}
	r := &pelicanConfigRepo{}
	s := &PelicanTestService{repo: r, groupRepo: pelicanConfigGroups{group: &Group{Platform: PlatformOpenAI}}}
	c, err := s.SaveGroupConfig(context.Background(), GroupPelicanConfig{GroupID: 7, Enabled: true, ModelID: " gpt-5.5 "})
	require.NoError(t, err)
	require.Equal(t, "gpt-5.5", c.ModelID)
	require.Equal(t, DefaultPelicanTestPrompt, c.Prompt)
	_, err = s.SaveGroupConfig(context.Background(), GroupPelicanConfig{GroupID: 7})
	require.NoError(t, err, "turning off needs no model")
	s.groupRepo = pelicanConfigGroups{group: &Group{Platform: PlatformAnthropic}}
	_, err = s.GetGroupConfig(context.Background(), 7)
	require.ErrorIs(t, err, ErrInvalidPelicanBatch)
}
func TestPelicanHistoryRejectsUnboundedPagination(t *testing.T) {
	s := &PelicanTestService{}
	for _, a := range [][3]int{{0, 1, 12}, {1, 0, 12}, {1, 1, 51}, {1, 1000001, 12}} {
		_, err := s.ListHistory(context.Background(), int64(a[0]), a[1], a[2])
		require.ErrorIs(t, err, ErrInvalidPelicanBatch)
	}
}

type pelicanResumeRepo struct {
	PelicanTestRepository
	mu       sync.Mutex
	claimed  bool
	finished chan string
}

func (r *pelicanResumeRepo) ClaimNext(context.Context) (*PelicanTest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return nil, nil
	}
	r.claimed = true
	return &PelicanTest{ID: 1, AccountID: 10}, nil
}
func (r *pelicanResumeRepo) ExpireStale(context.Context) error      { return nil }
func (r *pelicanResumeRepo) EnqueueDueGroups(context.Context) error { return nil }
func (r *pelicanResumeRepo) Finish(_ context.Context, _ int64, status, _, _, _ string, _ int64, _ AccountTestTokenUsage) error {
	r.finished <- status
	return nil
}

type pelicanMissingAccount struct{ AccountRepository }

func (pelicanMissingAccount) GetByID(context.Context, int64) (*Account, error) {
	return nil, errors.New("deleted")
}
func TestPelicanWorkerResumesPersistedQueueWithoutReplayingClaims(t *testing.T) {
	r := &pelicanResumeRepo{finished: make(chan string, 3)}
	s := NewPelicanTestService(r, pelicanMissingAccount{}, nil)
	s.Start()
	s.Start()
	select {
	case status := <-r.finished:
		require.Equal(t, "failed", status)
	case <-time.After(3 * time.Second):
		t.Fatal("persisted job not consumed")
	}
	s.Stop()
	s.Stop()
	require.Empty(t, r.finished)
}
