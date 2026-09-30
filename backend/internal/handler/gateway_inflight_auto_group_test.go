//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type inflightAutoGroupsProvider struct{ groups []service.Group }

func (p *inflightAutoGroupsProvider) GetAvailableGroups(context.Context, int64) ([]service.Group, error) {
	return p.groups, nil
}

func (*inflightAutoGroupsProvider) GetUserGroupRates(context.Context, int64) (map[int64]float64, error) {
	return nil, nil
}

type recordingInflightCache struct {
	*handlerInflightCache
	estimates []float64
}

func (c *recordingInflightCache) ReserveInflightBalance(ctx context.Context, uid int64, id string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	c.mu.Lock()
	c.estimates = append(c.estimates, amount)
	c.mu.Unlock()
	return c.handlerInflightCache.ReserveInflightBalance(ctx, uid, id, amount, balance, ttl)
}

func TestAutomaticGroupInflightUsesSelectedGroupPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"responses", "messages", "chat/completions"} {
		for _, failClosed := range []bool{false, true} {
			name := endpoint + "/fail_open"
			if failClosed {
				name = endpoint + "/fail_closed"
			}
			t.Run(name, func(t *testing.T) {
				cfg := &config.Config{}
				cfg.Default.RateMultiplier = 1
				cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60, FailClosedOnUnpriced: failClosed}
				price := 0.5
				group := service.Group{ID: 10, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, AllowAutoCheapestScheduling: true, AllowMessagesDispatch: true,
					ModelPricing: []service.ChannelModelPricing{{Models: []string{"audit-priced-alias"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &price}}}
				key := &service.APIKey{ID: 7, UserID: 7, User: &service.User{ID: 7, Status: service.StatusActive}, GroupSelectMode: service.APIKeyGroupSelectModeOpenAIAutoCheapest}
				cache := &recordingInflightCache{handlerInflightCache: newHandlerInflightCache(0.6)}
				billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				held, err := billing.ReserveInflight(context.Background(), key.User, nil, nil, 0.2)
				require.NoError(t, err)
				defer held.HandlerDone()
				repo := &grokCredentialHandlerRepo{accounts: []service.Account{{ID: 11, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{10}, Credentials: map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"audit-priced-alias": "gpt-5.1"}}}}}
				upstream := &grokCredentialHandlerUpstream{}
				billingService := service.NewBillingService(cfg, nil)
				gw := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, billingService, nil, billing, upstream, &service.DeferredService{}, nil, nil, service.NewModelPricingResolver(nil, billingService), nil, nil, nil, nil)
				gw.SetOpenAIAutoCheapestGroupResolver(service.NewOpenAIAutoCheapestGroupResolver(&inflightAutoGroupsProvider{groups: []service.Group{group}}), nil)
				concurrency := &concurrencyCacheMock{
					acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
					acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
				}
				h := NewOpenAIGatewayHandler(gw, service.NewConcurrencyService(concurrency), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
				body := []byte(`{"model":"audit-priced-alias","max_tokens":10,"input":"hello","messages":[{"role":"user","content":"hello"}]}`)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, bytes.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7, Concurrency: 1})
				switch endpoint {
				case "responses":
					h.Responses(c)
				case "messages":
					h.Messages(c)
				default:
					h.ChatCompletions(c)
				}
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Empty(t, upstream.accountHits(), "余额预留拒绝后不能请求上游")
				require.Equal(t, []float64{0.2, 0.5}, cache.estimates, "必须先选择分组，再使用该分组的价格")
				require.Equal(t, 1, cache.count(), "拒绝后仅保留原有预留")
			})
		}
	}
}
