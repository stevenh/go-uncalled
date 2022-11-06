package a

import (
	"database/sql"
)

func CheckedFunc(db *sql.DB) {
	rows, _ := db.Query("")
	resCloser, n := func(rs *sql.Rows, other int) {
		_ = rs.Err()
	}, 1
	resCloser(rows, n)
}
