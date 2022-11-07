//go:build go1.18

package a

import (
	"database/sql"
	"fmt"
	"io/ioutil"
)

var _ = CheckedGeneric[int64]

func CheckedGeneric[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	if rows.Err() != nil {
		// Handle error.
		fmt.Fprintln(ioutil.Discard, "error")
	}
}

func CheckedGenericAssign[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	if err := rows.Err(); err != nil {
		// Handle error.
		fmt.Fprintln(ioutil.Discard, err)
	}
}

func CheckedGenericDefer[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	defer func() {
		if err := rows.Err(); err != nil {
			// Handle error.
			fmt.Fprintln(ioutil.Discard, err)
		}
	}()
}
