package cliflags

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// metaLiterals marks the flag literals a file installs in urfave/cli's own
// package-level Flag variables.
type metaLiterals map[*ast.CompositeLit]bool

// literalBindings maps a function-local variable to the single &lit composite
// written to it in this file. A nil entry poisons the variable: it was written
// more than once, or with something that is no composite literal, and
// resolution refuses to vouch for it.
type literalBindings map[*types.Var]*ast.CompositeLit

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

// markMetaAssignments records each right-hand literal whose left-hand side
// installs a meta flag. A write a later one in the same statement list replaces
// is skipped, because it does not reach the framework — exempting its literal
// would hand a flag the exemption for one dead line.
func markMetaAssignments(pass *analysis.Pass, assign *ast.AssignStmt, state resolution) {
	for i, lhs := range assign.Lhs {
		if i < len(assign.Rhs) && !state.dead[assign.Rhs[i]] && metaFlagVar(pass, lhs) != nil {
			markLiteral(pass, state, assign.Rhs[i])
		}
	}
}

// metaFlagVar resolves expr to the framework's own Flag variable it names, or
// nil. Identity is the FRAMEWORK'S storage, not a list of names: a variable
// belonging to urfave/cli v3 whose type is that package's Flag interface. v3
// declares three — VersionFlag, HelpFlag and GenerateShellCompletionFlag — and
// naming two of them left the third judged as an ordinary flag, which is a
// false report on the shape upstream documents.
//
// What acquiring this costs is worth stating rather than claiming away. Nothing
// outside the module can add a variable to cli, so the SET is not forgeable —
// but installing an ordinary flag in one of these slots does silence its
// Sources requirement, and how much that costs differs per slot: replacing
// VersionFlag or HelpFlag changes what --version and --help do, while v3.10.1
// reads GenerateShellCompletionFlag nowhere (completion triggers on the literal
// argument), so installing a flag there costs one visible line and nothing
// else. Raised as cliflags.meta-slot-is-not-a-flag-list rather than settled
// here. The Flag-type test is measured INERT: v3.10.1 exports no Flag-typed
// struct field, and cmd.Flags is []Flag rather than Flag, so nothing can hold a
// flag literal and fail it.
func metaFlagVar(pass *analysis.Pass, expr ast.Expr) *types.Var {
	ident := namedIdent(expr)
	if ident == nil {
		return nil
	}
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok || obj.Pkg() == nil || obj.Pkg().Path() != cliPackage {
		return nil
	}
	if !isFrameworkFlagType(obj) {
		return nil
	}
	return obj
}

// namedIdent returns the identifier an expression names: the selected half of
// cli.HelpFlag or cli.EnvVars, or the bare name a dot import leaves behind. It
// reads past parentheses, which Go admits as readily as the plain form and the
// build treats identically. Matching *ast.SelectorExpr alone cut both ways —
// one pair of parentheses around an assignment target hid a REPLACING write,
// so a forged exemption survived in the same block it was replaced in; and it
// hid an honest sole override too, which was then told to bind the environment
// variable checkMetaSources forbids. WHICH package object an identifier
// resolves to is decided by its callers, from its package and its type; this
// only finds the identifier to ask about.
func namedIdent(expr ast.Expr) *ast.Ident {
	switch target := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		return target.Sel
	case *ast.Ident:
		return target
	}
	return nil
}

// isFrameworkFlagType reports whether obj's type is the framework's Flag
// interface — the single-flag storage urfave reads, as against the []Flag list
// an app fills with its own flags. It asks about the TYPE's package, which is a
// different question from metaFlagVar's about the VARIABLE's: a field of this
// package typed cli.Flag answers yes here and no there.
func isFrameworkFlagType(obj *types.Var) bool {
	named, ok := types.Unalias(obj.Type()).(*types.Named)
	return ok && named.Obj().Name() == "Flag" && named.Obj().Pkg().Path() == cliPackage
}

// markLiteral records expr's composite literal as a meta-flag override —
// either the direct &lit form, or a function-local variable resolved through
// its single &lit binding.
func markLiteral(pass *analysis.Pass, state resolution, expr ast.Expr) {
	if ident, ok := expr.(*ast.Ident); ok {
		markBinding(pass, state, ident)
		return
	}
	if lit := literalOf(expr); lit != nil {
		state.meta[lit] = true
	}
}

// markBinding records the literal a variable-indirected override resolves to,
// when its variable has a usable single &lit binding.
func markBinding(pass *analysis.Pass, state resolution, ident *ast.Ident) {
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok {
		return
	}
	if lit := state.bindings[obj]; lit != nil {
		state.meta[lit] = true
	}
}

// literalOf unwraps the usual &lit form to its composite literal, or nil for
// anything else. Parentheses are stripped first because Go REQUIRES them where
// a composite literal appears in an if, for or switch init clause, and reading
// past them was not optional: without it, `if cli.HelpFlag = (&cli.BoolFlag{});
// cond` lost the meta exemption and the override was told to bind an
// environment variable.
func literalOf(expr ast.Expr) *ast.CompositeLit {
	expr = ast.Unparen(expr)
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = ast.Unparen(unary.X)
	}
	lit, _ := expr.(*ast.CompositeLit)
	return lit
}

// checkMetaSources reports an env binding on a framework meta-flag override.
// urfave v3 evaluates its version and help checks before FlagBase.PostParse
// applies Sources, so a binding there never triggers the meta-flag — while the
// generated help output still advertises the variable
// (TestCheckMetaSourcesEnvBindingIsInertUpstream pins that upstream order
// against the real framework). The completion flag is inert for a stronger
// reason: v3.10.1 triggers completion on the literal argument
// "--generate-shell-completion" (completion.go:14, help.go:482) and reads
// GenerateShellCompletionFlag nowhere, which
// TestCheckMetaSourcesCompletionBindingIsInertUpstream pins. Dead and
// misleading, so the standard forbids it rather than requiring it.
func checkMetaSources(pass *analysis.Pass, lit *ast.CompositeLit, fields fieldMap, name flagName) {
	if bindsEnv(envVarCalls(pass, fields["Sources"])) {
		pass.Reportf(lit.Pos(), messageMetaEnv, name)
	}
}
