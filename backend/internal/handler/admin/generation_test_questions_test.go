//go:build unit

package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type questionHTTPRepo struct {
	kind string
	ids  []int64

	service.PelicanTestRepository
	user  int64
	saved service.GenerationTestQuestion
}

func (r *questionHTTPRepo) SaveQuestion(_ context.Context, q service.GenerationTestQuestion) (*service.GenerationTestQuestion, error) {
	r.saved = q
	return &q, nil
}
func (r *questionHTTPRepo) GetTestPreference(_ context.Context, u int64, k string) (*service.GenerationTestPreference, error) {
	r.user = u
	return &service.GenerationTestPreference{QuestionIDs: []int64{3, 4}}, nil
}
func TestGenerationQuestionHTTPIdentityAndPathID(t *testing.T) {
	repo := &questionHTTPRepo{}
	h := &AccountHandler{pelicanTestService: service.NewPelicanTestService(repo, nil, nil)}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
	router.PUT("/test-questions/:id", h.SaveGenerationTestQuestion)
	router.POST("/test-questions", h.SaveGenerationTestQuestion)
	router.GET("/preferences", h.GetGenerationTestPreference)
	body := `{"id":999,"test_kind":"intelligence","name":"Q","prompt":"Return a number","expected_answer":"0042","enabled":true}`
	for _, tc := range []struct {
		method, path string
		id           int64
	}{{"PUT", "/test-questions/7", 7}, {"POST", "/test-questions", 0}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, 200, w.Code, w.Body.String())
		require.Equal(t, tc.id, repo.saved.ID)
		require.Equal(t, "42", repo.saved.ExpectedAnswer)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/preferences?test_kind=intelligence&user_id=999", nil))
	require.Equal(t, 200, w.Code)
	require.EqualValues(t, 42, repo.user)
}
func TestGenerationQuestionHTTPRejectsScopedKeysAndMissingIdentity(t *testing.T) {
	h := &AccountHandler{pelicanTestService: service.NewPelicanTestService(&questionHTTPRepo{}, nil, nil)}
	for _, handler := range []gin.HandlerFunc{h.ListGenerationTestQuestions, h.SaveGenerationTestQuestion, h.GetGenerationTestPreference} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/", nil)
		c.Request = req.WithContext(adminaccess.WithPrincipal(req.Context(), &adminaccess.Principal{KeyID: "scoped"}))
		handler(c)
		require.Equal(t, 403, w.Code)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	h.GetGenerationTestPreference(c)
	require.Equal(t, 401, w.Code)
}

func (r *questionHTTPRepo) ExpireStale(context.Context) error { return nil }
func (r *questionHTTPRepo) CreateQuestionBatch(_ context.Context, b string, a []int64, m, k string, q []int64, u int64) ([]service.PelicanTest, error) {
	r.user = u
	r.kind = k
	r.ids = q
	return []service.PelicanTest{{BatchID: b, AccountID: a[0], QuestionID: q[0]}}, nil
}
func (r *questionHTTPRepo) ListLatestBatches(_ context.Context, a []int64, k, b string) ([]service.PelicanTest, error) {
	r.kind = k
	return []service.PelicanTest{}, nil
}
func TestGenerationQuestionHTTPBatchDispatchAndScopedLatest(t *testing.T) {
	repo := &questionHTTPRepo{}
	h := &AccountHandler{pelicanTestService: service.NewPelicanTestService(repo, nil, nil)}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
	router.POST("/accounts/intelligence-tests", h.StartPelicanTests)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/accounts/intelligence-tests", strings.NewReader(`{"account_ids":[11,12],"question_ids":[3,4],"model_id":"text"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "intelligence", repo.kind)
	require.EqualValues(t, 42, repo.user)
	require.Equal(t, []int64{3, 4}, repo.ids)
	router = gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{KeyID: "scoped"}))
	})
	router.POST("/accounts/pelican-tests/latest", h.ListLatestPelicanTests)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/accounts/pelican-tests/latest", strings.NewReader(`{"account_ids":[11],"include_batch":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
}
