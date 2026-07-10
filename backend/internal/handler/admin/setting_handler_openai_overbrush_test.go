package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIOverbrushSettingHandlerRepoStub struct {
	values map[string]string
}

func (s *openAIOverbrushSettingHandlerRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *openAIOverbrushSettingHandlerRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *openAIOverbrushSettingHandlerRepoStub) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *openAIOverbrushSettingHandlerRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *openAIOverbrushSettingHandlerRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *openAIOverbrushSettingHandlerRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *openAIOverbrushSettingHandlerRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSettingHandler_OpenAIOverbrushSettingsRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &openAIOverbrushSettingHandlerRepoStub{values: map[string]string{}}
	svc := service.NewSettingService(repo, nil)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/settings/openai-overbrush", handler.GetOpenAIOverbrushSettings)
	router.PUT("/settings/openai-overbrush", handler.UpdateOpenAIOverbrushSettings)

	put := httptest.NewRequest(http.MethodPut, "/settings/openai-overbrush", strings.NewReader(`{"consecutive_429_threshold":12}`))
	put.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, put)
	require.Equal(t, http.StatusOK, putRec.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"consecutive_429_threshold":12}}`, putRec.Body.String())

	get := httptest.NewRequest(http.MethodGet, "/settings/openai-overbrush", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, get)
	require.Equal(t, http.StatusOK, getRec.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"consecutive_429_threshold":12}}`, getRec.Body.String())
}

func TestSettingHandler_OpenAIOverbrushSettingsRejectsInvalidThreshold(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &openAIOverbrushSettingHandlerRepoStub{values: map[string]string{}}
	svc := service.NewSettingService(repo, nil)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/settings/openai-overbrush", handler.UpdateOpenAIOverbrushSettings)

	req := httptest.NewRequest(http.MethodPut, "/settings/openai-overbrush", strings.NewReader(`{"consecutive_429_threshold":0}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "consecutive_429_threshold must be between 1-100")
}
