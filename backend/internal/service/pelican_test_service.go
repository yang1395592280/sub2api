package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	PelicanTestMaxAccounts = 100
	PelicanTestMaxPrompt   = 4000
	pelicanTestMaxOutput   = 1 << 20
	pelicanTestTimeout     = 500 * time.Second
)

var ErrInvalidPelicanBatch = errors.New("invalid pelican test batch")

var (
	pelicanCodeFence         = regexp.MustCompile("(?is)```([^\\r\\n]*)\\r?\\n([\\s\\S]*?)\\r?\\n[ \\t]*```")
	pelicanHTMLDocumentStart = regexp.MustCompile(`(?is)<!doctype\s+html\b|<html(?:\s|>)`)
	pelicanHTMLDocumentEnd   = regexp.MustCompile(`(?is)</html\s*>`)
	pelicanDoctype           = regexp.MustCompile(`(?is)^\s*<!doctype\s+html\b`)
	pelicanHTMLStart         = regexp.MustCompile(`(?is)^\s*<html(?:\s|>)`)
)

type PelicanTestService struct {
	repo        PelicanTestRepository
	accountRepo AccountRepository
	accountTest *AccountTestService
	semaphore   chan struct{}
}

func NewPelicanTestService(repo PelicanTestRepository, accountRepo AccountRepository, accountTest *AccountTestService) *PelicanTestService {
	return &PelicanTestService{repo: repo, accountRepo: accountRepo, accountTest: accountTest, semaphore: make(chan struct{}, 3)}
}

