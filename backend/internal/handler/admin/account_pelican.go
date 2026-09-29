package admin

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type pelicanTestBatchRequest struct {
	AccountIDs []int64 `json:"account_ids"`
	ModelID    string  `json:"model_id"`
	Prompt     string  `json:"prompt"`
}

type pelicanTestLatestRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

// StartPelicanTests accepts one explicit batch. The existing admin route group
// supplies authentication; each worker checks its account before generation.
func (h *AccountHandler) StartPelicanTests(c *gin.Context) {
	var req pelicanTestBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AccountIDs) == 0 || len(req.AccountIDs) > service.PelicanTestMaxAccounts ||
		strings.TrimSpace(req.ModelID) == "" || strings.TrimSpace(req.Prompt) == "" || utf8.RuneCountInString(req.Prompt) > service.PelicanTestMaxPrompt {
		response.BadRequest(c, "Invalid pelican test selection, model, or prompt")
		return
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican test service unavailable")
		return
	}
	tests, err := h.pelicanTestService.StartBatch(c.Request.Context(), req.AccountIDs, req.ModelID, req.Prompt)
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
	var req pelicanTestLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AccountIDs) > service.PelicanTestMaxAccounts {
		response.BadRequest(c, "Invalid account selection")
		return
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican test service unavailable")
		return
	}
	tests, err := h.pelicanTestService.ListLatest(c.Request.Context(), req.AccountIDs)
	if err != nil {
		_ = c.Error(err)
		response.InternalError(c, "Failed to load pelican tests")
		return
	}
	response.Success(c, tests)
}

func (h *AccountHandler) GetPelicanTest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("test_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid test ID")
		return
	}
	if h.pelicanTestService == nil {
		response.InternalError(c, "Pelican test service unavailable")
		return
	}
	test, err := h.pelicanTestService.Get(c.Request.Context(), id)
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
