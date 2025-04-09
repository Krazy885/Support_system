package services

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"support_bot/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	STATE_NONE          = ""
	STATE_AWAITING_MSG  = "awaiting_message"
	STATE_AWAITING_RESP = "awaiting_response"
)

type UserState struct {
	State     string
	TicketID  int
	MessageID int
}

var userStates = make(map[int64]UserState)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, adminID int64) {
	if update.Message != nil {
		handleMessage(bot, update.Message, db, adminID)
	} else if update.CallbackQuery != nil {
		handleCallbackQuery(bot, update.CallbackQuery, db, adminID)
	}
}

func handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, db *sql.DB, adminID int64) {
	userID := msg.From.ID
	state, exists := userStates[userID]

	switch {
	case msg.IsCommand():
		switch msg.Command() {
		case "start":
			sendWelcomeMessage(bot, msg.Chat.ID)
		case "status":
			sendTicketStatus(bot, msg.Chat.ID, userID, db)
		default:
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "❌ Неизвестная команда. Используйте /start или /status!"))
		}

	case userID == adminID && exists && state.State == STATE_AWAITING_RESP:
		saveAdminResponse(bot, db, adminID, state.TicketID, msg.Text, msg.Chat.ID)
		delete(userStates, userID)

	case exists && state.State == STATE_AWAITING_MSG:
		saveTicket(bot, db, userID, msg.From.UserName, state.TicketID, msg.Text, adminID, msg.Chat.ID)
		delete(userStates, userID)

	default:
		startTicketCreation(bot, msg.Chat.ID, userID)
	}
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, db *sql.DB, adminID int64) {
	userID := callback.From.ID
	data := callback.Data
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	log.Printf("Received callback data: %q from userID: %d", data, userID)

	if data == "cancel" {
		delete(userStates, userID)
		bot.Send(tgbotapi.NewMessage(chatID, "🚫 Создание тикета отменено."))
		return
	}

	if callback.From.ID == adminID && strings.HasPrefix(data, "respond_:") {
		ticketID, err := strconv.Atoi(data[9:])
		if err != nil {
			log.Printf("Invalid ticket ID in respond callback: %s", data)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: неверный ID тикета."))
			return
		}
		userStates[adminID] = UserState{State: STATE_AWAITING_RESP, TicketID: ticketID}
		bot.Send(tgbotapi.NewMessage(chatID, "📝 Введите ответ на тикет:"))
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, callback.Message.Text)
		editMsg.ReplyMarkup = nil
		_, err = bot.Send(editMsg)
		if err != nil {
			log.Printf("Failed to edit message: %v", err)
		}
		return
	}

	if strings.HasPrefix(data, "reply_:") {
		ticketID, err := strconv.Atoi(data[7:])
		if err != nil {
			log.Printf("Invalid ticket ID in reply callback: %s", data)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: неверный ID тикета."))
			return
		}
		userStates[userID] = UserState{State: STATE_AWAITING_MSG, TicketID: ticketID}
		bot.Send(tgbotapi.NewMessage(chatID, "💬 Введите ваше сообщение для техподдержки:"))
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, callback.Message.Text)
		editMsg.ReplyMarkup = nil
		_, err = bot.Send(editMsg)
		if err != nil {
			log.Printf("Failed to edit message: %v", err)
		}
		return
	}

	if strings.HasPrefix(data, "close_:") {
		ticketID, err := strconv.Atoi(data[7:])
		if err != nil {
			log.Printf("Invalid ticket ID in close callback: %s", data)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: неверный ID тикета."))
			return
		}
		closeTicket(bot, db, ticketID, chatID, userID, adminID)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, callback.Message.Text)
		editMsg.ReplyMarkup = nil
		_, err = bot.Send(editMsg)
		if err != nil {
			log.Printf("Failed to edit message: %v", err)
		}
		return
	}

	validCategories := map[string]bool{
		"tech":    true,
		"billing": true,
		"general": true,
		"other":   true,
	}
	if !validCategories[data] {
		log.Printf("Invalid category received: %s", data)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: неверная категория или действие не поддерживается."))
		return
	}

	ticketID, err := createTicket(db, userID, callback.From.UserName, data)
	if err != nil {
		log.Printf("Failed to create ticket: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при создании тикета."))
		return
	}

	state, exists := userStates[userID]
	if exists && state.MessageID != 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, state.MessageID, "✅ Категория выбрана. Опишите вашу проблему:")
		editMsg.ReplyMarkup = nil
		_, err := bot.Send(editMsg)
		if err != nil {
			log.Printf("Failed to edit message: %v", err)
		}
	}

	userStates[userID] = UserState{State: STATE_AWAITING_MSG, TicketID: ticketID, MessageID: 0}
	bot.Send(tgbotapi.NewMessage(chatID, "📝 Опишите вашу проблему:"))
}

