package a

import (
	"database/sql"
)

func CheckedInlineFunc(db *sql.DB) {
	_ = func(db *sql.DB) {
		rows, _ := db.Query("") // OK
		_ = rows.Err()
	}
}
