//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type runtimeBlockRecorder struct {
	accounts   []*Account
	until      []time.Time
	reasons    []string
	clearedIDs []int64
}

func (r *runtimeBlockRecorder) BlockAccountScheduling(account *Account, until time.Time, reason string) {
	r.accounts = append(r.accounts, account)
	r.until = append(r.until, until)
	r.reasons = append(r.reasons, reason)
}

func (r *runtimeBlockRecorder) ClearAccountSchedulingBlock(accountID int64) {
	r.clearedIDs = append(r.clearedIDs, accountID)
}

func TestRateLimitService_HandleUpstreamError_OpenAI403FirstHitTempUnschedulable(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{1}}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	service.SetAccountRuntimeBlocker(blocker)
	account := &Account{
		ID:       301,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"temporary edge rejection"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, "temporary edge rejection")
	require.Contains(t, repo.lastTempReason, "(1/3)")
	require.Len(t, blocker.accounts, 1)
	require.Equal(t, account.ID, blocker.accounts[0].ID)
	require.Equal(t, "openai_403_temp", blocker.reasons[0])
	require.True(t, blocker.until[0].After(time.Now()))
}

func TestRateLimitService_HandleUpstreamError_OpenAI403ThresholdDisables(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{3}}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAI403CounterCache(counter)
	account := &Account{
		ID:       302,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"workspace forbidden by policy"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.Contains(t, repo.lastErrorMsg, "workspace forbidden by policy")
	require.Contains(t, repo.lastErrorMsg, "consecutive_403=3/3")
}

func TestOpenAI403ModelAccessFailureOnlyCoolsDownAccountModel(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{1}}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rateLimitService.SetOpenAI403CounterCache(counter)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	account := &Account{ID: 303, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	shouldDisable := gateway.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"code":"model_access_denied","message":"You do not have access to model gpt-5.5"}}`),
		"gpt-5.5",
	)

	require.True(t, shouldDisable)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, account.ID, repo.modelRateLimitCalls[0].accountID)
	require.Equal(t, "gpt-5.5", repo.modelRateLimitCalls[0].scope)
	require.Len(t, counter.counts, 1, "model-scoped 403 must not increment account-level 403 failures")
	require.Zero(t, repo.setErrCalls)
	require.Zero(t, repo.tempCalls)
}

func TestOpenAI403ContentPolicyDoesNotCoolDownAccount(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	counter := &openAI403CounterCacheStub{counts: []int64{1}}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rateLimitService.SetOpenAI403CounterCache(counter)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	account := &Account{ID: 304, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	shouldDisable := gateway.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"code":"content_policy_violation","message":"request blocked"}}`),
		"gpt-5.5",
	)

	require.False(t, shouldDisable)
	require.Len(t, counter.counts, 1)
	require.Empty(t, repo.modelRateLimitCalls)
	require.Zero(t, repo.setErrCalls)
	require.Zero(t, repo.tempCalls)
}
