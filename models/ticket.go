package models

import "database/sql"

type Ticket struct {
	ID        int
	UserID    int64
	Username  string
	Category  string
	Message   sql.NullString
	Response  sql.NullString
	Status    string
	CreatedAt string
}