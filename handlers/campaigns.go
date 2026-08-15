package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"adpanel/helpers"
	"adpanel/middleware"
	"adpanel/models"
	"adpanel/services"
)

func ListCampaigns(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	accounts, _ := models.ListActiveAdAccountsByUser(userID)
	templates, _ := models.ListTemplatesByUser(userID)

	accountID := uint64(0)
	if accIDStr := c.Query("account_id"); accIDStr != "" {
		accountID, _ = strconv.ParseUint(accIDStr, 10, 64)
	}

	// Date range — default hari ini WIB
	loc, _ := time.LoadLocation("Asia/Jakarta")
	nowWIB := time.Now().In(loc)

	dateRange := c.Query("range")
	if dateRange == "" {
		dateRange = "today"
	}

	var dateFrom, dateTo string
	today := nowWIB.Format("2006-01-02")
	switch dateRange {
	case "today":
		dateFrom = today
		dateTo = today
	case "yesterday":
		yesterday := nowWIB.AddDate(0, 0, -1).Format("2006-01-02")
		dateFrom = yesterday
		dateTo = yesterday
	case "7d":
		dateFrom = nowWIB.AddDate(0, 0, -7).Format("2006-01-02")
		dateTo = today
	case "30d":
		dateFrom = nowWIB.AddDate(0, 0, -30).Format("2006-01-02")
		dateTo = today
	case "custom":
		dateFrom = c.Query("date_from")
		dateTo = c.Query("date_to")
		if dateFrom == "" {
			dateFrom = today
		}
		if dateTo == "" {
			dateTo = today
		}
	default:
		dateFrom = today
		dateTo = today
	}

	var campaigns []models.Campaign
	var err error
	if accountID > 0 {
		campaigns, err = models.ListCampaignsByAdAccount(accountID, userID)
	} else {
		campaigns, err = models.ListCampaignsByUser(userID)
	}
	if err != nil {
		campaigns = []models.Campaign{}
	}

	// Currency map per account
	accountCurrency := make(map[uint64]string)
	for _, acc := range accounts {
		accountCurrency[acc.ID] = helpers.DefaultCurrency(acc.Currency)
	}
	pageCurrency := "IDR"
	if accountID > 0 {
		if cur, ok := accountCurrency[accountID]; ok {
			pageCurrency = cur
		}
	} else if len(accounts) > 0 {
		pageCurrency = helpers.DefaultCurrency(accounts[0].Currency)
	}

	// Insights per campaign untuk date range yang dipilih
	insightMap, _ := models.GetCampaignInsightMap(userID, dateFrom, dateTo)
	if insightMap == nil {
		insightMap = make(map[string]*models.InsightSummary)
	}

	c.HTML(http.StatusOK, "campaigns.html", gin.H{
		"title":            "Kampanye",
		"campaigns":        campaigns,
		"accounts":         accounts,
		"templates":        templates,
		"account_id":       accountID,
		"account_currency": accountCurrency,
		"currency":         pageCurrency,
		"insight_map":      insightMap,
		"date_range":       dateRange,
		"date_from":        dateFrom,
		"date_to":          dateTo,
		"user":             middleware.GetCurrentUser(c),
	})
}

func ShowCampaignWizard(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	accounts, _ := models.ListActiveAdAccountsByUser(userID)
	templates, _ := models.ListTemplatesByUser(userID)
	creatives, _ := models.ListCreativesByUser(userID)

	c.HTML(http.StatusOK, "campaign_wizard.html", gin.H{
		"title":     "Buat Kampanye",
		"accounts":  accounts,
		"templates": templates,
		"creatives": creatives,
		"user":      middleware.GetCurrentUser(c),
	})
}

