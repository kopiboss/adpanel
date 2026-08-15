package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"adpanel/helpers"
	"adpanel/middleware"
	"adpanel/models"
	"adpanel/services"
)

func ListSyncLogs(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	logs, err := models.ListSyncLogsByUser(userID, 100)
	if err != nil {
		logs = []models.SyncLog{}
	}

	c.HTML(http.StatusOK, "sync_log.html", gin.H{
		"title": "Sync Logs",
		"logs":  logs,
		"user":  middleware.GetCurrentUser(c),
	})
}

func SyncNow(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	account, err := models.GetAdAccountByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ad account tidak ditemukan"})
		return
	}

	go func() {
		_ = services.SyncAccountNow(account)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Sync dimulai untuk " + account.Name})
}

func UserDashboard(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	user := middleware.GetCurrentUser(c)

	accounts, _ := models.ListAdAccountsByUser(userID)
	creds, _ := models.ListCredentialsByUser(userID)

	// Deteksi currency dari akun pertama yang aktif; default IDR
	currency := "IDR"
	for _, acc := range accounts {
		if acc.IsActive && acc.Currency != "" {
			currency = acc.Currency
			break
		}
	}
	currency = helpers.DefaultCurrency(currency)

	now := time.Now()
	dateFrom := now.AddDate(0, 0, -7).Format("2006-01-02")
	dateTo := now.Format("2006-01-02")

	summary, _ := models.GetUserInsightSummary(userID, dateFrom, dateTo)
	if summary == nil {
		summary = &models.InsightSummary{}
	}

	recentLogs, _ := models.ListSyncLogsByUser(userID, 5)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":       "Dashboard",
		"user":        user,
		"accounts":    accounts,
		"credentials": creds,
		"currency":    currency,
		"summary":     summary,
		"recent_logs": recentLogs,
	})
}
