package services

import (
	"database/sql"
	"log"
	"time"
)

// CleanupOldTickets удаляет тикеты, созданные более недели назад
func CleanupOldTickets(db *sql.DB) {
	for {
		// Удаляем тикеты, у которых created_at старше 7 дней
		query := "DELETE FROM tickets WHERE created_at < NOW() - INTERVAL 7 DAY"
		result, err := db.Exec(query)
		if err != nil {
			log.Printf("Failed to cleanup old tickets: %v", err)
		} else {
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected > 0 {
				log.Printf("Deleted %d old tickets", rowsAffected)
			}
		}
		// Проверяем каждые 24 часа
		time.Sleep(24 * time.Hour)
	}
}