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
	"gopkg.in/yaml.v3"
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

// ConfigFile is an Analyzer option which loads its config from file.
// Default: embedded config.
func ConfigFile(file string) Option {
	return func(a *analyzer) error {
		return a.cfg.load(file)
	}
}

// ConfigOpt is an Analyzer option which specifies the config to use.
// Default: embedded config.
func ConfigOpt(cfg *Config) Option {
	return func(a *analyzer) error {
		// Take a copy so we don't alter the original.
		cpy := *cfg
		a.cfg = &cpy
		return nil
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

// TestWriter is an Analyzer option which configures its
// log to use t.
// Default: os.Stderr.
func TestWriter(t zerolog.TestingLog) Option {
	return func(a *analyzer) error {
		a.log = a.log.Output(zerolog.TestWriter{T: t, Frame: 6})
		return nil
	}
}

// NewAnalyzer returns a new Analyzer configured with options
// that checks for missing calls.
func NewAnalyzer(options ...Option) *analysis.Analyzer {
	l := &loader{
		options: options,
		log: log{
			Logger: zerolog.New(
				zerolog.ConsoleWriter{
					Out: os.Stderr,
					FormatTimestamp: func(any) string {
						return ""
					},
				},
			).Level(zerolog.InfoLevel).
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
	pass    *analysis.Pass
	options []Option
	cfg     *Config
	log     zerolog.Logger
}

// init initialises r to check packages.
func (a *analyzer) init(imports map[string]struct{}) error {
	if a.cfg == nil {
		// No config specified use default.
		a.log.Debug().Msg("load default config")
		a.cfg = &Config{}
		if err := yaml.Unmarshal(defaultConfig, a.cfg); err != nil {
			return fmt.Errorf("decode default config: %w", err)
		}
	}

	if err := a.buildConfig(imports); err != nil {
		return err
	}

	if a.log.GetLevel() <= zerolog.DebugLevel {
		a.log.Debug().Strs("imports", func() []string {
			s := make([]string, 0, len(imports))
			for k := range imports {
				s = append(s, k)
			}
			return s
		}()).Msg("imports")

		b, err := yaml.Marshal(a.cfg)
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}

		a.log.Printf("config\n%s", string(b))
	}

	return nil
}

// buildConfig builds the configuration using imports.
func (a *analyzer) buildConfig(imports map[string]struct{}) error {
	i := 0
	for _, rule := range a.cfg.Rules {
		if rule.Disabled {
			continue
		}

		j := 0
		for _, p := range rule.Packages {
			if _, ok := imports[p]; !ok {
				continue // Package wasn't imported.
			}
			rule.Packages[j] = p
			j++
		}
		rule.Packages = rule.Packages[:j]

		if len(rule.Packages) == 0 {
			continue // Doesn't match any of our imported packages.
		}

		if err := rule.validate(); err != nil {
			return err
		}

		a.cfg.Rules[i] = rule
		i++
	}
	a.cfg.Rules = a.cfg.Rules[:i]

	return nil
}

func (a *analyzer) run(pass *analysis.Pass) (interface{}, error) {
	// Check if we import one of checked packages.
	imports := pass.Pkg.Imports()
	pkgs := make(map[string]struct{}, len(imports))
	for _, imp := range imports {
		pkgs[imp.Path()] = struct{}{}
	}

	for _, f := range a.options {
		if err := f(a); err != nil {
			return nil, err
		}
	}

	if err := a.init(pkgs); err != nil {
		return nil, err
	}

	if len(a.cfg.Rules) == 0 {
		// No rules left so no need to check.
		a.log.Print("no rules matched code")
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

	for _, rule := range a.cfg.Rules {
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
	for _, rule := range a.cfg.Rules {
		if visit(a.pass, a.log, rule, ident, stmts[1:]) {
			continue
		}
		a.report(ident, rule, ident.Name)
	}
}

// report reports a missing call for rule at rng for variable name.
func (a *analyzer) report(rng analysis.Range, rule Rule, name string) {
	cat := a.cfg.DefaultCategory
	if rule.Category != "" {
		cat = rule.Category
	}
	name = rule.name(name)
	a.log.Debug().
		Str("rule", rule.Name).
		Str("name", name).
		Msg("not called")
	a.pass.Report(analysis.Diagnostic{
		Pos:      rng.Pos(),
		End:      rng.End(),
		Category: cat,
		Message:  fmt.Sprintf("%s must be called", name),
	})
}
