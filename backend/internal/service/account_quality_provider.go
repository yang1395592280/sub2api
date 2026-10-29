package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	quality "github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/tidwall/gjson"
)

// Quality probes borrow only transport/authentication primitives. They never
// enter usage recording, balance charging, latency scoring or ordinary health.
type accountQualityProvider struct {
	accounts    AccountRepository
	groups      GroupRepository
	channel     *ChannelService
	cfg         *config.Config
	upstream    HTTPUpstream
	tokens      *OpenAITokenProvider
	concurrency *ConcurrencyService
	tls         *TLSFingerprintProfileService
	settings    *SettingService
	dialer      openAIWSClientDialer
	renderer    *quality.FrameRenderer
	billing     *BillingService
}

type qualityInventory struct {
	groups   []Group
	accounts map[int64]*Account
	sources  map[int64][]int64
}

func (p *accountQualityProvider) inventory(ctx context.Context) (qualityInventory, error) {
	v := qualityInventory{accounts: map[int64]*Account{}, sources: map[int64][]int64{}}
	groups, err := p.groups.ListActiveByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return v, err
	}
	allIDs := map[int64]bool{}
	for _, g := range groups {
		if g.IsSelfHostedPool() {
			continue
		}
		v.groups = append(v.groups, g)
		ids := []int64{g.ID}
		if g.SelfHostedPoolGroupID != nil {
			pool, e := p.groups.GetByIDLite(ctx, *g.SelfHostedPoolGroupID)
			if e != nil {
				return v, e
			}
			if pool.Status == StatusActive && pool.IsSelfHostedPool() {
				ids = append(ids, pool.ID)
			}
		}
		members, e := p.groups.GetAccountIDsByGroupIDs(ctx, ids)
		if e != nil {
			return v, e
		}
		v.sources[g.ID] = members
		for _, id := range members {
			allIDs[id] = true
		}
	}
	ids := make([]int64, 0, len(allIDs))
	for id := range allIDs {
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		accounts, e := p.accounts.GetByIDs(ctx, ids)
		if e != nil {
			return v, e
		}
		for _, a := range accounts {
			if a.IsOpenAI() {
				v.accounts[a.ID] = a
			}
		}
	}
	return v, nil
}
func (p *accountQualityProvider) Targets(ctx context.Context) ([]quality.Target, error) {
	v, err := p.inventory(ctx)
	if err != nil {
		return nil, err
	}
	out := []quality.Target{}
	ids := make([]int64, 0, len(v.accounts))
	for id := range v.accounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		a := v.accounts[id]
		t := quality.Target{AccountID: id, AccountName: a.Name, Eligibility: "eligible", Reasons: []string{}}
		for _, g := range v.groups {
			for _, member := range v.sources[g.ID] {
				if member == id {
					t.CompatibleSourceGroupIDs = append(t.CompatibleSourceGroupIDs, g.ID)
					t.CanEnforce = t.CanEnforce || g.AllowAutoCheapestScheduling
					if t.SourceGroupID == 0 {
						t.SourceGroupID = g.ID
						t.SourceGroupName = g.Name
					}
					break
				}
			}
		}
		t.CanProbe = !a.IsOpenAIAgentIdentity() && (a.IsOpenAIOAuthLike() || a.IsOpenAIApiKey())
		t.CanEnforce = t.CanEnforce && t.CanProbe && !a.IsCredentialShadow()
		t.SupportsOutputLimit = t.CanProbe
		if !t.CanProbe {
			t.Reasons = append(t.Reasons, "unsupported_authentication")
		}
		out = append(out, t)
	}
	return out, nil
}
func qualityProtocol(a *Account) string {
	if a.IsOpenAIOAuthLike() || openai_compat.ShouldUseResponsesAPI(a.Extra) {
		return "responses"
	}
	return "chat_completions"
}

// The fingerprint contains routing metadata and an opaque administrator-edit
// generation, never a token, password, header value or hash of a secret.
func qualityRoutingFingerprint(a, credential *Account) string {
	return quality.Hash([]any{qualityAccountRoutingIdentity(a), qualityCredentialRoutingIdentity(credential)})
}

