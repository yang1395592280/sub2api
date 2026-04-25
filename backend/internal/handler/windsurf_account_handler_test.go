package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type windsurfAccountServiceStub struct {
	listItems        []service.WindsurfAccountListItem
	listPagination   *pagination.PaginationResult
	created          *service.WindsurfAccountListItem
	updated          *service.WindsurfAccountListItem
	updateErr        error
	lastUpdateInput  *service.UpdateWindsurfAccountCredentialsInput
	lastUpdateID     int64
	revealedPassword string
	revealErr        error
	lastRevealInput  *service.RevealWindsurfAccountPasswordInput
	lastRevealID     int64
	updateStatusErr  error
	lastStatusInput  *service.UpdateWindsurfAccountStatusInput
	deleteErr        error
	lastDeleteInput  *service.DeleteWindsurfAccountInput
	lastDeleteID     int64
}

func (s *windsurfAccountServiceStub) List(_ context.Context, _ pagination.PaginationParams, _ service.WindsurfAccountListFilters) ([]service.WindsurfAccountListItem, *pagination.PaginationResult, error) {
	return s.listItems, s.listPagination, nil
}

func (s *windsurfAccountServiceStub) Create(_ context.Context, _ *service.CreateWindsurfAccountInput) (*service.WindsurfAccountListItem, error) {
	return s.created, nil
}

func (s *windsurfAccountServiceStub) UpdateCredentials(_ context.Context, id int64, input *service.UpdateWindsurfAccountCredentialsInput) (*service.WindsurfAccountListItem, error) {
	s.lastUpdateID = id
	s.lastUpdateInput = input
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return s.updated, nil
}

func (s *windsurfAccountServiceStub) UpdateStatus(_ context.Context, _ int64, input *service.UpdateWindsurfAccountStatusInput) (*service.WindsurfAccountListItem, error) {
	s.lastStatusInput = input
	if s.updateStatusErr != nil {
		return nil, s.updateStatusErr
	}
	return s.updated, nil
}

func (s *windsurfAccountServiceStub) RevealPassword(_ context.Context, id int64, input *service.RevealWindsurfAccountPasswordInput) (string, error) {
	s.lastRevealID = id
	s.lastRevealInput = input
	if s.revealErr != nil {
		return "", s.revealErr
	}
	return s.revealedPassword, nil
}

func (s *windsurfAccountServiceStub) Delete(_ context.Context, id int64, input *service.DeleteWindsurfAccountInput) error {
	s.lastDeleteID = id
	s.lastDeleteInput = input
	return s.deleteErr
}

func TestWindsurfAccountHandlerListReturnsPaginatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{
		listItems: []service.WindsurfAccountListItem{
			{
				ID:                1,
				Account:           "windsurf@example.com",
				PasswordMasked:    "••••••",
				Enabled:           false,
				MaintainedByID:    7,
				MaintainedByName:  "alice",
				MaintainedByEmail: "alice@example.com",
				MaintainedAt:      time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
			},
		},
		listPagination: &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20, Pages: 1},
	}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/windsurf-accounts?page=1&page_size=20", nil)

	h.List(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Items []service.WindsurfAccountListItem `json:"items"`
			Total int64                             `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Len(t, payload.Data.Items, 1)
	require.Equal(t, "windsurf@example.com", payload.Data.Items[0].Account)
}

func TestWindsurfAccountHandlerUpdateStatusRejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Set(string(middleware2.ContextKeyUserRole), "user")
	c.Params = gin.Params{{Key: "id", Value: "3"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/windsurf-accounts/3/status", strings.NewReader(`{"enabled":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateStatus(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Nil(t, svc.lastStatusInput)
}

func TestWindsurfAccountHandlerRevealPasswordPassesActorAndRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{revealedPassword: "secret-value"}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Set(string(middleware2.ContextKeyUserRole), service.RoleAdmin)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/windsurf-accounts/9/password", nil)

	h.RevealPassword(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Password string `json:"password"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "secret-value", payload.Data.Password)
	require.Equal(t, int64(9), svc.lastRevealID)
	require.NotNil(t, svc.lastRevealInput)
	require.Equal(t, int64(7), svc.lastRevealInput.ActorID)
	require.True(t, svc.lastRevealInput.IsAdmin)
}

func TestWindsurfAccountHandlerUpdateCredentialsPassesAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{updated: &service.WindsurfAccountListItem{ID: 5}}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})
	c.Set(string(middleware2.ContextKeyUserRole), service.RoleAdmin)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/windsurf-accounts/5", strings.NewReader(`{"account":"admin@example.com","password":"secret"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateCredentials(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(5), svc.lastUpdateID)
	require.NotNil(t, svc.lastUpdateInput)
	require.Equal(t, int64(11), svc.lastUpdateInput.ActorID)
	require.True(t, svc.lastUpdateInput.IsAdmin)
	require.Equal(t, "admin@example.com", svc.lastUpdateInput.Account)
}

func TestWindsurfAccountHandlerUpdateCredentialsPassesNonAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{updated: &service.WindsurfAccountListItem{ID: 7}}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Set(string(middleware2.ContextKeyUserRole), "user")
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/windsurf-accounts/7", strings.NewReader(`{"account":"owner@example.com","password":"new-secret"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateCredentials(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(7), svc.lastUpdateID)
	require.NotNil(t, svc.lastUpdateInput)
	require.Equal(t, int64(7), svc.lastUpdateInput.ActorID)
	require.False(t, svc.lastUpdateInput.IsAdmin)
}

func TestWindsurfAccountHandlerDeleteRejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Set(string(middleware2.ContextKeyUserRole), "user")
	c.Params = gin.Params{{Key: "id", Value: "3"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/windsurf-accounts/3", nil)

	h.Delete(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Nil(t, svc.lastDeleteInput)
	require.Zero(t, svc.lastDeleteID)
}

func TestWindsurfAccountHandlerDeletePassesAdminActor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &windsurfAccountServiceStub{}
	h := NewWindsurfAccountHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})
	c.Set(string(middleware2.ContextKeyUserRole), service.RoleAdmin)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/windsurf-accounts/5", nil)

	h.Delete(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(5), svc.lastDeleteID)
	require.NotNil(t, svc.lastDeleteInput)
	require.Equal(t, int64(11), svc.lastDeleteInput.ActorID)
	require.True(t, svc.lastDeleteInput.IsAdmin)
}
