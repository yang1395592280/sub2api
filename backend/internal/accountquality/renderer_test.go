package accountquality

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrameRendererRejectsBadFramesAndRedirects(t *testing.T) {
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 960, 640))))
	encoded := base64.StdEncoding.EncodeToString(data.Bytes())
	for _, tc := range []struct {
		name   string
		status int
		frames []Frame
	}{
		{"missing frames", 200, []Frame{{Time: 0, PNG: encoded}}},
		{"duplicate time", 200, []Frame{{0, encoded}, {0, encoded}, {1, encoded}, {2, encoded}}},
		{"not PNG", 200, []Frame{{0, "bad"}, {1, "bad"}, {2, "bad"}, {3, "bad"}}},
		{"redirect", 302, nil},
		{"busy", 429, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				require.Equal(t, "Bearer private-test", r.Header.Get("Authorization"))
				w.Header().Set("Location", "/other")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(tc.frames)
			}))
			defer server.Close()
			renderer := NewFrameRenderer(server.URL, "private-test")
			_, err := renderer.Render(context.Background(), `<svg><circle r="10"/></svg>`)
			require.Error(t, err)
			require.Equal(t, 1, requests, "no redirect or paid retry")
			_, err = renderer.Render(context.Background(), `<svg><script/></svg>`)
			require.Error(t, err)
			require.Equal(t, 1, requests, "unsafe content stays local")
		})
	}
}
