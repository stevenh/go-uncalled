// Package uncalled defines an Analyzer that checks for missing calls.
package uncalled

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"os"

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

// ConfigFile is Analyzer option which configures the config by
// loading it from file.
func ConfigFile(file string) Option {
	return func(r *analyzer) error {
		return r.load(file)
	}
}

// ConfigOpt is Analyzer option which configures the config by
// specifying it directly.
func ConfigOpt(cfg *Config) Option {
	return func(r *analyzer) error {
		r.cfg = cfg
		return nil
	}
}

// NewAnalyzer returns a new Analyzer which checks the passed
// packages in addition to calls.
func NewAnalyzer(options ...Option) *analysis.Analyzer {
	r := &analyzer{options: options}
	a := &analysis.Analyzer{
		Name: name,
		Doc:  doc,
		Run:  r.run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}

	a.Flags.Init(a.Name, flag.ExitOnError)
	a.Flags.Var(r, "config", "configuration file to load")
	a.Flags.Var(version{}, "version", "print version and exit")
	return a
}

// analyzer checks for missing calls.
type analyzer struct {
	pass    *analysis.Pass
	options []Option
	cfg     *Config
}

func (a *analyzer) load(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defer f.Close()

	a.cfg = &Config{}
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(a.cfg); err != nil {
		return fmt.Errorf("decode config: %q: %w", file, err)
	}

	return nil
}

// init initialises r to check packages.
func (a *analyzer) init(imports map[string]struct{}) error {
	if a.cfg == nil {
		// No config specified use default.
		a.cfg = &Config{}
		if err := yaml.Unmarshal(defaultConfig, a.cfg); err != nil {
			return fmt.Errorf("decode default config: %w", err)
		}
	}

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

// String implements flag.Value.
func (a *analyzer) String() string {
	b, _ := yaml.Marshal(a.cfg)
	return string(b)
}

// String implements flag.Value.
func (a *analyzer) Set(file string) error {
	return a.load(file)
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
		return nil, nil //nolint: nilnil
	}

	a.pass = pass
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint: forcetypeassert
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, a.visit)

	return nil, nil //nolint: nilnil
}

// visit evaluates node to ensure checking each rule.
func (a *analyzer) visit(node ast.Node, push bool, stack []ast.Node) bool {
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
	if !rule.Call.matches(sig.Results()) {
		return // Function call is not related to this rule.
	}

	// Find the innermost containing block, and get the list
	// of statements starting with the one containing call.
	stmts := restOfBlock(stack)
	stmt, ok := stmts[0].(*ast.AssignStmt)
	if !ok {
		// First statement is not assignment so not checked.
		a.report(call, rule, "")
		return
	}

	ident := rootIdent(stmt.Lhs[0])
	if ident == nil {
		return // Not matching.
	}

	if len(stmts) < 2 {
		// Call to the sql function is the last statement of the block.
		a.report(ident, rule, ident.Name)
		return
	}

	// TODO(steve): avoid multiple passes.
	for _, rule := range a.cfg.Rules {
		if visit(a.pass, rule, ident, stmts) {
			continue
		}
		a.report(ident, rule, ident.Name)
	}
}

func (a *analyzer) report(rng analysis.Range, rule Rule, name string) {
	a.pass.Report(analysis.Diagnostic{
		Pos:      rng.Pos(),
		End:      rng.End(),
		Category: rule.Severity,
		Message:  fmt.Sprintf("%s must be called", rule.expects(name)),
	})
}
