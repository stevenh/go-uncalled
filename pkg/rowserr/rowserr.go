// Package rowserr defines an Analyzer that checks for missing database/sql.Rows.Err() calls.
package rowserr

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	doc = `checks for missing database/sql.Rows.Err() calls

Checks if a method on database/sql which returns an *sql.Rows is
called without calling *sql.Rows.Err() to check for an error.

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
connection becomes invalid, this causes *sql.Rows.Next() to return
false without processing all rows.`

	errMethod  = "Err"
	rowsType   = "Rows"
	sqlPackage = "database/sql"
	name       = "rowserr"
)

var (
	// Analyzer checks for missing *database/sql.Rows.Err() calls.
	Analyzer = &analysis.Analyzer{
		Name: name,
		Doc:  doc,
		Run:  run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}

	rowsPtrType = fmt.Sprintf("*%s.%s", sqlPackage, rowsType)
	callTypes   = []ast.Node{
		(*ast.CallExpr)(nil),
	}
	errorType = types.Universe.Lookup("error").Type()
)

type rowsChecker struct {
	pass *analysis.Pass
}

func run(pass *analysis.Pass) (interface{}, error) {
	if !imports(pass.Pkg, sqlPackage) {
		// Package isn't imported so no need to check.
		return nil, nil //nolint: nilnil
	}

	r := &rowsChecker{pass: pass}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint: forcetypeassert
	ins.WithStack(callTypes, r.run)

	return nil, nil //nolint: nilnil
}

// run evaluates node to ensure if it a rows method that the block
// calls Rows.Err().
func (r rowsChecker) run(node ast.Node, push bool, stack []ast.Node) bool {
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

	if rowsErrCalled(r.pass, rows, stmts) {
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

	if !isRowsPtrType(res.At(0).Type()) {
		return false // First return type is not *database/sql.Rows.
	}

	// Ensure the second return is an error.
	return types.Identical(res.At(1).Type(), errorType)
}
