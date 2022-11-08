package uncalled_test

import (
	"database/sql"
)

func CalledFunc(db *sql.DB) {
	rows, _ := db.Query("")
	resCloser, n := func(rs *sql.Rows, other int) {
		_ = rs.Err()
	}, 1
	resCloser(rows, n)
}
