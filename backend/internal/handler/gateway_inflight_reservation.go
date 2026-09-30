package handler

import (
	"context"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// inflightReservationEstimator 估算单请求在途预留金额（USD）；false 表示无法定价。
type inflightReservationEstimator interface {
	EstimateInflightReservation(ctx context.Context, apiKey *service.APIKey, req service.InflightEstimateRequest) (float64, bool)
}

// requestMaxOutputTokens 从请求体中提取输出 token 上限（兼容 Anthropic / OpenAI Chat / Responses / Gemini）。
func requestMaxOutputTokens(body []byte) int {
	for _, path := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens", "generationConfig.maxOutputTokens", "generation_config.max_output_tokens"} {
		if v := gjson.GetBytes(body, path); v.Exists() && v.Type == gjson.Number && v.Int() > 0 {
			return int(v.Int())
		}
	}
	return 0
}

// tokenInflightEstimate 文本类请求的估算输入。
func tokenInflightEstimate(model string, body []byte) service.InflightEstimateRequest {
	return service.InflightEstimateRequest{
		Model:     model,
		BodyBytes: len(body),
		MaxTokens: requestMaxOutputTokens(body),
		Kind:      service.InflightEstimateToken,
	}
}

func inflightNoop() {}

// requestInflightReservation 随实际分组重新估价；已提交的计费任务仍持有旧预留引用。
type requestInflightReservation struct {
	done func()
}

func (r *requestInflightReservation) Done() {
	if r.done != nil {
		r.done()
		r.done = nil
	}
}

func (r *requestInflightReservation) Reserve(c *gin.Context, billing *service.BillingCacheService, estimator inflightReservationEstimator, key *service.APIKey, sub *service.UserSubscription, req service.InflightEstimateRequest) error {
	r.Done()
	// 新 attempt 无预留（例如订阅分组）时也必须遮蔽旧 context 中的句柄。
	c.Request = c.Request.WithContext(service.WithInflightReservation(c.Request.Context(), nil))
	done, err := reserveInflightBalance(c, billing, estimator, key, sub, req)
	r.done = done
	return err
}

// websocketInflightReservations 按 turn 管理预留，避免空闲连接占用余额。
// BeforeTurn 与 AfterTurn 可能由不同 relay 协程调用，句柄通过锁交接。
type websocketInflightReservations struct {
	mu    sync.Mutex
	turns map[int]*service.InflightReservation
}

func (r *websocketInflightReservations) Reserve(ctx context.Context, turn int, billing *service.BillingCacheService, estimator inflightReservationEstimator, key *service.APIKey, sub *service.UserSubscription, req service.InflightEstimateRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.turns[turn]; exists {
		return nil
	}
	turnCtx, _, err := reserveInflightBalanceCtx(ctx, billing, estimator, key, sub, req)
	if err != nil {
		return err
	}
	if r.turns == nil {
		r.turns = make(map[int]*service.InflightReservation)
	}
	r.turns[turn] = service.InflightReservationFromContext(turnCtx)
	return nil
}

func (r *websocketInflightReservations) Context(ctx context.Context, turn int) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	return service.WithInflightReservation(ctx, r.turns[turn])
}

func (r *websocketInflightReservations) Done(turn int) {
	r.mu.Lock()
	res := r.turns[turn]
	delete(r.turns, turn)
	r.mu.Unlock()
	res.HandlerDone()
}

func (r *websocketInflightReservations) Close() {
	r.mu.Lock()
	turns := r.turns
	r.turns = nil
	r.mu.Unlock()
	for _, res := range turns {
		res.HandlerDone()
	}
}

