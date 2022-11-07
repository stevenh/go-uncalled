//go:build plugin

// Command uncalled is a plugin which checks for missing calls.
package main

import (
	"github.com/stevenh/go-uncalled/pkg/uncalled"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct{}

// GetAnalyzers returns uncalled Analyzer.
func (analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		uncalled.NewAnalyzer(),
	}
}

// AnalyzerPlugin is an Analyzer plugin.
var AnalyzerPlugin analyzerPlugin
