package services

import (
	"log"
	"support_bot/config"
	"support_bot/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func Run(cfg *config.Config) {
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	dbConn, err := db.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Запускаем очистку старых тикетов в фоновом режиме
	go CleanupOldTickets(dbConn)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		HandleUpdate(bot, update, dbConn, cfg.AdminID)
	}
}