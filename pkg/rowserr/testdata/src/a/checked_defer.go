package a

import (
	"database/sql"
)

func CheckedDefer(db *sql.DB) {
	rows, _ := db.Query("select id from tb")
	defer func() {
		_ = rows.Err()
	}()
}
