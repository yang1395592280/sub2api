//go:build unit

package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type fixedGroupFailoverUpstream struct {
	service.HTTPUpstream
	failure    string
	accountIDs []int64
}

type fixedGroupFailoverBillingRepo struct {
	service.UsageBillingRepository
}

func (fixedGroupFailoverBillingRepo) Apply(context.Context, *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	return &service.UsageBillingApplyResult{}, nil
}

func (u *fixedGroupFailoverUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	status := http.StatusForbidden
	contentType := "application/json"
	body := `{"error":{"type":"upstream_error","message":"Upstream access forbidden, please contact administrator"}}`
	if accountID == 30 || accountID == 10 {
		status = http.StatusOK
		body = `{"id":"resp_healthy","object":"response","model":"gpt-5.1","status":"completed","output":[{"id":"msg_ok","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"healthy account reached"}]}],"usage":{"input_tokens":0,"output_tokens":0}}`
		if strings.Contains(req.URL.Path, "/images/") {
			body = `{"created":1,"data":[{"b64_json":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+kvKsAAAAASUVORK5CYII="}]}`
		} else if strings.HasSuffix(req.URL.Path, "/alpha/search") {
			body = `{"results":[{"title":"healthy account reached","url":"https://example.test"}]}`
		}
		payload, _ := io.ReadAll(req.Body)
		if gjson.GetBytes(payload, "stream").Bool() {
			contentType = "text/event-stream"
			body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"healthy account reached\"}\n\ndata: {\"type\":\"response.completed\",\"response\":" + body + "}\n\n"
		}
	} else if u.failure != "http" {
		status = http.StatusOK
		contentType = "text/event-stream"
		switch strings.TrimSuffix(u.failure, "_eof") {
		case "error", "after_output":
			body = "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"upstream_error\",\"message\":\"Upstream access forbidden, please contact administrator\"}}\n\n"
			if u.failure == "after_output" {
				body = "data: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"content_index\":0,\"delta\":\"partial answer\"}\n\n" + body
			}
		case "policy":
			body = "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"type\":\"permission_error\",\"code\":\"content_policy_violation\",\"message\":\"Forbidden content\"}}}\n\n"
		default:
			body = "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_forbidden\",\"status\":\"failed\",\"error\":{\"type\":\"upstream_error\",\"message\":\"Upstream access forbidden, please contact administrator\"}}}\n\n"
		}
		if strings.HasSuffix(u.failure, "_eof") {
			body = strings.TrimRight(body, "\n")
		}
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
}

func runFixedGroupFailover(t *testing.T, endpoint, failure string, stream, passthrough, singleAccount bool) (*fixedGroupFailoverUpstream, *httptest.ResponseRecorder, *autoGroupFailoverUsageRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	group := service.Group{ID: 20, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 0.1, AllowMessagesDispatch: true}
	group.AllowImageGeneration = endpoint == "images/generations"
	var accounts []service.Account
	for _, id := range []int64{10, 20, 30} {
		if singleAccount && id == 30 {
			continue
		}
		groupID := group.ID
		if id == 10 {
			groupID = 99 // A cheaper healthy account outside the selected group must remain unreachable.
		}
		accounts = append(accounts, service.Account{
			ID: id, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Priority: int(id), GroupIDs: []int64{groupID},
			Credentials: map[string]any{"api_key": "test", "base_url": "https://example.test", "pool_mode": true, "pool_mode_retry_count": float64(1)},
			Extra:       map[string]any{"openai_passthrough": passthrough},
		})
	}
	repo := autoGroupFailoverAccountRepo{openAIImagesFailoverAccountRepo{accounts: accounts}}
	upstream := &fixedGroupFailoverUpstream{failure: failure}
	usage := &autoGroupFailoverUsageRepo{}
	cfg := &config.Config{}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(repo, usage, fixedGroupFailoverBillingRepo{}, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
	h.maxAccountSwitches = 1
	key := &service.APIKey{ID: 1, UserID: 9, User: &service.User{ID: 9}, GroupID: &group.ID, Group: &group}
	body := fmt.Sprintf(`{"model":"gpt-5.1","input":"hello","messages":[{"role":"user","content":"hello"}],"max_tokens":10,"stream":%t}`, stream)
	if endpoint == "images/generations" {
		body = `{"model":"gpt-image-1","prompt":"draw a cat","response_format":"b64_json"}`
	} else if endpoint == "alpha/search" {
		body = `{"model":"gpt-5.1","commands":{"search_query":[{"q":"news"}]}}`
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyAPIKey), key)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
	switch endpoint {
	case "images/generations":
		h.Images(c)
	case "alpha/search":
		h.AlphaSearch(c)
	case "messages":
		h.Messages(c)
	case "chat/completions":
		h.ChatCompletions(c)
	default:
		h.Responses(c)
	}
	return upstream, rec, usage
}

func TestOpenAIFixedGroupFailover_PrefersHealthyAccountBeforePoolRetry(t *testing.T) {
	for _, endpoint := range []string{"responses", "chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			for _, failure := range []string{"http", "response.failed", "error", "response.failed_eof", "error_eof"} {
				for _, passthrough := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/stream_%t/%s/passthrough_%t", endpoint, stream, failure, passthrough), func(t *testing.T) {
						upstream, rec, usage := runFixedGroupFailover(t, endpoint, failure, stream, passthrough, false)
						require.Equal(t, []int64{20, 30}, upstream.accountIDs, rec.Body.String())
						require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
						require.Contains(t, rec.Body.String(), "healthy account reached")
						require.Len(t, usage.logs, 1)
						require.Equal(t, int64(20), *usage.logs[0].GroupID, "fixed-group billing must not move to another group")
					})
				}
			}
		}
	}
}

