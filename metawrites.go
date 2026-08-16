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

// installedWrites maps each meta-flag variable one assignment writes to the
// expression it writes there.
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

// overwrittenWrites finds every meta-flag write the IMMEDIATELY FOLLOWING
// statement replaces. Adjacency is the whole rule, and it is what makes the
// justification true rather than merely plausible: nothing runs between two
// adjacent statements, so the earlier write is replaced before anything reads
// it. Any statement in between keeps that write LIVE, because the analyzer has
// no way to see what read the variable. Scanning the whole list instead
// reported the save /
// override / run / restore idiom — where the override is real and the restore
// is three lines later — and told the author to bind an environment variable
// on the help flag, which is the one thing checkMetaSources forbids.
//
// What it closes is the exemption going to any literal a file merely NAMES
// beside a meta variable: one dead line, immediately overwritten, permanently
// exempted a flag from the Sources rule and followed it into whatever Flags
// list actually used it. A live assignment costs the property — the flag
// really is the help flag. What remains outside this, and is stated rather
// than claimed away: a write in a dead branch, or replaced from a different
// block, still reads as installed.
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

// markOverwrites records, for each adjacent pair in one statement list, every
// meta-flag write the second replaces.
func markOverwrites(pass *analysis.Pass, list []ast.Stmt, dead deadWrites) {
	for at := 1; at < len(list); at++ {
		replaceWrites(pass, positionAssignment(list[at-1]), positionAssignment(list[at]), dead)
	}
}

// positionAssignment returns the assignment a statement performs AT its own
// place in the list, seeing through a label and through the init clause of an
// if, a for or a switch. Each of those runs at precisely the point the
// statement occupies, so a write wrapped in one replaces its predecessor
// exactly as a bare assignment does. Reading only a bare *ast.AssignStmt made
// one label, or one init clause, enough to keep an exemption a plain
// assignment on the same line would have lost.
func positionAssignment(stmt ast.Stmt) *ast.AssignStmt {
	switch inner := stmt.(type) {
	case *ast.AssignStmt:
		return inner
	case *ast.LabeledStmt:
		return positionAssignment(inner.Stmt)
	case *ast.IfStmt:
		return positionAssignment(inner.Init)
	case *ast.ForStmt:
		return positionAssignment(inner.Init)
	case *ast.SwitchStmt:
		return positionAssignment(inner.Init)
	case *ast.TypeSwitchStmt:
		return positionAssignment(inner.Init)
	}
	return nil
}

// replaceWrites marks each meta-flag write of the earlier statement dead when
// the later one writes the same variable.
func replaceWrites(pass *analysis.Pass, earlier, later *ast.AssignStmt, dead deadWrites) {
	if earlier == nil || later == nil {
		return
	}
	replaced := metaTargets(pass, later)
	for obj, written := range metaTargets(pass, earlier) {
		if _, ok := replaced[obj]; ok {
			dead[written] = true
		}
	}
}

// metaTargets maps each meta-flag variable an assignment writes to the
// expression written to it. A left-hand side with no right-hand side of its
// own — `cli.VersionFlag, cli.HelpFlag = two()` — writes no expression this can
// name, and indexing for one panics.
func metaTargets(pass *analysis.Pass, assign *ast.AssignStmt) installedWrites {
	targets := installedWrites{}
	for i, lhs := range assign.Lhs {
		if i >= len(assign.Rhs) {
			continue
		}
		if obj := metaFlagVar(pass, lhs); obj != nil {
			targets[obj] = assign.Rhs[i]
		}
	}
	return targets
}
