package service

import (
	"pie/internal/biz"
	"pie/internal/data/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserService struct {
	userBiz *biz.UserUsecase
	log     *zap.Logger
}

func NewUserService(userBiz *biz.UserUsecase, log *zap.Logger) *UserService {
	return &UserService{
		userBiz: userBiz,
		log:     log,
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

type UserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

func (s *UserService) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	user, err := s.userBiz.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, toUserResponse(user))
}

func (s *UserService) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}

	token, err := s.userBiz.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		Fail(c, err)
		return
	}

	user, err := s.userBiz.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

func (s *UserService) Me(c *gin.Context) {
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

	user, err := s.userBiz.Me(c.Request.Context(), id)
	if err != nil {
		Fail(c, err)
		return
	}

	OK(c, toUserResponse(user))
}

func toUserResponse(user *model.User) UserResponse {
	return UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
}
