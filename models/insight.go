package models

import (
	"time"

	"adpanel/database"
)

type Insight struct {
	ID          uint64    `db:"id" json:"id"`
	AdAccountID uint64    `db:"ad_account_id" json:"ad_account_id"`
	UserID      uint64    `db:"user_id" json:"user_id"`
	CampaignID  uint64    `db:"campaign_id" json:"campaign_id"`
	Date        time.Time `db:"date" json:"date"`
	Spend       float64   `db:"spend" json:"spend"`
	Impressions int64     `db:"impressions" json:"impressions"`
	Clicks      int64     `db:"clicks" json:"clicks"`
	CTR         float64   `db:"ctr" json:"ctr"`
	CPC         float64   `db:"cpc" json:"cpc"`
	CPM         float64   `db:"cpm" json:"cpm"`
	Reach       int64     `db:"reach" json:"reach"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`

	// Joined fields
	CampaignName  string `db:"campaign_name" json:"campaign_name,omitempty"`
	AdAccountName string `db:"ad_account_name" json:"ad_account_name,omitempty"`
}

type InsightSummary struct {
	TotalSpend       float64 `db:"total_spend" json:"total_spend"`
	TotalImpressions int64   `db:"total_impressions" json:"total_impressions"`
	TotalClicks      int64   `db:"total_clicks" json:"total_clicks"`
	AvgCTR           float64 `db:"avg_ctr" json:"avg_ctr"`
	AvgCPC           float64 `db:"avg_cpc" json:"avg_cpc"`
	AvgCPM           float64 `db:"avg_cpm" json:"avg_cpm"`
	TotalReach       int64   `db:"total_reach" json:"total_reach"`
}

type DailySpend struct {
	Date  string  `db:"date" json:"date"`
	Spend float64 `db:"spend" json:"spend"`
}

func UpsertInsight(i *Insight) error {
	_, err := database.DB.Exec(
		`INSERT INTO insights (ad_account_id, user_id, campaign_id, date, spend, impressions, clicks, ctr, cpc, cpm, reach)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE spend=VALUES(spend), impressions=VALUES(impressions),
		 clicks=VALUES(clicks), ctr=VALUES(ctr), cpc=VALUES(cpc), cpm=VALUES(cpm), reach=VALUES(reach)`,
		i.AdAccountID, i.UserID, i.CampaignID, i.Date, i.Spend, i.Impressions,
		i.Clicks, i.CTR, i.CPC, i.CPM, i.Reach,
	)
	return err
}

func GetInsightSummary(adAccountID, userID uint64, dateFrom, dateTo string) (*InsightSummary, error) {
	var s InsightSummary
	err := database.DB.Get(&s,
		`SELECT
		   COALESCE(SUM(spend),0) AS total_spend,
		   COALESCE(SUM(impressions),0) AS total_impressions,
		   COALESCE(SUM(clicks),0) AS total_clicks,
		   COALESCE(AVG(ctr),0) AS avg_ctr,
		   COALESCE(AVG(cpc),0) AS avg_cpc,
		   COALESCE(AVG(cpm),0) AS avg_cpm,
		   COALESCE(SUM(reach),0) AS total_reach
		 FROM insights
		 WHERE ad_account_id = ? AND user_id = ? AND date BETWEEN ? AND ?`,
		adAccountID, userID, dateFrom, dateTo)
	return &s, err
}

func GetDailySpend(adAccountID, userID uint64, dateFrom, dateTo string) ([]DailySpend, error) {
	var rows []DailySpend
	err := database.DB.Select(&rows,
		`SELECT DATE_FORMAT(date, '%Y-%m-%d') AS date, SUM(spend) AS spend
		 FROM insights
		 WHERE ad_account_id = ? AND user_id = ? AND date BETWEEN ? AND ?
		 GROUP BY date ORDER BY date ASC`,
		adAccountID, userID, dateFrom, dateTo)
	return rows, err
}

func GetCampaignInsights(adAccountID, userID uint64, dateFrom, dateTo string) ([]Insight, error) {
	var insights []Insight
	err := database.DB.Select(&insights,
		`SELECT i.*, c.name AS campaign_name
		 FROM insights i
		 LEFT JOIN campaigns c ON i.campaign_id = c.id
		 WHERE i.ad_account_id = ? AND i.user_id = ? AND i.date BETWEEN ? AND ?
		 GROUP BY i.campaign_id
		 ORDER BY i.spend DESC`,
		adAccountID, userID, dateFrom, dateTo)
	return insights, err
}

// GetCampaignInsightMap mengembalikan map[meta_campaign_id] → InsightSummary
// untuk ditampilkan inline di halaman kampanye
func GetCampaignInsightMap(userID uint64, dateFrom, dateTo string) (map[string]*InsightSummary, error) {
	type row struct {
		MetaCampaignID   string  `db:"meta_campaign_id"`
		TotalSpend       float64 `db:"total_spend"`
		TotalImpressions int64   `db:"total_impressions"`
		TotalClicks      int64   `db:"total_clicks"`
		TotalReach       int64   `db:"total_reach"`
		AvgCTR           float64 `db:"avg_ctr"`
		AvgCPC           float64 `db:"avg_cpc"`
		AvgCPM           float64 `db:"avg_cpm"`
	}
	var rows []row
	err := database.DB.Select(&rows, `
		SELECT
			c.meta_campaign_id,
			COALESCE(SUM(i.spend),0)       AS total_spend,
			COALESCE(SUM(i.impressions),0) AS total_impressions,
			COALESCE(SUM(i.clicks),0)      AS total_clicks,
			COALESCE(SUM(i.reach),0)       AS total_reach,
			COALESCE(AVG(i.ctr),0)         AS avg_ctr,
			COALESCE(AVG(i.cpc),0)         AS avg_cpc,
			COALESCE(AVG(i.cpm),0)         AS avg_cpm
		FROM insights i
		JOIN campaigns c ON i.campaign_id = c.id
		WHERE i.user_id = ? AND i.date BETWEEN ? AND ?
		GROUP BY c.meta_campaign_id`,
		userID, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*InsightSummary, len(rows))
	for _, r := range rows {
		result[r.MetaCampaignID] = &InsightSummary{
			TotalSpend:       r.TotalSpend,
			TotalImpressions: r.TotalImpressions,
			TotalClicks:      r.TotalClicks,
			TotalReach:       r.TotalReach,
			AvgCTR:           r.AvgCTR,
			AvgCPC:           r.AvgCPC,
			AvgCPM:           r.AvgCPM,
		}
	}
	return result, nil
}
	var s InsightSummary
	err := database.DB.Get(&s,
		`SELECT
		   COALESCE(SUM(spend),0) AS total_spend,
		   COALESCE(SUM(impressions),0) AS total_impressions,
		   COALESCE(SUM(clicks),0) AS total_clicks,
		   COALESCE(AVG(ctr),0) AS avg_ctr,
		   COALESCE(AVG(cpc),0) AS avg_cpc,
		   COALESCE(AVG(cpm),0) AS avg_cpm,
		   COALESCE(SUM(reach),0) AS total_reach
		 FROM insights
		 WHERE user_id = ? AND date BETWEEN ? AND ?`,
		userID, dateFrom, dateTo)
	return &s, err
}
