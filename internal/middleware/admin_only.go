// admin_only.go
package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		perms := c.GetStringSlice("userPermissions")
		isAdmin := false
		for _, p := range perms {
			if p == "admin" {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin privileges required"})
			c.Abort()
			return
		}
		c.Next()
		fmt.Println("PERMISSIONS:", perms)
	}

}
