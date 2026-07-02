package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/collection"
)

type sub2APICheckinProviderRepoStub struct {
	AccountRepository
	called chan struct{}
}

func (r *sub2APICheckinProviderRepoStub) ListSub2APICheckinCandidates(context.Context, int) ([]Account, error) {
	select {
	case r.called <- struct{}{}:
	default:
	}
	return nil, nil
}

func TestProvideTimingWheelService_ReturnsError(t *testing.T) {
	original := newTimingWheel
	t.Cleanup(func() { newTimingWheel = original })

	newTimingWheel = func(_ time.Duration, _ int, _ collection.Execute) (*collection.TimingWheel, error) {
		return nil, errors.New("boom")
	}

	svc, err := ProvideTimingWheelService()
	if err == nil {
		t.Fatalf("期望返回 error，但得到 nil")
	}
	if svc != nil {
		t.Fatalf("期望返回 nil svc，但得到非空")
	}
}

func TestProvideTimingWheelService_Success(t *testing.T) {
	svc, err := ProvideTimingWheelService()
	if err != nil {
		t.Fatalf("期望 err 为 nil，但得到: %v", err)
	}
	if svc == nil {
		t.Fatalf("期望 svc 非空，但得到 nil")
	}
	svc.Stop()
}

func TestProvideSub2APICheckinService_StartsWorker(t *testing.T) {
	repo := &sub2APICheckinProviderRepoStub{called: make(chan struct{}, 1)}

	svc := ProvideSub2APICheckinService(repo, nil)
	requireNotNil(t, svc)
	t.Cleanup(func() {
		svc.Stop()
	})

	select {
	case <-repo.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected check-in worker to query candidates after provider start")
	}
}

func requireNotNil(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		t.Fatal("expected non-nil value")
	}
}
