package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIWSChannelFrameConn struct {
	events chan []byte
	writes chan []byte
	closed chan struct{}
	once   sync.Once
}

func newOpenAIWSChannelFrameConn() *openAIWSChannelFrameConn {
	return &openAIWSChannelFrameConn{
		events: make(chan []byte, 8),
		writes: make(chan []byte, 8),
		closed: make(chan struct{}),
	}
}

func (c *openAIWSChannelFrameConn) WriteJSON(ctx context.Context, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.write(ctx, payload)
}

func (c *openAIWSChannelFrameConn) ReadMessage(ctx context.Context) ([]byte, error) {
	select {
	case payload := <-c.events:
		return append([]byte(nil), payload...), nil
	case <-c.closed:
		return nil, errors.New("upstream closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *openAIWSChannelFrameConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	payload, err := c.ReadMessage(ctx)
	return coderws.MessageText, payload, err
}

func (c *openAIWSChannelFrameConn) WriteFrame(ctx context.Context, _ coderws.MessageType, payload []byte) error {
	return c.write(ctx, payload)
}

func (c *openAIWSChannelFrameConn) write(ctx context.Context, payload []byte) error {
	select {
	case c.writes <- append([]byte(nil), payload...):
		return nil
	case <-c.closed:
		return errors.New("upstream closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *openAIWSChannelFrameConn) Ping(context.Context) error { return nil }

func (c *openAIWSChannelFrameConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

type openAIWSStaticClientDialer struct {
	conn openAIWSClientConn
}

func (d *openAIWSStaticClientDialer) Dial(context.Context, string, http.Header, string) (openAIWSClientConn, int, http.Header, error) {
	return d.conn, http.StatusSwitchingProtocols, nil, nil
}

func TestOpenAIGatewayService_PassthroughOverlappingTurnsKeepFIFOOutcomeIdentity(t *testing.T) {
	tests := []struct {
		name           string
		completeSecond bool
		wantSecond     string
	}{
		{name: "both terminals", completeSecond: true, wantSecond: OpenAIAutoSchedulerEventSuccess},
		{name: "second missing terminal", completeSecond: false, wantSecond: OpenAIAutoSchedulerEventError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.Enabled = false
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			cfg.Gateway.OpenAIWS.Enabled = true
			cfg.Gateway.OpenAIWS.OAuthEnabled = true
			cfg.Gateway.OpenAIWS.APIKeyEnabled = true
			cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
			cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true

			upstream := newOpenAIWSChannelFrameConn()
			groupID := int64(78)
			sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
			recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 8, 1)
			svc := &OpenAIGatewayService{
				cfg:                                cfg,
				httpUpstream:                       &httpUpstreamRecorder{},
				cache:                              &stubGatewayCache{},
				openaiWSResolver:                   NewOpenAIWSProtocolResolver(cfg),
				toolCorrector:                      NewCodexToolCorrector(),
				openaiWSPassthroughDialer:          &openAIWSStaticClientDialer{conn: upstream},
				openAIAutoSchedulerOutcomeRecorder: recorder,
			}
			account := &Account{
				ID: 455, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test"},
				Extra:       map[string]any{"openai_apikey_responses_websockets_v2_mode": OpenAIWSIngressModePassthrough},
			}
			var resultsMu sync.Mutex
			var turnResults []*OpenAIForwardResult
			var turnTimings sync.Map
			firstTiming := NewOpenAIRequestTiming()
			firstTiming.AddQueue(11 * time.Millisecond)
			turnTimings.Store(1, firstTiming)
			hooks := &OpenAIWSIngressHooks{
				BeforeRequest: func(turn int, _ []byte, _ string) error {
					if turn > 1 {
						timing := NewOpenAIRequestTiming()
						timing.AddQueue(22 * time.Millisecond)
						turnTimings.Store(turn, timing)
					}
					return nil
				},
				TimingForTurn: func(turn int) *OpenAIRequestTiming {
					value, _ := turnTimings.Load(turn)
					timing, _ := value.(*OpenAIRequestTiming)
					return timing
				},
				AfterTurn: func(_ int, result *OpenAIForwardResult, turnErr error) {
					if turnErr != nil || result == nil {
						return
					}
					resultsMu.Lock()
					turnResults = append(turnResults, result)
					resultsMu.Unlock()
				},
			}

			serverErrCh := make(chan error, 1)
			wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					serverErrCh <- err
					return
				}
				defer func() { _ = conn.CloseNow() }()
				ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ginCtx.Request = r
				ginCtx.Set("api_key", &APIKey{GroupID: &groupID})
				readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				msgType, firstMessage, readErr := conn.Read(readCtx)
				cancel()
				if readErr != nil {
					serverErrCh <- readErr
					return
				}
				if msgType != coderws.MessageText {
					serverErrCh <- errors.New("unexpected client frame type")
					return
				}
				serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "sk-test", firstMessage, hooks)
			}))
			defer wsServer.Close()

			client, _, err := coderws.Dial(context.Background(), "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
			require.NoError(t, err)
			defer func() { _ = client.CloseNow() }()
			require.NoError(t, client.Write(context.Background(), coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.4","service_tier":"priority","reasoning":{"effort":"high"}}`)))
			require.Contains(t, string(readTestChannel(t, upstream.writes)), "gpt-5.4")

			upstream.events <- []byte(`{"type":"response.created","response":{"id":"resp_overlap_1"}}`)
			_, _, err = client.Read(context.Background())
			require.NoError(t, err)
			time.Sleep(60 * time.Millisecond)
			require.NoError(t, client.Write(context.Background(), coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","service_tier":"flex","reasoning":{"effort":"low"}}`)))
			require.Contains(t, string(readTestChannel(t, upstream.writes)), "gpt-5.1")

			upstream.events <- []byte(`{"type":"response.completed","response":{"id":"resp_overlap_1","usage":{"input_tokens":100,"output_tokens":50}}}`)
			_, _, err = client.Read(context.Background())
			require.NoError(t, err)
			if tt.completeSecond {
				upstream.events <- []byte(`{"type":"response.completed","response":{"id":"resp_overlap_2","usage":{"input_tokens":100,"output_tokens":50}}}`)
				_, _, err = client.Read(context.Background())
				require.NoError(t, err)
			}
			_ = client.Close(coderws.StatusNormalClosure, "done")

			select {
			case serverErr := <-serverErrCh:
				require.NoError(t, serverErr)
			case <-time.After(5 * time.Second):
				t.Fatal("等待 overlapping passthrough 结束超时")
			}
			require.NoError(t, recorder.Stop(context.Background()))
			outcomes := sink.snapshot()
			require.Len(t, outcomes, 2)
			require.Equal(t, "gpt-5.4", outcomes[0].Model)
			require.Equal(t, OpenAIAutoSchedulerEventSuccess, outcomes[0].EventType)
			require.NotNil(t, outcomes[0].LatencyMS)
			require.GreaterOrEqual(t, *outcomes[0].LatencyMS, 40)
			require.Equal(t, "gpt-5.1", outcomes[1].Model)
			require.Equal(t, tt.wantSecond, outcomes[1].EventType)

			if tt.completeSecond {
				resultsMu.Lock()
				captured := append([]*OpenAIForwardResult(nil), turnResults...)
				resultsMu.Unlock()
				require.Len(t, captured, 2)

				usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
				billingSvc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				apiKey := &APIKey{ID: 1001, GroupID: &groupID, Group: &Group{ID: groupID, RateMultiplier: 1}}
				user := &User{ID: 2001}
				require.NoError(t, billingSvc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: captured[0], APIKey: apiKey, User: user, Account: account}))
				firstLog := usageRepo.lastLog
				require.NoError(t, billingSvc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: captured[1], APIKey: apiKey, User: user, Account: account}))
				secondLog := usageRepo.lastLog

				require.Equal(t, "gpt-5.4", firstLog.Model)
				require.Equal(t, "priority", *firstLog.ServiceTier)
				require.Equal(t, "high", *firstLog.ReasoningEffort)
				require.Equal(t, "gpt-5.1", secondLog.Model)
				require.Equal(t, "flex", *secondLog.ServiceTier)
				require.Equal(t, "low", *secondLog.ReasoningEffort)
				require.Equal(t, 11, *firstLog.QueueMs)
				require.Equal(t, 22, *secondLog.QueueMs)
				require.NotNil(t, firstLog.E2EFirstTokenMs)
				require.NotNil(t, secondLog.E2EFirstTokenMs)
				require.Equal(t, captured[0].FirstTokenMs, firstLog.FirstTokenMs)
				require.Equal(t, captured[1].FirstTokenMs, secondLog.FirstTokenMs)
				firstBase := expectedOpenAICost(t, billingSvc, "gpt-5.4", captured[0].Usage, 1)
				secondBase := expectedOpenAICost(t, billingSvc, "gpt-5.1", captured[1].Usage, 1)
				require.InDelta(t, firstBase.TotalCost*2, firstLog.TotalCost, 1e-10)
				require.InDelta(t, secondBase.TotalCost*0.5, secondLog.TotalCost, 1e-10)
			}
		})
	}
}

