//go:build unit

package admin

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPelicanManagementRejectsScopedKeysBeforeAccessingData(t *testing.T) {
	h := &AccountHandler{}
	for name, handler := range map[string]gin.HandlerFunc{
		"history":     h.ListPelicanHistory,
		"read config": h.GetGroupPelicanConfig,
		"save config": h.SaveGroupPelicanConfig,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest("GET", "/", nil)
			c.Request = req.WithContext(adminaccess.WithPrincipal(req.Context(), &adminaccess.Principal{KeyID: "scoped-test"}))
			handler(c)
			require.Equal(t, 403, w.Code)
		})
	}
}

func TestPelicanHistoryRejectsInvalidPaginationAtHTTPBoundary(t *testing.T) {
	h := &AccountHandler{pelicanTestService: &service.PelicanTestService{}}
	for _, query := range []string{"page=0", "page=1000001", "page=invalid", "page_size=51", "page_size=0"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "7"}}
			c.Request = httptest.NewRequest("GET", "/?"+query, nil)
			h.ListPelicanHistory(c)
			require.Equal(t, 400, w.Code)
		})
	}
}
