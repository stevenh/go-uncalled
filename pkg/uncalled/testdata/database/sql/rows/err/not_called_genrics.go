//go:build go1.18

package uncalled_test

import (
	"database/sql"
)

var _ = NotCalledGeneric[int64]

func NotCalledGeneric[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
	}
}

func NotCalledGenericDefer[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
	}
	defer func() {
		_ = rows.Close()
	}()
}
