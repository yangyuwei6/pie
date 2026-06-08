package search

import (
	"pie/internal/httpresp"
	"pie/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	searchService *service.SearchService
	log           *zap.Logger
}

func NewHandler(searchService *service.SearchService, log *zap.Logger) *Handler {
	return &Handler{
		searchService: searchService,
		log:           log,
	}
}

func (h *Handler) HybridSearch(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		httpresp.Unauthorized(c, "missing user context")
		return
	}

	id, ok := userID.(int64)
	if !ok {
		httpresp.Unauthorized(c, "invalid user context")
		return
	}

	var req HybridSearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}

	results, err := h.searchService.HybridSearch(c.Request.Context(), req.Query, req.TopK, id)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, results)
}