func qualityAccountRoutingIdentity(a *Account) any {
	if a == nil {
		return nil
	}
	var proxyVersion any
	if a.Proxy != nil {
		proxyVersion = []any{a.Proxy.ID, a.Proxy.UpdatedAt}
	}
	groupIDs := append([]int64(nil), a.GroupIDs...)
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	return []any{a.ID, a.CreatedAt, a.Platform, a.Type, a.ParentAccountID, a.QuotaDimension, groupIDs, a.ProxyID, proxyVersion, qualityEndpointIdentity(a.GetOpenAIBaseURL()), a.GetModelMapping(), a.Extra["quality_identity_version"]}
}

func qualityCredentialRoutingIdentity(credential *Account) any {
	if credential == nil {
		return nil
	}
	return []any{credential.ID, credential.CreatedAt, credential.Platform, credential.Type, credential.ProxyID, qualityEndpointIdentity(credential.GetOpenAIBaseURL()), credential.GetModelMapping(), credential.Extra["quality_identity_version"]}
}

// Keep endpoint identity useful for detecting a routing change while discarding
// URL userinfo, query parameters and fragments that could contain credentials.
func qualityEndpointIdentity(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimRight(u.EscapedPath(), "/")
}

// Group identity is intentionally excluded so equivalent automatic source
// groups can share one policy. Only routing controls that can affect the
// effective outbound request participate in this fingerprint.
func qualityGroupRoutingFingerprint(group *Group) string {
	if group == nil {
		return ""
	}
	var poolID any
	if group.SelfHostedPoolGroupID != nil {
		poolID = *group.SelfHostedPoolGroupID
	}
	return quality.Hash([]any{
		group.Platform,
		group.GroupRole,
		group.AllowAutoCheapestScheduling,
		poolID,
		group.DefaultMappedModel,
		group.MaxReasoningEffort,
		group.MaxReasoningEffortOverLimit,
		group.ReasoningEffortMappings,
		group.ForceOpenAIFast,
	})
}
func (p *accountQualityProvider) Resolve(ctx context.Context, id, source int64, c quality.Condition) (quality.Target, error) {
	if err := c.Validate(); err != nil {
		return quality.Target{}, err
	}
	v, err := p.inventory(ctx)
	if err != nil {
		return quality.Target{}, err
	}
	a := v.accounts[id]
	if a == nil {
		return quality.Target{}, quality.Fail(400, "QUALITY_TARGET_OUT_OF_SCOPE", "账号未参与可检测的 OpenAI 分组")
	}
	var group *Group
	for i := range v.groups {
		if v.groups[i].ID == source {
			group = &v.groups[i]
			break
		}
	}
	member := false
	for _, aid := range v.sources[source] {
		if aid == id {
			member = true
			break
		}
	}
	if group == nil || !member {
		return quality.Target{}, quality.Fail(400, "QUALITY_SOURCE_CONTEXT_INVALID", "来源分组已失效")
	}
	credential := a
	if a.IsCredentialShadow() {
		credential, err = resolveCredentialAccount(ctx, p.accounts, a)
		if err != nil {
			return quality.Target{}, quality.Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "母账号凭据不可用")
		}
	}
	t := quality.Target{AccountID: id, AccountName: a.Name, SourceGroupID: source, SourceGroupName: group.Name, PhysicalGroupID: source, Eligibility: "eligible", Condition: c, Reasons: []string{}}
	if group.SelfHostedPoolGroupID != nil {
		for _, gid := range a.GroupIDs {
			if gid == *group.SelfHostedPoolGroupID {
				t.PhysicalGroupID = gid
			}
		}
	}
	t.Fingerprint = quality.Hash([]any{qualityRoutingFingerprint(a, credential), qualityGroupRoutingFingerprint(group)})
	t.EffectiveModel = c.ModelID
	if p.channel != nil {
		m, restricted := p.channel.ResolveChannelMappingAndRestrict(ctx, &source, c.ModelID)
		if restricted {
			return t, quality.Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "模型被渠道限制")
		}
		if m.MappedModel != "" {
			t.EffectiveModel = m.MappedModel
		}
	}
	t.EffectiveModel = ResolveOpenAIAccountUpstreamModelForRequest(a, t.EffectiveModel, false)
	body := map[string]any{"model": t.EffectiveModel}
	if c.ReasoningEffort != "upstream_default" {
		if c.Protocol == "responses" {
			body["reasoning"] = map[string]any{"effort": c.ReasoningEffort}
		} else {
			body["reasoning_effort"] = c.ReasoningEffort
		}
	}
	if c.ServiceTier != "upstream_default" {
		body["service_tier"] = c.ServiceTier
	}
	raw, _ := json.Marshal(body)
	raw, _, err = ApplyOpenAIReasoningEffortPolicy(raw, group.MaxReasoningEffort, group.ReasoningEffortMappings, group.MaxReasoningEffortOverLimit)
	if err != nil {
		return t, quality.Invalid("reasoning_effort", "推理强度被来源分组拒绝")
	}
	helper := &OpenAIGatewayService{cfg: p.cfg, settingService: p.settings}
	if group.ForceOpenAIFast {
		body = map[string]any{}
		_ = json.Unmarshal(raw, &body)
		body["service_tier"] = "priority"
		raw, _ = json.Marshal(body)
	}
	raw, err = helper.applyOpenAIFastPolicyToBody(ctx, a, t.EffectiveModel, raw)
	if err != nil {
		return t, quality.Invalid("service_tier", "服务档位被全局规则拒绝")
	}
	t.EffectiveEffort = qualityEffort(raw, c.Protocol)
	t.EffectiveTier = qualityTier(raw)
	t.CanProbe = qualityProtocol(a) == c.Protocol && !credential.IsOpenAIAgentIdentity() && !isOpenAIImageModel(t.EffectiveModel) && supportsOpenAIRequestModel(a, c.ModelID, false)
	if c.Transport == "websocket_v2" && NewOpenAIWSProtocolResolver(p.cfg).Resolve(a).Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		t.CanProbe = false
		t.Reasons = append(t.Reasons, "websocket_v2_unavailable")
	}
	// Ordinary source groups can collect diagnostic evidence, but cannot grant
	// authority to change scheduling, even for accounts shared with auto groups.
	t.CanEnforce = t.CanProbe && !a.IsCredentialShadow() && group.AllowAutoCheapestScheduling
	t.SupportsOutputLimit = t.CanProbe
	if !t.CanProbe {
		t.Reasons = append(t.Reasons, "unsupported_condition")
	}
	// Exact route/parameters define equivalence; a later Resolve validates the
	// selected source again, so unrelated groups cannot grant probe authority.
	t.CompatibleSourceGroupIDs = []int64{source}
	return t, nil
}
func qualityEffort(body []byte, protocol string) string {
	key := "reasoning.effort"
	if protocol == "chat_completions" {
		key = "reasoning_effort"
	}
	v := gjson.GetBytes(body, key).String()
	if v == "" {
		return "upstream_default"
	}
	return strings.ToLower(v)
}
func qualityTier(body []byte) string {
	v := normalizedOpenAIServiceTierValue(gjson.GetBytes(body, "service_tier").String())
	if v == "" {
		return "upstream_default"
	}
	return v
}

