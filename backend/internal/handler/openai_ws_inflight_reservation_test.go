//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/testutil"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type wsInflightBillingCache struct{ *handlerInflightCache }

func (*wsInflightBillingCache) InvalidateUserBalance(context.Context, int64) error { return nil }

type wsInflightUserRepo struct {
	service.UserRepository
	cache *wsInflightBillingCache
}

func (r *wsInflightUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	return &service.User{ID: 1751, Status: service.StatusActive, Balance: r.cache.balance}, nil
}

func (r *wsInflightUserRepo) DeductBalance(_ context.Context, _ int64, amount float64) error {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	r.cache.balance -= amount
	return nil
}

func TestWebSocketCompletedTurnReleasesReservationWhileIdle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, _, err = conn.Read(ctx); err != nil {
			t.Error(err)
			return
		}
		if err = conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_inflight_idle","model":"gpt-5.1","usage":{"input_tokens":2,"output_tokens":1}}}`)); err != nil {
			t.Error(err)
			return
		}
		_, _, _ = conn.Read(ctx)
	}))
	t.Cleanup(upstreamServer.Close)
	harness, cache := newInflightWSHarness(t, upstreamServer.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, harness.clientConn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"hello"}`)))
	_, event, err := harness.clientConn.Read(ctx)
	require.NoError(t, err)
	require.Contains(t, string(event), "response.completed")
	require.Eventually(t, func() bool {
		balance, _ := cache.GetUserBalance(ctx, 1751)
		return balance < 0.05 && cache.count() == 0
	}, time.Second, 5*time.Millisecond)
	balance, err := cache.GetUserBalance(ctx, 1751)
	require.NoError(t, err)
	require.Greater(t, balance, 0.001)
	release, err := harness.handler.billingCacheService.ReserveInflightBalance(ctx, harness.apiKey.User, nil, nil, 0.001)
	defer release()
	t.Logf("completed idle websocket: balance=%g, idle reservations=%d; unrelated 0.001 request error=%v", balance, cache.count(), err)
	require.NoError(t, err, "a completed idle websocket must not block an unrelated affordable request")
}

func TestWebSocketFollowUpTurnReservesItsOwnEstimateBeforeForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var forwarded atomic.Int32
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
			forwarded.Add(1)
			if err := conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_turn","model":"gpt-5.1","usage":{"input_tokens":2,"output_tokens":1}}}`)); err != nil {
				return
			}
		}
	}))
	t.Cleanup(upstreamServer.Close)
	harness, cache := newInflightWSHarness(t, upstreamServer.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, harness.clientConn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"hello","max_output_tokens":1}`)))
	_, _, err := harness.clientConn.Read(ctx)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		balance, _ := cache.GetUserBalance(ctx, 1751)
		return balance < 0.05 && cache.count() == 0
	}, time.Second, 5*time.Millisecond)
	held, err := harness.handler.billingCacheService.ReserveInflight(ctx, harness.apiKey.User, nil, nil, 0.02)
	require.NoError(t, err)
	defer held.HandlerDone()
	// 第二轮的输出上限提高到 2000，预留约 0.03，超过余额减去其他请求预留后的余量。
	require.NoError(t, harness.clientConn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"hello","max_output_tokens":2000}`)))
	_, _, err = harness.clientConn.Read(ctx)
	require.Equal(t, coderws.StatusPolicyViolation, coderws.CloseStatus(err))
	require.Equal(t, int32(1), forwarded.Load(), "被余额预留拒绝的第二轮不得转发")
	require.Equal(t, 1, cache.count(), "失败轮次不能泄漏预留")
}

