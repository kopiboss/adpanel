package models

import (
	"time"

	"adpanel/database"
)

type AdAccount struct {
	ID            uint64    `db:"id" json:"id"`
	CredentialID  uint64    `db:"credential_id" json:"credential_id"`
	UserID        uint64    `db:"user_id" json:"user_id"`
	MetaAccountID string    `db:"meta_account_id" json:"meta_account_id"`
	Name          string    `db:"name" json:"name"`
	Currency      string    `db:"currency" json:"currency"`
	Timezone      string    `db:"timezone" json:"timezone"`
	AccountStatus int       `db:"account_status" json:"account_status"`
	IsActive      bool      `db:"is_active" json:"is_active"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`

	// Joined fields
	CredentialLabel string `db:"credential_label" json:"credential_label,omitempty"`
}

func GetAdAccountByID(id, userID uint64) (*AdAccount, error) {
	var a AdAccount
	err := database.DB.Get(&a,
		"SELECT * FROM ad_accounts WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func GetAdAccountByMetaID(metaAccountID string, userID uint64) (*AdAccount, error) {
	var a AdAccount
	err := database.DB.Get(&a,
		"SELECT * FROM ad_accounts WHERE meta_account_id = ? AND user_id = ?",
		metaAccountID, userID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func ListAdAccountsByUser(userID uint64) ([]AdAccount, error) {
	var accounts []AdAccount
	err := database.DB.Select(&accounts,
		`SELECT aa.*, c.label AS credential_label
		 FROM ad_accounts aa
		 JOIN credentials c ON aa.credential_id = c.id
		 WHERE aa.user_id = ?
		 ORDER BY aa.created_at DESC`, userID)
	return accounts, err
}

func ListActiveAdAccountsByUser(userID uint64) ([]AdAccount, error) {
	var accounts []AdAccount
	err := database.DB.Select(&accounts,
		"SELECT * FROM ad_accounts WHERE user_id = ? AND is_active = 1 ORDER BY name ASC",
		userID)
	return accounts, err
}

func ListAllActiveAdAccounts() ([]AdAccount, error) {
	var accounts []AdAccount
	err := database.DB.Select(&accounts,
		"SELECT * FROM ad_accounts WHERE is_active = 1 ORDER BY user_id ASC")
	return accounts, err
}

func ListAdAccountsByCredential(credentialID, userID uint64) ([]AdAccount, error) {
	var accounts []AdAccount
	err := database.DB.Select(&accounts,
		"SELECT * FROM ad_accounts WHERE credential_id = ? AND user_id = ?",
		credentialID, userID)
	return accounts, err
}

func CreateAdAccount(a *AdAccount) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO ad_accounts (credential_id, user_id, meta_account_id, name, currency, timezone, account_status, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE name=VALUES(name), currency=VALUES(currency),
		 timezone=VALUES(timezone), account_status=VALUES(account_status)`,
		a.CredentialID, a.UserID, a.MetaAccountID, a.Name, a.Currency,
		a.Timezone, a.AccountStatus, a.IsActive,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateAdAccountActive(id, userID uint64, isActive bool) error {
	_, err := database.DB.Exec(
		"UPDATE ad_accounts SET is_active = ? WHERE id = ? AND user_id = ?",
		isActive, id, userID)
	return err
}

func DeleteAdAccount(id, userID uint64) error {
	_, err := database.DB.Exec(
		"DELETE FROM ad_accounts WHERE id = ? AND user_id = ?", id, userID)
	return err
}

func CountAdAccountsByUser(userID uint64) (int, error) {
	var count int
	err := database.DB.Get(&count,
		"SELECT COUNT(*) FROM ad_accounts WHERE user_id = ?", userID)
	return count, err
}