func TestOpenAIFixedGroupFailover_PreservesSingleAccountPoolRetry(t *testing.T) {
	upstream, _, _ := runFixedGroupFailover(t, "responses", "http", false, false, true)
	require.Equal(t, []int64{20, 20}, upstream.accountIDs)
}

func TestOpenAIFixedGroupFailover_ImageAndSearchPreferHealthyAccount(t *testing.T) {
	for _, endpoint := range []string{"images/generations", "alpha/search"} {
		t.Run(endpoint, func(t *testing.T) {
			upstream, rec, usage := runFixedGroupFailover(t, endpoint, "http", false, false, false)
			require.Equal(t, []int64{20, 30}, upstream.accountIDs, rec.Body.String())
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Len(t, usage.logs, 1)
			require.Equal(t, int64(20), *usage.logs[0].GroupID)
		})
	}
}

func TestOpenAIFixedGroupFailover_DoesNotRotateForContentPolicy(t *testing.T) {
	for _, endpoint := range []string{"responses", "chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream_%t", endpoint, stream), func(t *testing.T) {
				upstream, rec, _ := runFixedGroupFailover(t, endpoint, "policy", stream, false, false)
				require.Equal(t, []int64{20}, upstream.accountIDs, rec.Body.String())
				require.NotContains(t, rec.Body.String(), "healthy account reached")
			})
		}
	}
}

func TestOpenAIFixedGroupFailover_DoesNotReplayAfterOutput(t *testing.T) {
	for _, endpoint := range []string{"responses", "chat/completions", "messages"} {
		for _, passthrough := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/passthrough_%t", endpoint, passthrough), func(t *testing.T) {
				upstream, rec, _ := runFixedGroupFailover(t, endpoint, "after_output", true, passthrough, false)
				require.Equal(t, []int64{20}, upstream.accountIDs, rec.Body.String())
				require.Contains(t, rec.Body.String(), "partial answer")
				require.NotContains(t, rec.Body.String(), "healthy account reached")
			})
		}
	}
}
