//go:build unit

package admin

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type refundAdminService struct {
	service.AdminService
	calls               int
	paid, welfare       float64
	source, operationID string
	actorID             int64
}

func (s *refundAdminService) RefundUserWalletBalance(_ context.Context, id int64, paid, welfare float64, source string, actorID int64, notes, operationID string) (*service.User, error) {
	s.calls++
	s.paid = paid
	s.welfare = welfare
	s.source = source
	s.actorID = actorID
	s.operationID = operationID
	return &service.User{ID: id, Email: "private@example.com", Balance: 5, GiftBalance: 3}, nil
}
func TestUserWalletRefundHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })
	for _, source := range []string{"checkin", "liyu", "gift"} {
		for _, restricted := range []bool{false, true} {
			t.Run(source+map[bool]string{true: "restricted", false: "full"}[restricted], func(t *testing.T) {
				service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(newMemoryIdempotencyRepoStub(), service.DefaultIdempotencyConfig()))
				svc := &refundAdminService{}
				h := NewUserHandler(svc, nil, nil, nil, nil, nil, nil)
				r := gin.New()
				r.Use(func(c *gin.Context) {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
					if restricted {
						c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{KeyID: "test", Policy: adminaccess.Policy{GlobalPermissions: []string{"users.balance.write"}}}))
					}
					c.Next()
				})
				r.POST("/users/:id/balance", h.UpdateBalance)
				for i := 0; i < 2; i++ {
					body := `{"balance":1,"welfare_balance":2,"welfare_source":"` + source + `","operation":"subtract"}`
					req := httptest.NewRequest("POST", "/users/7/balance", strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Idempotency-Key", "refund")
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					require.Equal(t, 200, w.Code, w.Body.String())
					if restricted {
						require.JSONEq(t, `{"code":0,"message":"success","data":{"id":7}}`, w.Body.String())
					} else {
						require.Contains(t, w.Body.String(), "private@example.com")
					}
				}
				require.Equal(t, 1, svc.calls)
				require.Equal(t, 1., svc.paid)
				require.Equal(t, 2., svc.welfare)
				require.Equal(t, source, svc.source)
				require.Equal(t, int64(9), svc.actorID)
				require.NotEmpty(t, svc.operationID)
			})
		}
	}
}
func TestUserWalletRefundHTTPRejectsInvalidSource(t *testing.T) {
	for _, body := range []string{
		`{"balance":1,"welfare_balance":2,"operation":"subtract"}`,
		`{"balance":1,"welfare_balance":2,"welfare_source":"other","operation":"subtract"}`,
		`{"balance":1,"welfare_balance":2,"welfare_source":"checkin","operation":"add"}`,
		`{"balance":1,"welfare_balance":2,"operation":"set"}`,
		`{"balance":0,"welfare_balance":0,"operation":"subtract"}`,
	} {
		svc := &refundAdminService{}
		h := NewUserHandler(svc, nil, nil, nil, nil, nil, nil)
		r := gin.New()
		r.POST("/users/:id/balance", h.UpdateBalance)
		req := httptest.NewRequest("POST", "/users/7/balance", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, 400, w.Code, w.Body.String())
		require.Zero(t, svc.calls)
	}
}
