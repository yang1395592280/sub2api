package accountquality

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDisabledModuleBypassesLegacySchedulingAndBlocksNewDispatch(t *testing.T) {
	s := NewService(nil, nil)
	s.SetModuleCheck(func(context.Context) bool { return false })
	// A disabled module must not read the legacy policy store at all.
	s.Refresh(context.Background())
	require.False(t, s.Enabled())
	decision, err := s.Decision(7, "scope", time.Now())
	require.NoError(t, err)
	require.Equal(t, "baseline", decision)
	require.ErrorContains(t, s.dispatch(context.Background(), Run{}, "candy"), "质量监控总开关已关闭")
	r := gin.New()
	s.Register(r.Group("/admin"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/admin/account-quality/settings", nil))
	require.Equal(t, 404, w.Code)
	require.Contains(t, w.Body.String(), "QUALITY_MODULE_DISABLED")
}
