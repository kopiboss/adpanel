package models

import (
	"time"

	"adpanel/database"
)

type Credential struct {
	ID              uint64    `db:"id" json:"id"`
	UserID          uint64    `db:"user_id" json:"user_id"`
	Label           string    `db:"label" json:"label"`
	AppID           string    `db:"app_id" json:"app_id"`
	AppSecretEnc    string    `db:"app_secret_enc" json:"-"`
	AccessTokenEnc  string    `db:"access_token_enc" json:"-"`
	TokenStatus     string    `db:"token_status" json:"token_status"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`

	// Populated at runtime, not in DB
	AppSecret   string `db:"-" json:"-"`
	AccessToken string `db:"-" json:"-"`
}

func GetCredentialByID(id, userID uint64) (*Credential, error) {
	var c Credential
	err := database.DB.Get(&c,
		"SELECT * FROM credentials WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func GetCredentialByIDAdmin(id uint64) (*Credential, error) {
	var c Credential
	err := database.DB.Get(&c, "SELECT * FROM credentials WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func ListCredentialsByUser(userID uint64) ([]Credential, error) {
	var creds []Credential
	err := database.DB.Select(&creds,
		"SELECT * FROM credentials WHERE user_id = ? ORDER BY created_at DESC", userID)
	return creds, err
}

func CreateCredential(c *Credential) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO credentials (user_id, label, app_id, app_secret_enc, access_token_enc, token_status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.UserID, c.Label, c.AppID, c.AppSecretEnc, c.AccessTokenEnc, c.TokenStatus,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateCredential(c *Credential) error {
	_, err := database.DB.Exec(
		`UPDATE credentials SET label=?, app_id=?, app_secret_enc=?, access_token_enc=?, token_status=?
		 WHERE id=? AND user_id=?`,
		c.Label, c.AppID, c.AppSecretEnc, c.AccessTokenEnc, c.TokenStatus, c.ID, c.UserID,
	)
	return err
}

func UpdateCredentialTokenStatus(id uint64, status string) error {
	_, err := database.DB.Exec(
		"UPDATE credentials SET token_status = ? WHERE id = ?", status, id)
	return err
}

func DeleteCredential(id, userID uint64) error {
	_, err := database.DB.Exec(
		"DELETE FROM credentials WHERE id = ? AND user_id = ?", id, userID)
	return err
}
