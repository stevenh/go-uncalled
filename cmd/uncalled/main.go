//go:build !plugin

// Command uncalled checks for missing calls.
package main

import (
	"github.com/stevenh/go-uncalled/pkg/uncalled"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(uncalled.NewAnalyzer())
}
