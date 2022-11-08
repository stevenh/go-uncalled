package uncalled_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
)

func RowsErrNotCalled(db *sql.DB) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
		fmt.Fprintln(ioutil.Discard, "error")
	}
}