func (p *accountQualityProvider) request(ctx context.Context, r quality.Run, stage, prompt string, frames []quality.Frame, dispatch func(string) error) quality.Result {
	start := time.Now()
	failed := func(reason string) quality.Result {
		return quality.Result{Verdict: "error", ReasonCode: reason, ExpectedAnswer: 21, LatencyMS: time.Since(start).Milliseconds()}
	}
	t, err := p.Resolve(ctx, r.AccountID, r.Snapshot.SourceGroupID, r.Snapshot.Condition)
	if err != nil || !t.CanProbe {
		return failed("QUALITY_TARGET_OUT_OF_SCOPE")
	}
	current := r.Snapshot
	current.SetTarget(t)
	if current.ScopeHash != r.ScopeHash {
		return failed("scope_changed")
	}
	a, err := p.accounts.GetByID(ctx, r.AccountID)
	if err != nil || !a.IsSchedulable() {
		return failed("account_unavailable")
	}
	if paused, _ := shouldAutoPauseOpenAIAccountByQuota(ctx, a); paused {
		return failed("quota_auto_pause")
	}
	credential := a
	if a.IsCredentialShadow() {
		credential, err = resolveCredentialAccount(ctx, p.accounts, a)
		if err != nil || !credential.IsSchedulable() {
			return failed("credential_unavailable")
		}
	}
	slot, err := p.concurrency.AcquireAccountSlot(ctx, a.ID, a.Concurrency)
	if err != nil || slot == nil || !slot.Acquired {
		return failed("account_busy")
	}
	defer slot.ReleaseFunc()
	token := ""
	if credential.IsOpenAIOAuthLike() {
		token, err = p.tokens.GetAccessToken(ctx, credential)
	} else {
		token = credential.GetOpenAIProtocolAPIKey()
	}
	if err != nil || token == "" {
		return failed("authentication_unavailable")
	}
	c := r.Snapshot.Condition
	content := []any{map[string]any{"type": "input_text", "text": prompt}}
	for _, frame := range frames {
		content = append(content, map[string]any{"type": "input_text", "text": fmt.Sprintf("Frame at %.3f seconds", frame.Time)}, map[string]any{"type": "input_image", "image_url": "data:image/png;base64," + frame.PNG, "detail": "high"})
	}
	payload := map[string]any{"model": t.EffectiveModel, "stream": true, "max_output_tokens": r.Snapshot.Rules.MaxOutputTokens, "instructions": "Follow the user's requested output format exactly.", "input": []any{map[string]any{"role": "user", "content": content}}}
	if c.Protocol == "chat_completions" {
		delete(payload, "instructions")
		delete(payload, "input")
		delete(payload, "max_output_tokens")
		payload["max_completion_tokens"] = r.Snapshot.Rules.MaxOutputTokens
		payload["stream_options"] = map[string]any{"include_usage": true}
		chatContent := []any{map[string]any{"type": "text", "text": prompt}}
		for _, frame := range frames {
			chatContent = append(chatContent, map[string]any{"type": "text", "text": fmt.Sprintf("Frame at %.3f seconds", frame.Time)}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64," + frame.PNG, "detail": "high"}})
		}
		payload["messages"] = []any{map[string]any{"role": "user", "content": chatContent}}
	}
	if credential.IsOpenAIOAuthLike() {
		payload["store"] = false
	}
	if t.EffectiveEffort != "upstream_default" {
		if c.Protocol == "responses" {
			payload["reasoning"] = map[string]any{"effort": t.EffectiveEffort}
		} else {
			payload["reasoning_effort"] = t.EffectiveEffort
		}
	}
	if t.EffectiveTier != "upstream_default" {
		payload["service_tier"] = t.EffectiveTier
	}
	apiURL := chatgptCodexAPIURL
	helper := &OpenAIGatewayService{cfg: p.cfg}
	if !credential.IsOpenAIOAuthLike() {
		base := credential.GetOpenAIBaseURL()
		if base == "" {
			base = "https://api.openai.com"
		}
		base, err = helper.validateUpstreamBaseURL(base)
		if err != nil {
			return failed("invalid_upstream_url")
		}
		apiURL = buildOpenAIResponsesURLForPlatform(a.Platform, base)
		if c.Protocol == "chat_completions" {
			apiURL = buildOpenAIChatCompletionsURL(base)
		}
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), http.MethodPost, apiURL, bytes.NewReader(raw))
	if err != nil {
		return failed("request_build_failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	if credential.IsOpenAIOAuthLike() {
		req.Host = "chatgpt.com"
		req.Header.Set("OpenAI-Beta", "responses=experimental")
		canonical := resolveCodexOutboundIdentity("")
		req.Header.Set("Originator", canonical.originator)
		req.Header.Set("User-Agent", canonical.userAgent)
		setOpenAIChatGPTAccountHeaders(req.Header, credential)
		enforceCodexIdentityHeadersWithUA(req.Header, credential.GetOpenAIUserAgent())
	} else {
		applyOpenAICodexProbeHeaders(req.Header)
	}
	credential.ApplyHeaderOverrides(req.Header)
	setOpenAICodexRoutingHint(req.Header, a, t.EffectiveModel, t.EffectiveTier)
	proxy := ""
	if a.Proxy != nil {
		proxy = a.Proxy.URL()
	}
	// Re-resolve immediately before the durable send boundary; no transaction is
	// held while waiting for credentials, capacity or the network.
	check, e := p.Resolve(ctx, r.AccountID, r.Snapshot.SourceGroupID, c)
	if e != nil || !check.CanProbe {
		return failed("scope_changed")
	}
	candidate := r.Snapshot
	candidate.SetTarget(check)
	if candidate.ScopeHash != r.ScopeHash {
		return failed("scope_changed")
	}
	if err = dispatch(stage); err != nil {
		return failed(quality.DispatchReason(err))
	}
	collector := qualityCollector{protocol: c.Protocol}
	if c.Transport == "websocket_v2" {
		u, e := url.Parse(apiURL)
		if e != nil {
			return failed("invalid_upstream_url")
		}
		if u.Scheme == "https" {
			u.Scheme = "wss"
		} else {
			u.Scheme = "ws"
		}
		conn, _, _, e := p.dialer.Dial(ctx, u.String(), req.Header, proxy)
		if e != nil {
			return failed("websocket_connect_failed")
		}
		defer conn.Close()
		payload["type"] = "response.create"
		if e = conn.WriteJSON(ctx, payload); e != nil {
			return failed("upstream_acceptance_unknown")
		}
		for !collector.done {
			msg, e := conn.ReadMessage(ctx)
			if e != nil {
				break
			}
			if !collector.consume(msg) {
				break
			}
		}
	} else {
		resp, e := p.upstream.DoWithTLS(req, proxy, a.ID, a.Concurrency, p.tls.ResolveTLSProfile(a))
		if e != nil {
			return failed("upstream_acceptance_unknown")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return failed(fmt.Sprintf("upstream_http_%d", resp.StatusCode))
		}
		if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			scanner := bufio.NewScanner(resp.Body)
			scanner.Buffer(make([]byte, 8192), 256<<10)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data:") {
					if !collector.consume([]byte(strings.TrimSpace(line[5:]))) {
						break
					}
				}
				if collector.done {
					break
				}
			}
		} else {
			b, e := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
			if e == nil {
				collector.consume(b)
			}
		}
	}
	text := redactPelicanAccountSecrets(credential, collector.text.String())
	result := quality.Result{Verdict: "unknown", ReasonCode: "generation_completed", ResponseText: text}
	if stage == "candy" {
		result = quality.ScoreCandy(text, collector.complete, r.Snapshot.Condition)
	}
	result.GenerationCompleted = collector.complete
	if len(result.ResponseText) > quality.MaxResponseBytes {
		result.ResponseText = strings.ToValidUTF8(result.ResponseText[:quality.MaxResponseBytes], "")
		result.ResponseTruncated = true
		result.GenerationCompleted = false
		result.ReasonCode = "response_too_large"
	}
	if !collector.complete && !result.ResponseTruncated {
		result.ReasonCode = "incomplete_response"
	}
	result.InputTokens = collector.input
	result.OutputTokens = collector.output
	result.TotalTokens = collector.total
	result.ReasoningTokens = collector.reasoning
	result.LatencyMS = time.Since(start).Milliseconds()
	if collector.output != nil && result.LatencyMS > 0 {
		v := 1000 * float64(*collector.output) / float64(result.LatencyMS)
		result.TokensPerSecond = &v
	}
	if p.billing != nil && collector.input != nil && collector.output != nil {
		cached := int64(0)
		if collector.cached != nil {
			cached = min(*collector.cached, *collector.input)
		}
		cost, e := p.billing.CalculateCostWithServiceTier(t.EffectiveModel, UsageTokens{InputTokens: int(*collector.input - cached), OutputTokens: int(*collector.output), CacheReadTokens: int(cached)}, 1, t.EffectiveTier)
		if e == nil && cost != nil {
			result.EstimatedCostUSD = &cost.TotalCost
		}
	}
	if collector.reason != "" {
		result.Verdict = "error"
		result.ReasonCode = collector.reason
	}
	if ctx.Err() != nil {
		result.Verdict = "unknown"
		result.ReasonCode = "probe_timeout"
	}
	return result
}

