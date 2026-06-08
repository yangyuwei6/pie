package router

import (
	searchhandler "pie/internal/handler/search"
	uploadhandler "pie/internal/handler/upload"
	userhandler "pie/internal/handler/user"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, userHandler *userhandler.Handler, uploadHandler *uploadhandler.Handler, searchHandler *searchhandler.Handler, jwtMiddleware gin.HandlerFunc) {
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

	upload := api.Group("/upload")
	upload.Use(jwtMiddleware)
	{
		upload.POST("/check", uploadHandler.CheckFile)
		upload.POST("/chunk", uploadHandler.UploadChunk)
		upload.POST("/merge", uploadHandler.MergeChunks)
		upload.GET("/status", uploadHandler.GetUploadStatus)
		upload.GET("/supported-types", uploadHandler.GetSupportedFileTypes)
	}

	search := api.Group("/search")
	search.Use(jwtMiddleware)
	{
		search.GET("/hybrid", searchHandler.HybridSearch)
	}
}
