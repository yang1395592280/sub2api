package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_UpdateSettings_PersistsAndReturnsJoinGroupSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyJoinGroupEnabled:    "false",
		service.SettingKeyJoinGroupURL:        "",
		service.SettingKeyJoinGroupPopupImage: "",
	}}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"join_group_enabled":     true,
		"join_group_url":         " https://qm.qq.com/q/example ",
		"join_group_popup_image": " data:image/png;base64,QUJD ",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyJoinGroupEnabled])
	require.Equal(t, "https://qm.qq.com/q/example", repo.lastUpdates[service.SettingKeyJoinGroupURL])
	require.Equal(t, "data:image/png;base64,QUJD", repo.lastUpdates[service.SettingKeyJoinGroupPopupImage])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["join_group_enabled"])
	require.Equal(t, "https://qm.qq.com/q/example", data["join_group_url"])
	require.Equal(t, "data:image/png;base64,QUJD", data["join_group_popup_image"])
}

func TestSettingHandler_UpdateSettings_PreservesOmittedJoinGroupSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyJoinGroupEnabled:    "true",
		service.SettingKeyJoinGroupURL:        "https://qm.qq.com/q/existing",
		service.SettingKeyJoinGroupPopupImage: "data:image/png;base64,QUJD",
	}}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyJoinGroupEnabled])
	require.Equal(t, "https://qm.qq.com/q/existing", repo.lastUpdates[service.SettingKeyJoinGroupURL])
	require.Equal(t, "data:image/png;base64,QUJD", repo.lastUpdates[service.SettingKeyJoinGroupPopupImage])
}
