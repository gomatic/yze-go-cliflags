package cliflags

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// metaLiterals marks the flag literals assigned to urfave/cli's package-level
// VersionFlag/HelpFlag variables.
type metaLiterals map[*ast.CompositeLit]bool

// metaFlagLiterals collects the flag literals a file assigns to cli.VersionFlag
// or cli.HelpFlag. Overriding those globals is how an app reshapes the built-in
// meta-flags, and their env-binding rule inverts there (see checkMetaSources).
func metaFlagLiterals(pass *analysis.Pass, file *ast.File) metaLiterals {
	meta := metaLiterals{}
	ast.Inspect(file, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			markMetaAssignments(pass, assign, meta)
		}
		return true
	})
	return meta
}

// markMetaAssignments records each right-hand literal whose left-hand side is a
// meta-flag variable.
func markMetaAssignments(pass *analysis.Pass, assign *ast.AssignStmt, meta metaLiterals) {
	for i, lhs := range assign.Lhs {
		if i < len(assign.Rhs) && isMetaFlagVar(pass, lhs) {
			markLiteral(meta, assign.Rhs[i])
		}
	}
}

// isMetaFlagVar reports whether expr resolves to urfave/cli v3's VersionFlag or
// HelpFlag package variable.
func isMetaFlagVar(pass *analysis.Pass, expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	if !ok || obj.Pkg() == nil || obj.Pkg().Path() != cliPackage {
		return false
	}
	return obj.Name() == "VersionFlag" || obj.Name() == "HelpFlag"
}

// markLiteral records expr's composite literal (unwrapping the usual &lit
// form) as a meta-flag override.
func markLiteral(meta metaLiterals, expr ast.Expr) {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	if lit, ok := expr.(*ast.CompositeLit); ok {
		meta[lit] = true
	}
}

// checkMetaSources reports an env binding on a cli.VersionFlag/cli.HelpFlag
// override. urfave v3 evaluates its version and help checks before
// FlagBase.PostParse applies Sources, so a binding there never triggers the
// meta-flag — while the generated help output still advertises the variable
// (TestCheckMetaSourcesEnvBindingIsInertUpstream pins that upstream order against the real
// framework). Dead and misleading, so the standard forbids it rather than
// requiring it.
func checkMetaSources(pass *analysis.Pass, lit *ast.CompositeLit, fields fieldMap, name flagName) {
	if bindsEnv(envVarCalls(pass, fields["Sources"])) {
		pass.Reportf(lit.Pos(), messageMetaEnv, name)
	}
}