func TestWebSocketAutomaticGroupReservesSelectedGroupPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var upstreamDials atomic.Int32
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamDials.Add(1)
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
	}))
	t.Cleanup(upstreamServer.Close)
	harness, cache := newInflightWSHarness(t, upstreamServer.URL)
	key := harness.apiKey
	key.UserID = key.User.ID
	key.GroupID = nil
	key.Group = nil
	key.GroupSelectMode = service.APIKeyGroupSelectModeOpenAIAutoCheapest
	price := 0.5
	group := service.Group{ID: 4301, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, AllowAutoCheapestScheduling: true,
		ModelPricing: []service.ChannelModelPricing{{Models: []string{"priced-ws-alias"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &price}}}
	harness.handler.gatewayService.SetOpenAIAutoCheapestGroupResolver(service.NewOpenAIAutoCheapestGroupResolver(&inflightAutoGroupsProvider{groups: []service.Group{group}}), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, err := harness.handler.billingCacheService.ReserveInflight(ctx, key.User, nil, nil, 0.01)
	require.NoError(t, err)
	defer held.HandlerDone()
	require.NoError(t, harness.clientConn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"priced-ws-alias","input":"hello"}`)))
	_, _, err = harness.clientConn.Read(ctx)
	require.Equal(t, coderws.StatusPolicyViolation, coderws.CloseStatus(err))
	require.Zero(t, upstreamDials.Load(), "实际分组价格超出可用余额时，首轮也不能连接上游")
	require.Equal(t, 1, cache.count())
}

func newInflightWSHarness(t *testing.T, upstreamURL string, settings ...map[string]string) (*openAIWSPassthroughHandlerHarness, *wsInflightBillingCache) {
	t.Helper()
	gatewayCache := testutil.NewRedisGatewayCache(t)

	settingRepo := &contentModerationHandlerSettingRepo{values: map[string]string{
		service.SettingKeyRiskControlEnabled:          "true",
		service.SettingKeyCyberSessionBlockEnabled:    "true",
		service.SettingKeyCyberSessionBlockTTLSeconds: "60",
	}}
	for _, overrides := range settings {
		for key, value := range overrides {
			settingRepo.values[key] = value
		}
	}
	moderationRepo := &contentModerationHandlerTestRepo{}
	moderationSvc := service.NewContentModerationService(settingRepo, moderationRepo, nil, nil, nil, nil, nil, nil)
	settingSvc := service.NewSettingService(settingRepo, nil)

	groupID := int64(4301)
	account := service.Account{
		ID:          9951,
		Name:        "openai-ws-passthrough-cyber",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": upstreamURL},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
		},
	}
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeStandard
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60, DefaultMaxTokens: 8192}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 3

	accountRepo := &openAIWSUsageHandlerAccountRepoStub{account: account}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
	balanceCache := &wsInflightBillingCache{newHandlerInflightCache(0.05)}
	userRepo := &wsInflightUserRepo{cache: balanceCache}
	billingCacheSvc := service.NewBillingCacheService(balanceCache, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheSvc.Stop)
	billingService := service.NewBillingService(cfg, nil)
	gatewaySvc := service.NewOpenAIGatewayService(
		accountRepo, usageRepo, nil, userRepo, nil, nil, gatewayCache, cfg, nil, nil,
		billingService, nil, billingCacheSvc, nil, &service.DeferredService{},
		nil, nil, service.NewModelPricingResolver(nil, billingService), nil, nil, settingSvc, nil,
	)
	concurrencyCache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := &OpenAIGatewayHandler{
		cfg:                      cfg,
		gatewayService:           gatewaySvc,
		billingCacheService:      billingCacheSvc,
		apiKeyService:            &service.APIKeyService{},
		contentModerationService: moderationSvc,
		concurrencyHelper:        NewConcurrencyHelper(service.NewConcurrencyService(concurrencyCache), SSEPingFormatNone, time.Second),
	}

	apiKey := &service.APIKey{
		ID:      1851,
		UserID:  1751,
		Name:    "ws-cyber-key",
		Key:     "sk-handler-cyber-test",
		GroupID: &groupID,
		User:    &service.User{ID: 1751, Status: service.StatusActive},
	}
	handlerDone := make(chan struct{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.GET("/openai/v1/responses", func(c *gin.Context) {
		h.ResponsesWebSocket(c)
		close(handlerDone)
	})
	handlerServer := httptest.NewServer(router)
	t.Cleanup(handlerServer.Close)

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(handlerServer.URL, "http")+"/openai/v1/responses", nil)
	cancelDial()
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.CloseNow() })

	return &openAIWSPassthroughHandlerHarness{
		handler:        h,
		clientConn:     clientConn,
		handlerDone:    handlerDone,
		moderationRepo: moderationRepo,
		gatewayCache:   gatewayCache,
		apiKey:         apiKey,
	}, balanceCache
}
