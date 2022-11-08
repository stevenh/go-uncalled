package uncalled

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/tools/go/analysis"
)

// visitor is an ast.Vistor which searches for a call to Rows.Err()
// if successful found is set to true, false otherwise.
type visitor struct {
	pass *analysis.Pass

	// identObjs contains ident.Obj entries which match the interested ident.
	identObjs map[*ast.Object]struct{}

	// calledArgs maps literal function object to argument positions that
	// resulted in a successful rule calls.
	calledArgs map[*ast.Object]map[int]struct{}

	// found is set to true if we found a call to our interested
	// ident.Err().
	found bool

	// rule contains the rule to check against.
	rule Rule

	// log is the logger to use for debugging.
	log zerolog.Logger
}

// visit returns true if ident.Err() is called, false otherwise.
func visit(pass *analysis.Pass, log zerolog.Logger, rule Rule, ident *ast.Ident, stmts []ast.Stmt) bool {
	log.Debug().Stringer("ident", ident).Msg("visit")
	ec := &visitor{
		pass: pass,
		identObjs: map[*ast.Object]struct{}{
			ident.Obj: {},
		},
		calledArgs: make(map[*ast.Object]map[int]struct{}),
		rule:       rule,
		log:        log,
	}

	for _, s := range stmts {
		if ec.walk(s) {
			return true
		}
	}

	return false
}

// walk returns true if the given if statement calls the rules expected method.
func (ec *visitor) walk(stmt ast.Stmt) bool {
	ast.Walk(ec, stmt)

	return ec.found
}

// Visit implements ast.Visitor.
func (ec *visitor) Visit(node ast.Node) (w ast.Visitor) {
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
func (ec *visitor) visitAssignStmt(stmt *ast.AssignStmt) (w ast.Visitor) {
	ec.assignStmtMatches(stmt)
	ec.assignStmtFuncLit(stmt)

	return ec
}

// containsType returns true if expr represents on of our expected types, false otherwise.
func (ec *visitor) containsType(expr ast.Expr) bool {
	tv, ok := ec.pass.TypesInfo.Types[expr]
	if !ok {
		return false // Unknown type.
	}

	return containsType(tv.Type, ec.rule.expectedTypes)
}

// dump dumps the details of node.
func (ec *visitor) dump(node ast.Node) {
	ast.Print(token.NewFileSet(), node)
}

// assignStmtFuncLit checks stmt for functions that call *database/sql.Err().
// If any calls are found they are registered in ec.called.
func (ec *visitor) assignStmtFuncLit(stmt *ast.AssignStmt) {
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
			if !ec.containsType(f.Type) {
				continue // Not an expected parameter.
			}

			for j, param := range f.Names {
				if visit(ec.pass, ec.log, ec.rule, param, lit.Body.List) {
					// Rule matched call for this parameter.
					args := ec.calledArgs[ident.Obj]
					if args == nil {
						args = make(map[int]struct{})
						ec.calledArgs[ident.Obj] = args
					}
					args[j] = struct{}{}
				}
			}
		}
	}
}

// assignStmtMatches checks stmt for assignments from variables known to match the
// identifier we're interested in. If any are found they registered in ec.matches.
func (ec *visitor) assignStmtMatches(stmt *ast.AssignStmt) {
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
func (ec *visitor) visitCallExpr(call *ast.CallExpr) (w ast.Visitor) {
	switch t := call.Fun.(type) {
	case *ast.SelectorExpr:
		return ec.visitCallNode(call, t)
	case *ast.Ident:
		return ec.visitCallIdent(call, t)
	default:
		return ec
	}
}

// visitCallIdent checks if call resulted in any of interested idents having
// the expected call called.
// If a match was found it returns nil, otherwise ec.
func (ec *visitor) visitCallIdent(call *ast.CallExpr, ident *ast.Ident) (w ast.Visitor) {
	if ec.visitCallNode(call, ident) == nil {
		return nil // Call to the ident.
	}

	for i, expr := range call.Args {
		arg, ok := expr.(*ast.Ident)
		if !ok {
			continue // Not an ident arg.
		}

		for obj := range ec.identObjs {
			if arg.Obj == obj {
				if _, ok := ec.calledArgs[ident.Obj][i]; ok {
					ec.found = true
					return nil // Expected function was called.
				}
			}
		}
	}

	return ec
}

// visitCallNode checks if call was a call to our interested variable.
func (ec *visitor) visitCallNode(call *ast.CallExpr, node ast.Node) (w ast.Visitor) {
	parts := names(node)
	if parts == nil {
		ec.log.Error().Msgf("node %#v: nil name", node)
		return ec
	}

	name := strings.Join(parts[1:], ".")
	matches := ec.rule.matchesCall(call, name)
	ec.log.Debug().
		Bool("matches", matches).
		Str("call", name).
		Str("name", strings.Join(parts, ".")).
		Msg("matchesCall")
	if !matches {
		return ec // Doesn't match method name or args.
	}

	ident := rootIdent(node)
	typ, ok := ec.pass.TypesInfo.Types[ident]
	if !ok {
		return ec // Unknown type
	}

	if name == "" {
		name = typ.Type.String()
	} else {
		name = typ.Type.String() + "." + name
	}
	if _, ok := ec.rule.expectedCalls[name]; !ok {
		return ec // Type doesn't match.
	}

	if _, ok := ec.identObjs[ident.Obj]; !ok {
		return ec // Call receiver didn't match an expected objects.
	}

	// Expected function was called.
	ec.found = true

	return nil
}
