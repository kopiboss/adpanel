package models

import (
	"time"

	"adpanel/database"
)

type CampaignTemplate struct {
	ID           uint64    `db:"id" json:"id"`
	UserID       uint64    `db:"user_id" json:"user_id"`
	Name         string    `db:"name" json:"name"`
	Objective    string    `db:"objective" json:"objective"`
	Targeting    string    `db:"targeting" json:"targeting"`
	DailyBudget  int64     `db:"daily_budget" json:"daily_budget"`
	Placements   string    `db:"placements" json:"placements"`
	SettingsJSON string    `db:"settings_json" json:"settings_json"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

func GetTemplateByID(id, userID uint64) (*CampaignTemplate, error) {
	var t CampaignTemplate
	err := database.DB.Get(&t,
		"SELECT * FROM campaign_templates WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func ListTemplatesByUser(userID uint64) ([]CampaignTemplate, error) {
	var templates []CampaignTemplate
	err := database.DB.Select(&templates,
		"SELECT * FROM campaign_templates WHERE user_id = ? ORDER BY created_at DESC", userID)
	return templates, err
}

func CreateTemplate(t *CampaignTemplate) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO campaign_templates (user_id, name, objective, targeting, daily_budget, placements, settings_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.UserID, t.Name, t.Objective, t.Targeting, t.DailyBudget, t.Placements, t.SettingsJSON,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateTemplate(t *CampaignTemplate) error {
	_, err := database.DB.Exec(
		`UPDATE campaign_templates SET name=?, objective=?, targeting=?, daily_budget=?,
		 placements=?, settings_json=? WHERE id=? AND user_id=?`,
		t.Name, t.Objective, t.Targeting, t.DailyBudget, t.Placements, t.SettingsJSON, t.ID, t.UserID,
	)
	return err
}

func DeleteTemplate(id, userID uint64) error {
	_, err := database.DB.Exec(
		"DELETE FROM campaign_templates WHERE id = ? AND user_id = ?", id, userID)
	return err
}

type PlatformSetting struct {
	ID        uint64    `db:"id" json:"id"`
	Key       string    `db:"key" json:"key"`
	Value     string    `db:"value" json:"value"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func GetSetting(key string) (string, error) {
	var s PlatformSetting
	err := database.DB.Get(&s, "SELECT * FROM platform_settings WHERE `key` = ?", key)
	if err != nil {
		return "", err
	}
	return s.Value, nil
}

func SetSetting(key, value string) error {
	_, err := database.DB.Exec(
		"INSERT INTO platform_settings (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value=VALUES(value)",
		key, value)
	return err
}

func GetAllSettings() (map[string]string, error) {
	var settings []PlatformSetting
	err := database.DB.Select(&settings, "SELECT * FROM platform_settings")
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, s := range settings {
		m[s.Key] = s.Value
	}
	return m, nil
}