func TestOpenAIWSPassthroughPendingTurnsConcurrentPopAndDrainDoNotDuplicate(t *testing.T) {
	queue := &openAIWSPassthroughPendingTurns{}
	for i := 0; i < 100; i++ {
		queue.append(time.Now(), "gpt-5", nil, nil, nil)
	}

	start := make(chan struct{})
	results := make(chan openAIWSPassthroughPendingTurn, 100)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for {
			turn, ok := queue.pop()
			if !ok {
				return
			}
			results <- turn
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for _, turn := range queue.drain() {
			results <- turn
		}
	}()
	close(start)
	workers.Wait()
	close(results)

	seen := make(map[int]struct{}, 100)
	for turn := range results {
		require.NotContains(t, seen, turn.turnNo)
		seen[turn.turnNo] = struct{}{}
	}
	require.Len(t, seen, 100)
	for turnNo := 1; turnNo <= 100; turnNo++ {
		require.Contains(t, seen, turnNo)
	}
}

func TestOpenAIWSPassthroughPendingTurnFreezesBillingMetadata(t *testing.T) {
	queue := &openAIWSPassthroughPendingTurns{}
	priority, high := "priority", "high"
	flex, low := "flex", "low"

	queue.append(time.Now(), "gpt-5.4", &priority, &high, nil)
	queue.append(time.Now(), "gpt-5.5", &flex, &low, nil)

	first, ok := queue.pop()
	require.True(t, ok)
	second, ok := queue.pop()
	require.True(t, ok)
	require.Equal(t, "priority", *first.serviceTier)
	require.Equal(t, "high", *first.reasoningEffort)
	require.Equal(t, "flex", *second.serviceTier)
	require.Equal(t, "low", *second.reasoningEffort)
}

