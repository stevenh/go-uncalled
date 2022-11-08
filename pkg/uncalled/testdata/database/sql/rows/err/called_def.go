package uncalled_test

import (
	"database/sql"
)

func CalledDefer(db *sql.DB) {
	rows, _ := db.Query("select id from tb")
	defer func() {
		_ = rows.Err()
	}()
}
