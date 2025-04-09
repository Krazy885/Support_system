package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken  string
	DBHost    string
	DBUser    string
	DBPass    string
	DBName    string
	AdminID   int64
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	adminID, err := strconv.ParseInt(os.Getenv("ADMIN_ID"), 10, 64)
	if err != nil {
		return nil, err
	}

	return &Config{
		BotToken: os.Getenv("BOT_TOKEN"),
		DBHost:   os.Getenv("DB_HOST"),
		DBUser:   os.Getenv("DB_USER"),
		DBPass:   os.Getenv("DB_PASS"),
		DBName:   os.Getenv("DB_NAME"),
		AdminID:  adminID,
	}, nil
}