// Only final assistant output_text/message content enters scoring. Reasoning
// and tool arguments are never concatenated into the answer.
type qualityCollector struct {
	protocol             string
	text                 strings.Builder
	done, complete       bool
	reason               string
	input, output, total *int64
	reasoning            *int64
	cached               *int64
}

func (c *qualityCollector) consume(raw []byte) bool {
	if string(raw) == "[DONE]" {
		c.done = true
		return false
	}
	if !gjson.ValidBytes(raw) {
		return true
	}
	v := gjson.ParseBytes(raw)
	usage := v.Get("usage")
	if !usage.Exists() {
		usage = v.Get("response.usage")
	}
	for _, item := range []struct {
		key  string
		dest **int64
	}{{"input_tokens", &c.input}, {"prompt_tokens", &c.input}, {"output_tokens", &c.output}, {"completion_tokens", &c.output}, {"total_tokens", &c.total}} {
		if n := usage.Get(item.key); n.Type == gjson.Number && n.Int() >= 0 {
			value := n.Int()
			*item.dest = &value
		}
	}
	for _, key := range []string{"output_tokens_details.reasoning_tokens", "completion_tokens_details.reasoning_tokens"} {
		if n := usage.Get(key); n.Exists() {
			v := n.Int()
			if v >= 0 {
				c.reasoning = &v
			}
		}
	}
	for _, key := range []string{"input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens"} {
		if n := usage.Get(key); n.Type == gjson.Number && n.Int() >= 0 {
			v := n.Int()
			c.cached = &v
		}
	}
	if c.protocol == "chat_completions" {
		choice := v.Get("choices.0")
		if delta := choice.Get("delta.content"); delta.Exists() {
			c.text.WriteString(delta.String())
		}
		if msg := choice.Get("message.content"); msg.Exists() {
			c.text.Reset()
			c.text.WriteString(msg.String())
		}
		if finish := choice.Get("finish_reason"); finish.Exists() && finish.Type != gjson.Null {
			c.complete = finish.String() == "stop"
		}
		if v.Get("error").Exists() {
			c.reason = "upstream_error"
			c.done = true
		}
	} else {
		switch v.Get("type").String() {
		case "response.output_text.delta":
			c.text.WriteString(v.Get("delta").String())
		case "error", "response.failed":
			c.reason = "upstream_error"
			c.done = true
		case "response.completed", "response.incomplete":
			c.done = true
			response := v.Get("response")
			c.complete = v.Get("type").String() == "response.completed" && response.Get("status").String() == "completed"
			if response.Get("output").Exists() {
				c.text.Reset()
				qualityFinalText(response, &c.text)
			}
		}
		if v.Get("object").String() == "response" {
			c.done = true
			c.complete = v.Get("status").String() == "completed"
			c.text.Reset()
			qualityFinalText(v, &c.text)
		}
	}
	return c.text.Len() <= quality.MaxResponseBytes && !c.done
}
func qualityFinalText(v gjson.Result, b *strings.Builder) {
	for _, item := range v.Get("output").Array() {
		if item.Get("type").String() != "message" || item.Get("role").String() != "assistant" {
			continue
		}
		for _, part := range item.Get("content").Array() {
			if part.Get("type").String() == "output_text" {
				b.WriteString(part.Get("text").String())
			}
		}
	}
}
