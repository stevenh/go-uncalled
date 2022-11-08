package uncalled_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
)

func Called(db *sql.DB) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	if rows.Err() != nil {
		// Handle error.
		fmt.Fprintln(ioutil.Discard, "error")
	}
}