// StartBatch only persists work after validating the selection. Each job
// then uses the selected account ID, never the public gateway scheduler.
func (s *PelicanTestService) StartBatch(ctx context.Context, accountIDs []int64, modelID, prompt string) ([]PelicanTest, error) {
	modelID = strings.TrimSpace(modelID)
	prompt = strings.TrimSpace(prompt)
	if len(accountIDs) == 0 || len(accountIDs) > PelicanTestMaxAccounts {
		return nil, fmt.Errorf("%w: select 1 to %d accounts", ErrInvalidPelicanBatch, PelicanTestMaxAccounts)
	}
	if modelID == "" || len(modelID) > 255 || prompt == "" || utf8.RuneCountInString(prompt) > PelicanTestMaxPrompt {
		return nil, fmt.Errorf("%w: model or prompt is empty or too long", ErrInvalidPelicanBatch)
	}
	seen := make(map[int64]bool, len(accountIDs))
	for _, id := range accountIDs {
		if id <= 0 || seen[id] {
			return nil, fmt.Errorf("%w: account IDs must be positive and unique", ErrInvalidPelicanBatch)
		}
		seen[id] = true
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	batchID := hex.EncodeToString(random[:])
	if err := s.repo.ExpireStale(ctx); err != nil {
		return nil, err
	}
	tests, err := s.repo.CreateBatch(ctx, batchID, accountIDs, modelID, prompt)
	if err != nil {
		return nil, err
	}
	if len(tests) > 0 {
		go s.runBatch(tests)
	}
	return tests, nil
}

func (s *PelicanTestService) runBatch(tests []PelicanTest) {
	for _, test := range tests {
		s.semaphore <- struct{}{}
		go func(test PelicanTest) {
			defer func() { <-s.semaphore }()
			s.runOne(test)
		}(test)
	}
}

func (s *PelicanTestService) runOne(test PelicanTest) {
	ctx, cancel := context.WithTimeout(context.Background(), pelicanTestTimeout)
	defer cancel()
	started, err := s.repo.MarkRunning(ctx, test.ID)
	if err != nil {
		log.Printf("pelican test %d account %d: mark running: %v", test.ID, test.AccountID, err)
		return
	}
	if !started {
		// A stale queued job may already have been marked failed by another request.
		return
	}
	status, output, html, reason, latency, usage := s.generate(ctx, test)
	// Persist a terminal state even when the upstream request timed out.
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err := s.repo.Finish(finishCtx, test.ID, status, output, html, reason, latency, usage); err != nil {
		log.Printf("pelican test %d account %d: save result: %v", test.ID, test.AccountID, err)
	}
}

func (s *PelicanTestService) generate(ctx context.Context, test PelicanTest) (status, output, html, reason string, latency int64, usage AccountTestTokenUsage) {
	account, err := s.accountRepo.GetByID(ctx, test.AccountID)
	if err != nil {
		return "failed", "", "", "账号已删除或不可用", 0, usage
	}
	// Only use routes whose existing account test path sends the supplied prompt.
	// Other protocols must fail explicitly instead of silently testing "hi".
	protocol := account.GetAPIProtocol()
	mappedModel := account.GetMappedModel(test.ModelID)
	supportsPrompt := account.IsOpenAI() || account.IsGemini() || account.IsGrok() ||
		(account.IsAnthropic() && (account.Type == AccountTypeOAuth || account.Type == AccountTypeAPIKey)) ||
		(account.IsCNProvider() && (protocol == APIProtocolChatCompletions || protocol == APIProtocolResponses || protocol == APIProtocolAnthropic)) ||
		account.IsOpenCodeGo() ||
		(account.Platform == PlatformAntigravity && account.Type == AccountTypeAPIKey)
	if !supportsPrompt {
		return "failed", "", "", "该账号协议暂不支持自定义 HTML 生成提示词", 0, usage
	}
	if !account.IsModelSupported(test.ModelID) {
		return "failed", "", "", "账号未配置所选模型", 0, usage
	}
	if isImageGenerationModel(mappedModel) || isOpenAIImageModel(mappedModel) || isGrokImageGenerationModel(mappedModel) || isGrokVideoGenerationModel(mappedModel) {
		return "failed", "", "", "图片或视频模型不支持 HTML 生成测试", 0, usage
	}
	result, err := s.accountTest.RunPromptBackground(ctx, test.AccountID, test.ModelID, test.Prompt)
	if err != nil {
		return "failed", "", "", "测试请求失败", 0, usage
	}
	latency = result.LatencyMs
	usage = AccountTestTokenUsage{InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, TotalTokens: result.TotalTokens}
	output = result.ResponseText
	if len(output) > pelicanTestMaxOutput {
		limited := output[:pelicanTestMaxOutput]
		for !utf8.ValidString(limited) {
			limited = limited[:len(limited)-1]
		}
		return "failed", limited, "", "返回内容超过 1 MiB 限制", latency, usage
	}
	if result.Status != "success" {
		reason = redactPelicanAccountSecrets(account, sanitizeUpstreamErrorMessage(result.ErrorMessage))
		if len(reason) > 2000 {
			reason = reason[:2000]
		}
		if ctx.Err() != nil {
			reason = "测试超时（500 秒）"
		}
		return "failed", output, "", reason, latency, usage
	}
	html = extractPelicanHTML(output)
	if html == "" {
		return "unpreviewable", output, "", "未返回完整 HTML 文档", latency, usage
	}
	return "previewable", output, html, "", latency, usage
}

func redactPelicanAccountSecrets(account *Account, message string) string {
	secrets := []string{account.GetOpenAIApiKey(), account.GetOpenAIProtocolAPIKey()}
	for key, value := range account.Credentials {
		name := strings.ToLower(key)
		if !strings.Contains(name, "token") && !strings.Contains(name, "key") && !strings.Contains(name, "secret") &&
			!strings.Contains(name, "password") && !strings.Contains(name, "cookie") {
			continue
		}
		if secret, ok := value.(string); ok {
			secrets = append(secrets, secret)
		}
	}
	for _, secret := range secrets {
		if len(secret) >= 8 {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	return message
}

func extractPelicanHTML(output string) string {
	candidate := strings.TrimSpace(output)
	for _, match := range pelicanCodeFence.FindAllStringSubmatch(candidate, -1) {
		language := ""
		if fields := strings.Fields(match[1]); len(fields) > 0 {
			language = strings.ToLower(fields[0])
		}
		if html := extractPelicanHTMLDocument(match[2]); html != "" {
			if language == "" || language == "html" {
				return html
			}
			return ""
		}
	}
	return extractPelicanHTMLDocument(candidate)
}

func extractPelicanHTMLDocument(output string) string {
	start := pelicanHTMLDocumentStart.FindStringIndex(output)
	if start == nil {
		return ""
	}
	ends := pelicanHTMLDocumentEnd.FindAllStringIndex(output, -1)
	if len(ends) == 0 || ends[len(ends)-1][1] <= start[0] {
		return ""
	}
	candidate := strings.TrimSpace(output[start[0]:ends[len(ends)-1][1]])
	if !(pelicanDoctype.MatchString(candidate) || pelicanHTMLStart.MatchString(candidate)) {
		return ""
	}
	return candidate
}

func (s *PelicanTestService) ListLatest(ctx context.Context, accountIDs []int64) ([]PelicanTest, error) {
	if len(accountIDs) > PelicanTestMaxAccounts {
		return nil, fmt.Errorf("at most %d account IDs", PelicanTestMaxAccounts)
	}
	if err := s.repo.ExpireStale(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListLatest(ctx, accountIDs)
}

func (s *PelicanTestService) Get(ctx context.Context, id int64) (*PelicanTest, error) {
	return s.repo.Get(ctx, id)
}
