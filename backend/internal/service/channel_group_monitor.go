package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

var ErrGroupMonitorBusy = errors.New("监控正在运行或已达到每日请求上限")
var ErrGroupMonitorInvalid = errors.New("无效的分组监控配置")

type GroupMonitorConfig struct {
	Models          []string `json:"models"`
	Enabled         bool     `json:"enabled"`
	ProbeInterval   int      `json:"probe_interval_seconds"`
	PelicanEnabled  bool     `json:"pelican_enabled"`
	PelicanInterval int      `json:"pelican_interval_seconds"`
	PelicanPrompt   string   `json:"pelican_prompt"`
	DailyLimit      int      `json:"daily_request_limit"`
	KeepHistory     int      `json:"keep_history"`
}
type GroupMonitor struct {
	LatestRuns    []GroupMonitorRun  `json:"latest_runs"`
	ID            int64              `json:"id"`
	GroupID       int64              `json:"group_id"`
	GroupName     string             `json:"group_name"`
	Platform      string             `json:"platform"`
	Config        GroupMonitorConfig `json:"config"`
	NextProbeAt   time.Time          `json:"next_probe_at"`
	NextPelicanAt time.Time          `json:"next_pelican_at"`
	LeaseUntil    *time.Time         `json:"lease_until"`
	RequestsUsed  int                `json:"requests_used"`
}
type GroupMonitorRun struct {
	ID         int64                 `json:"id"`
	MonitorID  int64                 `json:"monitor_id"`
	Kind       string                `json:"kind"`
	Model      string                `json:"model"`
	Status     string                `json:"status"`
	CreatedAt  time.Time             `json:"created_at"`
	FinishedAt *time.Time            `json:"finished_at"`
	Detail     GroupMonitorRunDetail `json:"detail"`
}
type GroupMonitorRunDetail struct {
	AccountID         int64  `json:"account_id,omitempty"`
	AccountName       string `json:"account_name,omitempty"`
	MappedModel       string `json:"mapped_model,omitempty"`
	TraceID           string `json:"trace_id,omitempty"`
	UpstreamMessageID string `json:"upstream_message_id,omitempty"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
	Error             string `json:"error,omitempty"`
	LatencyMs         int64  `json:"latency_ms"`
	TTFTMs            *int64 `json:"ttft_ms,omitempty"`
	InputTokens       *int64 `json:"input_tokens,omitempty"`
	OutputTokens      *int64 `json:"output_tokens,omitempty"`
	TotalTokens       *int64 `json:"total_tokens,omitempty"`
	HTML              string `json:"html,omitempty"`
	Review            string `json:"review"`
}
type GroupMonitorClaim struct {
	Monitor GroupMonitor
	Token   string
	Runs    []GroupMonitorRun
}
type GroupMonitorRepository interface {
	Recover(context.Context) error
	List(context.Context) ([]GroupMonitor, error)
	SaveBatch(context.Context, []int64, GroupMonitorConfig) error
	Delete(context.Context, int64) error
	Claim(context.Context, int64, string, string, bool) (*GroupMonitorClaim, error)
	FinishRun(context.Context, int64, string, GroupMonitorRun) error
	Release(context.Context, int64, string, int) error
	History(context.Context, int64, string, string, int) ([]GroupMonitorRun, error)
	GetRun(context.Context, int64, int64) (*GroupMonitorRun, error)
	Review(context.Context, int64, int64, string) error
}

type groupMonitorAccountSelector func(context.Context, *Group, string) (*Account, error)
type groupMonitorExecutor func(context.Context, int64, string, string) (*ScheduledTestResult, error)
type GroupMonitorService struct {
	repo          GroupMonitorRepository
	groups        GroupRepository
	users         UserRepository
	subscriptions UserSubscriptionRepository
	settings      *SettingService
	selectAccount groupMonitorAccountSelector
	execute       groupMonitorExecutor
	slots         chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	once          sync.Once
	mu            sync.Mutex
	stopped       bool
}

func NewGroupMonitorService(repo GroupMonitorRepository, groups GroupRepository, users UserRepository, subscriptions UserSubscriptionRepository, settings *SettingService, gateway *GatewayService, openai *OpenAIGatewayService, tester *AccountTestService) *GroupMonitorService {
	ctx, cancel := context.WithCancel(context.Background())
	s := &GroupMonitorService{repo: repo, groups: groups, users: users, subscriptions: subscriptions, settings: settings, slots: make(chan struct{}, 2), ctx: ctx, cancel: cancel}
	s.execute = tester.RunMonitorPromptBackground
	s.selectAccount = func(ctx context.Context, g *Group, model string) (*Account, error) {
		if domain.UsesOpenAIGateway(g.Platform) {
			return openai.SelectAccountForTokenCount(ctx, &g.ID, "", model, "", g.Platform)
		}
		// This administrator account sample must stay in the configured group;
		// client restrictions must not redirect it to a fallback group.
		return gateway.SelectAccountForModel(SetClaudeCodeClient(ctx, true), &g.ID, "", model)
	}
	return s
}
func validateGroupMonitorConfig(c *GroupMonitorConfig) error {
	if len(c.Models) < 1 || len(c.Models) > 5 || c.ProbeInterval < 60 || c.ProbeInterval > 86400 || c.PelicanInterval < 600 || c.PelicanInterval > 604800 || c.DailyLimit < len(c.Models) || c.DailyLimit > 10000 || c.KeepHistory < 10 || c.KeepHistory > 500 || len(c.PelicanPrompt) > 12000 || (c.PelicanEnabled && strings.TrimSpace(c.PelicanPrompt) == "") {
		return ErrGroupMonitorInvalid
	}
	seen := map[string]bool{}
	for i, m := range c.Models {
		m = strings.TrimSpace(m)
		if m == "" || len(m) > 255 || seen[m] || isImageGenerationModel(m) || isOpenAIImageModel(m) || isGrokImageGenerationModel(m) || isGrokVideoGenerationModel(m) {
			return ErrGroupMonitorInvalid
		}
		seen[m] = true
		c.Models[i] = m
	}
	return nil
}
func (s *GroupMonitorService) Save(ctx context.Context, ids []int64, c GroupMonitorConfig) error {
	if len(ids) < 1 || len(ids) > 50 {
		return ErrGroupMonitorInvalid
	}
	if err := validateGroupMonitorConfig(&c); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return ErrGroupMonitorInvalid
		}
		seen[id] = true
		g, err := s.groups.GetByID(ctx, id)
		if err != nil {
			return err
		}
		// Composite routes may rewrite the target group/model per request. Keep
		// account sampling explicit until those rewrites can be retained per model.
		if !g.IsActive() || g.IsSelfHostedPool() || g.Platform == PlatformComposite {
			return fmt.Errorf("%w: 请选择启用的普通分组", ErrGroupMonitorInvalid)
		}
	}
	return s.repo.SaveBatch(ctx, ids, c)
}
func (s *GroupMonitorService) List(ctx context.Context) ([]GroupMonitor, error) {
	return s.repo.List(ctx)
}
func (s *GroupMonitorService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
func (s *GroupMonitorService) History(ctx context.Context, id int64, kind, model string) ([]GroupMonitorRun, error) {
	if kind != "" && kind != "probe" && kind != "pelican" {
		return nil, ErrGroupMonitorInvalid
	}
	return s.repo.History(ctx, id, kind, model, 500)
}
func (s *GroupMonitorService) Review(ctx context.Context, id, runID int64, review string) error {
	if review != "unreviewed" && review != "pass" && review != "fail" && review != "uncertain" {
		return ErrGroupMonitorInvalid
	}
	return s.repo.Review(ctx, id, runID, review)
}
func (s *GroupMonitorService) active(ctx context.Context, kind string) bool {
	r := s.settings.GetChannelMonitorRuntime(ctx)
	return s.settings.CustomFeatureEnabled(ctx, SettingKeyChannelMonitorEnabled) && r.Enabled && r.Mode == ChannelMonitorModeV1 && (kind != "pelican" || s.settings.CustomFeatureEnabled(ctx, SettingKeyMonitorPelicanEnabled))
}

// StartRun claims synchronously so administrators see lease/budget conflicts.
// A round chooses once and uses that exact account for every configured model.
func (s *GroupMonitorService) StartRun(ctx context.Context, id int64, kind string, manual bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return context.Canceled
	}
	if (kind != "probe" && kind != "pelican") || !s.active(ctx, kind) {
		return fmt.Errorf("%w: 检测开关未开启或不是 V1 模式", ErrGroupMonitorInvalid)
	}
	select {
	case <-s.ctx.Done():
		return context.Canceled
	default:
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return ErrGroupMonitorBusy
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		<-s.slots
		return err
	}
	claim, err := s.repo.Claim(ctx, id, kind, hex.EncodeToString(b[:]), manual)
	if err != nil {
		<-s.slots
		return err
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); defer func() { <-s.slots }(); s.run(claim) }()
	return nil
}
func (s *GroupMonitorService) run(claim *GroupMonitorClaim) {
	ctx, cancel := context.WithTimeout(s.ctx, 17*time.Minute)
	defer cancel()
	g, err := s.groups.GetByID(ctx, claim.Monitor.GroupID)
	var account *Account
	if err == nil && (!g.IsActive() || g.IsSelfHostedPool() || g.Platform == PlatformComposite) {
		err = errors.New("分组已停用或不支持账号抽测")
	}
	if err == nil {
		account, err = s.selectAccount(ctx, g, claim.Monitor.Config.Models[0])
	}
	for _, run := range claim.Runs {
		run.Detail = GroupMonitorRunDetail{TraceID: fmt.Sprintf("gm-%s-%d", claim.Token, run.ID), Review: "unreviewed"}
		run.Status = "failed"
		if err != nil || account == nil {
			run.Detail.Error = "分组无可用账号或选择账号失败"
		} else {
			run.Detail.AccountID = account.ID
			run.Detail.AccountName = account.Name
			run.Detail.MappedModel = account.GetMappedModel(run.Model)
			if run.Kind == "pelican" && !accountSupportsPelicanPrompt(account) {
				run.Detail.Error = "该账号协议暂不支持鹈鹕提示词"
			} else if isImageGenerationModel(run.Detail.MappedModel) || isOpenAIImageModel(run.Detail.MappedModel) || isGrokImageGenerationModel(run.Detail.MappedModel) || isGrokVideoGenerationModel(run.Detail.MappedModel) {
				run.Detail.Error = "映射后的媒体模型不支持文本检测"
			} else if !account.IsModelSupported(run.Model) {
				run.Detail.Error = "本轮选中账号不支持该模型；未切换账号"
			} else {
				prompt := "请只回复 OK"
				timeout := 45 * time.Second
				if run.Kind == "pelican" {
					prompt = claim.Monitor.Config.PelicanPrompt
					timeout = 180 * time.Second
				}
				requestCtx, requestCancel := context.WithTimeout(ctx, timeout)
				result, testErr := s.execute(requestCtx, account.ID, run.Model, prompt)
				requestCancel()
				if testErr != nil || result == nil {
					run.Detail.Error = "账号测试执行失败"
				} else {
					run.Status = result.Status
					run.Detail.LatencyMs = result.LatencyMs
					run.Detail.TTFTMs = result.TTFTMs
					run.Detail.UpstreamMessageID = result.UpstreamMessageID
					run.Detail.UpstreamRequestID = result.UpstreamRequestID
					run.Detail.InputTokens = result.InputTokens
					run.Detail.OutputTokens = result.OutputTokens
					run.Detail.TotalTokens = result.TotalTokens
					run.Detail.Error = redactPelicanAccountSecrets(account, result.ErrorMessage)
					if run.Kind == "pelican" && run.Status == "success" {
						output := redactPelicanAccountSecrets(account, result.ResponseText)
						if len(output) > pelicanTestMaxOutput {
							run.Status = "failed"
							run.Detail.Error = "生成结果超过存储上限"
						} else {
							run.Detail.HTML = extractPelicanHTML(output)
							if run.Detail.HTML == "" {
								run.Status = "failed"
								run.Detail.Error = "上游未生成完整 HTML"
							}
						}
					}
					if run.Kind == "probe" && run.Status == "success" && strings.TrimSpace(result.ResponseText) == "" {
						run.Status = "failed"
						run.Detail.Error = "上游返回空内容"
					}
				}
			}
		}
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if saveErr := s.repo.FinishRun(saveCtx, claim.Monitor.ID, claim.Token, run); saveErr != nil {
			slog.Error("group monitor save failed", "trace_id", run.Detail.TraceID, "error", saveErr)
		}
		saveCancel()
		slog.Info("group monitor run completed", "monitor_id", claim.Monitor.ID, "run_id", run.ID, "account_id", run.Detail.AccountID, "model", run.Model, "status", run.Status, "trace_id", run.Detail.TraceID, "upstream_request_id", run.Detail.UpstreamRequestID, "upstream_message_id", run.Detail.UpstreamMessageID)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err := s.repo.Release(finishCtx, claim.Monitor.ID, claim.Token, claim.Monitor.Config.KeepHistory); err != nil {
		slog.Error("group monitor release failed", "monitor_id", claim.Monitor.ID, "error", err)
	}
}
func (s *GroupMonitorService) Start() {
	s.once.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.ctx.Done():
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
					if err := s.repo.Recover(ctx); err != nil {
						slog.Error("group monitor recovery failed", "error", err)
					}
					plans, err := s.repo.List(ctx)
					if err != nil {
						slog.Error("group monitor schedule failed", "error", err)
					} else {
						for _, p := range plans {
							if p.Config.Enabled {
								for _, kind := range []string{"probe", "pelican"} {
									if kind == "pelican" && !p.Config.PelicanEnabled {
										continue
									}
									if err := s.StartRun(ctx, p.ID, kind, false); err != nil && !errors.Is(err, ErrGroupMonitorBusy) && !errors.Is(err, ErrGroupMonitorInvalid) {
										slog.Warn("group monitor claim failed", "monitor_id", p.ID, "error", err)
									}
								}
							}
						}
					}
					cancel()
				}
			}
		}()
	})
}
func (s *GroupMonitorService) Stop() {
	s.mu.Lock()
	s.stopped = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

// Visible authorizes every user endpoint from current account/group/subscription
// state, rather than trusting the list previously loaded by a browser.
func (s *GroupMonitorService) Visible(ctx context.Context, userID int64, p GroupMonitor) bool {
	if !p.Config.Enabled {
		return false
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil || !u.IsActive() {
		return false
	}
	g, err := s.groups.GetByID(ctx, p.GroupID)
	if err != nil || !g.IsActive() || g.IsSelfHostedPool() || g.Platform == PlatformComposite {
		return false
	}
	if g.IsSubscriptionType() {
		sub, e := s.subscriptions.GetActiveByUserIDAndGroupID(ctx, userID, g.ID)
		return e == nil && sub != nil && sub.IsActive() && !sub.StartsAt.After(time.Now())
	}
	return u.CanBindGroup(g.ID, g.IsExclusive)
}

func (s *GroupMonitorService) FilterVisible(ctx context.Context, userID int64, plans []GroupMonitor) ([]GroupMonitor, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := []GroupMonitor{}
	if !user.IsActive() {
		return out, nil
	}
	groups, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]Group{}
	needsSubscriptions := false
	for _, g := range groups {
		byID[g.ID] = g
	}
	for _, p := range plans {
		if g, ok := byID[p.GroupID]; ok && g.IsSubscriptionType() {
			needsSubscriptions = true
		}
	}
	subscribed := map[int64]bool{}
	if needsSubscriptions {
		items, e := s.subscriptions.ListActiveByUserID(ctx, userID)
		if e != nil {
			return nil, e
		}
		for _, sub := range items {
			if sub.IsActive() && !sub.StartsAt.After(time.Now()) {
				subscribed[sub.GroupID] = true
			}
		}
	}
	for _, p := range plans {
		g, ok := byID[p.GroupID]
		if !ok || !g.IsActive() || g.IsSelfHostedPool() || g.Platform == PlatformComposite || !p.Config.Enabled {
			continue
		}
		if (g.IsSubscriptionType() && subscribed[g.ID]) || (!g.IsSubscriptionType() && user.CanBindGroup(g.ID, g.IsExclusive)) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *GroupMonitorService) GetRun(ctx context.Context, id, runID int64) (*GroupMonitorRun, error) {
	return s.repo.GetRun(ctx, id, runID)
}
