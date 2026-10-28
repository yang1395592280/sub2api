package accountquality

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Frame struct {
	Time float64 `json:"time"`
	PNG  string  `json:"png"`
}
type FrameRenderer struct {
	endpoint string
	token    string
	client   *http.Client
}

// The endpoint is an operator-provided private service, never an admin-supplied
// arbitrary URL. Credentials are not transmitted to the renderer.
func NewFrameRenderer(endpoint, token string) *FrameRenderer {
	u, e := url.Parse(strings.TrimSpace(endpoint))
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil
	}
	return &FrameRenderer{endpoint: strings.TrimRight(u.String(), "/") + "/render", token: token, client: &http.Client{Timeout: 35 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (r *FrameRenderer) Configured() bool { return r != nil && r.endpoint != "" }
func (r *FrameRenderer) Render(ctx context.Context, svg string) ([]Frame, error) {
	if !r.Configured() {
		return nil, errors.New("renderer not configured")
	}
	safe, check := InspectSVG(svg, "", true)
	if safe == "" || !check.Previewable {
		return nil, errors.New("unsafe SVG")
	}
	body, _ := json.Marshal(map[string]string{"svg": safe})
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, e := r.client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("renderer unavailable")
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if e != nil || len(raw) > 8<<20 {
		return nil, errors.New("renderer output exceeds limit")
	}
	var frames []Frame
	if e = json.Unmarshal(raw, &frames); e != nil || len(frames) != 4 {
		return nil, errors.New("invalid frames")
	}
	last := -1.0
	for _, frame := range frames {
		if math.IsNaN(frame.Time) || math.IsInf(frame.Time, 0) || frame.Time < 0 || frame.Time > 10 || frame.Time <= last {
			return nil, errors.New("invalid frame time")
		}
		last = frame.Time
		b, e := base64.StdEncoding.DecodeString(frame.PNG)
		if e != nil || len(b) > 2<<20 {
			return nil, errors.New("invalid frame encoding")
		}
		cfg, e := png.DecodeConfig(bytes.NewReader(b))
		if e != nil || cfg.Width != 960 || cfg.Height != 640 {
			return nil, errors.New("invalid frame dimensions")
		}
	}
	return frames, nil
}

func DispatchReason(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "dispatch_rejected"
}
