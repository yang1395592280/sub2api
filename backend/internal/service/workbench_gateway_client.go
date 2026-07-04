package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

type HTTPWorkbenchGatewayClient struct {
	client  *http.Client
	baseURL string
}

type workbenchImageGatewayData struct {
	URL           string `json:"url"`
	B64JSON       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt"`
}

type workbenchImageUpload struct {
	Data        []byte
	ContentType string
	FileName    string
}

var (
	workbenchGatewaySecretPattern = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+\b`)
	workbenchGatewayBearerPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._-]+\b`)
)

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
		Created int64                       `json:"created"`
		Data    []workbenchImageGatewayData `json:"data"`
	}
	headers := map[string]string{}
	if host := strings.TrimSpace(req.PublicHost); host != "" {
		headers["X-Forwarded-Host"] = host
	}
	if scheme := strings.TrimSpace(req.PublicScheme); scheme != "" {
		headers["X-Forwarded-Proto"] = strings.TrimRight(scheme, ":/")
	}

	path := workbenchImageGatewayPath(req)
	if path == openAIImagesEditsEndpoint {
		body, contentType, ok, err := buildWorkbenchImageEditMultipart(payload)
		if err != nil {
			return WorkbenchGatewayImageResponse{}, err
		}
		if ok {
			if err := c.postMultipartWithHeaders(ctx, path, authorization, body, contentType, headers, &resp); err != nil {
				return WorkbenchGatewayImageResponse{}, err
			}
			return buildWorkbenchImageResponse(resp.Created, resp.Data), nil
		}
	}

	if err := c.postJSONWithHeaders(ctx, path, authorization, payload, headers, &resp); err != nil {
		return WorkbenchGatewayImageResponse{}, err
	}
	return buildWorkbenchImageResponse(resp.Created, resp.Data), nil
}

func buildWorkbenchImageResponse(created int64, data []workbenchImageGatewayData) WorkbenchGatewayImageResponse {
	images := make([]WorkbenchImageOutput, 0, len(data))
	for _, item := range data {
		images = append(images, WorkbenchImageOutput{
			URL:     item.URL,
			B64JSON: item.B64JSON,
		})
	}
	return WorkbenchGatewayImageResponse{
		Images:   images,
		Metadata: map[string]any{"created": created, "image_count": len(images)},
	}
}

func workbenchImageGatewayPath(req WorkbenchGatewayImageRequest) string {
	switch req.Endpoint {
	case WorkbenchEndpointImagesEdits:
		return "/v1/images/edits"
	case WorkbenchEndpointImagesGenerations:
		return "/v1/images/generations"
	}
	if workbenchImageOptionsHasInputs(req.Options) {
		return "/v1/images/edits"
	}
	return "/v1/images/generations"
}

func workbenchImageOptionsHasInputs(options map[string]any) bool {
	if len(options) == 0 {
		return false
	}
	if value, ok := options["images"]; ok && value != nil {
		return true
	}
	if value, ok := options["image"]; ok && value != nil {
		return true
	}
	return false
}

func buildWorkbenchImageEditMultipart(payload map[string]any) (*bytes.Buffer, string, bool, error) {
	uploads, err := extractWorkbenchImageUploads(payload)
	if err != nil {
		return nil, "", false, err
	}
	if len(uploads) == 0 {
		return nil, "", false, nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writeWorkbenchMultipartFields(writer, payload); err != nil {
		_ = writer.Close()
		return nil, "", false, err
	}
	for _, upload := range uploads {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, upload.FileName))
		header.Set("Content-Type", upload.ContentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			_ = writer.Close()
			return nil, "", false, err
		}
		if _, err := part.Write(upload.Data); err != nil {
			_ = writer.Close()
			return nil, "", false, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", false, err
	}
	return body, writer.FormDataContentType(), true, nil
}

func writeWorkbenchMultipartFields(writer *multipart.Writer, payload map[string]any) error {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		if key == "images" || key == "image" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, ok, err := workbenchMultipartFieldValue(payload[key])
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	return nil
}

