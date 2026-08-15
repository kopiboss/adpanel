package models

import (
	"time"

	"adpanel/database"
)

type SyncLog struct {
	ID           uint64    `db:"id" json:"id"`
	AdAccountID  uint64    `db:"ad_account_id" json:"ad_account_id"`
	UserID       uint64    `db:"user_id" json:"user_id"`
	SyncType     string    `db:"sync_type" json:"sync_type"`
	Status       string    `db:"status" json:"status"`
	ErrorMessage string    `db:"error_message" json:"error_message"`
	DurationMs   int64     `db:"duration_ms" json:"duration_ms"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`

	// Joined fields
	AdAccountName string `db:"ad_account_name" json:"ad_account_name,omitempty"`
}

func CreateSyncLog(s *SyncLog) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO sync_logs (ad_account_id, user_id, sync_type, status, error_message, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.AdAccountID, s.UserID, s.SyncType, s.Status, s.ErrorMessage, s.DurationMs,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateSyncLog(id uint64, status, errMsg string, durationMs int64) error {
	_, err := database.DB.Exec(
		"UPDATE sync_logs SET status=?, error_message=?, duration_ms=? WHERE id=?",
		status, errMsg, durationMs, id)
	return err
}

func ListSyncLogsByUser(userID uint64, limit int) ([]SyncLog, error) {
	var logs []SyncLog
	err := database.DB.Select(&logs,
		`SELECT sl.*, aa.name AS ad_account_name FROM sync_logs sl
		 LEFT JOIN ad_accounts aa ON sl.ad_account_id = aa.id
		 WHERE sl.user_id = ?
		 ORDER BY sl.created_at DESC LIMIT ?`,
		userID, limit)
	return logs, err
}

func ListAllSyncLogs(limit int) ([]SyncLog, error) {
	var logs []SyncLog
	err := database.DB.Select(&logs,
		`SELECT sl.*, aa.name AS ad_account_name FROM sync_logs sl
		 LEFT JOIN ad_accounts aa ON sl.ad_account_id = aa.id
		 ORDER BY sl.created_at DESC LIMIT ?`, limit)
	return logs, err
}

func ChangeLog(userID uint64, entityType string, entityID uint64, action, oldVal, newVal string) error {
	_, err := database.DB.Exec(
		`INSERT INTO change_logs (user_id, entity_type, entity_id, action, old_value, new_value)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, entityType, entityID, action, oldVal, newVal)
	return err
}