func sendWelcomeMessage(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "👋 Добро пожаловать в службу поддержки! Отправьте сообщение, чтобы создать тикет ✉️")
	bot.Send(msg)
}

func startTicketCreation(bot *tgbotapi.BotAPI, chatID, userID int64) {
	msg := tgbotapi.NewMessage(chatID, "📋 Выберите категорию вашего запроса:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔧 Техническая", "tech"),
			tgbotapi.NewInlineKeyboardButtonData("💰 Биллинг", "billing"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Общее", "general"),
			tgbotapi.NewInlineKeyboardButtonData("❓ Другое", "other"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚫 Отмена", "cancel"),
		),
	)
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Failed to send category selection message: %v", err)
		return
	}
	userStates[userID] = UserState{State: STATE_NONE, TicketID: 0, MessageID: sentMsg.MessageID}
}

func createTicket(db *sql.DB, userID int64, username, category string) (int, error) {
	query := "INSERT INTO tickets (user_id, username, category) VALUES (?, ?, ?)"
	result, err := db.Exec(query, userID, username, category)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return int(id), nil
}

func saveTicket(bot *tgbotapi.BotAPI, db *sql.DB, userID int64, username string, ticketID int, message string, adminID, chatID int64) {
	var currentMessage sql.NullString
	err := db.QueryRow("SELECT message FROM tickets WHERE id = ? AND user_id = ?", ticketID, userID).Scan(&currentMessage)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to get current ticket message: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при обновлении тикета."))
		return
	}

	updatedMessage := message
	if currentMessage.Valid {
		updatedMessage = fmt.Sprintf("%s\n---\n%s", currentMessage.String, message)
	}

	query := "UPDATE tickets SET message = ?, status = 'open', response = NULL, updated_at = NOW() WHERE id = ? AND user_id = ?"
	_, err = db.Exec(query, updatedMessage, ticketID, userID)
	if err != nil {
		log.Printf("Failed to save ticket message: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при сохранении тикета."))
		return
	}

	userMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Тикет #%d обновлен! Мы скоро ответим вам ✉️", ticketID))
	userMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✔️ Проблема решена!", fmt.Sprintf("close_:%d", ticketID)),
		),
	)
	sentUserMsg, err := bot.Send(userMsg)
	if err != nil {
		log.Printf("Failed to send user message: %v", err)
		return
	}

	adminMsg := tgbotapi.NewMessage(adminID, fmt.Sprintf(
		"✨ Обновление тикета #%d\n👤 Пользователь: @%s\n📋 Категория: %s\n💬 Сообщение: %s",
		ticketID, username, getCategoryName(queryCategory(db, ticketID)), updatedMessage,
	))
	adminMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Ответить", fmt.Sprintf("respond_:%d", ticketID)),
			tgbotapi.NewInlineKeyboardButtonData("✔️ Проблема решена!", fmt.Sprintf("close_:%d", ticketID)),
		),
	)
	sentAdminMsg, err := bot.Send(adminMsg)
	if err != nil {
		log.Printf("Failed to send admin message: %v", err)
		return
	}

	userStates[userID] = UserState{State: STATE_NONE, TicketID: ticketID, MessageID: sentUserMsg.MessageID}
	userStates[adminID] = UserState{State: STATE_NONE, TicketID: ticketID, MessageID: sentAdminMsg.MessageID}
}

