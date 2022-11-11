// Package uncalled defines an Analyzer that checks for missing calls.
package uncalled

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"os"

	"github.com/rs/zerolog"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	name = "uncalled"
	doc  = `checks for missing calls

Checks if a method on packages which returns a specific types is
called without calling a specified method.

For example it can check for missing Rows.Err() calls when using
methods on database/sql types.

	rows, err := db.Query("select id from tb")
	if err != nil {
		// handle error
	}
	for rows.Next(){
		// handle rows
	}
	// (rows.Err() check should be here)

This helps uncover errors which will result in incomplete data if an
error is triggered while processing rows. This can happen when a
connection becomes invalid, this causes Rows.Next() to return
false without processing all rows.`
)

// Option represents an Analyzer option.
type Option func(*analyzer) error

// ConfigFile is an Analyzer option which loads and merges a config from file
// to our default config.
// Default: embedded config.
func ConfigFile(file string) Option {
	return func(a *analyzer) error {
		return a.cfg.loadFile(file)
	}
}

// ConfigOpt is an Analyzer option which merges in cfg to our default config.
// Default: embedded config.
func ConfigOpt(cfg *Config) Option {
	return func(a *analyzer) error {
		return a.cfg.merge(cfg)
	}
}

// LogLevel is an Analyzer option which configures its log level.
// Default: info.
func LogLevel(level string) Option {
	return func(a *analyzer) error {
		lvl, err := zerolog.ParseLevel(level)
		if err != nil {
			return fmt.Errorf("parse level %q: %w", level, err)
		}

		a.log = a.log.Level(lvl)

		return nil
	}
}

// testWriter is an Analyzer option which configures its log to use t
// and sets its log level to debug.
// Default: os.Stderr.
func testWriter(t zerolog.TestingLog) Option {
	return func(a *analyzer) error {
		a.log = a.log.Output(
			newConsoleWriter(zerolog.TestWriter{T: t, Frame: 6}),
		).Level(zerolog.DebugLevel)
		return nil
	}
}

// logger is an Analyzer option which sets its log.
// Default: console writer to os.Stderr.
func logger(log zerolog.Logger) Option {
	return func(a *analyzer) error {
		a.log = log
		return nil
	}
}

// NewAnalyzer returns a new Analyzer configured with options
// that checks for missing calls.
func NewAnalyzer(options ...Option) *analysis.Analyzer {
	l := &loader{
		options: options,
		log: log{
			Logger: zerolog.New(newConsoleWriter(os.Stderr)).
				Level(zerolog.InfoLevel).
				With().
				Timestamp().
				Logger(),
		},
	}

	a := &analysis.Analyzer{
		Name: name,
		Doc:  doc,
		Run:  l.run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}

	a.Flags.Init(a.Name, flag.ExitOnError)
	a.Flags.Var(l, "config", "configuration file to load")
	a.Flags.Var(version{}, "version", "print version and exit")
	a.Flags.Var(&l.log, "verbose", "increases the log level")

	return a
}

// analyzer checks for missing calls.
type analyzer struct {
	pass *analysis.Pass
	cfg  *Config
	log  zerolog.Logger
}

// newAnalyzer returns a new analyzer with options configured.
func newAnalyzer(options ...Option) (*analyzer, error) {
	cfg, err := loadDefaultConfig()
	if err != nil {
		return nil, err
	}

	a := &analyzer{cfg: cfg}
	for _, f := range options {
		if err := f(a); err != nil {
			return nil, err
		}
	}

	return a, nil
}

