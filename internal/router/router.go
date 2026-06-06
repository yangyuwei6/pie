package router

import (
	"pie/internal/service"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, userService *service.UserService, jwtMiddleware gin.HandlerFunc) {
	api := r.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.POST("/refreshToken", userService.RefreshToken)
	}

	users := api.Group("/users")
	{
		users.POST("/register", userService.Register)
		users.POST("/login", userService.Login)

		authed := users.Group("/")
		authed.Use(jwtMiddleware)
		{
			authed.GET("/me", userService.Me)
		}
	}
}