func TestOpenAIGatewayService_PassthroughFollowupLocalRejectCompletesLifecycleOnce(t *testing.T) {
	tests := []struct {
		name               string
		beforeRequestErr   error
		configureFastBlock bool
	}{
		{name: "before request error", beforeRequestErr: errors.New("content moderation rejected follow-up")},
		{name: "fast policy block", configureFastBlock: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.Enabled = false
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			cfg.Gateway.OpenAIWS.Enabled = true
			cfg.Gateway.OpenAIWS.OAuthEnabled = true
			cfg.Gateway.OpenAIWS.APIKeyEnabled = true
			cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
			cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true

			upstream := newOpenAIWSChannelFrameConn()
			groupID := int64(81)
			sink := &collectingOpenAIAutoSchedulerOutcomeSink{}
			recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 4, 1)
			svc := &OpenAIGatewayService{
				cfg:                                cfg,
				httpUpstream:                       &httpUpstreamRecorder{},
				cache:                              &stubGatewayCache{},
				openaiWSResolver:                   NewOpenAIWSProtocolResolver(cfg),
				toolCorrector:                      NewCodexToolCorrector(),
				openaiWSPassthroughDialer:          &openAIWSStaticClientDialer{conn: upstream},
				openAIAutoSchedulerOutcomeRecorder: recorder,
			}
			if tt.configureFastBlock {
				settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
					ServiceTier:    OpenAIFastTierPriority,
					Action:         BetaPolicyActionBlock,
					Scope:          BetaPolicyScopeAll,
					ErrorMessage:   "priority blocked for lifecycle test",
					ModelWhitelist: []string{"gpt-5.5"},
					FallbackAction: BetaPolicyActionPass,
				}}}
				repo := &openAIFastPolicyRepoStub{values: map[string]string{}}
				raw, err := json.Marshal(settings)
				require.NoError(t, err)
				repo.values[SettingKeyOpenAIFastPolicySettings] = string(raw)
				svc.settingService = NewSettingService(repo, cfg)
			}
			account := &Account{
				ID: 456, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test"},
				Extra:       map[string]any{"openai_apikey_responses_websockets_v2_mode": OpenAIWSIngressModePassthrough},
			}

			var lifecycleMu sync.Mutex
			turn2Active := 0
			turn2AfterCalls := 0
			var turn2AfterErr error
			hooks := &OpenAIWSIngressHooks{
				BeforeRequest: func(turn int, _ []byte, _ string) error {
					if turn != 2 {
						return nil
					}
					lifecycleMu.Lock()
					turn2Active++
					lifecycleMu.Unlock()
					return tt.beforeRequestErr
				},
				AfterTurn: func(turn int, _ *OpenAIForwardResult, turnErr error) {
					if turn != 2 {
						return
					}
					lifecycleMu.Lock()
					defer lifecycleMu.Unlock()
					turn2Active--
					turn2AfterCalls++
					turn2AfterErr = turnErr
				},
			}

			serverErrCh := make(chan error, 1)
			wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					serverErrCh <- err
					return
				}
				defer func() { _ = conn.CloseNow() }()
				ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ginCtx.Request = r
				ginCtx.Set("api_key", &APIKey{GroupID: &groupID})
				readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				msgType, firstMessage, readErr := conn.Read(readCtx)
				cancel()
				if readErr != nil {
					serverErrCh <- readErr
					return
				}
				if msgType != coderws.MessageText {
					serverErrCh <- errors.New("unexpected client frame type")
					return
				}
				serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "sk-test", firstMessage, hooks)
			}))
			defer wsServer.Close()

			client, _, err := coderws.Dial(context.Background(), "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
			require.NoError(t, err)
			defer func() { _ = client.CloseNow() }()
			require.NoError(t, client.Write(context.Background(), coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.5"}`)))
			_ = readTestChannel(t, upstream.writes)
			upstream.events <- []byte(`{"type":"response.completed","response":{"id":"resp_lifecycle_1"}}`)
			_, _, err = client.Read(context.Background())
			require.NoError(t, err)

			secondFrame := []byte(`{"type":"response.create","model":"gpt-5.5"}`)
			if tt.configureFastBlock {
				secondFrame = []byte(`{"type":"response.create","model":"gpt-5.5","service_tier":"priority"}`)
			}
			require.NoError(t, client.Write(context.Background(), coderws.MessageText, secondFrame))
			if tt.configureFastBlock {
				readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_, _, _ = client.Read(readCtx)
				cancel()
			}

			select {
			case serverErr := <-serverErrCh:
				require.Error(t, serverErr)
			case <-time.After(5 * time.Second):
				t.Fatal("等待 follow-up local reject 结束超时")
			}
			require.NoError(t, recorder.Stop(context.Background()))
			outcomes := sink.snapshot()
			require.Len(t, outcomes, 1, "本地拒绝不得伪造 upstream scheduler outcome")
			require.Equal(t, OpenAIAutoSchedulerEventSuccess, outcomes[0].EventType)
			select {
			case unexpected := <-upstream.writes:
				t.Fatalf("本地拒绝不应发送第二条上游 frame: %s", unexpected)
			default:
			}
			lifecycleMu.Lock()
			require.Equal(t, 1, turn2AfterCalls)
			require.Zero(t, turn2Active, "turn2 slot 必须释放")
			require.Error(t, turn2AfterErr)
			lifecycleMu.Unlock()
		})
	}
}

func readTestChannel[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		var zero T
		t.Fatal("等待测试 channel 超时")
		return zero
	}
}
