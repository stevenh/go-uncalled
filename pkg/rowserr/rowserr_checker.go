package rowserr

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// rowsErrChecker is an ast.Vistor which searches for a call to Rows.Err()
// if successful found is set to true, false otherwise.
type rowsErrChecker struct {
	pass *analysis.Pass

	// identObjs contains ident.Obj entries which match the interested ident.
	identObjs map[*ast.Object]struct{}

	// called maps literal function object to argument positions that
	// resulted in a successful *database/sql.Rows.Err() call.
	called map[*ast.Object]map[int]struct{}

	// found is set to true if we found a call to our interested
	// ident.Err().
	found bool
}

// rowsErrCalled returns true if ident.Err() is called, false otherwise.
func rowsErrCalled(pass *analysis.Pass, rows *ast.Ident, stmts []ast.Stmt) bool {
	ec := &rowsErrChecker{
		pass: pass,
		identObjs: map[*ast.Object]struct{}{
			rows.Obj: {},
		},
		called: make(map[*ast.Object]map[int]struct{}),
	}

	for _, s := range stmts {
		if ec.errCalled(s) {
			return true
		}
	}

	return false
}

// errCalled returns true if the given if statement checks rows.Err().
func (ec *rowsErrChecker) errCalled(stmt ast.Stmt) bool {
	ast.Walk(ec, stmt)

	return ec.found
}

// Visit implements ast.Visitor.
func (ec *rowsErrChecker) Visit(node ast.Node) (w ast.Visitor) {
	if ec.found || node == nil {
		return nil // Already found or walk complete
	}

	switch t := node.(type) {
	case *ast.CallExpr:
		return ec.visitCallExpr(t)
	case *ast.AssignStmt:
		return ec.visitAssignStmt(t)
	default:
		return ec
	}
}

// visitAssignStmt visits stmt.
func (ec *rowsErrChecker) visitAssignStmt(stmt *ast.AssignStmt) (w ast.Visitor) {
	ec.assignStmtMatches(stmt)
	ec.assignStmtFuncLit(stmt)

	return ec
}

// rowsExpr returns true if expr represents a *database/sql.Rows, false otherwise.
func (ec *rowsErrChecker) rowsExpr(expr ast.Expr) bool {
	tv, ok := ec.pass.TypesInfo.Types[expr]
	if !ok {
		return false // Unknown type.
	}

	return isRowsPtrType(tv.Type)
}

// dump dumps the details of node.
func (ec *rowsErrChecker) dump(node ast.Node) {
	ast.Print(token.NewFileSet(), node)
}

// assignStmtFuncLit checks stmt for functions that call *database/sql.Err().
// If any calls are found they are registered in ec.called.
func (ec *rowsErrChecker) assignStmtFuncLit(stmt *ast.AssignStmt) {
	for i, rhs := range stmt.Rhs {
		lit, ok := rhs.(*ast.FuncLit)
		if !ok {
			continue // Not a function literal.
		}

		if len(lit.Body.List) == 0 {
			continue // Empty body.
		}

		ident, ok := stmt.Lhs[i].(*ast.Ident)
		if !ok {
			continue // Assigned not ident.
		}

		for _, f := range lit.Type.Params.List {
			if !ec.rowsExpr(f.Type) {
				continue // Not a *database/sql.Rows parameter.
			}

			for j, param := range f.Names {
				if rowsErrCalled(ec.pass, param, lit.Body.List) {
					// Rows.Err was called for this parameter.
					args := ec.called[ident.Obj]
					if args == nil {
						args = make(map[int]struct{})
						ec.called[ident.Obj] = args
					}
					args[j] = struct{}{}
				}
			}
		}
	}
}

// assignStmtMatches checks stmt for assignments from variables known to match the
// identifier we're interested in. If any are found they registered in ec.matches.
func (ec *rowsErrChecker) assignStmtMatches(stmt *ast.AssignStmt) {
	for i, rhs := range stmt.Rhs {
		identRHS, ok := rhs.(*ast.Ident)
		if !ok {
			continue // Not an Ident.
		}

		if _, ok := ec.identObjs[identRHS.Obj]; ok {
			// Right hand side matches.
			lhs := stmt.Lhs[i]
			identLHS, ok := lhs.(*ast.Ident)
			if !ok {
				continue // Not an Ident.
			}

			// Assignment match found.
			ec.identObjs[identLHS.Obj] = struct{}{}
		}
	}
}

// visitCallExpr visits call.
func (ec *rowsErrChecker) visitCallExpr(call *ast.CallExpr) (w ast.Visitor) {
	switch t := call.Fun.(type) {
	case *ast.SelectorExpr:
		return ec.rowsErrCalled(call, t)
	case *ast.Ident:
		return ec.visitCallExprIdent(call, t)
	default:
		return ec
	}
}

// visitCallExprIdent checks if call resulted in any of interested idents having .Err() called.
// If a match was found it returns nil, otherwise ec.
func (ec *rowsErrChecker) visitCallExprIdent(call *ast.CallExpr, ident *ast.Ident) (w ast.Visitor) {
	for i, expr := range call.Args {
		arg, ok := expr.(*ast.Ident)
		if !ok {
			continue // Not an ident arg.
		}
		for obj := range ec.identObjs {
			if arg.Obj == obj {
				if _, ok := ec.called[ident.Obj][i]; ok {
					ec.found = true
					return nil // Parameter called .Err().
				}
			}
		}
	}

	return ec
}

// rowsErrCalled checks if call was a call to our interested variable.
func (ec *rowsErrChecker) rowsErrCalled(call *ast.CallExpr, sel *ast.SelectorExpr) (w ast.Visitor) {
	if len(call.Args) != 0 {
		return ec // Not a no argument call.
	}

	if sel.Sel.Name != errMethod {
		return ec // Method not Err.
	}

	typ, ok := ec.pass.TypesInfo.Types[sel.X]
	if !ok {
		return ec // Unknown type
	}

	if !isRowsPtrType(typ.Type) {
		return ec // Type is not *database/sql.Rows.
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ec // Not an call to receiver.func.
	}

	if _, ok := ec.identObjs[ident.Obj]; !ok {
		return ec // Call receiver didn't match rowsObj.
	}

	// Rows.Err() was called.
	ec.found = true

	return nil
}
