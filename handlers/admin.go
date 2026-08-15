package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"adpanel/models"
	"adpanel/services"
)

func AdminDashboard(c *gin.Context) {
	totalUsers, _ := models.CountUsers()
	pendingUsers, _ := models.CountUsersByStatus("pending")
	activeUsers, _ := models.CountUsersByStatus("active")

	recentLogs, _ := models.ListAllSyncLogs(20)

	c.HTML(http.StatusOK, "admin/dashboard.html", gin.H{
		"title":        "Admin Dashboard",
		"total_users":  totalUsers,
		"pending":      pendingUsers,
		"active":       activeUsers,
		"recent_logs":  recentLogs,
		"is_admin":     true,
	})
}

func AdminListUsers(c *gin.Context) {
	users, err := models.ListAllUsers()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/users.html", gin.H{
			"error": "Gagal memuat data users",
		})
		return
	}

	c.HTML(http.StatusOK, "admin/users.html", gin.H{
		"title":    "Kelola Users",
		"users":    users,
		"is_admin": true,
	})
}

func AdminApproveUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := models.UpdateUserStatus(id, "active"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal approve user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User diapprove"})
}

func AdminRejectUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := models.DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal reject user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User direject dan dihapus"})
}

func AdminSuspendUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := models.UpdateUserStatus(id, "suspended"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal suspend user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User disuspend"})
}

func AdminUnsuspendUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := models.UpdateUserStatus(id, "active"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal unsuspend user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User diaktifkan kembali"})
}

func AdminShowSettings(c *gin.Context) {
	settings, _ := models.GetAllSettings()

	c.HTML(http.StatusOK, "admin/settings.html", gin.H{
		"title":    "Platform Settings",
		"settings": settings,
		"is_admin": true,
	})
}

func AdminSaveSettings(c *gin.Context) {
	keys := []string{
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_CHAT_ID",
		"GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET",
		"SITE_NAME",
		"ALLOW_REGISTRATION",
	}

	for _, key := range keys {
		val := c.PostForm(key)
		if err := models.SetSetting(key, val); err != nil {
			c.HTML(http.StatusOK, "admin/settings.html", gin.H{
				"error":    "Gagal menyimpan setting: " + key,
				"is_admin": true,
			})
			return
		}
	}

	// Reload Telegram config
	if services.Bot != nil {
		services.Bot.ReloadConfig()
	}

	settings, _ := models.GetAllSettings()
	c.HTML(http.StatusOK, "admin/settings.html", gin.H{
		"success":  "Settings berhasil disimpan",
		"settings": settings,
		"is_admin": true,
		"title":    "Platform Settings",
	})
}
