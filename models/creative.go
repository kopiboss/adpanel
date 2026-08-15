package models

import (
	"time"

	"adpanel/database"
)

type Creative struct {
	ID            uint64    `db:"id" json:"id"`
	AdAccountID   uint64    `db:"ad_account_id" json:"ad_account_id"`
	UserID        uint64    `db:"user_id" json:"user_id"`
	Name          string    `db:"name" json:"name"`
	Type          string    `db:"type" json:"type"`
	MetaImageHash string    `db:"meta_image_hash" json:"meta_image_hash"`
	MetaVideoID   string    `db:"meta_video_id" json:"meta_video_id"`
	ThumbnailURL  string    `db:"thumbnail_url" json:"thumbnail_url"`
	FileSize      int64     `db:"file_size" json:"file_size"`
	UploadStatus  string    `db:"upload_status" json:"upload_status"`
	ErrorMessage  string    `db:"error_message" json:"error_message"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`

	// Joined fields
	AdAccountName string `db:"ad_account_name" json:"ad_account_name,omitempty"`
}

func GetCreativeByID(id, userID uint64) (*Creative, error) {
	var c Creative
	err := database.DB.Get(&c,
		"SELECT * FROM creatives WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetCreativeByIDOnly cari berdasarkan ID saja tanpa filter user_id.
// Dipakai sebagai fallback saat polling status, dengan security check di handler.
func GetCreativeByIDOnly(id uint64) (*Creative, error) {
	var c Creative
	err := database.DB.Get(&c, "SELECT * FROM creatives WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func ListCreativesByAdAccount(adAccountID, userID uint64) ([]Creative, error) {
	var creatives []Creative
	err := database.DB.Select(&creatives,
		`SELECT c.*, aa.name AS ad_account_name FROM creatives c
		 JOIN ad_accounts aa ON c.ad_account_id = aa.id
		 WHERE c.ad_account_id = ? AND c.user_id = ? ORDER BY c.created_at DESC`,
		adAccountID, userID)
	return creatives, err
}

func ListCreativesByUser(userID uint64) ([]Creative, error) {
	var creatives []Creative
	err := database.DB.Select(&creatives,
		`SELECT c.*, aa.name AS ad_account_name FROM creatives c
		 JOIN ad_accounts aa ON c.ad_account_id = aa.id
		 WHERE c.user_id = ? ORDER BY c.created_at DESC`, userID)
	return creatives, err
}

func ListPendingCreatives() ([]Creative, error) {
	var creatives []Creative
	err := database.DB.Select(&creatives,
		"SELECT * FROM creatives WHERE upload_status = 'pending' ORDER BY created_at ASC")
	return creatives, err
}

func CreateCreative(c *Creative) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO creatives (ad_account_id, user_id, name, type, file_size, upload_status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.AdAccountID, c.UserID, c.Name, c.Type, c.FileSize, c.UploadStatus,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateCreativeStatus(id uint64, status, errMsg string) error {
	_, err := database.DB.Exec(
		"UPDATE creatives SET upload_status = ?, error_message = ? WHERE id = ?",
		status, errMsg, id)
	return err
}

func UpdateCreativeAfterUpload(id uint64, imageHash, videoID, thumbnailURL string, status string) error {
	_, err := database.DB.Exec(
		`UPDATE creatives SET meta_image_hash=?, meta_video_id=?, thumbnail_url=?, upload_status=?
		 WHERE id=?`,
		imageHash, videoID, thumbnailURL, status, id)
	return err
}

func DeleteCreative(id, userID uint64) error {
	_, err := database.DB.Exec(
		"DELETE FROM creatives WHERE id = ? AND user_id = ?", id, userID)
	return err
}
