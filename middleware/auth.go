package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"adpanel/config"
	"adpanel/models"
)

const SessionUserID = "user_id"
const SessionUserRole = "user_role"
const SessionIsAdmin = "is_admin"

// RequireAuth checks that the user is logged in (regular user or admin)
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// Check super admin session
		if isAdmin, ok := session.Get(SessionIsAdmin).(bool); ok && isAdmin {
			c.Set("is_admin", true)
			c.Set("user_role", "superadmin")
			c.Set("user_name", config.App.AdminEmail)
			c.Next()
			return
		}

		// Check regular user session
		userID, ok := session.Get(SessionUserID).(uint64)
		if !ok || userID == 0 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		user, err := models.GetUserByID(userID)
		if err != nil || user == nil {
			session.Clear()
			session.Save()
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		if user.Status != "active" {
			session.Clear()
			session.Save()
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("user", user)
		c.Set("user_role", user.Role)
		c.Set("user_name", user.Name)
		c.Set("is_admin", false)
		c.Next()
	}
}

// RequireGuest redirects logged-in users away from auth pages
func RequireGuest() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		if isAdmin, ok := session.Get(SessionIsAdmin).(bool); ok && isAdmin {
			c.Redirect(http.StatusFound, "/admin")
			c.Abort()
			return
		}

		if userID, ok := session.Get(SessionUserID).(uint64); ok && userID > 0 {
			c.Redirect(http.StatusFound, "/dashboard")
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCurrentUserID extracts user ID from gin context
func GetCurrentUserID(c *gin.Context) uint64 {
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := id.(uint64); ok {
			return uid
		}
	}
	return 0
}

// GetCurrentUser extracts user from gin context
func GetCurrentUser(c *gin.Context) *models.User {
	if u, exists := c.Get("user"); exists {
		if user, ok := u.(*models.User); ok {
			return user
		}
	}
	return nil
}
