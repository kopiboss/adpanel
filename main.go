package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"adpanel/config"
	"adpanel/database"
	"adpanel/handlers"
	"adpanel/helpers"
	"adpanel/middleware"
	"adpanel/services"
)

func main() {
	config.Load()
	database.Connect()
	defer database.Close()

	services.InitGoogleOAuth()
	services.InitTelegramBot()

	go services.Bot.StartPolling(services.HandleTelegramUpdate)

	cr := cron.New()
	_, _ = cr.AddFunc("*/30 * * * *", func() {
		log.Println("Cron: starting scheduled sync")
		services.SyncAllAccounts()
	})
	cr.Start()
	defer cr.Stop()

	r := gin.Default()

	funcMap := template.FuncMap{
		"divf": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"js": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"slice": func(s string, i, j int) string {
			runes := []rune(s)
			if i >= len(runes) {
				return ""
			}
			if j > len(runes) {
				j = len(runes)
			}
			return string(runes[i:j])
		},
		"gt0":            func(n int) bool { return n > 0 },
		"formatMoney":    helpers.FormatMoney,
		"formatBudget":   helpers.FormatBudget,
		"currencySymbol": helpers.CurrencySymbol,
		"formatFileSize": func(bytes int64) string {
			if bytes <= 0 {
				return ""
			}
			const (
				KB = 1024
				MB = 1024 * KB
				GB = 1024 * MB
			)
			switch {
			case bytes >= GB:
				return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
			case bytes >= MB:
				return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
			case bytes >= KB:
				return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
			default:
				return fmt.Sprintf("%d B", bytes)
			}
		},
	}

	readFile := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("Cannot read template %s: %v", path, err)
		}
		return string(b)
	}

	// Setiap halaman punya template set sendiri:
	// - Auth pages: hanya file itu sendiri (standalone HTML)
	// - Page lain: layout.html + page file
	// Dengan cara ini setiap set hanya punya SATU {{ define "content" }}
	// sehingga tidak ada conflict antar halaman.

	layout := readFile("templates/layout.html")

	makePageTemplate := func(pageFile string) *template.Template {
		t := template.Must(
			template.New("layout.html").Funcs(funcMap).Parse(layout),
		)
		template.Must(t.New(pageFile).Funcs(funcMap).Parse(readFile("templates/" + pageFile)))
		return t
	}

	makeAuthTemplate := func(pageFile string) *template.Template {
		return template.Must(
			template.New(pageFile).Funcs(funcMap).Parse(readFile("templates/" + pageFile)),
		)
	}

	templateMap := map[string]*template.Template{
		// Auth (standalone, no layout)
		"auth/login.html":    makeAuthTemplate("auth/login.html"),
		"auth/register.html": makeAuthTemplate("auth/register.html"),
		"auth/pending.html":  makeAuthTemplate("auth/pending.html"),

		// Admin pages
		"admin/dashboard.html": makePageTemplate("admin/dashboard.html"),
		"admin/users.html":     makePageTemplate("admin/users.html"),
		"admin/settings.html":  makePageTemplate("admin/settings.html"),

		// User pages
		"dashboard.html":       makePageTemplate("dashboard.html"),
		"credentials.html":     makePageTemplate("credentials.html"),
		"ad_accounts.html":     makePageTemplate("ad_accounts.html"),
		"campaigns.html":       makePageTemplate("campaigns.html"),
		"campaign_wizard.html": makePageTemplate("campaign_wizard.html"),
		"creatives.html":       makePageTemplate("creatives.html"),
		"analytics.html":       makePageTemplate("analytics.html"),
		"sync_log.html":        makePageTemplate("sync_log.html"),
	}

	r.HTMLRender = &multiTemplateRenderer{templates: templateMap}

	store := cookie.NewStore([]byte(config.App.AppSecret))
	r.Use(sessions.Sessions("adpanel_session", store))
	r.Static("/static", "./static")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})

	guest := r.Group("/")
	guest.Use(middleware.RequireGuest())
	{
		guest.GET("/login", handlers.ShowLogin)
		guest.POST("/login", handlers.HandleLogin)
		guest.GET("/register", handlers.ShowRegister)
		guest.POST("/register", handlers.HandleRegister)
		guest.GET("/auth/google", handlers.GoogleOAuthStart)
		guest.GET("/auth/google/callback", handlers.GoogleOAuthCallback)
	}

	r.GET("/pending", handlers.ShowPending)
	r.GET("/logout", handlers.HandleLogout)

	admin := r.Group("/admin")
	admin.Use(middleware.RequireAuth(), middleware.RequireAdmin())
	{
		admin.GET("", handlers.AdminDashboard)
		admin.GET("/users", handlers.AdminListUsers)
		admin.POST("/users/:id/approve", handlers.AdminApproveUser)
		admin.POST("/users/:id/reject", handlers.AdminRejectUser)
		admin.POST("/users/:id/suspend", handlers.AdminSuspendUser)
		admin.POST("/users/:id/unsuspend", handlers.AdminUnsuspendUser)
		admin.GET("/settings", handlers.AdminShowSettings)
		admin.POST("/settings", handlers.AdminSaveSettings)
	}

	usr := r.Group("/")
	usr.Use(middleware.RequireAuth(), middleware.RequireUser())
	{
		usr.GET("/dashboard", handlers.UserDashboard)
		usr.GET("/credentials", handlers.ListCredentials)
		usr.POST("/credentials", handlers.CreateCredential)
		usr.POST("/credentials/:id", handlers.UpdateCredential)
		usr.DELETE("/credentials/:id", handlers.DeleteCredential)
		usr.GET("/credentials/:id/ad-accounts", handlers.FetchAdAccounts)
		usr.GET("/credentials/:id/validate", handlers.ValidateToken)
		usr.GET("/ad-accounts", handlers.ListAdAccounts)
		usr.POST("/ad-accounts", handlers.SaveAdAccounts)
		usr.POST("/ad-accounts/:id/toggle", handlers.ToggleAdAccount)
		usr.DELETE("/ad-accounts/:id", handlers.DeleteAdAccount)
		usr.GET("/campaigns", handlers.ListCampaigns)
		usr.GET("/campaigns/new", handlers.ShowCampaignWizard)
		usr.POST("/campaigns/launch", handlers.LaunchCampaign)
		usr.POST("/campaigns/bulk", handlers.BulkCampaignAction)
		usr.POST("/campaigns/:id/toggle", handlers.ToggleCampaignStatus)
		usr.POST("/campaigns/:id/budget", handlers.UpdateCampaignBudget)
		usr.POST("/campaigns/:id/duplicate", handlers.DuplicateCampaign)
		usr.DELETE("/campaigns/:id", handlers.DeleteCampaignHandler)
		usr.GET("/templates", handlers.ListTemplates)
		usr.GET("/templates/:id", handlers.GetTemplate)
		usr.DELETE("/templates/:id", handlers.DeleteTemplate)
		usr.GET("/creatives", handlers.ListCreatives)
		usr.POST("/creatives/upload", handlers.UploadCreative)
		usr.POST("/creatives/upload-multi", handlers.UploadToMultipleAccounts)
		usr.GET("/creatives/:id/status", handlers.GetCreativeStatus)
		usr.GET("/creatives/:id/thumbnail", handlers.ProxyThumbnail)
		usr.POST("/creatives/:id/refresh-thumbnail", handlers.RefreshCreativeThumbnail)
		usr.DELETE("/creatives/:id", handlers.DeleteCreativeHandler)
		usr.GET("/analytics", handlers.ShowAnalytics)
		usr.GET("/analytics/export", handlers.ExportAnalyticsCSV)
		usr.GET("/sync-logs", handlers.ListSyncLogs)
		usr.POST("/sync/:id", handlers.SyncNow)
	}

	addr := ":" + config.App.AppPort
	log.Printf("AdPanel starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