func saveAdminResponse(bot *tgbotapi.BotAPI, db *sql.DB, adminID int64, ticketID int, response string, chatID int64) {
	var userID int64
	var currentMessage sql.NullString
	err := db.QueryRow("SELECT user_id, message FROM tickets WHERE id = ?", ticketID).Scan(&userID, &currentMessage)
	if err != nil {
		log.Printf("Failed to get ticket user or message: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: тикет не найден."))
		return
	}

	updatedMessage := response
	if currentMessage.Valid {
		updatedMessage = fmt.Sprintf("%s\n---\n%s", currentMessage.String, response)
	}

	query := "UPDATE tickets SET message = ?, response = ?, status = 'answered', updated_at = NOW() WHERE id = ?"
	_, err = db.Exec(query, updatedMessage, response, ticketID)
	if err != nil {
		log.Printf("Failed to save response: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при сохранении ответа."))
		return
	}

	bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("📤 Ответ на тикет #%d отправлен! ✅", ticketID)))

	userMsg := tgbotapi.NewMessage(userID, fmt.Sprintf(
		"✨ Ответ на ваш тикет #%d:\n\n💬 %s", ticketID, response,
	))
	userMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Ответить", fmt.Sprintf("reply_:%d", ticketID)),
			tgbotapi.NewInlineKeyboardButtonData("✔️ Проблема решена!", fmt.Sprintf("close_:%d", ticketID)),
		),
	)
	sentUserMsg, err := bot.Send(userMsg)
	if err != nil {
		log.Printf("Failed to send user message: %v", err)
		return
	}

	userStates[userID] = UserState{State: STATE_NONE, TicketID: ticketID, MessageID: sentUserMsg.MessageID}
}

func closeTicket(bot *tgbotapi.BotAPI, db *sql.DB, ticketID int, chatID, userID, adminID int64) {
	var ticketUserID int64
	err := db.QueryRow("SELECT user_id FROM tickets WHERE id = ?", ticketID).Scan(&ticketUserID)
	if err != nil {
		log.Printf("Failed to get ticket user: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: тикет не найден."))
		return
	}

	query := "UPDATE tickets SET status = 'closed', updated_at = NOW() WHERE id = ?"
	_, err = db.Exec(query, ticketID)
	if err != nil {
		log.Printf("Failed to close ticket: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при закрытии тикета."))
		return
	}

	bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Тикет #%d закрыт! 🎉", ticketID)))

	notifyID := adminID
	if userID == adminID {
		notifyID = ticketUserID
	}
	notifyMsg := tgbotapi.NewMessage(notifyID, fmt.Sprintf("ℹ️ Тикет #%d был закрыт.", ticketID))
	bot.Send(notifyMsg)
}

func sendTicketStatus(bot *tgbotapi.BotAPI, chatID, userID int64, db *sql.DB) {
	log.Printf("Checking status for userID: %d", userID)
	rows, err := db.Query("SELECT id, category, message, response, status, created_at FROM tickets WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		log.Printf("Failed to query tickets: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при получении статуса тикетов."))
		return
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		err := rows.Scan(&t.ID, &t.Category, &t.Message, &t.Response, &t.Status, &t.CreatedAt)
		if err != nil {
			log.Printf("Failed to scan ticket: %v", err)
			continue
		}
		tickets = append(tickets, t)
	}

	log.Printf("Found %d tickets for userID %d", len(tickets), userID)
	if len(tickets) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "ℹ️ У вас нет тикетов."))
		return
	}

	for _, t := range tickets {
		statusMsg := fmt.Sprintf(
			"📋 Тикет #%d\n📌 Категория: %s\n💬 Сообщение: %s\n📈 Статус: %s\n🕒 Создан: %s",
			t.ID, getCategoryName(t.Category), t.Message.String, getStatusName(t.Status), t.CreatedAt,
		)
		if t.Response.Valid {
			statusMsg += fmt.Sprintf("\n📩 Последний ответ: %s", t.Response.String)
		}
		bot.Send(tgbotapi.NewMessage(chatID, statusMsg))
	}
}

func getCategoryName(category string) string {
	switch category {
	case "tech":
		return "🔧 Техническая"
	case "billing":
		return "💰 Биллинг"
	case "general":
		return "ℹ️ Общее"
	case "other":
		return "❓ Другое"
	default:
		return "❔ Неизвестно"
	}
}

func getStatusName(status string) string {
	switch status {
	case "open":
		return "🔓 Открыт"
	case "answered":
		return "📩 Отвечен"
	case "closed":
		return "🔒 Закрыт"
	default:
		return "❔ Неизвестно"
	}
}

func queryCategory(db *sql.DB, ticketID int) string {
	var category string
	err := db.QueryRow("SELECT category FROM tickets WHERE id = ?", ticketID).Scan(&category)
	if err != nil {
		log.Printf("Failed to query category: %v", err)
		return "Неизвестно"
	}
	return category
}