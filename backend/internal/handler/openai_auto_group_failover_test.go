//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type autoGroupFailoverAccountRepo struct {
	openAIImagesFailoverAccountRepo
}

func TestOpenAIShouldRetrySameAccount_PreservesFixedGroupPolicy(t *testing.T) {
	err := &service.UpstreamFailoverError{
		StatusCode: http.StatusTooManyRequests, RetryableOnSameAccount: true,
		SameAccountRetryDeadline: time.Now().Add(time.Minute),
	}
	require.True(t, openAIShouldRetrySameAccount(nil, err, false))
	require.True(t, openAIShouldRetrySameAccount(&service.APIKey{}, err, false))
	require.False(t, openAIShouldRetrySameAccount(&service.APIKey{GroupSelectMode: service.APIKeyGroupSelectModeOpenAIAutoCheapest}, err, false))
	require.False(t, openAIShouldRetrySameAccount(nil, nil, false))
	require.True(t, openAIShouldRetrySameAccount(&service.APIKey{GroupSelectMode: service.APIKeyGroupSelectModeOpenAIAutoCheapest}, err, true))
}

func (r autoGroupFailoverAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	var accounts []service.Account
	for _, account := range r.accounts {
		for _, id := range account.GroupIDs {
			if id == groupID && account.Platform == platform {
				accounts = append(accounts, account)
				break
			}
		}
	}
	return accounts, nil
}

type autoGroupFailoverUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
}

type autoGroupFailoverUsageRepo struct {
	service.UsageLogRepository
	logs []*service.UsageLog
}

func (r *autoGroupFailoverUsageRepo) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	r.logs = append(r.logs, log)
	return true, nil
}

func (u *autoGroupFailoverUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	status := http.StatusServiceUnavailable
	body := `{"error":{"type":"server_error","message":"upstream unavailable"}}`
	contentType := "application/json"
	if accountID == 30 {
		status = http.StatusOK
		body = `{"id":"resp_higher_group","object":"response","model":"gpt-5.1","status":"completed","output":[{"id":"msg_ok","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"higher group reached"}]}],"usage":{"input_tokens":0,"output_tokens":0}}`
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if gjson.GetBytes(payload, "stream").Bool() {
			contentType = "text/event-stream"
			body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"higher group reached\"}\n\ndata: {\"type\":\"response.completed\",\"response\":" + body + "}\n\n"
		}
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
}

func TestOpenAIAutoGroupFailover_PrefersOtherGroupWithinSwitchBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"responses", "chat/completions", "messages"} {
		for _, maxRate := range []float64{0.5, 0.2} {
			name := endpoint + "/allows_higher_group"
			if maxRate == 0.2 {
				name = endpoint + "/respects_rate_cap"
			}
			t.Run(name, func(t *testing.T) {
				groups := []service.Group{
					{ID: 20, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 0.2, AllowAutoCheapestScheduling: true, AllowMessagesDispatch: true},
					{ID: 30, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 0.3, AllowAutoCheapestScheduling: true, AllowMessagesDispatch: true},
				}
				var accounts []service.Account
				for _, id := range []int64{20, 21, 22, 23, 24, 30} {
					groupID := int64(20)
					if id == 30 {
						groupID = 30
					}
					accounts = append(accounts, service.Account{
						ID: id, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
						Status: service.StatusActive, Schedulable: true, Priority: int(id), GroupIDs: []int64{groupID},
						Credentials: map[string]any{"api_key": "test", "pool_mode": true, "pool_mode_retry_status_codes": []any{float64(503)}},
					})
				}
				repo := autoGroupFailoverAccountRepo{openAIImagesFailoverAccountRepo{accounts: accounts}}
				upstream := &autoGroupFailoverUpstream{}
				usage := &autoGroupFailoverUsageRepo{}
				cfg := &config.Config{}
				// Keep real group membership selection while isolating billing.
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}, nil)
				t.Cleanup(billing.Stop)
				gateway := service.NewOpenAIGatewayService(repo, usage, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
				gateway.SetOpenAIAutoCheapestGroupResolver(service.NewOpenAIAutoCheapestGroupResolver(&inflightAutoGroupsProvider{groups: groups}), nil)
				h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
				h.maxAccountSwitches = 1
				key := &service.APIKey{ID: 1, UserID: 9, User: &service.User{ID: 9}, GroupSelectMode: service.APIKeyGroupSelectModeOpenAIAutoCheapest, OpenAIAutoGroupMaxRateMultiplier: &maxRate}
				body := `{"model":"gpt-5.1","stream":false,"max_tokens":10,"input":"hello","messages":[{"role":"user","content":"hello"}]}`
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, bytes.NewBufferString(body))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
				switch endpoint {
				case "responses":
					h.Responses(c)
				case "messages":
					h.Messages(c)
				default:
					h.ChatCompletions(c)
				}
				if maxRate == 0.5 {
					require.Equal(t, []int64{20, 30}, upstream.accountIDs, rec.Body.String())
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
					require.Contains(t, rec.Body.String(), "higher group reached")
					require.Len(t, usage.logs, 1)
					require.Equal(t, int64(30), *usage.logs[0].GroupID, "usage must follow the selected group")
				} else {
					require.Equal(t, []int64{20, 21}, upstream.accountIDs, rec.Body.String())
				}
			})
		}
	}
}
