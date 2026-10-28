package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type AccountQualityHandler struct{ service *accountquality.Service }

func NewAccountQualityHandler(s *accountquality.Service) *AccountQualityHandler {
	return &AccountQualityHandler{service: s}
}
func (h *AccountQualityHandler) Register(parent *gin.RouterGroup) {
	group := parent.Group("")
	group.Use(func(c *gin.Context) {
		role, ok := middleware.GetUserRoleFromContext(c)
		subject, authenticated := middleware.GetAuthSubjectFromContext(c)
		if !ok || role != "admin" || !authenticated || adminaccess.FromContext(c.Request.Context()) != nil {
			c.AbortWithStatusJSON(403, gin.H{"code": 403, "reason": "QUALITY_ADMIN_REQUIRED", "message": "质量管理仅允许完整管理员访问"})
			return
		}
		c.Set("quality_actor_id", subject.UserID)
		c.Next()
	})
	h.service.Register(group)
}
