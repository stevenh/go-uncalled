// Package rowserr defines an Analyzer that checks for missing database/sql.Rows.Err() calls.
package rowserr

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	doc = `checks for missing database/sql.Rows.Err() calls

Checks if a method on sql packages which returns a *Rows is
called without calling *Rows.Err() to check for an error.

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
connection becomes invalid, this causes *Rows.Next() to return
false without processing all rows.`

	errMethod = "Err"
	rowsType  = "Rows"
	name      = "rowserr"
)

var (
	sqlPackages = []string{
		"database/sql",
		"github.com/jmoiron/sqlx",
	}
	callTypes = []ast.Node{
		(*ast.CallExpr)(nil),
	}
	errorType = types.Universe.Lookup("error").Type()
)

// NewAnalyzer returns a new Analyzer which checks the passed
// packages in addition to database/sql for missing
// *database/sql.Rows.Err() calls.
func NewAnalyzer(packages ...string) *analysis.Analyzer {
	r := newRowsChecker(packages...)
	a := &analysis.Analyzer{
		Name: name,
		Doc:  doc,
		Run:  r.run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}

	a.Flags.Init(a.Name, flag.ExitOnError)
	a.Flags.Var(r, "packages", "additional package to check")
	a.Flags.Var(version{}, "version", "print version and exit")
	return a
}

// rowsChecker checks for missing database/sql.Rows.Err() calls.
type rowsChecker struct {
	pass     *analysis.Pass
	packages map[string]struct{}
	types    map[string]struct{}
}

// newRowsChecker returns rowsChecker which checks against the
// specified packages and database/sql.
func newRowsChecker(packages ...string) *rowsChecker {
	r := &rowsChecker{}
	r.init(packages...)

	return r
}

// init initialises r to check packages.
func (r *rowsChecker) init(packages ...string) {
	r.packages = make(map[string]struct{})
	r.types = make(map[string]struct{})
	for _, p := range append(sqlPackages, packages...) {
		r.packages[p] = struct{}{}
		r.types[fmt.Sprintf("*%s.%s", p, rowsType)] = struct{}{}
	}
}

// String implements flag.Value.
func (r *rowsChecker) String() string {
	pkgs := make([]string, 0, len(r.packages))
	for p := range r.packages {
		pkgs = append(pkgs, p)
	}
	return strings.Join(pkgs, ",")
}

// String implements flag.Value.
func (r *rowsChecker) Set(val string) error {
	if len(val) != 0 {
		r.init(strings.Split(val, ",")...)
	}
	return nil
}

func (r *rowsChecker) run(pass *analysis.Pass) (interface{}, error) {
	// Check if we import one of checked packages.
	imports := pass.Pkg.Imports()
	paths := make(map[string]struct{}, len(imports))
	for _, imp := range imports {
		paths[imp.Path()] = struct{}{}
	}

	for pkg := range r.packages {
		if _, ok := paths[pkg]; !ok {
			delete(r.packages, pkg)
		}
	}

	if len(r.packages) == 0 {
		// None of the specified packages where imported so no need to check.
		return nil, nil //nolint: nilnil
	}

	r.pass = pass
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint: forcetypeassert
	ins.WithStack(callTypes, r.visit)

	return nil, nil //nolint: nilnil
}

// visit evaluates node to ensure if it a rows method that the block
// calls Rows.Err().
func (r rowsChecker) visit(node ast.Node, push bool, stack []ast.Node) bool {
	if !push {
		return true
	}

	call := node.(*ast.CallExpr) //nolint: forcetypeassert
	if !r.rowsMethod(call) {
		return true // Function call is not related to this check.
	}

	// Find the innermost containing block, and get the list
	// of statements starting with the one containing call.
	stmts := restOfBlock(stack)
	stmt, ok := stmts[0].(*ast.AssignStmt)
	if !ok {
		// First statement is not assignment so not checked.
		r.pass.ReportRangef(call, "rows.Err() must be checked")
		return true
	}

	rows := rootIdent(stmt.Lhs[0])
	if rows == nil {
		return true // sql.Rows not found in the assignment.
	}

	if len(stmts) < 2 {
		// Call to the sql function is the last statement of the block.
		r.pass.ReportRangef(rows, "%s.Err() must be checked", rows.Name)
		return true
	}

	if rowsErrCalled(r.pass, r.types, rows, stmts) {
		return true
	}

	r.pass.ReportRangef(rows, "%s.Err() must be checked", rows.Name)
	return true
}

// rowsMethod returns true if the given call expression is on one
// of types and returns (pkg.Rows, error), false otherwise.
// It matches these methods like these:
//
//	func (db *DB) Query(query string, args ...any) (*Rows, error)
//	func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error)
func (r rowsChecker) rowsMethod(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	typ, ok := r.pass.TypesInfo.Types[sel]
	if !ok {
		return false // Unknown type.
	}

	sig, ok := typ.Type.(*types.Signature)
	if !ok {
		return false // Call is not of the form x.f().
	}

	res := sig.Results()
	if res.Len() != 2 {
		return false // Function doesn't return two values.
	}

	if !isRowsPtrType(res.At(0).Type(), r.types) {
		return false // First return type is not *database/sql.Rows.
	}

	// Ensure the second return is an error.
	return types.Identical(res.At(1).Type(), errorType)
}