// buildConfig builds the configuration for imports and
// returns true if there are active rules, false otherwise.
func (a *analyzer) buildConfig(imports []*types.Package) bool {
	// Check if we import one of checked packages.
	paths := make(map[string]struct{}, len(imports))
	pathList := make([]string, len(imports))
	for i, imp := range imports {
		p := imp.Path()
		pathList[i] = p
		paths[p] = struct{}{}
	}

	rules := make([]Rule, 0, len(a.cfg.active))
	for _, rule := range a.cfg.active {
		rules = append(rules, rule)
	}

	active := make([]string, 0, len(a.cfg.active))
	for _, rule := range rules {
		j := 0
		for _, p := range rule.Packages {
			if _, ok := paths[p]; !ok {
				continue // Package wasn't imported.
			}
			rule.Packages[j] = p
			j++
		}
		rule.Packages = rule.Packages[:j]

		if len(rule.Packages) == 0 {
			a.log.Debug().
				Str("rule", rule.Name).
				Msg("skip no matching packages")
			delete(a.cfg.active, rule.Name)
			continue
		}
		active = append(active, rule.Name)
	}

	a.log.Debug().Strs("imports", pathList).Msg("imports")
	a.log.Debug().Strs("rules", active).Msg("active")
	a.log.Trace().Msgf("config\n%s", a.cfg.string())

	if len(a.cfg.active) == 0 {
		a.log.Print("skip no active rules")
		return false
	}

	return true
}

func (a *analyzer) run(pass *analysis.Pass) (interface{}, error) {
	// Check if we import one of checked packages.
	imports := pass.Pkg.Imports()
	pkgs := make(map[string]struct{}, len(imports))
	for _, imp := range imports {
		pkgs[imp.Path()] = struct{}{}
	}

	if !a.buildConfig(pass.Pkg.Imports()) {
		// No rules left so no need to check.
		return nil, nil //nolint: nilnil
	}

	a.pass = pass
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint: forcetypeassert
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, a.visit)

	return nil, nil //nolint: nilnil
}

// visit evaluates node to ensure checking each rule.
func (a *analyzer) visit(node ast.Node, push bool, stack []ast.Node) bool {
	a.log.Trace().Str("node", fmt.Sprintf("%#v", node)).Msg("visit")
	if !push {
		return true
	}

	call := node.(*ast.CallExpr) //nolint: forcetypeassert
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return true
	}

	typ, ok := a.pass.TypesInfo.Types[sel]
	if !ok {
		return true // Unknown type.
	}

	sig, ok := typ.Type.(*types.Signature)
	if !ok {
		return true // Call is not of the form x.f().
	}

	for _, rule := range a.cfg.active {
		a.checkRule(rule, call, sig, stack)
	}

	return true
}

// checkRule checks rule against given call.
func (a *analyzer) checkRule(rule Rule, call *ast.CallExpr, sig *types.Signature, stack []ast.Node) {
	match := rule.matchesResults(sig.Results())
	a.log.Debug().
		Str("rule", rule.Name).
		Stringer("sig", sig).
		Bool("match", match).
		Msg("matchesResults")
	if !match {
		return // Function call is not related to this rule.
	}

	// Find the innermost containing block, and get the list
	// of statements starting with the one containing call.
	stmts := restOfBlock(stack)
	stmt, ok := stmts[0].(*ast.AssignStmt)
	if !ok {
		// First statement is not assignment so not called.
		a.log.Debug().Msg("return not assigned")
		a.report(call, rule, "")
		return
	}

	node := stmt.Lhs[rule.expects.idx]
	ident := rootIdent(node)
	if ident == nil {
		a.log.Error().Msgf("node %#v: nil root", node)
		return // Not matching.
	}

	if len(stmts) < 2 {
		// Call to the sql function is the last statement of the block.
		a.log.Debug().Msg("no statements")
		a.report(ident, rule, ident.Name)
		return
	}

	// TODO(steve): avoid multiple passes.
	for _, rule := range a.cfg.active {
		if visit(a.pass, a.log, rule, ident, stmts[1:]) {
			continue
		}
		a.report(ident, rule, ident.Name)
	}
}

// report reports a missing call for rule at rng for variable name.
func (a *analyzer) report(rng analysis.Range, rule Rule, name string) {
	name = rule.name(name)
	a.log.Debug().
		Str("rule", rule.Name).
		Str("name", name).
		Msg("not called")
	a.pass.Report(analysis.Diagnostic{
		Pos:      rng.Pos(),
		End:      rng.End(),
		Category: rule.Category,
		Message:  fmt.Sprintf("%s must be called", name),
	})
}
