package cliflags

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// metaLiterals marks the flag literals assigned to urfave/cli's package-level
// VersionFlag/HelpFlag variables.
type metaLiterals map[*ast.CompositeLit]bool

// literalBindings maps a function-local variable to the single &lit composite
// written to it in this file. A nil entry poisons the variable: it was written
// more than once, or with something that is no composite literal, and
// resolution refuses to vouch for it.
type literalBindings map[*types.Var]*ast.CompositeLit

// metaFlagLiterals collects the flag literals a file assigns to cli.VersionFlag
// or cli.HelpFlag — directly (&lit) or through a function-local variable's
// single &lit binding. Overriding those globals is how an app reshapes the
// built-in meta-flags, and their env-binding rule inverts there (see
// checkMetaSources).
func metaFlagLiterals(pass *analysis.Pass, file *ast.File) metaLiterals {
	meta := metaLiterals{}
	bindings := collectBindings(pass, file)
	ast.Inspect(file, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			markMetaAssignments(pass, assign, meta, bindings)
		}
		return true
	})
	return meta
}

// collectBindings records every function-local variable's single &lit binding
// in the file. The resolution boundary is deliberate — same file, function
// scope, single write: a function-local variable is writable only within its
// declaring file, so this file's writes are all of them, and a variable
// written once with &lit holds that literal at the meta-flag assignment.
// Anything beyond that (package-level variables, reassignments, values flowing
// through calls) would need whole-program dataflow, so those stay unresolved
// and the literal is judged as an ordinary flag.
func collectBindings(pass *analysis.Pass, file *ast.File) literalBindings {
	bindings := literalBindings{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			bindAssignments(pass, stmt, bindings)
		case *ast.ValueSpec:
			bindSpecs(pass, stmt, bindings)
		}
		return true
	})
	return bindings
}

// bindAssignments records each assignment pair (both := and =) as a write to
// its left-hand variable.
func bindAssignments(pass *analysis.Pass, assign *ast.AssignStmt, bindings literalBindings) {
	for i, lhs := range assign.Lhs {
		if i < len(assign.Rhs) {
			bind(pass, lhs, assign.Rhs[i], bindings)
		}
	}
}

// bindSpecs records each var-declaration pair (var ident = value) as a write
// to its declared variable.
func bindSpecs(pass *analysis.Pass, spec *ast.ValueSpec, bindings literalBindings) {
	for i, name := range spec.Names {
		if i < len(spec.Values) {
			bind(pass, name, spec.Values[i], bindings)
		}
	}
}

// bind records one write to a function-local variable: the first write binds
// its &lit composite (or nil for a non-literal), and any further write poisons
// the entry — a multiply-written variable is outside the single-assignment
// resolution boundary.
func bind(pass *analysis.Pass, lhs, rhs ast.Expr, bindings literalBindings) {
	obj := localVar(pass, lhs)
	if obj == nil {
		return
	}
	if _, written := bindings[obj]; written {
		bindings[obj] = nil
		return
	}
	bindings[obj] = literalOf(rhs)
}

// localVar resolves lhs to the function-local variable it writes, or nil. A
// package-level variable is refused: another file can rebind it, so this
// file's single-assignment view is not authoritative there.
func localVar(pass *analysis.Pass, lhs ast.Expr) *types.Var {
	ident, ok := lhs.(*ast.Ident)
	if !ok {
		return nil
	}
	obj, ok := identVar(pass, ident)
	if !ok || obj.Parent() == obj.Pkg().Scope() {
		return nil
	}
	return obj
}

// identVar resolves an identifier to the variable it declares (:=, var) or
// names (=).
func identVar(pass *analysis.Pass, ident *ast.Ident) (*types.Var, bool) {
	if obj, ok := pass.TypesInfo.Defs[ident].(*types.Var); ok {
		return obj, true
	}
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	return obj, ok
}

// markMetaAssignments records each right-hand literal whose left-hand side is
// a meta-flag variable.
func markMetaAssignments(pass *analysis.Pass, assign *ast.AssignStmt, meta metaLiterals, bindings literalBindings) {
	for i, lhs := range assign.Lhs {
		if i < len(assign.Rhs) && isMetaFlagVar(pass, lhs) {
			markLiteral(pass, meta, bindings, assign.Rhs[i])
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

// markLiteral records expr's composite literal as a meta-flag override —
// either the direct &lit form, or a function-local variable resolved through
// its single &lit binding.
func markLiteral(pass *analysis.Pass, meta metaLiterals, bindings literalBindings, expr ast.Expr) {
	if ident, ok := expr.(*ast.Ident); ok {
		markBinding(pass, meta, bindings, ident)
		return
	}
	if lit := literalOf(expr); lit != nil {
		meta[lit] = true
	}
}

// markBinding records the literal a variable-indirected override resolves to,
// when its variable has a usable single &lit binding.
func markBinding(pass *analysis.Pass, meta metaLiterals, bindings literalBindings, ident *ast.Ident) {
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok {
		return
	}
	if lit := bindings[obj]; lit != nil {
		meta[lit] = true
	}
}

// literalOf unwraps the usual &lit form to its composite literal, or nil for
// anything else.
func literalOf(expr ast.Expr) *ast.CompositeLit {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	lit, _ := expr.(*ast.CompositeLit)
	return lit
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
