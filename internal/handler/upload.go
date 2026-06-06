package handler

import (
	"pie/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UploadHandler struct {
	uploadService *service.UploadService
	log           *zap.Logger
}

func NewUploadHandler(uploadService *service.UploadService, log *zap.Logger) *UploadHandler {
	return &UploadHandler{
		uploadService: uploadService,
		log:           log,
	}
}

type CheckFileRequest struct {
	MD5 string `json:"md5" binding:"required"`
}

func (h *UploadHandler) CheckFile(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		Unauthorized(c, "missing user context")
		return
	}

	id, ok := userID.(int64)
	if !ok {
		Unauthorized(c, "invalid user context")
		return
	}

	var req CheckFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	result, err := h.uploadService.CheckFile(c.Request.Context(), req.MD5, id)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, result)
}
