package postgres

import (
	"database/sql"
)

type Account struct {
	ID        string
	Email     sql.NullString
	Password  sql.NullString
	CreatedAt sql.NullTime
	Platform  sql.NullInt16
	UpdatedAt sql.NullTime
}
