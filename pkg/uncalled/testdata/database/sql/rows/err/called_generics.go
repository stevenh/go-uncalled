//go:build go1.18

package uncalled_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
)

var _ = CalledGeneric[int64]

func CalledGeneric[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	if rows.Err() != nil {
		// Handle error.
		fmt.Fprintln(ioutil.Discard, "error")
	}
}

func CalledGenericAssign[T ~int64](db *sql.DB, a T) {
	rows, _ := db.Query("select id from tb")
	for rows.Next() {
		// Handle row.
	}
	if err := rows.Err(); err != nil {
		// Handle error.
		fmt.Fprintln(ioutil.Discard, err)
	}
}

func CalledGenericDefer[T ~int64](db *sql.DB, a T) {
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
