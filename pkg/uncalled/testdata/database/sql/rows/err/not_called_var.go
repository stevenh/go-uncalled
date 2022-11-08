package uncalled_test

import (
	"database/sql"
	"fmt"
	"io"
)

func RowsErrNotCalledVar(db *sql.DB) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
	}

	rows2, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}

	if err := rows2.Err(); err != nil {
		// Handle Error.
		fmt.Fprint(io.Discard, err)
	}
}
