package a

import (
	"database/sql"
	"fmt"
	"io"
)

func CheckedVars(db *sql.DB) {
	rows, err := db.Query("") // OK
	if err != nil {
		// handle error
		fmt.Fprint(io.Discard, err)
	}

	rows1 := rows
	rows2 := rows1
	_ = rows2.Err()

	rows3, err := db.Query("") // OK
	rowsX3 := rows3
	_ = rowsX3.Err()
	if err != nil {
		// handle error
		fmt.Fprint(io.Discard, err)
	}

	rows3, err = db.Query("") // want "rows3.Err\\(\\) must be called"
	_ = rowsX3.Err()
	if err != nil {
		// handle error
		fmt.Fprint(io.Discard, err)
	}
}
