package middleware

import (
	"net/http"
	"strings"

	"pie/internal/auth"
	"pie/internal/service"

	"github.com/gin-gonic/gin"
)

func JWT(manager *auth.JWTManager, userService *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "missing authorization header",
			})
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		if tokenString == header || tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid authorization format",
			})
			return
		}

		blacklisted, err := userService.IsTokenBlacklisted(c.Request.Context(), tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "check token failed",
			})
			return
		}
		if blacklisted {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "token has been logged out",
			})
			return
		}

		claims, err := manager.VerifyAccessToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid token",
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("access_token", tokenString)

		c.Next()
	}
}
