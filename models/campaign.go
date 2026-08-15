package models

import (
	"time"

	"adpanel/database"
)

type Campaign struct {
	ID                  uint64     `db:"id" json:"id"`
	AdAccountID         uint64     `db:"ad_account_id" json:"ad_account_id"`
	UserID              uint64     `db:"user_id" json:"user_id"`
	MetaCampaignID      string     `db:"meta_campaign_id" json:"meta_campaign_id"`
	Name                string     `db:"name" json:"name"`
	Status              string     `db:"status" json:"status"`
	Objective           string     `db:"objective" json:"objective"`
	SpecialAdCategories string     `db:"special_ad_categories" json:"special_ad_categories"`
	DailyBudget         int64      `db:"daily_budget" json:"daily_budget"`
	StartTime           *time.Time `db:"start_time" json:"start_time"`
	EndTime             *time.Time `db:"end_time" json:"end_time"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`

	// Joined fields
	AdAccountName string `db:"ad_account_name" json:"ad_account_name,omitempty"`
}

type AdSet struct {
	ID           uint64     `db:"id" json:"id"`
	CampaignID   uint64     `db:"campaign_id" json:"campaign_id"`
	UserID       uint64     `db:"user_id" json:"user_id"`
	MetaAdSetID  string     `db:"meta_adset_id" json:"meta_adset_id"`
	Name         string     `db:"name" json:"name"`
	Status       string     `db:"status" json:"status"`
	DailyBudget  int64      `db:"daily_budget" json:"daily_budget"`
	BidStrategy  string     `db:"bid_strategy" json:"bid_strategy"`
	Targeting    string     `db:"targeting" json:"targeting"`
	StartTime    *time.Time `db:"start_time" json:"start_time"`
	EndTime      *time.Time `db:"end_time" json:"end_time"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

type Ad struct {
	ID          uint64    `db:"id" json:"id"`
	AdSetID     uint64    `db:"ad_set_id" json:"ad_set_id"`
	UserID      uint64    `db:"user_id" json:"user_id"`
	MetaAdID    string    `db:"meta_ad_id" json:"meta_ad_id"`
	Name        string    `db:"name" json:"name"`
	Status      string    `db:"status" json:"status"`
	CreativeID  uint64    `db:"creative_id" json:"creative_id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type CampaignNote struct {
	ID         uint64    `db:"id" json:"id"`
	CampaignID uint64    `db:"campaign_id" json:"campaign_id"`
	UserID     uint64    `db:"user_id" json:"user_id"`
	Note       string    `db:"note" json:"note"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

func GetCampaignByID(id, userID uint64) (*Campaign, error) {
	var c Campaign
	err := database.DB.Get(&c,
		`SELECT c.*, aa.name AS ad_account_name FROM campaigns c
		 JOIN ad_accounts aa ON c.ad_account_id = aa.id
		 WHERE c.id = ? AND c.user_id = ?`, id, userID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func ListCampaignsByAdAccount(adAccountID, userID uint64) ([]Campaign, error) {
	var campaigns []Campaign
	err := database.DB.Select(&campaigns,
		"SELECT * FROM campaigns WHERE ad_account_id = ? AND user_id = ? ORDER BY created_at DESC",
		adAccountID, userID)
	return campaigns, err
}

func ListCampaignsByUser(userID uint64) ([]Campaign, error) {
	var campaigns []Campaign
	err := database.DB.Select(&campaigns,
		`SELECT c.*, aa.name AS ad_account_name FROM campaigns c
		 JOIN ad_accounts aa ON c.ad_account_id = aa.id
		 WHERE c.user_id = ? ORDER BY c.created_at DESC`, userID)
	return campaigns, err
}

func CreateCampaign(c *Campaign) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO campaigns (ad_account_id, user_id, meta_campaign_id, name, status, objective,
		 special_ad_categories, daily_budget, start_time, end_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.AdAccountID, c.UserID, c.MetaCampaignID, c.Name, c.Status, c.Objective,
		c.SpecialAdCategories, c.DailyBudget, c.StartTime, c.EndTime,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateCampaign(c *Campaign) error {
	_, err := database.DB.Exec(
		`UPDATE campaigns SET meta_campaign_id=?, name=?, status=?, objective=?,
		 special_ad_categories=?, daily_budget=?, start_time=?, end_time=?
		 WHERE id=? AND user_id=?`,
		c.MetaCampaignID, c.Name, c.Status, c.Objective,
		c.SpecialAdCategories, c.DailyBudget, c.StartTime, c.EndTime,
		c.ID, c.UserID,
	)
	return err
}

func UpdateCampaignStatus(id, userID uint64, status string) error {
	_, err := database.DB.Exec(
		"UPDATE campaigns SET status = ? WHERE id = ? AND user_id = ?",
		status, id, userID)
	return err
}

func UpdateCampaignBudget(id, userID uint64, budget int64) error {
	_, err := database.DB.Exec(
		"UPDATE campaigns SET daily_budget = ? WHERE id = ? AND user_id = ?",
		budget, id, userID)
	return err
}

func DeleteCampaign(id, userID uint64) error {
	_, err := database.DB.Exec(
		"DELETE FROM campaigns WHERE id = ? AND user_id = ?", id, userID)
	return err
}

func UpsertCampaignFromMeta(c *Campaign) error {
	_, err := database.DB.Exec(
		`INSERT INTO campaigns (ad_account_id, user_id, meta_campaign_id, name, status, objective, daily_budget)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE name=VALUES(name), status=VALUES(status),
		 objective=VALUES(objective), daily_budget=VALUES(daily_budget)`,
		c.AdAccountID, c.UserID, c.MetaCampaignID, c.Name, c.Status, c.Objective, c.DailyBudget,
	)
	return err
}

func CreateAdSet(a *AdSet) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO ad_sets (campaign_id, user_id, meta_adset_id, name, status, daily_budget,
		 bid_strategy, targeting, start_time, end_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.CampaignID, a.UserID, a.MetaAdSetID, a.Name, a.Status, a.DailyBudget,
		a.BidStrategy, a.Targeting, a.StartTime, a.EndTime,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func CreateAd(a *Ad) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO ads (ad_set_id, user_id, meta_ad_id, name, status, creative_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.AdSetID, a.UserID, a.MetaAdID, a.Name, a.Status, a.CreativeID,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func AddCampaignNote(n *CampaignNote) error {
	_, err := database.DB.Exec(
		"INSERT INTO campaign_notes (campaign_id, user_id, note) VALUES (?, ?, ?)",
		n.CampaignID, n.UserID, n.Note)
	return err
}

func ListCampaignNotes(campaignID uint64) ([]CampaignNote, error) {
	var notes []CampaignNote
	err := database.DB.Select(&notes,
		"SELECT * FROM campaign_notes WHERE campaign_id = ? ORDER BY created_at DESC",
		campaignID)
	return notes, err
}
