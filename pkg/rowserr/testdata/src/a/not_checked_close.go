package a

import (
	"database/sql"
)

func RowsErrNotCheckClose(db *sql.DB) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be checked"
	for rows.Next() {
		// Handle row.
	}
	defer func() {
		_ = rows.Close()
	}()
}
