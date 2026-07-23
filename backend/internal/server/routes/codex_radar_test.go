package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexRadarEmbedRemovesCommunitySectionAndInjectsBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/", r.URL.Path)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Radar</title></head><body><section class="site-announcement"><p>keep announcement</p><p class="site-announcement-hint">quick links</p><div class="site-announcement-actions"><a href="#codex-community">join community</a></div></section><main>content</main><section id="codex-community"><p>remove me</p></section></body></html>`))
	}))
	t.Cleanup(upstream.Close)

	proxy, err := newCodexRadarProxy(upstream.Client(), upstream.URL+"/")
	require.NoError(t, err)
	proxy.cacheTTL = time.Minute

	router := gin.New()
	router.GET("/embed", proxy.serveEmbed)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/embed", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "remove me")
	require.NotContains(t, recorder.Body.String(), `<section id="codex-community"`)
	require.NotContains(t, recorder.Body.String(), "quick links")
	require.NotContains(t, recorder.Body.String(), "join community")
	require.Contains(t, recorder.Body.String(), "keep announcement")
	require.Contains(t, recorder.Body.String(), `<base href="https://codexradar.com/"`)
	require.Contains(t, recorder.Body.String(), "/api/v1/codex-radar/upstream")
	require.Contains(t, recorder.Body.String(), `codex_radar_theme","light")`)
	require.Equal(t, "SAMEORIGIN", recorder.Header().Get("X-Frame-Options"))
	require.Contains(t, recorder.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'")
}

func TestCodexRadarReadOnlyAPIForwardsAllowedGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/model-ratings", r.URL.Path)
		require.Equal(t, "14", r.URL.Query().Get("history"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)

	proxy, err := newCodexRadarProxy(upstream.Client(), upstream.URL+"/")
	require.NoError(t, err)
	router := gin.New()
	router.GET("/upstream/*path", proxy.serveReadOnlyAPI)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/upstream/model-ratings?history=14", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestCodexRadarReadOnlyAPIRejectsUnknownPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	proxy, err := newCodexRadarProxy(&http.Client{}, "https://codexradar.com/")
	require.NoError(t, err)
	router := gin.New()
	router.GET("/upstream/*path", proxy.serveReadOnlyAPI)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/upstream/subscribe", strings.NewReader(`{"email":"user@example.com"}`)))

	require.Equal(t, http.StatusNotFound, recorder.Code)
}
