package accountquality

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func writeError(c *gin.Context, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = &Error{Status: 503, Code: "QUALITY_PERSISTENCE_UNAVAILABLE", Message: "质量数据暂不可用，请稍后重试"}
	}
	body := gin.H{"code": e.Status, "message": e.Message, "reason": e.Code}
	if e.Field != "" {
		body["metadata"] = map[string]string{"field": e.Field}
	}
	c.JSON(e.Status, body)
}
func success(c *gin.Context, status int, v any) {
	c.JSON(status, gin.H{"code": 0, "message": "success", "data": v})
}
func decode(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return Invalid("body", "请求字段或格式错误")
	}
	if _, err := d.Token(); err != io.EOF {
		return Invalid("body", "请求只能包含一个JSON对象")
	}
	return nil
}
func body(c *gin.Context) ([]byte, error) {
	reader := http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
	raw, err := io.ReadAll(reader)
	if err != nil || len(raw) == 0 {
		return nil, Invalid("body", "请求为空或超过256KiB")
	}
	return raw, nil
}
func (s *Service) readHTTP(c *gin.Context, f func(*Tx) (any, error)) {
	v, err := s.Read(c.Request.Context(), f)
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, 200, v)
}
func (s *Service) mutateHTTP(c *gin.Context, action string, status int, raw []byte, f func(*Tx) (any, error)) {
	var payload any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&payload); err != nil {
		writeError(c, Invalid("body", "无效JSON"))
		return
	}
	if _, err := d.Token(); err != io.EOF {
		writeError(c, Invalid("body", "请求只能包含一个JSON对象"))
		return
	}
	var replay bool
	httpStatus := status
	v, err := s.Write(c.Request.Context(), func(t *Tx) (any, error) {
		out, code, replayed, e := t.Idempotent(c.GetInt64("quality_actor_id"), action, c.GetHeader("Idempotency-Key"), payload, status, func() (any, error) { return f(t) })
		httpStatus = code
		replay = replayed
		return out, e
	})
	if err != nil {
		writeError(c, err)
		return
	}
	if replay {
		c.Header("X-Idempotency-Replayed", "true")
	}
	success(c, httpStatus, v)
}
func parseID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, Invalid("id", "ID必须为正整数")
	}
	return id, nil
}
func parsePage(c *gin.Context) (int, int, error) {
	p, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || p < 1 || p > 1000000 {
		return 0, 0, Invalid("page", "页码错误")
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || size < 1 || size > 100 {
		return 0, 0, Invalid("page_size", "每页必须为1–100")
	}
	return p, size, nil
}
func queryID(c *gin.Context, name string) (int64, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, Invalid(name, "ID必须为正整数")
	}
	return id, nil
}
func queryBool(c *gin.Context, name string) (*bool, error) {
	raw, ok := c.GetQuery(name)
	if !ok {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, Invalid(name, "必须为布尔值")
	}
	return &v, nil
}
func queryTime(c *gin.Context, name string) (*time.Time, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	v, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, Invalid(name, "时间必须为RFC3339")
	}
	return &v, nil
}
func policyFilter(c *gin.Context) (PolicyFilter, error) {
	var f PolicyFilter
	var err error
	f.AccountID, err = queryID(c, "account_id")
	if err != nil {
		return f, err
	}
	f.SourceGroupID, err = queryID(c, "source_group_id")
	if err != nil {
		return f, err
	}
	f.ControlEnabled, err = queryBool(c, "control_enabled")
	if err != nil {
		return f, err
	}
	f.AutoEnabled, err = queryBool(c, "auto_check_enabled")
	if err != nil {
		return f, err
	}
	archived, err := queryBool(c, "archived")
	if err != nil {
		return f, err
	}
	f.Archived = archived != nil && *archived
	f.State = c.Query("state")
	f.ModelID = c.Query("model_id")
	f.Eligibility = c.Query("eligibility")
	f.Freshness = c.Query("freshness")
	f.Search = c.Query("search")
	return f, nil
}
func parseCreate(raw []byte, limit int) ([]CreateItem, error) {
	var request struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := decode(raw, &request); err != nil {
		return nil, err
	}
	if len(request.Items) == 0 || len(request.Items) > 100 {
		return nil, Invalid("items", "请选择1–100个策略")
	}
	out := []CreateItem{}
	for _, item := range request.Items {
		v := CreateItem{Rules: DefaultRules(limit)}
		if err := decode(item, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func fullRules(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Invalid("rules", "规则格式错误")
	}
	keys := []string{"auto_check_enabled", "control_enabled", "interval_seconds", "jitter_seconds", "failure_threshold", "recovery_threshold", "cooldown_seconds", "recovery_interval_seconds", "evidence_window_seconds", "timeout_seconds", "max_output_tokens", "daily_call_limit"}
	for _, key := range keys {
		v, ok := fields[key]
		if !ok || string(v) == "null" {
			return Invalid("rules."+key, "更新必须提供完整规则")
		}
	}
	return nil
}
func parseUpdates(raw []byte, singleID int64) ([]UpdateItem, error) {
	var items []json.RawMessage
	if singleID > 0 {
		items = []json.RawMessage{raw}
	} else {
		var request struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := decode(raw, &request); err != nil {
			return nil, err
		}
		items = request.Items
	}
	if len(items) == 0 || len(items) > 100 {
		return nil, Invalid("items", "请选择1–100个策略")
	}
	out := []UpdateItem{}
	for _, item := range items {
		var req struct {
			PolicyID         int64           `json:"policy_id"`
			ExpectedRevision int64           `json:"expected_revision"`
			Rules            json.RawMessage `json:"rules"`
		}
		if err := decode(item, &req); err != nil {
			return nil, err
		}
		if singleID > 0 {
			if req.PolicyID != 0 && req.PolicyID != singleID {
				return nil, Invalid("policy_id", "路径与策略不一致")
			}
			req.PolicyID = singleID
		}
		if err := fullRules(req.Rules); err != nil {
			return nil, err
		}
		var rules Rules
		if err := decode(req.Rules, &rules); err != nil {
			return nil, err
		}
		out = append(out, UpdateItem{req.PolicyID, req.ExpectedRevision, rules})
	}
	return out, nil
}
func parseSettings(raw []byte) (Settings, int64, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return Settings{}, 0, Invalid("body", "配置格式错误")
	}
	var expected int64
	if err := json.Unmarshal(object["expected_revision"], &expected); err != nil || expected < 1 {
		return Settings{}, 0, Invalid("expected_revision", "缺少设置版本")
	}
	delete(object, "expected_revision")
	if _, ok := object["control_epoch"]; ok {
		return Settings{}, 0, Invalid("control_epoch", "控制代次不可编辑")
	}
	if _, ok := object["revision"]; ok {
		return Settings{}, 0, Invalid("revision", "版本不可编辑")
	}
	defaultJSON, _ := json.Marshal(DefaultSettings())
	var fields map[string]any
	_ = json.Unmarshal(defaultJSON, &fields)
	delete(fields, "revision")
	delete(fields, "control_epoch")
	for key := range fields {
		v, ok := object[key]
		if !ok || string(v) == "null" {
			return Settings{}, 0, Invalid(key, "更新必须包含完整全局配置")
		}
	}
	b, _ := json.Marshal(object)
	var value Settings
	if err := decode(b, &value); err != nil {
		return value, 0, err
	}
	return value, expected, value.Validate()
}

// Register must be mounted under the existing administrator authentication and
// audit middleware. The admin adapter supplies the authenticated actor ID.
func (s *Service) Register(parent *gin.RouterGroup) {
	g := parent.Group("/account-quality")
	g.Use(s.prepareRoute)
	g.GET("/settings", func(c *gin.Context) {
		s.readHTTP(c, func(t *Tx) (any, error) {
			health, err := t.RuntimeHealth()
			if err != nil {
				return nil, err
			}
			if p, ok := s.provider.(interface{ RendererConfigured() bool }); ok {
				health["renderer_configured"] = p.RendererConfigured()
			}
			budget, err := t.Budget("global", 0)
			return map[string]any{"settings": t.Settings, "health": health, "budget": budget}, err
		})
	})
	g.PUT("/settings", func(c *gin.Context) {
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		settings, expected, err := parseSettings(raw)
		if err != nil {
			writeError(c, err)
			return
		}
		s.mutateHTTP(c, "settings", 200, raw, func(t *Tx) (any, error) {
			return s.ChangeSettings(t, expected, settings, c.GetInt64("quality_actor_id"), false, "settings_updated")
		})
	})
	g.POST("/settings/emergency-disable", func(c *gin.Context) {
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if err = decode(raw, &req); err != nil {
			writeError(c, err)
			return
		}
		s.mutateHTTP(c, "emergency-disable", 200, raw, func(t *Tx) (any, error) {
			return s.ChangeSettings(t, 0, Settings{}, c.GetInt64("quality_actor_id"), true, req.Reason)
		})
	})
	g.GET("/benchmarks", func(c *gin.Context) {
		success(c, 200, gin.H{"items": []gin.H{
			{"id": "candy", "version": "1", "scorer_version": "strict-json-v1", "name": "糖果逻辑题", "expected_format": "{\"answer\": integer}", "prompt": CandyPrompt, "output_limit": MaxResponseBytes},
			{"id": "dual", "version": "1", "scorer_version": "dual-visual-v1", "name": "糖果 + 鹈鹕双重检测", "prompt": PelicanPrompt, "candy_prompt": CandyPrompt, "review_prompt": VisualReviewPrompt, "max_model_calls": 3, "output_limit": MaxResponseBytes},
			{"id": "candy", "version": "2", "scorer_version": "answer-explanation-v2", "name": "糖果逻辑题（含解题说明）", "prompt": CandyExplanationPrompt, "output_limit": MaxResponseBytes},
			{"id": "dual", "version": "2", "scorer_version": "dual-visual-v2", "name": "糖果详解 + 鹈鹕双重检测", "prompt": PelicanPrompt, "candy_prompt": CandyExplanationPrompt, "review_prompt": VisualReviewPrompt, "max_model_calls": 3, "output_limit": MaxResponseBytes},
		}})
	})
	g.GET("/targets", s.targetsHTTP)
	g.POST("/capabilities", s.capabilitiesHTTP)
	g.POST("/preview", s.previewHTTP)
	g.GET("/overview", func(c *gin.Context) {
		f, err := policyFilter(c)
		if err != nil {
			writeError(c, err)
			return
		}
		targets, err := s.ListTargets(c.Request.Context())
		if err != nil {
			writeError(c, err)
			return
		}
		s.readHTTP(c, func(t *Tx) (any, error) { return s.Overview(t, f, targets) })
	})
	g.GET("/policies", func(c *gin.Context) {
		f, err := policyFilter(c)
		if err != nil {
			writeError(c, err)
			return
		}
		p, size, err := parsePage(c)
		if err != nil {
			writeError(c, err)
			return
		}
		s.readHTTP(c, func(t *Tx) (any, error) {
			items, e := t.Policies(f.Archived)
			if e != nil {
				return nil, e
			}
			return page(filterPolicies(items, f, t.Settings, t.Now), p, size), nil
		})
	})
	g.POST("/policies", func(c *gin.Context) {
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		s.mutateHTTP(c, "create-policies", 201, raw, func(t *Tx) (any, error) {
			items, e := parseCreate(raw, t.Settings.DefaultPolicyDailyCallLimit)
			if e != nil {
				return nil, e
			}
			targets := make([]Target, 0, len(items))
			for _, item := range items {
				target, e := s.Resolve(c.Request.Context(), item)
				if e != nil {
					return nil, e
				}
				targets = append(targets, target)
			}
			return s.Create(t, items, targets, c.GetInt64("quality_actor_id"))
		})
	})
	g.POST("/policies/batch-update", func(c *gin.Context) {
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, err := parseUpdates(raw, 0)
		if err != nil {
			writeError(c, err)
			return
		}
		s.mutateHTTP(c, "batch-update", 200, raw, func(t *Tx) (any, error) { return s.Update(t, items, c.GetInt64("quality_actor_id")) })
	})
	g.GET("/policies/:id/monitor", func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			writeError(c, err)
			return
		}
		seconds := 0
		if raw := c.Query("bucket_seconds"); raw != "" {
			seconds, err = strconv.Atoi(raw)
			if err != nil || seconds < 300 || seconds > 86400 {
				writeError(c, Invalid("bucket_seconds", "时间桶错误"))
				return
			}
		}
		view := c.DefaultQuery("evidence_view", "current_epoch")
		s.readHTTP(c, func(t *Tx) (any, error) { return t.Monitor(id, seconds, view) })
	})
	g.GET("/policies/:id", func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			writeError(c, err)
			return
		}
		s.readHTTP(c, func(t *Tx) (any, error) {
			p, e := t.Policy(id, false)
			if e != nil {
				return nil, e
			}
			return Present(p, t.Settings, t.Now), nil
		})
	})
	g.PUT("/policies/:id", func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			writeError(c, err)
			return
		}
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, err := parseUpdates(raw, id)
		if err != nil {
			writeError(c, err)
			return
		}
		s.mutateHTTP(c, "update:"+strconv.FormatInt(id, 10), 200, raw, func(t *Tx) (any, error) { return s.Update(t, items, c.GetInt64("quality_actor_id")) })
	})
	for _, action := range []string{"archive", "release"} {
		g.POST("/policies/:id/"+action, func(c *gin.Context) {
			id, err := parseID(c)
			if err != nil {
				writeError(c, err)
				return
			}
			raw, err := body(c)
			if err != nil {
				writeError(c, err)
				return
			}
			var req struct {
				ExpectedRevision int64  `json:"expected_revision"`
				Reason           string `json:"reason"`
			}
			if err = decode(raw, &req); err != nil {
				writeError(c, err)
				return
			}
			s.mutateHTTP(c, action+":"+strconv.FormatInt(id, 10), 200, raw, func(t *Tx) (any, error) {
				p, e := t.Policy(id, true)
				if e != nil {
					return nil, e
				}
				p = Present(p, t.Settings, t.Now)
				if p.Revision != req.ExpectedRevision {
					return nil, revisionConflict()
				}
				return s.Lifecycle(t, &p, action, req.Reason, c.GetInt64("quality_actor_id"))
			})
		})
	}
	g.POST("/runs", func(c *gin.Context) {
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		var req struct {
			PolicyIDs []int64 `json:"policy_ids"`
		}
		if err = decode(raw, &req); err != nil {
			writeError(c, err)
			return
		}
		s.mutateHTTP(c, "submit-runs", 202, raw, func(t *Tx) (any, error) { return s.Submit(t, req.PolicyIDs, c.GetInt64("quality_actor_id")) })
	})
	g.GET("/runs", s.runsHTTP)
	g.GET("/gallery", s.runsHTTP)
	g.GET("/runs/:id", func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			writeError(c, err)
			return
		}
		include, err := queryBool(c, "include_response")
		if err != nil {
			writeError(c, err)
			return
		}
		s.readHTTP(c, func(t *Tx) (any, error) {
			r, e := t.Run(id, false)
			if e != nil {
				return nil, e
			}
			available := r.ResponseText != nil && r.ResponseExpiresAt != nil && r.ResponseExpiresAt.After(t.Now)
			expired := r.ResponseExpiresAt != nil && !r.ResponseExpiresAt.After(t.Now)
			if include == nil || !*include || !available {
				r.ResponseText = nil
			}
			r.PresentEvidence(available && include != nil && *include)
			return map[string]any{"run": r, "response_available": available, "response_expired": expired, "candy": PresentCandy(r)}, nil
		})
	})
	g.POST("/runs/:id/cancel", func(c *gin.Context) {
		id, err := parseID(c)
		if err != nil {
			writeError(c, err)
			return
		}
		raw, err := body(c)
		if err != nil {
			writeError(c, err)
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if err = decode(raw, &req); err != nil {
			writeError(c, err)
			return
		}
		if strings.TrimSpace(req.Reason) == "" || len(req.Reason) > 500 {
			writeError(c, Invalid("reason", "请填写取消原因"))
			return
		}
		s.mutateHTTP(c, "cancel:"+strconv.FormatInt(id, 10), 200, raw, func(t *Tx) (any, error) { return s.Cancel(t, id, req.Reason, c.GetInt64("quality_actor_id")) })
	})
	g.POST("/summaries", s.summariesHTTP)
	g.GET("/events", func(c *gin.Context) {
		p, size, err := parsePage(c)
		if err != nil {
			writeError(c, err)
			return
		}
		policy, err := queryID(c, "policy_id")
		if err != nil {
			writeError(c, err)
			return
		}
		account, err := queryID(c, "account_id")
		if err != nil {
			writeError(c, err)
			return
		}
		run, err := queryID(c, "run_id")
		if err != nil {
			writeError(c, err)
			return
		}
		from, err := queryTime(c, "from")
		if err != nil {
			writeError(c, err)
			return
		}
		to, err := queryTime(c, "to")
		if err != nil {
			writeError(c, err)
			return
		}
		s.readHTTP(c, func(t *Tx) (any, error) {
			items, total, e := t.Events(policy, account, run, c.Query("event_type"), from, to, size, (p-1)*size)
			if e != nil {
				return nil, e
			}
			return gin.H{"items": items, "total": total, "page": p, "page_size": size, "pages": max(1, (total+size-1)/size)}, nil
		})
	})
}
func (s *Service) targetsHTTP(c *gin.Context) {
	p, size, err := parsePage(c)
	if err != nil {
		writeError(c, err)
		return
	}
	source, err := queryID(c, "source_group_id")
	if err != nil {
		writeError(c, err)
		return
	}
	account, err := queryID(c, "account_id")
	if err != nil {
		writeError(c, err)
		return
	}
	items, err := s.ListTargets(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	out := []Target{}
	for _, target := range items {
		if account > 0 && target.AccountID != account {
			continue
		}
		if !containsFold(target.AccountName, c.Query("search")) {
			continue
		}
		if source > 0 {
			found := false
			for _, id := range target.CompatibleSourceGroupIDs {
				found = found || id == source
			}
			if !found {
				continue
			}
			target.SourceGroupID = source
		}
		out = append(out, target)
	}
	success(c, 200, page(out, p, size))
}
func (s *Service) capabilitiesHTTP(c *gin.Context) {
	raw, err := body(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var req struct {
		Items []struct {
			AccountID     int64     `json:"account_id"`
			SourceGroupID int64     `json:"source_group_id"`
			Condition     Condition `json:"condition"`
		} `json:"items"`
	}
	if err = decode(raw, &req); err != nil {
		writeError(c, err)
		return
	}
	if len(req.Items) == 0 || len(req.Items) > 100 {
		writeError(c, Invalid("items", "请选择1–100个条件"))
		return
	}
	out := []Target{}
	for _, item := range req.Items {
		t, e := s.Resolve(c.Request.Context(), CreateItem{AccountID: item.AccountID, SourceGroupID: item.SourceGroupID, Condition: item.Condition})
		if e != nil {
			var qe *Error
			if !errors.As(e, &qe) || qe.Status >= 500 {
				writeError(c, e)
				return
			}
			t = Target{AccountID: item.AccountID, SourceGroupID: item.SourceGroupID, Condition: item.Condition, Reasons: []string{qe.Code}, Eligibility: "source_context_invalid"}
		}
		out = append(out, t)
	}
	success(c, 200, gin.H{"items": out})
}
func (s *Service) runsHTTP(c *gin.Context) {
	p, size, err := parsePage(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var f RunFilter
	if strings.HasSuffix(c.FullPath(), "/gallery") {
		if size > 12 {
			size = 12
		}
		f.IncludeEvidence = true
		f.BenchmarkID = "dual"
	}
	f.Limit = size
	f.Offset = (p - 1) * size
	f.PolicyID, err = queryID(c, "policy_id")
	if err != nil {
		writeError(c, err)
		return
	}
	f.AccountID, err = queryID(c, "account_id")
	if err != nil {
		writeError(c, err)
		return
	}
	f.SourceGroupID, err = queryID(c, "source_group_id")
	if err != nil {
		writeError(c, err)
		return
	}
	f.ModelID = strings.TrimSpace(c.Query("model_id"))
	f.Search = strings.TrimSpace(c.Query("search"))
	f.From, err = queryTime(c, "from")
	if err != nil {
		writeError(c, err)
		return
	}
	f.To, err = queryTime(c, "to")
	if err != nil {
		writeError(c, err)
		return
	}
	f.BatchID = c.Query("batch_id")
	f.ScopeHash = c.Query("scope_hash")
	f.Status = c.Query("status")
	f.Verdict = c.Query("verdict")
	f.Trigger = c.Query("trigger_source")
	for _, entry := range []struct {
		name  string
		field **int64
	}{{"global_control_epoch", &f.GlobalEpoch}, {"policy_control_epoch", &f.PolicyEpoch}} {
		if raw, ok := c.GetQuery(entry.name); ok {
			v, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || v < 0 {
				writeError(c, Invalid(entry.name, "代次错误"))
				return
			}
			*entry.field = &v
		}
	}
	s.readHTTP(c, func(t *Tx) (any, error) {
		asOf, e := queryTime(c, "monitor_as_of")
		if e != nil {
			return nil, e
		}
		_, hasSeconds := c.GetQuery("bucket_seconds")
		_, hasIndex := c.GetQuery("bucket_index")
		if asOf != nil || hasSeconds || hasIndex {
			if asOf == nil || !hasSeconds || !hasIndex || f.From != nil || f.To != nil || f.PolicyID == 0 || f.ScopeHash == "" {
				return nil, Invalid("monitor_as_of", "下钻必须完整提供策略、范围和时间桶参数")
			}
			seconds, e := strconv.Atoi(c.Query("bucket_seconds"))
			if e != nil || seconds < 300 || seconds > 86400 {
				return nil, Invalid("bucket_seconds", "时间桶错误")
			}
			index, e := strconv.Atoi(c.Query("bucket_index"))
			if e != nil || index < 0 || index >= (86400+seconds-1)/seconds {
				return nil, Invalid("bucket_index", "时间桶序号错误")
			}
			if asOf.After(t.Now) || asOf.Before(t.Now.Add(-time.Duration(t.Settings.MetadataRetentionDays)*24*time.Hour)) {
				return nil, Invalid("monitor_as_of", "时间超出有效保留期")
			}
			start := asOf.Add(-24 * time.Hour).Add(time.Duration(index*seconds) * time.Second)
			end := start.Add(time.Duration(seconds) * time.Second)
			if !end.Before(*asOf) {
				end = *asOf
				f.IncludeEnd = true
			}
			f.From = &start
			f.To = &end
		} else {
			if f.From == nil {
				start := t.Now.Add(-7 * 24 * time.Hour)
				f.From = &start
			}
			if f.To == nil {
				end := t.Now
				f.To = &end
			}
			if !f.From.Before(*f.To) || f.To.Sub(*f.From) > 180*24*time.Hour {
				return nil, Invalid("from", "时间范围必须为正且不超过180天")
			}
		}
		items, total, e := t.Runs(f)
		if e != nil {
			return nil, e
		}
		return gin.H{"items": items, "total": total, "page": p, "page_size": size, "pages": max(1, (total+size-1)/size)}, nil
	})
}
func (s *Service) summariesHTTP(c *gin.Context) {
	raw, err := body(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err = decode(raw, &req); err != nil {
		writeError(c, err)
		return
	}
	if len(req.AccountIDs) == 0 || len(req.AccountIDs) > 100 {
		writeError(c, Invalid("account_ids", "请选择1–100个账号"))
		return
	}
	s.readHTTP(c, func(t *Tx) (any, error) {
		all, e := t.Policies(false)
		if e != nil {
			return nil, e
		}
		out := []map[string]any{}
		rank := map[string]int{"normal": 0, "untested": 1, "observing": 2, "recovering": 3, "quarantined": 4}
		for _, id := range req.AccountIDs {
			if id <= 0 {
				return nil, Invalid("account_ids", "账号ID错误")
			}
			policies := filterPolicies(all, PolicyFilter{AccountID: id}, t.Settings, t.Now)
			worst := "untested"
			fresh, stale, enforced := 0, 0, 0
			for _, p := range policies {
				if rank[p.State.Name] > rank[worst] {
					worst = p.State.Name
				}
				if p.State.Freshness == "fresh" {
					fresh++
				} else if p.State.Freshness == "stale" {
					stale++
				}
				if p.Control.Effective {
					enforced++
				}
			}
			out = append(out, map[string]any{"account_id": id, "configured": len(policies) > 0, "active_policy_count": len(policies), "worst_state": worst, "fresh": fresh, "stale": stale, "enforced_count": enforced, "conditions": policies, "applies_to": RoutingMode})
		}
		return gin.H{"items": out}, nil
	})
}