func workbenchMultipartFieldValue(value any) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}
	switch v := value.(type) {
	case string:
		return v, true, nil
	case bool:
		return strconv.FormatBool(v), true, nil
	case int:
		return strconv.Itoa(v), true, nil
	case int64:
		return strconv.FormatInt(v, 10), true, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true, nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", false, err
		}
		return string(encoded), true, nil
	}
}

func extractWorkbenchImageUploads(payload map[string]any) ([]workbenchImageUpload, error) {
	dataURLs := collectWorkbenchDataURLs(payload["images"], nil)
	dataURLs = collectWorkbenchDataURLs(payload["image"], dataURLs)
	uploads := make([]workbenchImageUpload, 0, len(dataURLs))
	for i, dataURL := range dataURLs {
		data, contentType, err := decodeWorkbenchImageDataURL(dataURL)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, workbenchImageUpload{
			Data:        data,
			ContentType: contentType,
			FileName:    fmt.Sprintf("reference-%d%s", i+1, workbenchImageExtension(contentType)),
		})
	}
	return uploads, nil
}

func collectWorkbenchDataURLs(value any, out []string) []string {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(strings.TrimSpace(v), "data:image/") {
			return append(out, strings.TrimSpace(v))
		}
	case []any:
		for _, item := range v {
			out = collectWorkbenchDataURLs(item, out)
		}
	case map[string]any:
		for _, key := range []string{"image_url", "data_url", "url"} {
			if raw, ok := v[key].(string); ok {
				out = collectWorkbenchDataURLs(raw, out)
			}
		}
	}
	return out
}

func decodeWorkbenchImageDataURL(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	comma := strings.Index(raw, ",")
	if comma <= len("data:") {
		return nil, "", fmt.Errorf("invalid image data URL")
	}
	metadata := strings.ToLower(raw[len("data:"):comma])
	if !strings.Contains(metadata, ";base64") {
		return nil, "", fmt.Errorf("image data URL must be base64 encoded")
	}
	contentType := strings.TrimSpace(strings.Split(metadata, ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("image data URL content type must be an image")
	}
	encoded := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', ' ':
			return -1
		default:
			return r
		}
	}, raw[comma+1:])
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", fmt.Errorf("decode image data URL: %w", err)
	}
	return data, contentType, nil
}

func workbenchImageExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".bin"
	}
}

func (c *HTTPWorkbenchGatewayClient) postJSON(ctx context.Context, path, authorization string, payload any, out any) error {
	return c.postJSONWithHeaders(ctx, path, authorization, payload, nil, out)
}

func (c *HTTPWorkbenchGatewayClient) postJSONWithHeaders(ctx context.Context, path, authorization string, payload any, headers map[string]string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.postBodyWithHeaders(ctx, path, authorization, bytes.NewReader(body), "application/json", headers, out)
}

func (c *HTTPWorkbenchGatewayClient) postMultipartWithHeaders(ctx context.Context, path, authorization string, body *bytes.Buffer, contentType string, headers map[string]string, out any) error {
	return c.postBodyWithHeaders(ctx, path, authorization, bytes.NewReader(body.Bytes()), contentType, headers, out)
}

func (c *HTTPWorkbenchGatewayClient) postBodyWithHeaders(ctx context.Context, path, authorization string, body io.Reader, contentType string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return workbenchGatewayError(resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func workbenchGatewayError(statusCode int, body []byte) error {
	message := strings.TrimSpace(gjson.GetBytes(body, "error.message").String())
	message = sanitizeWorkbenchGatewayErrorMessage(message)
	if message == "" {
		return fmt.Errorf("gateway returned %d", statusCode)
	}
	return fmt.Errorf("gateway returned %d: %s", statusCode, message)
}

func sanitizeWorkbenchGatewayErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "\r", " ")
	message = workbenchGatewaySecretPattern.ReplaceAllString(message, "[redacted]")
	message = workbenchGatewayBearerPattern.ReplaceAllString(message, "Bearer [redacted]")
	return truncateWorkbenchText(message, workbenchErrorMessageMax)
}
