package cliflags

// Which literal a file INSTALLS in the framework's meta-flag storage: the
// resolution state, and the writes a later one in the same statement list
// replaces before anything can read them. metaflags.go answers the other half —
// which variable is the framework's, and which literal a write resolves to.

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// deadWrites holds the right-hand expressions of meta-flag assignments a later
// assignment in the same statement list replaces. Such a write installs
// nothing, so the literal it names is not the framework's meta flag.
type deadWrites map[ast.Expr]bool

// installedWrites is the write standing for each meta-flag variable so far in
// one statement list.
type installedWrites map[*types.Var]ast.Expr

// resolution is one file's meta-flag resolution state: the literals marked as
// installed, the single-write bindings a variable-indirected override resolves
// through, and the writes a later one replaces.
type resolution struct {
	meta     metaLiterals
	bindings literalBindings
	dead     deadWrites
}

// metaFlagLiterals collects the flag literals a file INSTALLS in one of the
// framework's package-level Flag variables — directly (&lit) or through a
// function-local variable's single &lit binding. Overriding those globals is
// how an app reshapes the built-in meta-flags, and their env-binding rule
// inverts there (see checkMetaSources).
func metaFlagLiterals(pass *analysis.Pass, file *ast.File) metaLiterals {
	state := resolution{
		meta:     metaLiterals{},
		bindings: collectBindings(pass, file),
		dead:     overwrittenWrites(pass, file),
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			markMetaAssignments(pass, assign, state)
		}
		return true
	})
	return state.meta
}

// overwrittenWrites finds every meta-flag write a later write to the same
// variable replaces within ONE statement list. Nothing runs between two
// statements, so the earlier write is dead by construction — no dataflow, and
// no judgement about reachability. What that closes is the whole exemption
// going to any literal a file merely NAMES beside a meta variable: one dead
// line, immediately overwritten, permanently exempted a flag from the Sources
// rule and followed it into whatever Flags list actually used it. A live
// assignment costs the property — the flag really is the help flag. What
// remains outside this, and is stated rather than claimed away: a write in a
// dead branch, or replaced from a different block, still reads as installed.
func overwrittenWrites(pass *analysis.Pass, file *ast.File) deadWrites {
	dead := deadWrites{}
	ast.Inspect(file, func(node ast.Node) bool {
		markOverwrites(pass, statementList(node), dead)
		return true
	})
	return dead
}

// statementList returns the statements a node holds as ONE sequence, which is
// where a later write provably replaces an earlier one.
func statementList(node ast.Node) []ast.Stmt {
	switch stmt := node.(type) {
	case *ast.BlockStmt:
		return stmt.List
	case *ast.CaseClause:
		return stmt.Body
	case *ast.CommClause:
		return stmt.Body
	}
	return nil
}

// markOverwrites records, across one statement list, every meta-flag write a
// later one replaces.
func markOverwrites(pass *analysis.Pass, list []ast.Stmt, dead deadWrites) {
	installed := installedWrites{}
	for _, stmt := range list {
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			replaceWrites(pass, assign, installed, dead)
		}
	}
}

// replaceWrites records one assignment's meta-flag writes, marking whatever
// each of them replaces as dead.
func replaceWrites(pass *analysis.Pass, assign *ast.AssignStmt, installed installedWrites, dead deadWrites) {
	for i, lhs := range assign.Lhs {
		obj := metaFlagVar(pass, lhs)
		if obj == nil || i >= len(assign.Rhs) {
			continue
		}
		if prior, written := installed[obj]; written {
			dead[prior] = true
		}
		installed[obj] = assign.Rhs[i]
	}
}
