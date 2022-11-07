package a

import (
	"database/sql"
	"fmt"
	"io"
)

func NotCheckedVars(db *sql.DB) {
	rows, err := db.Query("") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
	}
	rowsX := &sql.Rows{}
	_ = rowsX.Err()
	if err != nil {
		// handle error
		fmt.Fprint(io.Discard, err)
	}
}
