package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupHandler_GetCapacityUsersReturnsPaginatedDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(10)
	override := 6
	capacitySvc := service.NewGroupCapacityService(
		nil,
		&groupCapacityHandlerGroupRepoStub{groups: map[int64]*service.Group{
			groupID: {ID: groupID, Name: "exclusive", IsExclusive: true, RPMLimit: 10},
		}},
		service.NewConcurrencyService(&groupCapacityHandlerUserLoadCacheStub{loads: map[int64]*service.UserLoadInfo{
			1: {UserID: 1, CurrentConcurrency: 2},
		}}),
		nil,
		nil,
		&groupCapacityHandlerUserRepoStub{usersByGroup: map[int64][]service.User{
			groupID: {
				{ID: 1, Username: "alpha", Email: "a@example.com", Status: service.StatusActive, Concurrency: 3, RPMLimit: 20},
			},
		}},
		nil,
		&groupCapacityHandlerRateRepoStub{overrides: map[int64]map[int64]*int{1: {groupID: &override}}},
		&groupCapacityHandlerUserRPMCacheStub{groupCounts: map[int64]map[int64]int{1: {groupID: 4}}},
	)
	handler := NewGroupHandler(newStubAdminService(), nil, capacitySvc)
	router := gin.New()
	router.GET("/api/v1/admin/groups/:id/capacity-users", handler.GetCapacityUsers)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/10/capacity-users?page=1&page_size=20&active_only=true", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data struct {
			Items []struct {
				UserID             int64  `json:"user_id"`
				Username           string `json:"username"`
				CurrentConcurrency int    `json:"current_concurrency"`
				CurrentRPM         int    `json:"current_rpm"`
				EffectiveRPMLimit  int    `json:"effective_rpm_limit"`
				RPMLimitSource     string `json:"rpm_limit_source"`
				RPMOverride        *int   `json:"rpm_override"`
			} `json:"items"`
			Total int64 `json:"total"`
			Page  int   `json:"page"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.EqualValues(t, 1, body.Data.Total)
	require.Equal(t, 1, body.Data.Page)
	require.Len(t, body.Data.Items, 1)
	require.Equal(t, int64(1), body.Data.Items[0].UserID)
	require.Equal(t, "alpha", body.Data.Items[0].Username)
	require.Equal(t, 2, body.Data.Items[0].CurrentConcurrency)
	require.Equal(t, 4, body.Data.Items[0].CurrentRPM)
	require.Equal(t, 6, body.Data.Items[0].EffectiveRPMLimit)
	require.Equal(t, "override", body.Data.Items[0].RPMLimitSource)
	require.NotNil(t, body.Data.Items[0].RPMOverride)
	require.Equal(t, 6, *body.Data.Items[0].RPMOverride)
}

type groupCapacityHandlerGroupRepoStub struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (s *groupCapacityHandlerGroupRepoStub) GetByID(_ context.Context, id int64) (*service.Group, error) {
	return s.groups[id], nil
}

type groupCapacityHandlerUserRepoStub struct {
	service.UserRepository
	usersByGroup map[int64][]service.User
}

func (s *groupCapacityHandlerUserRepoStub) ListAllowedUsersByGroupID(_ context.Context, groupID int64) ([]service.User, error) {
	return append([]service.User(nil), s.usersByGroup[groupID]...), nil
}

func (s *groupCapacityHandlerUserRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, _ service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	return []service.User{}, &pagination.PaginationResult{}, nil
}

type groupCapacityHandlerUserLoadCacheStub struct {
	service.ConcurrencyCache
	loads map[int64]*service.UserLoadInfo
}

func (s *groupCapacityHandlerUserLoadCacheStub) GetUsersLoadBatch(_ context.Context, users []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	out := make(map[int64]*service.UserLoadInfo, len(users))
	for _, user := range users {
		if load := s.loads[user.ID]; load != nil {
			copied := *load
			out[user.ID] = &copied
		}
	}
	return out, nil
}

type groupCapacityHandlerRateRepoStub struct {
	service.UserGroupRateRepository
	overrides map[int64]map[int64]*int
}

func (s *groupCapacityHandlerRateRepoStub) GetRPMOverrideByUserAndGroup(_ context.Context, userID, groupID int64) (*int, error) {
	if s.overrides[userID] == nil {
		return nil, nil
	}
	return s.overrides[userID][groupID], nil
}

type groupCapacityHandlerUserRPMCacheStub struct {
	service.UserRPMCache
	groupCounts map[int64]map[int64]int
}

func (s *groupCapacityHandlerUserRPMCacheStub) GetUserGroupRPM(_ context.Context, userID, groupID int64) (int, error) {
	if s.groupCounts[userID] == nil {
		return 0, nil
	}
	return s.groupCounts[userID][groupID], nil
}

func (s *groupCapacityHandlerUserRPMCacheStub) GetUserRPM(context.Context, int64) (int, error) {
	return 0, nil
}
