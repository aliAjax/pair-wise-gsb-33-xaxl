package router

import (
	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/config"
	"github.com/gbplantwiki/gbplantwiki/internal/handler"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
)

func registerArticleRoutes(v1 *gin.RouterGroup, cfg *config.Config, h *handler.CareArticleHandler,
	dh *handler.KnowledgeDraftHandler, limiter *middleware.RateLimiter) {
	articles := v1.Group("/articles")
	articles.GET("", h.List)
	// Optional auth: published articles stay public while owners may open
	// their own (invalidated) drafts with the same URL.
	articles.GET("/:id", middleware.AuthOptional(cfg), h.Get)
	auth := articles.Group("", middleware.AuthRequired(cfg))
	auth.POST("", limiter.Limit(), h.Create)
	auth.PUT("/:id", h.Update)
	auth.DELETE("/:id", h.Delete)
	auth.PUT("/:id/publish", dh.Publish)

	// Owner draft box (registered outside /articles so it never collides
	// with the /articles/:id route).
	v1.GET("/me/articles", middleware.AuthRequired(cfg), h.ListMine)
}
