package models

import (
	"time"

	"adpanel/database"
)

type User struct {
	ID             uint64     `db:"id" json:"id"`
	Name           string     `db:"name" json:"name"`
	Email          string     `db:"email" json:"email"`
	PasswordHash   string     `db:"password_hash" json:"-"`
	GoogleID       string     `db:"google_id" json:"google_id"`
	Role           string     `db:"role" json:"role"`
	Status         string     `db:"status" json:"status"`
	TelegramChatID string     `db:"telegram_chat_id" json:"telegram_chat_id"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

func GetUserByID(id uint64) (*User, error) {
	var u User
	err := database.DB.Get(&u, "SELECT * FROM users WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUserByEmail(email string) (*User, error) {
	var u User
	err := database.DB.Get(&u, "SELECT * FROM users WHERE email = ?", email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUserByGoogleID(googleID string) (*User, error) {
	var u User
	err := database.DB.Get(&u, "SELECT * FROM users WHERE google_id = ?", googleID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func CreateUser(u *User) (uint64, error) {
	res, err := database.DB.Exec(
		`INSERT INTO users (name, email, password_hash, google_id, role, status, telegram_chat_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Name, u.Email, u.PasswordHash, u.GoogleID, u.Role, u.Status, u.TelegramChatID,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func UpdateUserStatus(id uint64, status string) error {
	_, err := database.DB.Exec("UPDATE users SET status = ? WHERE id = ?", status, id)
	return err
}

func UpdateUserProfile(id uint64, name, telegramChatID string) error {
	_, err := database.DB.Exec(
		"UPDATE users SET name = ?, telegram_chat_id = ? WHERE id = ?",
		name, telegramChatID, id,
	)
	return err
}

func UpdateUserPassword(id uint64, hash string) error {
	_, err := database.DB.Exec("UPDATE users SET password_hash = ? WHERE id = ?", hash, id)
	return err
}

func UpdateUserGoogleID(id uint64, googleID string) error {
	_, err := database.DB.Exec("UPDATE users SET google_id = ? WHERE id = ?", googleID, id)
	return err
}

func ListAllUsers() ([]User, error) {
	var users []User
	err := database.DB.Select(&users, "SELECT * FROM users ORDER BY created_at DESC")
	return users, err
}

func ListPendingUsers() ([]User, error) {
	var users []User
	err := database.DB.Select(&users, "SELECT * FROM users WHERE status = 'pending' ORDER BY created_at DESC")
	return users, err
}

func DeleteUser(id uint64) error {
	_, err := database.DB.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func CountUsers() (int, error) {
	var count int
	err := database.DB.Get(&count, "SELECT COUNT(*) FROM users")
	return count, err
}

func CountUsersByStatus(status string) (int, error) {
	var count int
	err := database.DB.Get(&count, "SELECT COUNT(*) FROM users WHERE status = ?", status)
	return count, err
}
