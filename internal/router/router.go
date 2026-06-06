package router

import (
	"pie/internal/handler"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, userHandler *handler.UserHandler, jwtMiddleware gin.HandlerFunc) {
	api := r.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.POST("/refreshToken", userHandler.RefreshToken)
	}

	users := api.Group("/users")
	{
		users.POST("/register", userHandler.Register)
		users.POST("/login", userHandler.Login)

		authed := users.Group("/")
		authed.Use(jwtMiddleware)
		{
			authed.GET("/me", userHandler.Me)
			authed.POST("/logout", userHandler.Logout)
			authed.GET("/org-tags", userHandler.GetUserOrgTags)
			authed.PUT("/primary-org", userHandler.SetPrimaryOrg)
		}
	}
}
