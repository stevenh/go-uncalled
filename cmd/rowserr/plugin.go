//go:build plugin

// Command rowserr is a plugin which checks for missing database/sql.Rows.Err() calls.
package main

import (
	"github.com/stevenh/go-rowserr/pkg/rowserr"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct{}

// GetAnalyzers returns rowserr Analyzer.
func (analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		rowserr.Analyzer,
	}
}

// AnalyzerPlugin is an Analyzer plugin.
var AnalyzerPlugin analyzerPlugin
