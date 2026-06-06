package handler

import (
	"pie/internal/data/model"
	"pie/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserHandler struct {
	userService *service.UserService
	log         *zap.Logger
}

func NewUserHandler(userService *service.UserService, log *zap.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		log:         log,
	}
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type SetPrimaryOrgRequest struct {
	PrimaryOrg string `json:"primaryOrg" binding:"required"`
}

type UserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type LoginResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refreshToken"`
	User         UserResponse `json:"user"`
}

func (h *UserHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	user, err := h.userService.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, toUserResponse(user))
}

func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	token, refreshToken, err := h.userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		Fail(c, err)
		return
	}

	user, err := h.userService.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         toUserResponse(user),
	})
}

func (h *UserHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	token, refreshToken, err := h.userService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, gin.H{
		"token":        token,
		"refreshToken": refreshToken,
	})
}

func (h *UserHandler) Logout(c *gin.Context) {
	accessToken, ok := c.Get("access_token")
	if !ok {
		Unauthorized(c, "missing user token")
		return
	}

	token, ok := accessToken.(string)
	if !ok || token == "" {
		Unauthorized(c, "invalid user token")
		return
	}

	var req LogoutRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			BadRequest(c, "invalid request")
			return
		}
	}

	if err := h.userService.Logout(c.Request.Context(), token, req.RefreshToken); err != nil {
		Fail(c, err)
		return
	}

	OK(c, nil)
}

func (h *UserHandler) Me(c *gin.Context) {
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

	user, err := h.userService.Me(c.Request.Context(), id)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, toUserResponse(user))
}

func (h *UserHandler) GetUserOrgTags(c *gin.Context) {
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

	orgTags, err := h.userService.GetUserOrgTags(c.Request.Context(), id)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, orgTags)
}

func (h *UserHandler) SetPrimaryOrg(c *gin.Context) {
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

	var req SetPrimaryOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	if err := h.userService.SetPrimaryOrg(c.Request.Context(), id, req.PrimaryOrg); err != nil {
		Fail(c, err)
		return
	}

	OK(c, nil)
}

func toUserResponse(user *model.User) UserResponse {
	return UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
}
