package upload

import (
	"pie/internal/httpresp"
	"pie/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	uploadService *service.UploadService
	log           *zap.Logger
}

func NewHandler(uploadService *service.UploadService, log *zap.Logger) *Handler {
	return &Handler{
		uploadService: uploadService,
		log:           log,
	}
}

func (h *Handler) CheckFile(c *gin.Context) {
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

	var req CheckFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	result, err := h.uploadService.CheckFile(c.Request.Context(), req.MD5, id)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, result)
}

func (h *Handler) UploadChunk(c *gin.Context) {
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

	var req UploadChunkRequest
	if err := c.ShouldBind(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}
	if req.TotalSize <= 0 {
		httpresp.BadRequest(c, "invalid totalSize")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		httpresp.BadRequest(c, "missing upload file")
		return
	}
	defer file.Close()

	result, err := h.uploadService.UploadChunk(
		c.Request.Context(),
		req.FileMD5,
		req.FileName,
		req.TotalSize,
		*req.ChunkIndex,
		file,
		header.Size,
		id,
		req.OrgTag,
		req.IsPublic,
	)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, result)
}

func (h *Handler) MergeChunks(c *gin.Context) {
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

	var req MergeChunksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	result, err := h.uploadService.MergeChunks(c.Request.Context(), req.FileMD5, req.FileName, id)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, result)
}

func (h *Handler) GetUploadStatus(c *gin.Context) {
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

	var req UploadStatusRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	result, err := h.uploadService.GetUploadStatus(c.Request.Context(), req.FileMD5, id)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, result)
}

func (h *Handler) GetSupportedFileTypes(c *gin.Context) {
	httpresp.OK(c, h.uploadService.GetSupportedFileTypes())
}
