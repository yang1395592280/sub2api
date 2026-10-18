package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicGroupMonitorRunExcludesAdminIdentity(t *testing.T) {
	run := service.GroupMonitorRun{ID: 1, Kind: "pelican", Status: "success", Detail: service.GroupMonitorRunDetail{AccountID: 42, AccountName: "private-account", TraceID: "internal-trace", UpstreamMessageID: "msg-secret", UpstreamRequestID: "req-secret", Error: "upstream-error", MappedModel: "private-mapping", HTML: "<html></html>", Review: "pass"}}
	encoded, err := json.Marshal(publicGroupRun(run))
	require.NoError(t, err)
	for _, key := range []string{"account_id", "account_name", "trace_id", "upstream_message_id", "upstream_request_id", "error", "mapped_model", "private-account", "msg-secret"} {
		require.NotContains(t, string(encoded), key)
	}
	require.Contains(t, string(encoded), `"review":"pass"`)
}

type groupMonitorHTTPRepo struct {
	service.GroupMonitorRepository
	plan  service.GroupMonitor
	run   service.GroupMonitorRun
	reads int
}

func (r *groupMonitorHTTPRepo) List(context.Context) ([]service.GroupMonitor, error) {
	return []service.GroupMonitor{r.plan}, nil
}
func (r *groupMonitorHTTPRepo) History(context.Context, int64, string, string, int) ([]service.GroupMonitorRun, error) {
	r.reads++
	return []service.GroupMonitorRun{r.run}, nil
}
func (r *groupMonitorHTTPRepo) GetRun(context.Context, int64, int64) (*service.GroupMonitorRun, error) {
	r.reads++
	return &r.run, nil
}

type groupMonitorHTTPGroups struct {
	service.GroupRepository
	group *service.Group
}

func (r groupMonitorHTTPGroups) GetByID(context.Context, int64) (*service.Group, error) {
	return r.group, nil
}
func (r groupMonitorHTTPGroups) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{*r.group}, nil
}

type groupMonitorHTTPUsers struct {
	service.UserRepository
	user *service.User
}

func (r groupMonitorHTTPUsers) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}

type groupMonitorHTTPSettings struct {
	service.SettingRepository
	values map[string]string
}

func (r groupMonitorHTTPSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}
func TestGroupMonitorHTTPPermissionsAndPreviewFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 1, Status: service.StatusActive}
	repo := &groupMonitorHTTPRepo{plan: service.GroupMonitor{ID: 3, GroupID: 9, Config: service.GroupMonitorConfig{Enabled: true}, LatestRuns: []service.GroupMonitorRun{}}, run: service.GroupMonitorRun{ID: 7, Kind: "pelican", Detail: service.GroupMonitorRunDetail{AccountID: 42, AccountName: "secret-account", TraceID: "private-trace", HTML: "<html>preview</html>"}}}
	values := map[string]string{service.SettingKeyChannelMonitorEnabled: "true", service.SettingKeyChannelMonitorMode: "v1", service.SettingKeyMonitorUserVisible: "true", service.SettingKeyMonitorPelicanUserVisible: "true"}
	settings := service.NewSettingService(groupMonitorHTTPSettings{values: values}, &config.Config{})
	svc := service.NewGroupMonitorService(repo, groupMonitorHTTPGroups{group: &service.Group{ID: 9, IsExclusive: true, Status: service.StatusActive}}, groupMonitorHTTPUsers{user: user}, nil, settings, nil, nil, &service.AccountTestService{})
	defer svc.Stop()
	h := NewGroupMonitorHandler(svc, settings)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1}) })
	router.GET("/monitors", h.UserList)
	router.GET("/monitors/:id/history", h.UserHistory)
	router.GET("/monitors/:id/runs/:run_id", h.UserRun)
	request := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	require.Equal(t, 404, request("/monitors/3/history?kind=pelican").Code)
	require.Equal(t, 404, request("/monitors/3/runs/7").Code)
	require.Zero(t, repo.reads)
	user.AllowedGroups = []int64{9}
	for _, path := range []string{"/monitors", "/monitors/3/history?kind=pelican", "/monitors/3/runs/7"} {
		w := request(path)
		require.Equal(t, 200, w.Code)
		require.NotContains(t, w.Body.String(), "secret-account")
		require.NotContains(t, w.Body.String(), "private-trace")
		require.NotContains(t, w.Body.String(), "account_id")
	}
	values[service.SettingKeyMonitorPelicanUserVisible] = "false"
	before := repo.reads
	require.Equal(t, 404, request("/monitors/3/history?kind=pelican").Code)
	require.Equal(t, before, repo.reads)
	require.Equal(t, 404, request("/monitors/3/runs/7").Code)
	values[service.SettingKeyMonitorUserVisible] = "false"
	require.Equal(t, 404, request("/monitors/3/runs/7").Code)
	require.Contains(t, request("/monitors").Body.String(), `"data":[]`)
}
