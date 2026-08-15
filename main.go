package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

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

	// Build template set with all templates named by relative path
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
		// Currency helpers — auto-detect dari currency field ad account
		"formatMoney":   helpers.FormatMoney,   // formatMoney amount currency
		"formatBudget":  helpers.FormatBudget,  // formatBudget smallestUnit currency
		"currencySymbol": helpers.CurrencySymbol, // currencySymbol currency
	}

	tmpl := template.New("").Funcs(funcMap)
	patterns := []string{
		"templates/*.html",
		"templates/auth/*.html",
		"templates/admin/*.html",
	}
	for _, pattern := range patterns {
		files, _ := filepath.Glob(pattern)
		for _, f := range files {
			rel, _ := filepath.Rel("templates", f)
			content, err := os.ReadFile(f)
			if err != nil {
				log.Fatalf("Failed to read template %s: %v", f, err)
			}
			template.Must(tmpl.New(rel).Funcs(funcMap).Parse(string(content)))
		}
	}
	r.SetHTMLTemplate(tmpl)

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
