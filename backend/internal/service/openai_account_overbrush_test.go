package service

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fixedOpenAIOverbrushSettingService(t *testing.T, threshold int) *SettingService {
	t.Helper()

	settings, err := json.Marshal(OpenAIOverbrushSettings{Consecutive429Threshold: threshold})
	require.NoError(t, err)

	return NewSettingService(&rateLimit429SettingRepoStub{data: map[string]string{
		SettingKeyOpenAIOverbrushSettings: string(settings),
	}}, nil)
}

func openAIOverbrushAPIKeyAccount() *Account {
	return &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{},
		Extra:       map[string]any{"openai_overbrush_enabled": true},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func TestOpenAIOverbrush429SkipsLimitBeforeThreshold(t *testing.T) {
	svc := &OpenAIGatewayService{settingService: fixedOpenAIOverbrushSettingService(t, 3)}
	account := openAIOverbrushAPIKeyAccount()

	require.True(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
	require.True(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
	require.False(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
}

func TestOpenAIOverbrush429ConcurrentThresholdUpdateIsAtomic(t *testing.T) {
	const requestCount = 256

	svc := &OpenAIGatewayService{settingService: fixedOpenAIOverbrushSettingService(t, 2)}
	account := openAIOverbrushAPIKeyAccount()
	start := make(chan struct{})
	results := make(chan bool, requestCount)
	var waitGroup sync.WaitGroup

	for range requestCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results <- svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests)
		}()
	}

	close(start)
	waitGroup.Wait()
	close(results)

	skipped := 0
	for shouldSkip := range results {
		if shouldSkip {
			skipped++
		}
	}

	require.Equal(t, requestCount/2, skipped)
}

func TestOpenAIOverbrush429SuccessResetStartsNewSequence(t *testing.T) {
	svc := &OpenAIGatewayService{settingService: fixedOpenAIOverbrushSettingService(t, 2)}
	account := openAIOverbrushAPIKeyAccount()

	require.True(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
	svc.ResetOpenAIOverbrush429Count(account)

	// The next 429 starts a fresh sequence at one instead of using the prior count.
	require.True(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
	require.False(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
}

func TestResetOpenAIOverbrush429Count_RemovesStoredCounter(t *testing.T) {
	svc := &OpenAIGatewayService{settingService: fixedOpenAIOverbrushSettingService(t, 2)}
	account := openAIOverbrushAPIKeyAccount()

	require.True(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
	_, loaded := svc.openaiOverbrush429Counts.Load(account.ID)
	require.True(t, loaded)

	svc.ResetOpenAIOverbrush429Count(account)

	_, loaded = svc.openaiOverbrush429Counts.Load(account.ID)
	require.False(t, loaded)
}

func TestOpenAIOverbrush429NotApplicable(t *testing.T) {
	svc := &OpenAIGatewayService{settingService: fixedOpenAIOverbrushSettingService(t, 3)}

	for _, account := range []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{}, Extra: map[string]any{"openai_overbrush_enabled": true}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Credentials: map[string]any{}, Extra: map[string]any{"openai_overbrush_enabled": true}},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"upstream_admin_type": "sub2api"}, Extra: map[string]any{"openai_overbrush_enabled": true}},
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}, Extra: map[string]any{}},
		{ID: 5, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{}, Extra: map[string]any{"openai_overbrush_enabled": true}},
	} {
		require.False(t, svc.shouldSkipOpenAI429LimitForOverbrush(context.Background(), account, http.StatusTooManyRequests))
	}
}

func TestHandleOpenAIAccountUpstreamError_OverbrushDefersExistingRateLimit(t *testing.T) {
	repo := &openAIOverbrushAccountRepoStub{}
	rateLimitSvc := &RateLimitService{accountRepo: repo}
	svc := &OpenAIGatewayService{
		rateLimitService: rateLimitSvc,
		settingService:   fixedOpenAIOverbrushSettingService(t, 2),
	}
	account := openAIOverbrushAPIKeyAccount()

	disabled := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, nil)

	require.False(t, disabled)
	require.Empty(t, repo.rateLimitedIDs)
	_, runtimeBlocked := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, runtimeBlocked)

	disabled = svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, nil)

	require.False(t, disabled)
	require.Len(t, repo.rateLimitedIDs, 1)
}

type openAIOverbrushAccountRepoStub struct {
	stubAntigravityAccountRepo
	rateLimitedIDs []int64
}

func (r *openAIOverbrushAccountRepoStub) SetRateLimited(_ context.Context, id int64, _ time.Time) error {
	r.rateLimitedIDs = append(r.rateLimitedIDs, id)
	return nil
}
