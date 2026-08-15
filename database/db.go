package database

import (
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"adpanel/config"
)

var DB *sqlx.DB

func Connect() {
	var err error
	DB, err = sqlx.Connect("mysql", config.App.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * time.Minute)
	DB.SetConnMaxIdleTime(2 * time.Minute)

	log.Println("Database connected successfully")
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