// reserveInflightBalance 在 CheckBillingEligibility 之后为余额模式请求登记在途预留。
//
// 成功时把预留句柄挂到 c.Request 的 context 上：之后通过 submit*UsageRecordTask 提交的
// 计费任务会接管一个引用，直到余额缓存实际扣减后才释放。返回的 done 必须 defer 调用
// （停止续期并归还 handler 引用；没有提交计费任务时即刻释放）。
// 开关关闭、订阅模式、Redis 故障时返回 no-op（fail-open）；无法定价时默认 fail-open，
// 配置 fail_closed_on_unpriced=true 时返回 ErrInsufficientBalance。
func reserveInflightBalance(
	c *gin.Context,
	billing *service.BillingCacheService,
	estimator inflightReservationEstimator,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	req service.InflightEstimateRequest,
) (func(), error) {
	if c == nil || c.Request == nil {
		return inflightNoop, nil
	}
	ctx, done, err := reserveInflightBalanceCtx(c.Request.Context(), billing, estimator, apiKey, subscription, req)
	if err != nil {
		return inflightNoop, err
	}
	c.Request = c.Request.WithContext(ctx)
	return done, nil
}

// reserveInflightBalanceCtx 同 reserveInflightBalance，但返回携带预留句柄的新 context
// （供 WebSocket 等自管 context 的路径）。
func reserveInflightBalanceCtx(
	ctx context.Context,
	billing *service.BillingCacheService,
	estimator inflightReservationEstimator,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	req service.InflightEstimateRequest,
) (context.Context, func(), error) {
	if billing == nil || estimator == nil || apiKey == nil || apiKey.User == nil || !billing.InflightReservationEnabled() {
		return ctx, inflightNoop, nil
	}
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() && subscription != nil {
		return ctx, inflightNoop, nil
	}
	estimate, priced := estimator.EstimateInflightReservation(ctx, apiKey, req)
	if !priced && billing.InflightReservationFailClosedOnUnpriced() {
		return ctx, inflightNoop, service.ErrInsufficientBalance
	}
	if estimate <= 0 {
		return ctx, inflightNoop, nil
	}
	res, err := billing.ReserveInflight(ctx, apiKey.User, apiKey.Group, subscription, estimate)
	if err != nil {
		return ctx, inflightNoop, err
	}
	if res == nil {
		return ctx, inflightNoop, nil
	}
	return service.WithInflightReservation(ctx, res), res.HandlerDone, nil
}

// grokMediaInflightEstimate 媒体生成请求的估算输入；状态/内容查询返回空模型（不预留：
// 查询会为已生成的媒体计费，不能因余额预留而拦截用户取回已付费结果）。
func grokMediaInflightEstimate(endpoint service.GrokMediaEndpoint, model string, info service.GrokMediaRequestInfo, body []byte) service.InflightEstimateRequest {
	if !endpoint.IsGenerationRequest() {
		return service.InflightEstimateRequest{}
	}
	switch endpoint {
	case service.GrokMediaEndpointImagesGenerations, service.GrokMediaEndpointImagesEdits:
		return service.InflightEstimateRequest{Model: model, BodyBytes: len(body), Kind: service.InflightEstimateImage, Units: info.N}
	default:
		return service.InflightEstimateRequest{
			Model:                model,
			BodyBytes:            len(body),
			Kind:                 service.InflightEstimateVideo,
			Units:                1,
			VideoResolution:      info.Resolution,
			VideoDurationSeconds: info.DurationSeconds,
		}
	}
}

// grokVoiceSTTBytesPerSecond STT 时长粗估（~128kbps 压缩音频）。
const grokVoiceSTTBytesPerSecond = 16000

// grokVoiceInflightEstimate 语音 HTTP 接口估算：TTS 按输入字符数（百万字符），STT 按音频字节粗估时长（小时）。
// 其他接口（custom-voices）无音频计量，返回的估算为 0（不预留）。
func grokVoiceInflightEstimate(endpoint string, body []byte) service.InflightEstimateRequest {
	req := service.InflightEstimateRequest{Model: endpoint, Kind: service.InflightEstimateAudio, AudioMode: endpoint}
	switch endpoint {
	case "tts":
		req.AudioUnits = float64(len([]rune(extractGrokTTSInputText(body)))) / 1e6
	case "stt":
		req.AudioUnits = float64(len(body)) / grokVoiceSTTBytesPerSecond / 3600
	}
	return req
}