type CampaignWizardRequest struct {
	// Step 1 - Campaign
	AccountIDs          []uint64 `json:"account_ids"`
	CampaignName        string   `json:"campaign_name"`
	Objective           string   `json:"objective"`
	SpecialAdCategories string   `json:"special_ad_categories"`

	// Step 2 - Ad Set
	AdSetName   string  `json:"adset_name"`
	DailyBudget float64 `json:"daily_budget"`
	Currency    string  `json:"currency"` // dari frontend, untuk konversi ke unit terkecil
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	BidStrategy string  `json:"bid_strategy"`
	Countries   []string `json:"countries"`
	AgeMin      int     `json:"age_min"`
	AgeMax      int     `json:"age_max"`
	Gender      string  `json:"gender"`
	Placements  []string `json:"placements"`

	// Step 3 - Ad
	AdName      string `json:"ad_name"`
	CreativeID  uint64 `json:"creative_id"`
	Format      string `json:"format"`
	PrimaryText string `json:"primary_text"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	CTA         string `json:"cta"`
	DestURL     string `json:"dest_url"`

	// Template
	SaveAsTemplate   bool   `json:"save_as_template"`
	TemplateName     string `json:"template_name"`
}

func LaunchCampaign(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var req CampaignWizardRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request tidak valid: " + err.Error()})
		return
	}

	if len(req.AccountIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih minimal 1 ad account"})
		return
	}

	// Build targeting JSON
	var genders []int
	switch req.Gender {
	case "male":
		genders = []int{1}
	case "female":
		genders = []int{2}
	}

	if req.AgeMin == 0 {
		req.AgeMin = 18
	}
	if req.AgeMax == 0 {
		req.AgeMax = 65
	}

	targetingJSON := services.BuildTargetingJSON(req.Countries, req.AgeMin, req.AgeMax, genders, req.Placements)

	// Konversi budget ke unit terkecil sesuai currency.
	// IDR & VND & JPY dll: tidak ada sub-unit, divisor=1 → budget langsung disimpan bulat.
	// USD, SGD, dll: ada sen, divisor=100.
	budgetSmallestUnit := helpers.ToSmallestUnit(req.DailyBudget, req.Currency)

	// Get creative
	var creative *models.Creative
	if req.CreativeID > 0 {
		creative, _ = models.GetCreativeByID(req.CreativeID, userID)
	}

	type result struct {
		AccountID uint64 `json:"account_id"`
		Name      string `json:"name"`
		Success   bool   `json:"success"`
		Error     string `json:"error"`
		CampaignID uint64 `json:"campaign_id"`
	}

	results := make([]result, 0, len(req.AccountIDs))
	resultCh := make(chan result, len(req.AccountIDs))

	for _, accountID := range req.AccountIDs {
		go func(accID uint64) {
			r := result{AccountID: accID}

			account, err := models.GetAdAccountByID(accID, userID)
			if err != nil {
				r.Error = "Ad account tidak ditemukan"
				resultCh <- r
				return
			}
			r.Name = account.Name

			cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
			if err != nil {
				r.Error = "Gagal load kredensial"
				resultCh <- r
				return
			}

			accessToken, err := services.Decrypt(cred.AccessTokenEnc)
			if err != nil {
				r.Error = "Gagal dekripsi token"
				resultCh <- r
				return
			}

			client := services.NewMetaClient(accessToken)

			// Create campaign on Meta
			metaCampaignID, err := client.CreateCampaign(account.MetaAccountID, services.CreateCampaignReq{
				Name:                req.CampaignName,
				Objective:           req.Objective,
				SpecialAdCategories: []string{req.SpecialAdCategories},
				Status:              "PAUSED",
				DailyBudget:         budgetSmallestUnit, // cents
			})
			if err != nil {
				r.Error = "Gagal buat campaign: " + err.Error()
				resultCh <- r
				return
			}

			// Save campaign to DB
			var startTime, endTime *time.Time
			if req.StartDate != "" {
				t, err := time.Parse("2006-01-02", req.StartDate)
				if err == nil {
					startTime = &t
				}
			}
			if req.EndDate != "" {
				t, err := time.Parse("2006-01-02", req.EndDate)
				if err == nil {
					endTime = &t
				}
			}

			campaign := &models.Campaign{
				AdAccountID:         accID,
				UserID:              userID,
				MetaCampaignID:      metaCampaignID,
				Name:                req.CampaignName,
				Status:              "PAUSED",
				Objective:           req.Objective,
				SpecialAdCategories: req.SpecialAdCategories,
				DailyBudget:         budgetSmallestUnit,
				StartTime:           startTime,
				EndTime:             endTime,
			}
			campaignDBID, err := models.CreateCampaign(campaign)
			if err != nil {
				r.Error = "Gagal simpan campaign ke DB"
				resultCh <- r
				return
			}

			// Create ad set on Meta
			metaAdSetID, err := client.CreateAdSet(account.MetaAccountID, services.CreateAdSetReq{
				Name:          req.AdSetName,
				CampaignID:    metaCampaignID,
				DailyBudget:   budgetSmallestUnit,
				BidStrategy:   req.BidStrategy,
				TargetingJSON: targetingJSON,
				StartTime:     req.StartDate,
				EndTime:       req.EndDate,
				Status:        "PAUSED",
			})
			if err != nil {
				r.Error = "Gagal buat ad set: " + err.Error()
				resultCh <- r
				return
			}

			// Save ad set to DB
			adSet := &models.AdSet{
				CampaignID:  campaignDBID,
				UserID:      userID,
				MetaAdSetID: metaAdSetID,
				Name:        req.AdSetName,
				Status:      "PAUSED",
				DailyBudget: budgetSmallestUnit,
				BidStrategy: req.BidStrategy,
				Targeting:   targetingJSON,
				StartTime:   startTime,
				EndTime:     endTime,
			}
			adSetDBID, err := models.CreateAdSet(adSet)
			if err != nil {
				r.Error = "Gagal simpan ad set ke DB"
				resultCh <- r
				return
			}

			// Create ad creative and ad if creative is selected
			if creative != nil {
				creativeReq := services.CreateAdCreativeReq{
					Name:        req.AdName,
					Format:      req.Format,
					PrimaryText: req.PrimaryText,
					Headline:    req.Headline,
					Description: req.Description,
					CTA:         req.CTA,
					DestURL:     req.DestURL,
				}

				if creative.Type == "image" {
					creativeReq.ImageHash = creative.MetaImageHash
				} else {
					creativeReq.VideoID = creative.MetaVideoID
				}

				metaCreativeID, err := client.CreateAdCreative(account.MetaAccountID, creativeReq)
				if err != nil {
					r.Error = "Gagal buat creative: " + err.Error()
					resultCh <- r
					return
				}

				// Create ad
				metaAdID, err := client.CreateAd(
					account.MetaAccountID, metaAdSetID, metaCreativeID, req.AdName, "PAUSED",
				)
				if err != nil {
					r.Error = "Gagal buat ad: " + err.Error()
					resultCh <- r
					return
				}

				ad := &models.Ad{
					AdSetID:    adSetDBID,
					UserID:     userID,
					MetaAdID:   metaAdID,
					Name:       req.AdName,
					Status:     "PAUSED",
					CreativeID: req.CreativeID,
				}
				_, _ = models.CreateAd(ad)
			}

			r.Success = true
			r.CampaignID = campaignDBID
			resultCh <- r
		}(accountID)
	}

	for range req.AccountIDs {
		r := <-resultCh
		results = append(results, r)
	}

	// Save as template if requested
	if req.SaveAsTemplate && req.TemplateName != "" {
		placementsJSON, _ := json.Marshal(req.Placements)
		settingsJSON, _ := json.Marshal(map[string]interface{}{
			"ad_name":     req.AdName,
			"primary_text": req.PrimaryText,
			"headline":    req.Headline,
			"description": req.Description,
			"cta":         req.CTA,
			"format":      req.Format,
		})

		template := &models.CampaignTemplate{
			UserID:       userID,
			Name:         req.TemplateName,
			Objective:    req.Objective,
			Targeting:    services.BuildTargetingJSON(req.Countries, req.AgeMin, req.AgeMax, genders, req.Placements),
			DailyBudget:  budgetSmallestUnit,
			Placements:   string(placementsJSON),
			SettingsJSON: string(settingsJSON),
		}
		_, _ = models.CreateTemplate(template)
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

func ToggleCampaignStatus(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	campaign, err := models.GetCampaignByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign tidak ditemukan"})
		return
	}

	// Get credential for API call
	account, err := models.GetAdAccountByID(campaign.AdAccountID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal load ad account"})
		return
	}

	cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal load kredensial"})
		return
	}

	accessToken, err := services.Decrypt(cred.AccessTokenEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dekripsi token"})
		return
	}

	newStatus := "ACTIVE"
	if campaign.Status == "ACTIVE" {
		newStatus = "PAUSED"
	}

	client := services.NewMetaClient(accessToken)
	if err := client.UpdateCampaignStatus(campaign.MetaCampaignID, newStatus); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status di Meta: " + err.Error()})
		return
	}

	if err := models.UpdateCampaignStatus(id, userID, newStatus); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status di DB"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  newStatus,
		"message": "Status berhasil diubah",
	})
}

func UpdateCampaignBudget(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	budgetStr := c.PostForm("daily_budget")
	budget, err := strconv.ParseFloat(budgetStr, 64)
	if err != nil || budget <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Budget tidak valid"})
		return
	}

	campaign, err := models.GetCampaignByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign tidak ditemukan"})
		return
	}

	account, err := models.GetAdAccountByID(campaign.AdAccountID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal load ad account"})
		return
	}

	cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal load kredensial"})
		return
	}

	accessToken, err := services.Decrypt(cred.AccessTokenEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dekripsi token"})
		return
	}

	budgetCents := int64(budget * 100)
	client := services.NewMetaClient(accessToken)

	if err := client.UpdateCampaignBudget(campaign.MetaCampaignID, budgetCents); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update budget di Meta: " + err.Error()})
		return
	}

	if err := models.UpdateCampaignBudget(id, userID, budgetCents); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update budget di DB"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Budget berhasil diupdate"})
}

func BulkCampaignAction(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var req struct {
		Action      string   `json:"action"`
		CampaignIDs []uint64 `json:"campaign_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request tidak valid"})
		return
	}

	if len(req.CampaignIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih minimal 1 kampanye"})
		return
	}

	success := 0
	failed := 0

	for _, campaignID := range req.CampaignIDs {
		campaign, err := models.GetCampaignByID(campaignID, userID)
		if err != nil {
			failed++
			continue
		}

		account, err := models.GetAdAccountByID(campaign.AdAccountID, userID)
		if err != nil {
			failed++
			continue
		}

		cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
		if err != nil {
			failed++
			continue
		}

		accessToken, err := services.Decrypt(cred.AccessTokenEnc)
		if err != nil {
			failed++
			continue
		}

		client := services.NewMetaClient(accessToken)

		switch req.Action {
		case "pause":
			if err := client.UpdateCampaignStatus(campaign.MetaCampaignID, "PAUSED"); err == nil {
				_ = models.UpdateCampaignStatus(campaignID, userID, "PAUSED")
				success++
			} else {
				failed++
			}
		case "activate":
			if err := client.UpdateCampaignStatus(campaign.MetaCampaignID, "ACTIVE"); err == nil {
				_ = models.UpdateCampaignStatus(campaignID, userID, "ACTIVE")
				success++
			} else {
				failed++
			}
		case "delete":
			if err := client.DeleteCampaign(campaign.MetaCampaignID); err == nil {
				_ = models.DeleteCampaign(campaignID, userID)
				success++
			} else {
				failed++
			}
		default:
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": strconv.Itoa(success) + " kampanye berhasil, " + strconv.Itoa(failed) + " gagal",
		"success": success,
		"failed":  failed,
	})
}

