package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAdmin ensures only super admin can access the route
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, _ := c.Get("is_admin")
		if admin, ok := isAdmin.(bool); !ok || !admin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireUser ensures only regular (non-admin) users can access the route
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, _ := c.Get("is_admin")
		if admin, ok := isAdmin.(bool); ok && admin {
			c.Redirect(http.StatusFound, "/admin")
			c.Abort()
			return
		}

		userID := GetCurrentUserID(c)
		if userID == 0 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}
