package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupHandlerReadEditPreservesPriceOverrides(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newStubAdminService()
	search, realtime, tts, stt := 0.0, 1.25, 2.5, 3.75
	group := service.Group{
		ID: 7, Name: "priced group", Platform: service.PlatformGemini, GroupRole: service.GroupRoleStandard,
		SearchPricePer1k:             &search,
		AudioRealtimePricePerMin:     &realtime,
		AudioTTSPricePerMillionChars: &tts,
		AudioSTTPricePerHour:         &stt,
		VideoModelPrices:             map[string]map[string]float64{"veo-3": {"720p": 0, "1080p": 0.4}},
	}
	svc.groups = []service.Group{group}
	h := NewGroupHandler(svc, nil, nil)
	r := gin.New()
	r.GET("/groups", h.List)
	r.GET("/groups/all", h.GetAll)
	r.GET("/groups/:id", h.GetByID)
	r.PUT("/groups/:id", h.Update)

	for _, path := range []string{"/groups", "/groups/all", "/groups/7"} {
		t.Run(path, func(t *testing.T) {
			res := httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusOK, res.Code)
			var body struct {
				Data json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
			var fields map[string]any
			switch path {
			case "/groups":
				var page struct {
					Items []map[string]any `json:"items"`
				}
				require.NoError(t, json.Unmarshal(body.Data, &page))
				require.Len(t, page.Items, 1)
				fields = page.Items[0]
			case "/groups/all":
				var groups []map[string]any
				require.NoError(t, json.Unmarshal(body.Data, &groups))
				require.Len(t, groups, 1)
				fields = groups[0]
			default:
				require.NoError(t, json.Unmarshal(body.Data, &fields))
			}
			require.Equal(t, search, fields["search_price_per_1k"])
			require.Equal(t, realtime, fields["audio_realtime_price_per_min"])
			require.Equal(t, tts, fields["audio_tts_price_per_million_chars"])
			require.Equal(t, stt, fields["audio_stt_price_per_hour"])
			require.Equal(t, map[string]any{
				"veo-3": map[string]any{"720p": float64(0), "1080p": 0.4},
			}, fields["video_model_prices"])

			// 模拟编辑表单只改名称，原价格随读取结果提交，不能变成清空覆盖价。
			fields["name"] = "renamed group"
			payload, err := json.Marshal(fields)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPut, "/groups/7", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			res = httptest.NewRecorder()
			r.ServeHTTP(res, req)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())
			updated := svc.updatedGroups[len(svc.updatedGroups)-1]
			require.Equal(t, "renamed group", updated.Name)
			require.Equal(t, group.SearchPricePer1k, updated.SearchPricePer1k)
			require.Equal(t, group.AudioRealtimePricePerMin, updated.AudioRealtimePricePerMin)
			require.Equal(t, group.AudioTTSPricePerMillionChars, updated.AudioTTSPricePerMillionChars)
			require.Equal(t, group.AudioSTTPricePerHour, updated.AudioSTTPricePerHour)
			require.Equal(t, group.VideoModelPrices, updated.VideoModelPrices)
		})
	}
}
