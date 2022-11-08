package uncalled_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
)

func CalledAssign(db *sql.DB) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	if err := rows.Err(); err != nil {
		// Handle error.
		fmt.Fprintln(ioutil.Discard, err)
	}
}
