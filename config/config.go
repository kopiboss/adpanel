package config

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort    string
	AppSecret  string
	AppURL     string

	AdminEmail    string
	AdminPassword string

	DBHost string
	DBPort string
	DBUser string
	DBPass string
	DBName string

	EncryptionKey []byte

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	TelegramBotToken string
	TelegramChatID   string
}

var App *Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	encKeyHex := getEnv("ENCRYPTION_KEY", "")
	encKey, err := hex.DecodeString(encKeyHex)
	if err != nil || len(encKey) != 32 {
		log.Fatal("ENCRYPTION_KEY must be a valid 64-character hex string (32 bytes)")
	}

	App = &Config{
		AppPort:   getEnv("APP_PORT", "8080"),
		AppSecret: getEnv("APP_SECRET", "default_secret_change_me"),
		AppURL:    getEnv("APP_URL", "http://localhost:8080"),

		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@adpanel.com"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),

		DBHost: getEnv("DB_HOST", "localhost"),
		DBPort: getEnv("DB_PORT", "3306"),
		DBUser: getEnv("DB_USER", "adpanel"),
		DBPass: getEnv("DB_PASS", ""),
		DBName: getEnv("DB_NAME", "adpanel"),

		EncryptionKey: encKey,

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
	}

	if App.AdminPassword == "" {
		log.Fatal("ADMIN_PASSWORD must be set in environment")
	}
}

func (c *Config) DSN() string {
	port, _ := strconv.Atoi(c.DBPort)
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		c.DBUser, c.DBPass, c.DBHost, port, c.DBName)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