func DeleteCampaignHandler(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	campaign, err := models.GetCampaignByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign tidak ditemukan"})
		return
	}

	account, err := models.GetAdAccountByID(campaign.AdAccountID, userID)
	if err == nil {
		cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
		if err == nil {
			if accessToken, err := services.Decrypt(cred.AccessTokenEnc); err == nil {
				client := services.NewMetaClient(accessToken)
				_ = client.DeleteCampaign(campaign.MetaCampaignID)
			}
		}
	}

	if err := models.DeleteCampaign(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hapus campaign"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaign berhasil dihapus"})
}

func ListTemplates(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	templates, err := models.ListTemplatesByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat templates"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func GetTemplate(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	template, err := models.GetTemplateByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, template)
}

func DeleteTemplate(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := models.DeleteTemplate(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hapus template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template berhasil dihapus"})
}

func DuplicateCampaign(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	original, err := models.GetCampaignByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign tidak ditemukan"})
		return
	}

	targetAccountIDStr := c.PostForm("target_account_id")
	targetAccountID := original.AdAccountID
	if targetAccountIDStr != "" {
		if tid, err := strconv.ParseUint(targetAccountIDStr, 10, 64); err == nil {
			targetAccountID = tid
		}
	}

	newName := strings.TrimSpace(c.PostForm("name"))
	if newName == "" {
		newName = "Copy of " + original.Name
	}

	newCampaign := &models.Campaign{
		AdAccountID:         targetAccountID,
		UserID:              userID,
		Name:                newName,
		Status:              "PAUSED",
		Objective:           original.Objective,
		SpecialAdCategories: original.SpecialAdCategories,
		DailyBudget:         original.DailyBudget,
	}

	newID, err := models.CreateCampaign(newCampaign)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal duplikasi campaign"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Campaign berhasil diduplikasi",
		"campaign_id": newID,
	})
}
