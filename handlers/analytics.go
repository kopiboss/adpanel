package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"adpanel/helpers"
	"adpanel/middleware"
	"adpanel/models"
)

func ShowAnalytics(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	accounts, _ := models.ListActiveAdAccountsByUser(userID)

	accountID := uint64(0)
	if accIDStr := c.Query("account_id"); accIDStr != "" {
		accountID, _ = strconv.ParseUint(accIDStr, 10, 64)
	}

	// Deteksi currency dari akun yang dipilih.
	// Kalau "semua akun" dipilih, ambil currency dari akun pertama yang ada.
	// Default IDR jika belum ada akun.
	currency := "IDR"
	for _, acc := range accounts {
		if accountID == 0 || acc.ID == accountID {
			if acc.Currency != "" {
				currency = acc.Currency
			}
			break
		}
	}
	currency = helpers.DefaultCurrency(currency)

	// Date range
	dateRange := c.Query("range")
	if dateRange == "" {
		dateRange = "7"
	}

	now := time.Now()
	dateTo := now.Format("2006-01-02")
	var dateFrom string

	switch dateRange {
	case "today":
		dateFrom = dateTo
	case "7":
		dateFrom = now.AddDate(0, 0, -7).Format("2006-01-02")
	case "30":
		dateFrom = now.AddDate(0, 0, -30).Format("2006-01-02")
	case "custom":
		dateFrom = c.Query("date_from")
		dateTo = c.Query("date_to")
		if dateFrom == "" {
			dateFrom = now.AddDate(0, 0, -7).Format("2006-01-02")
		}
		if dateTo == "" {
			dateTo = now.Format("2006-01-02")
		}
	default:
		dateFrom = now.AddDate(0, 0, -7).Format("2006-01-02")
	}

	var summary *models.InsightSummary
	var dailySpend []models.DailySpend
	var campaignInsights []models.Insight

	if accountID > 0 {
		summary, _ = models.GetInsightSummary(accountID, userID, dateFrom, dateTo)
		dailySpend, _ = models.GetDailySpend(accountID, userID, dateFrom, dateTo)
		campaignInsights, _ = models.GetCampaignInsights(accountID, userID, dateFrom, dateTo)
	} else {
		summary, _ = models.GetUserInsightSummary(userID, dateFrom, dateTo)
	}

	if summary == nil {
		summary = &models.InsightSummary{}
	}

	spendLabels := make([]string, 0, len(dailySpend))
	spendData := make([]float64, 0, len(dailySpend))
	for _, d := range dailySpend {
		spendLabels = append(spendLabels, d.Date)
		spendData = append(spendData, d.Spend)
	}

	c.HTML(http.StatusOK, "analytics.html", gin.H{
		"title":             "Analytics",
		"accounts":          accounts,
		"account_id":        accountID,
		"date_range":        dateRange,
		"date_from":         dateFrom,
		"date_to":           dateTo,
		"currency":          currency,
		"summary":           summary,
		"daily_spend":       dailySpend,
		"spend_labels":      spendLabels,
		"spend_data":        spendData,
		"campaign_insights": campaignInsights,
		"user":              middleware.GetCurrentUser(c),
	})
}

func ExportAnalyticsCSV(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	accountIDStr := c.Query("account_id")
	accountID, _ := strconv.ParseUint(accountIDStr, 10, 64)

	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	if dateFrom == "" {
		dateFrom = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if dateTo == "" {
		dateTo = time.Now().Format("2006-01-02")
	}

	// Deteksi currency untuk kolom CSV header
	currency := "IDR"
	if accountID > 0 {
		if acc, err := models.GetAdAccountByID(accountID, userID); err == nil && acc.Currency != "" {
			currency = acc.Currency
		}
	}

	var insights []models.Insight
	if accountID > 0 {
		insights, _ = models.GetCampaignInsights(accountID, userID, dateFrom, dateTo)
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="analytics_%s_%s.csv"`, dateFrom, dateTo))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	spendHeader := fmt.Sprintf("Spend (%s)", currency)
	_ = w.Write([]string{
		"Tanggal", "Kampanye", spendHeader, "Impressions", "Clicks", "CTR (%)", "CPC", "CPM", "Reach",
	})

	decimals := helpers.CurrencyDecimals(currency)
	fmtFloat := fmt.Sprintf("%%.%df", decimals)

	for _, ins := range insights {
		_ = w.Write([]string{
			ins.Date.Format("2006-01-02"),
			ins.CampaignName,
			fmt.Sprintf(fmtFloat, ins.Spend),
			strconv.FormatInt(ins.Impressions, 10),
			strconv.FormatInt(ins.Clicks, 10),
			fmt.Sprintf("%.4f", ins.CTR),
			fmt.Sprintf(fmtFloat, ins.CPC),
			fmt.Sprintf(fmtFloat, ins.CPM),
			strconv.FormatInt(ins.Reach, 10),
		})
	}
}
