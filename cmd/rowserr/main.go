//go:build !plugin

// Command rowserr checks for missing database/sql.Rows.Err() calls.
package main

import (
	"github.com/stevenh/go-rowserr/pkg/rowserr"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(rowserr.NewAnalyzer())
}
