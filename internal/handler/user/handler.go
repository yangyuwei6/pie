package user

import (
	"pie/internal/httpresp"
	"pie/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	userService *service.UserService
	log         *zap.Logger
}

func NewHandler(userService *service.UserService, log *zap.Logger) *Handler {
	return &Handler{
		userService: userService,
		log:         log,
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	user, err := h.userService.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, toUserResponse(user))
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	token, refreshToken, err := h.userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	user, err := h.userService.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         toUserResponse(user),
	})
}

func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	token, refreshToken, err := h.userService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, gin.H{
		"token":        token,
		"refreshToken": refreshToken,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	accessToken, ok := c.Get("access_token")
	if !ok {
		httpresp.Unauthorized(c, "missing user token")
		return
	}

	token, ok := accessToken.(string)
	if !ok || token == "" {
		httpresp.Unauthorized(c, "invalid user token")
		return
	}

	var req LogoutRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httpresp.BadRequest(c, "invalid request")
			return
		}
	}

	if err := h.userService.Logout(c.Request.Context(), token, req.RefreshToken); err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, nil)
}

func (h *Handler) Me(c *gin.Context) {
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

	user, err := h.userService.Me(c.Request.Context(), id)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, toUserResponse(user))
}

func (h *Handler) GetUserOrgTags(c *gin.Context) {
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

	orgTags, err := h.userService.GetUserOrgTags(c.Request.Context(), id)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, orgTags)
}

func (h *Handler) SetPrimaryOrg(c *gin.Context) {
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

	var req SetPrimaryOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "invalid request")
		return
	}

	if err := h.userService.SetPrimaryOrg(c.Request.Context(), id, req.PrimaryOrg); err != nil {
		httpresp.Fail(c, err)
		return
	}

	httpresp.OK(c, nil)
}
