package a

import (
	"database/sql"
)

func NotAssigned(db *sql.DB) {
	db.Query("") // want "rows.Err\\(\\) must be checked"
}

func NotAssignedRows(db *sql.DB) {
	_, err := db.Query("") // want "_.Err\\(\\) must be checked"
	if err != nil {
		// handle error
	}
}
