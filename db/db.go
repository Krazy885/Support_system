package db

import (
	"database/sql"
	"fmt"
	"support_bot/config"

	_ "github.com/go-sql-driver/mysql"
)

func Init(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT,
			username VARCHAR(255),
			category ENUM('tech', 'billing', 'general', 'other') DEFAULT 'other',
			message TEXT,
			response TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			status ENUM('open', 'answered', 'closed') DEFAULT 'open'
		)
	`)
	if err != nil {
		return nil, err
	}

	return db, nil
}