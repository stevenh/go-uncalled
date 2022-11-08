package uncalled_test

import (
	"database/sql"
)

func CalledInlineFunc(db *sql.DB) {
	_ = func(db *sql.DB) {
		rows, _ := db.Query("") // OK
		_ = rows.Err()
	}
}
