package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
	"github.com/gbplantwiki/gbplantwiki/internal/service"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// KnowledgeDraftHandler exposes Q&A sedimentation draft endpoints: retry a
// failed/invalidated draft generation and publish a draft as a care article.
type KnowledgeDraftHandler struct {
	svc    *service.KnowledgeDraftService
	logger *slog.Logger
}

// NewKnowledgeDraftHandler creates a KnowledgeDraftHandler.
func NewKnowledgeDraftHandler(svc *service.KnowledgeDraftService, logger *slog.Logger) *KnowledgeDraftHandler {
	return &KnowledgeDraftHandler{svc: svc, logger: logger}
}

// Retry handles PUT /questions/:id/draft/retry.
func (h *KnowledgeDraftHandler) Retry(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid question id"))
		return
	}
	draft, err := h.svc.RetryDraft(middleware.GetUserID(c), uint(questionID))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(draft))
}

// GetForQuestion handles GET /questions/:id/draft (question owner only).
func (h *KnowledgeDraftHandler) GetForQuestion(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid question id"))
		return
	}
	draft, err := h.svc.GetDraftForQuestion(middleware.GetUserID(c), uint(questionID))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(draft))
}

// Publish handles PUT /articles/:id/publish.
func (h *KnowledgeDraftHandler) Publish(c *gin.Context) {
	articleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid article id"))
		return
	}
	var req dto.DraftPublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam))
		return
	}
	art, err := h.svc.PublishDraft(middleware.GetUserID(c), uint(articleID), req.TopicTag)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(art))
}
