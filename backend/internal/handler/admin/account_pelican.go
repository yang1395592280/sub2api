package admin

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type pelicanTestBatchRequest struct {
	QuestionIDs []int64 `json:"question_ids"`
	AccountIDs  []int64 `json:"account_ids"`
	ModelID     string  `json:"model_id"`
	Prompt      string  `json:"prompt"`
}

type pelicanTestLatestRequest struct {
	AccountIDs   []int64 `json:"account_ids"`
	IncludeBatch bool    `json:"include_batch"`
	BatchID      string  `json:"batch_id"`
}

// StartPelicanTests accepts one explicit batch. The existing admin route group
// supplies authentication; each worker checks its account before generation.
func (h *AccountHandler) StartPelicanTests(c *gin.Context) {
	kind := generationTestKind(c)
	if kind == service.TestKindIntelligence && !h.pelicanFullAdmin(c) {
		return
	}
	var req pelicanTestBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AccountIDs) == 0 || len(req.AccountIDs) > service.PelicanTestMaxAccounts ||
		strings.TrimSpace(req.ModelID) == "" || (req.QuestionIDs == nil && kind == service.TestKindPelican && strings.TrimSpace(req.Prompt) == "") || utf8.RuneCountInString(req.Prompt) > service.PelicanTestMaxPrompt {
		response.BadRequest(c, "Invalid pelican test selection, model, or prompt")
		return
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican test service unavailable")
		return
	}
	var tests []service.PelicanTest
	var err error
	if req.QuestionIDs != nil {
		if !h.pelicanFullAdmin(c) {
			return
		}
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			response.Unauthorized(c, "Administrator identity required")
			return
		}
		tests, err = h.pelicanTestService.StartQuestionBatch(c.Request.Context(), req.AccountIDs, req.ModelID, kind, req.QuestionIDs, subject.UserID)
	} else {
		tests, err = h.pelicanTestService.StartBatch(c.Request.Context(), req.AccountIDs, req.ModelID, req.Prompt, kind)
	}
	if err != nil {
		if errors.Is(err, service.ErrInvalidPelicanBatch) {
			response.BadRequest(c, err.Error())
		} else {
			_ = c.Error(err)
			response.InternalError(c, "Failed to start pelican tests")
		}
		return
	}
	response.Success(c, tests)
}

func (h *AccountHandler) ListLatestPelicanTests(c *gin.Context) {
	if generationTestKind(c) == service.TestKindIntelligence && !h.pelicanFullAdmin(c) {
		return
	}
	var req pelicanTestLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AccountIDs) > service.PelicanTestMaxAccounts {
		response.BadRequest(c, "Invalid account selection")
		return
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican test service unavailable")
		return
	}
	var tests []service.PelicanTest
	var err error
	if req.IncludeBatch || req.BatchID != "" {
		if !h.pelicanFullAdmin(c) {
			return
		}
		tests, err = h.pelicanTestService.ListLatestBatches(c.Request.Context(), req.AccountIDs, generationTestKind(c), req.BatchID)
	} else {
		tests, err = h.pelicanTestService.ListLatest(c.Request.Context(), req.AccountIDs, generationTestKind(c))
	}
	if err != nil {
		if errors.Is(err, service.ErrInvalidPelicanBatch) {
			response.BadRequest(c, err.Error())
			return
		}
		_ = c.Error(err)
		response.InternalError(c, "Failed to load pelican tests")
		return
	}
	response.Success(c, tests)
}

func (h *AccountHandler) GetPelicanTest(c *gin.Context) {
	if generationTestKind(c) == service.TestKindIntelligence && !h.pelicanFullAdmin(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("test_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid test ID")
		return
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican test service unavailable")
		return
	}
	test, err := h.pelicanTestService.Get(c.Request.Context(), id, generationTestKind(c))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "Pelican test not found")
		} else {
			_ = c.Error(err)
			response.InternalError(c, "Failed to load pelican test")
		}
		return
	}
	response.Success(c, test)
}

func (h *AccountHandler) ListPelicanHistory(c *gin.Context) {
	if !h.pelicanFullAdmin(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	if err != nil || id <= 0 || pageErr != nil || sizeErr != nil || page < 1 || page > 1000000 || size < 1 || size > 50 {
		response.BadRequest(c, "Invalid account or pagination")
		return
	}
	var result *service.PelicanTestHistory
	if c.Query("group_batches") == "1" {
		result, err = h.pelicanTestService.ListBatchHistory(c.Request.Context(), id, page, size, generationTestKind(c))
	} else {
		result, err = h.pelicanTestService.ListHistory(c.Request.Context(), id, page, size, generationTestKind(c))
	}
	if err != nil {
		_ = c.Error(err)
		response.InternalError(c, "Failed to load pelican history")
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) GetGroupPelicanConfig(c *gin.Context) {
	if !h.pelicanFullAdmin(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	result, err := h.pelicanTestService.GetGroupConfig(c.Request.Context(), id, generationTestKind(c))
	if err != nil {
		pelicanConfigError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) SaveGroupPelicanConfig(c *gin.Context) {
	if !h.pelicanFullAdmin(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	var req service.GroupPelicanConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid pelican configuration")
		return
	}
	req.GroupID = id
	req.TestKind = generationTestKind(c)
	result, err := h.pelicanTestService.SaveGroupConfig(c.Request.Context(), req)
	if err != nil {
		pelicanConfigError(c, err)
		return
	}
	response.Success(c, result)
}

func pelicanConfigError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrInvalidPelicanBatch) {
		response.BadRequest(c, err.Error())
		return
	}
	response.ErrorFrom(c, err)
}

func (h *AccountHandler) pelicanFullAdmin(c *gin.Context) bool {
	if adminaccess.FromContext(c.Request.Context()) != nil {
		response.Error(c, 403, "Pelican management requires a full administrator")
		return false
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican service unavailable")
		return false
	}
	return true
}

func generationTestKind(c *gin.Context) string {
	if strings.Contains(c.FullPath(), "/intelligence-test") {
		return service.TestKindIntelligence
	}
	return service.TestKindPelican
}
