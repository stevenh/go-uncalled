//go:build go1.18

package a

import (
	"database/sql"
)

var _ = NotCheckedGeneric[int64]

func NotCheckedGeneric[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
	}
}

func NotCheckedGenericDefer[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb") // want "rows.Err\\(\\) must be called"
	for rows.Next() {
		// Handle row.
	}
	defer func() {
		_ = rows.Close()
	}()
}
