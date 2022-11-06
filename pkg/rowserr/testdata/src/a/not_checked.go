package a

import (
	"database/sql"
	"fmt"
	"io/ioutil"
)

func RowsErrNotChecked(db *sql.DB) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be checked"
	for rows.Next() {
		// Handle row.
		fmt.Fprintln(ioutil.Discard, "error")
	}
}
