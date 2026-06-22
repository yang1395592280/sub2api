package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type HTTPWorkbenchGatewayClient struct {
	client  *http.Client
	baseURL string
}

func NewHTTPWorkbenchGatewayClient(cfg *config.Config) WorkbenchGatewayClient {
	host := "127.0.0.1"
	port := 8080
	if cfg != nil {
		if strings.TrimSpace(cfg.Server.Host) != "" && cfg.Server.Host != "0.0.0.0" && cfg.Server.Host != "::" {
			host = strings.TrimSpace(cfg.Server.Host)
		}
		if cfg.Server.Port > 0 {
			port = cfg.Server.Port
		}
	}
	return &HTTPWorkbenchGatewayClient{
		client:  &http.Client{Timeout: 5 * time.Minute},
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
	}
}

func (c *HTTPWorkbenchGatewayClient) SendChat(ctx context.Context, authorization string, req WorkbenchGatewayChatRequest) (WorkbenchGatewayChatResponse, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	}
	for k, v := range req.Options {
		if v != nil {
			payload[k] = v
		}
	}

	var resp struct {
		ID      string         `json:"id"`
		Usage   map[string]any `json:"usage"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.postJSON(ctx, "/v1/chat/completions", authorization, payload, &resp); err != nil {
		return WorkbenchGatewayChatResponse{}, err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return WorkbenchGatewayChatResponse{
		Content:  content,
		Metadata: map[string]any{"id": resp.ID, "usage": resp.Usage},
	}, nil
}

func (c *HTTPWorkbenchGatewayClient) GenerateImage(ctx context.Context, authorization string, req WorkbenchGatewayImageRequest) (WorkbenchGatewayImageResponse, error) {
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	for k, v := range req.Options {
		if v != nil {
			payload[k] = v
		}
	}

	var resp struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	headers := map[string]string{}
	if host := strings.TrimSpace(req.PublicHost); host != "" {
		headers["X-Forwarded-Host"] = host
	}
	if scheme := strings.TrimSpace(req.PublicScheme); scheme != "" {
		headers["X-Forwarded-Proto"] = strings.TrimRight(scheme, ":/")
	}
	if err := c.postJSONWithHeaders(ctx, "/v1/images/generations", authorization, payload, headers, &resp); err != nil {
		return WorkbenchGatewayImageResponse{}, err
	}

	images := make([]WorkbenchImageOutput, 0, len(resp.Data))
	for _, item := range resp.Data {
		images = append(images, WorkbenchImageOutput{
			URL:     item.URL,
			B64JSON: item.B64JSON,
		})
	}
	return WorkbenchGatewayImageResponse{
		Images:   images,
		Metadata: map[string]any{"created": resp.Created, "image_count": len(images)},
	}, nil
}

func (c *HTTPWorkbenchGatewayClient) postJSON(ctx context.Context, path, authorization string, payload any, out any) error {
	return c.postJSONWithHeaders(ctx, path, authorization, payload, nil, out)
}

func (c *HTTPWorkbenchGatewayClient) postJSONWithHeaders(ctx context.Context, path, authorization string, payload any, headers map[string]string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)
	for k, v := range headers {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			req.Header.Set(k, strings.TrimSpace(v))
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("gateway returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
