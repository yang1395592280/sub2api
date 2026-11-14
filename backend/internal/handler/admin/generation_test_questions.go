package admin

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AccountHandler) ListGenerationTestQuestions(c *gin.Context) {
	if !h.pelicanFullAdmin(c) {
		return
	}
	questions, err := h.pelicanTestService.ListQuestions(c.Request.Context(), c.DefaultQuery("test_kind", service.TestKindPelican))
	if err != nil {
		generationQuestionError(c, err)
		return
	}
	response.Success(c, questions)
}

func (h *AccountHandler) SaveGenerationTestQuestion(c *gin.Context) {
	if !h.pelicanFullAdmin(c) {
		return
	}
	var question service.GenerationTestQuestion
	if err := c.ShouldBindJSON(&question); err != nil {
		response.BadRequest(c, "Invalid question")
		return
	}
	question.ID = 0
	if rawID := c.Param("id"); rawID != "" {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid question ID")
			return
		}
		question.ID = id
	}
	out, err := h.pelicanTestService.SaveQuestion(c.Request.Context(), question)
	if err != nil {
		generationQuestionError(c, err)
		return
	}
	response.Success(c, out)
}

func (h *AccountHandler) GetGenerationTestPreference(c *gin.Context) {
	if !h.pelicanFullAdmin(c) {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Administrator identity required")
		return
	}
	out, err := h.pelicanTestService.GetTestPreference(c.Request.Context(), subject.UserID, c.DefaultQuery("test_kind", service.TestKindPelican))
	if err != nil {
		generationQuestionError(c, err)
		return
	}
	response.Success(c, out)
}

func generationQuestionError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrInvalidPelicanBatch) {
		response.BadRequest(c, err.Error())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Question not found")
		return
	}
	_ = c.Error(err)
	response.InternalError(c, "Failed to update or load test questions")
}
