package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	quality "github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/tidwall/gjson"
)

type qualityGateKey struct{}
type openAIQualityGate struct {
	service      *OpenAIGatewayService
	group        *Group
	model        string
	body         []byte
	transport    string
	baselineOnly bool
	immutable    bool
}

// SetOpenAIQualityRequestBody accepts only a server-created auto context. No
// public header/body flag can install a gate exemption or change routing mode.
func SetOpenAIQualityRequestBody(ctx context.Context, body []byte) {
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return
	}
	state.mu.Lock()
	state.qualityBody = append([]byte(nil), body...)
	state.mu.Unlock()
}
func (s *OpenAIGatewayService) withQualitySelection(ctx context.Context, model, transport string, baseline, immutable bool) context.Context {
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil || s.accountQuality == nil || !s.accountQuality.Enabled() {
		return ctx
	}
	state.mu.RLock()
	body := append([]byte(nil), state.qualityBody...)
	state.mu.RUnlock()
	return context.WithValue(ctx, qualityGateKey{}, &openAIQualityGate{service: s, group: openAIEffectiveGroupFromContext(ctx), model: model, body: body, transport: transport, baselineOnly: baseline, immutable: immutable})
}
func qualityScope(a *Account, model, protocol, transport, effort, tier string, group *Group) string {
	routing := quality.Hash([]any{qualityRoutingFingerprint(a, a), qualityGroupRoutingFingerprint(group)})
	return quality.Hash([]any{a.ID, model, routing, protocol, transport, effort, tier})
}
func (g *openAIQualityGate) decision(ctx context.Context, a *Account) (string, error) {
	if a == nil || !a.IsOpenAI() || a.IsCredentialShadow() {
		return "baseline", nil
	}
	if g.group != nil {
		ctx = withOpenAIEffectiveGroup(ctx, g.group)
	}
	protocol := qualityProtocol(a)
	transport := "http"
	if protocol == "responses" && (g.transport == string(OpenAIUpstreamTransportResponsesWebsocketV2) || g.transport == string(OpenAIUpstreamTransportResponsesWebsocketV2Ingress) || g.service.openaiWSResolver != nil && g.service.openaiWSResolver.Resolve(a).Transport == OpenAIUpstreamTransportResponsesWebsocketV2) {
		transport = "websocket_v2"
	}
	model := ResolveOpenAIAccountUpstreamModelForRequest(a, g.model, false)
	var payload map[string]any
	_ = json.Unmarshal(g.body, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["model"] = model
	// Client Chat can be converted to Responses by the real forwarding path.
	if protocol == "responses" {
		if effort, ok := payload["reasoning_effort"]; ok {
			payload["reasoning"] = map[string]any{"effort": effort}
			delete(payload, "reasoning_effort")
		}
	} else {
		if effort := gjson.GetBytes(g.body, "reasoning.effort"); effort.Exists() {
			payload["reasoning_effort"] = effort.String()
			delete(payload, "reasoning")
		}
	}
	raw, _ := json.Marshal(payload)
	var err error
	if group := openAIEffectiveGroupFromContext(ctx); group != nil {
		raw, _, err = ApplyOpenAIReasoningEffortPolicy(raw, group.MaxReasoningEffort, group.ReasoningEffortMappings, group.MaxReasoningEffortOverLimit)
		if group.ForceOpenAIFast {
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			if m != nil {
				m["service_tier"] = "priority"
				raw, _ = json.Marshal(m)
			}
		}
	} else {
		raw, _, err = ApplyOpenAIReasoningEffortPolicyFromContext(ctx, raw)
	}
	if err != nil {
		return "baseline", nil
	} // ordinary policy reports its own rejection
	raw, err = g.service.applyOpenAIFastPolicyToBody(ctx, a, model, raw)
	if err != nil {
		return "baseline", nil
	}
	group := g.group
	if current := openAIEffectiveGroupFromContext(ctx); current != nil {
		group = current
	}
	return g.service.accountQuality.Decision(a.ID, qualityScope(a, model, protocol, transport, qualityEffort(raw, protocol), qualityTier(raw), group), time.Now())
}
func qualityCandidateFailure(ctx context.Context, a *Account) string {
	gate, _ := ctx.Value(qualityGateKey{}).(*openAIQualityGate)
	if gate == nil {
		return ""
	}
	action, err := gate.decision(ctx, a)
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state != nil {
		state.mu.Lock()
		defer state.mu.Unlock()
	}
	if err != nil {
		if state != nil {
			state.qualityError = err
		}
		return "quality_state_unavailable"
	}
	if action == "deny" {
		if state != nil {
			state.qualityDenied = true
			if gate.immutable {
				state.qualityError = quality.Fail(409, "QUALITY_SESSION_RESTART_REQUIRED", "当前会话绑定账号已隔离，请新建会话")
			}
		}
		return "quality_quarantined"
	}
	if gate.baselineOnly && action == "fallback" {
		return "quality_observing"
	}
	if state != nil {
		state.qualityAllowed = true
	}
	return ""
}
func qualitySelectionError(ctx context.Context) error {
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.qualityError != nil {
		return state.qualityError
	}
	if state.qualityDenied && !state.qualityAllowed {
		return quality.Fail(503, "QUALITY_NO_ELIGIBLE_ACCOUNTS", "当前匹配账号被质量策略隔离，请稍后重试")
	}
	return nil
}

// CheckOpenAIQualityOutbound is the final guard after slot acquisition and all
// request transforms. The original mode marker survives effective-key clones.
func (s *OpenAIGatewayService) CheckOpenAIQualityOutbound(ctx context.Context, a *Account, raw []byte, protocol, transport string, session bool) error {
	if _, ok := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState); !ok || s == nil || s.accountQuality == nil || !s.accountQuality.Enabled() || a == nil || !a.IsOpenAI() || a.IsCredentialShadow() {
		return nil
	}
	model := strings.TrimSpace(gjson.GetBytes(raw, "model").String())
	if model == "" {
		return nil
	}
	group := openAIEffectiveGroupFromContext(ctx)
	if gate, ok := ctx.Value(qualityGateKey{}).(*openAIQualityGate); ok && gate.group != nil {
		group = gate.group
	}
	action, err := s.accountQuality.Decision(a.ID, qualityScope(a, model, protocol, transport, qualityEffort(raw, protocol), qualityTier(raw), group), time.Now())
	if err != nil {
		return err
	}
	if action == "deny" {
		if session || gjson.GetBytes(raw, "previous_response_id").String() != "" {
			return quality.Fail(409, "QUALITY_SESSION_RESTART_REQUIRED", "当前会话绑定账号已隔离，请新建会话")
		}
		return quality.Fail(503, "QUALITY_NO_ELIGIBLE_ACCOUNTS", "账号已被质量策略隔离，请重试")
	}
	return nil
}
func IsOpenAIQualityError(err error) bool {
	var qe *quality.Error
	return errors.As(err, &qe) && strings.HasPrefix(qe.Code, "QUALITY_")
}
func OpenAIQualityErrorDetails(err error) (int, string, string) {
	var qe *quality.Error
	if errors.As(err, &qe) {
		return qe.Status, qe.Code, qe.Message
	}
	return 503, "QUALITY_STATE_UNAVAILABLE", "质量状态暂不可用"
}

func (s *OpenAIGatewayService) CheckOpenAISelectionQuality(ctx context.Context, selection *AccountSelectionResult) error {
	if selection == nil || selection.qualityGate == nil {
		return nil
	}
	action, err := selection.qualityGate.decision(ctx, selection.Account)
	if err != nil {
		return err
	}
	if action == "deny" {
		return quality.Fail(503, "QUALITY_NO_ELIGIBLE_ACCOUNTS", "账号已被质量策略隔离，请重试")
	}
	return nil
}
