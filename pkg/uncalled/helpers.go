package uncalled

import (
	"go/ast"
	"go/types"
)

// containsType returns true if typ not nil and contained in rowTypes, false otherwise.
func containsType(typ types.Type, rowTypes map[string]struct{}) bool {
	if typ == nil {
		return false
	}

	_, ok := rowTypes[typ.String()]
	return ok
}

// rootIdent finds the root identifier x in a chain of selections x.y.z, or nil if not found.
func rootIdent(node ast.Node) *ast.Ident {
	switch node := node.(type) {
	case *ast.SelectorExpr:
		return rootIdent(node.X)
	case *ast.Ident:
		return node
	default:
		return nil
	}
}

// names returns all names a in chain of selections x.y.z, or nil if not found.
func names(node ast.Node) []string {
	switch node := node.(type) {
	case *ast.SelectorExpr:
		return append(names(node.X), node.Sel.String())
	case *ast.Ident:
		return []string{node.String()}
	default:
		return nil
	}
}

// restOfBlock, given a traversal stack, finds the innermost containing block
// and returns the suffix of its statements starting with the current node.
func restOfBlock(stack []ast.Node) []ast.Stmt {
	for i := len(stack) - 1; i >= 0; i-- {
		if b, ok := stack[i].(*ast.BlockStmt); ok {
			for j, v := range b.List {
				if v == stack[i+1] {
					return b.List[j:]
				}
			}
			return nil
		}
	}

	return nil
}
