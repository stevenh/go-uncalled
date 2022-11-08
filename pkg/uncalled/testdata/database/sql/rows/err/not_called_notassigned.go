package uncalled_test

import (
	"database/sql"
)

func NotAssigned(db *sql.DB) {
	db.Query("") // want "Rows.Err\\(\\) must be called"
}

func NotAssignedRows(db *sql.DB) {
	_, err := db.Query("") // want "_.Err\\(\\) must be called"
	if err != nil {
		// handle error
	}
}
