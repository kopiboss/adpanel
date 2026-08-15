package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"adpanel/database"
	"adpanel/models"
)

type MetaInsightData struct {
	CampaignID  string `json:"campaign_id"`
	DateStart   string `json:"date_start"`
	DateStop    string `json:"date_stop"`
	Spend       string `json:"spend"`
	Impressions string `json:"impressions"`
	Clicks      string `json:"clicks"`
	CTR         string `json:"ctr"`
	CPC         string `json:"cpc"`
	CPM         string `json:"cpm"`
	Reach       string `json:"reach"`
}

// FetchInsights retrieves campaign insights from Meta for a given date range
func (c *MetaClient) FetchInsights(adAccountID, dateFrom, dateTo string) ([]MetaInsightData, error) {
	params := url.Values{
		"fields":     {"campaign_id,spend,impressions,clicks,ctr,cpc,cpm,reach"},
		"level":      {"campaign"},
		"time_range": {fmt.Sprintf(`{"since":"%s","until":"%s"}`, dateFrom, dateTo)},
		"limit":      {"500"},
	}

	body, err := c.get(fmt.Sprintf("act_%s/insights", adAccountID), params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []MetaInsightData `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// SyncInsights fetches insights for the last N days and saves to DB
func SyncInsights(client *MetaClient, adAccount *models.AdAccount, userID uint64, days int) error {
	now := time.Now()
	dateTo := now.Format("2006-01-02")
	dateFrom := now.AddDate(0, 0, -days).Format("2006-01-02")

	insights, err := client.FetchInsights(adAccount.MetaAccountID, dateFrom, dateTo)
	if err != nil {
		return fmt.Errorf("fetch insights: %w", err)
	}

	for _, ins := range insights {
		spend, _ := strconv.ParseFloat(ins.Spend, 64)
		impressions, _ := strconv.ParseInt(ins.Impressions, 10, 64)
		clicks, _ := strconv.ParseInt(ins.Clicks, 10, 64)
		ctr, _ := strconv.ParseFloat(ins.CTR, 64)
		cpc, _ := strconv.ParseFloat(ins.CPC, 64)
		cpm, _ := strconv.ParseFloat(ins.CPM, 64)
		reach, _ := strconv.ParseInt(ins.Reach, 10, 64)

		date, err := time.Parse("2006-01-02", ins.DateStart)
		if err != nil {
			continue
		}

		var campaignID uint64
		if ins.CampaignID != "" {
			var campaign models.Campaign
			if err := database.DB.Get(&campaign,
				"SELECT id FROM campaigns WHERE meta_campaign_id = ? AND user_id = ?",
				ins.CampaignID, userID); err == nil {
				campaignID = campaign.ID
			}
		}

		insight := &models.Insight{
			AdAccountID: adAccount.ID,
			UserID:      userID,
			CampaignID:  campaignID,
			Date:        date,
			Spend:       spend,
			Impressions: impressions,
			Clicks:      clicks,
			CTR:         ctr,
			CPC:         cpc,
			CPM:         cpm,
			Reach:       reach,
		}

		if err := models.UpsertInsight(insight); err != nil {
			return fmt.Errorf("upsert insight: %w", err)
		}
	}

	return nil
}
