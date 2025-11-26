// auth_middleware.go
package middleware

import (
	"net/http"
	"order-status-service-2/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middleware que valida el token y guarda la info del usuario en el contexto
func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		user, err := authService.ValidateToken(token)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Guardamos los datos del usuario en el contexto
		c.Set("userID", user.ID)
		c.Set("userName", user.Name)
		c.Set("userPermissions", user.Permissions)
		c.Next()
	}
}
