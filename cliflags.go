// Package cliflags provides a go/analysis analyzer enforcing the flag shape of
// the opinionated CLI standard on every urfave/cli v3 flag literal. Each rule
// is one line of the go-cli standard:
//
//   - every flag binds an environment variable via Sources: cli.EnvVars(...)
//   - every flag whose zero value is not its sensible default carries an
//     explicit default via Value. A boolean's false and a container's empty
//     ARE that default, so bool, slice and map flags carry none; every other
//     value type v3 ships has a zero that is a value rather than an emptiness
//   - flag names are kebab-case, the empty name included: it names no flag
//   - an app-specific environment variable is UPPERCASE_SNAKE_CASE (prefixed
//     with the app name when -app is set), while a well-known external
//     variable (PG*, AWS_*, DOCKER_*, ...) is used UNPREFIXED — no
//     KILROY_PGHOST; libpq already owns PGHOST
//   - no flag is Required — a required flag is a positional argument in
//     disguise
//
// A flag literal is a single-package fact, which is what makes these rules
// analyzer-shaped; the surrounding package structure is yze/cliapp's and
// stickler/clilayout's business. The well-known namespace list is seeded from
// the standard's own examples and extended with -external, whose entries are
// trimmed and validated when the setting is applied — a namespace that cannot
// name an environment variable is refused rather than registered.
//
// One exemption inverts a rule: a flag literal a file installs in one of the
// framework's own package-level Flag variables (cli.VersionFlag,
// cli.HelpFlag, cli.GenerateShellCompletionFlag) is forbidden Sources instead
// of required to have them, because v3 reads those before Sources apply.
package cliflags

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
)

// Diagnostic messages, one per rule of the flag standard.
const (
	messageSources   = "flag %q must bind an environment variable: Sources: cli.EnvVars(...)"
	messageValue     = "flag %q must carry a sensible default via Value"
	messageKebab     = "flag name %q must be kebab-case"
	messageRequired  = "flag %q must not be Required — a required flag is a positional argument in disguise"
	messageSnake     = "environment variable %q must be UPPERCASE_SNAKE_CASE"
	messagePrefixed  = "environment variable %q prefixes the well-known external namespace %s — use the external name unprefixed"
	messageAppPrefix = "app-specific environment variable %q must be prefixed %q"
	messageMetaEnv   = "flag %q overrides a framework meta-flag (cli.VersionFlag/cli.HelpFlag/cli.GenerateShellCompletionFlag) — an env binding there is inert (urfave reads them before Sources apply) yet advertised in help output; remove it"
)

// cliPackage is the import path of the sanctioned CLI framework.
const cliPackage = "github.com/urfave/cli/v3"

// externalPrefix is one well-known external environment-variable namespace,
// e.g. "PG" or "AWS_".
type externalPrefix string

// defaultExternals seeds the namespace list from the go-cli standard's own
// examples (PGHOST/PGPORT, AWS_REGION, DOCKER_*).
var defaultExternals = namespaces{"PG", "AWS_", "DOCKER_"}

// upperSnake is the shape of an app-specific environment variable name.
var upperSnake = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

// kebabCase is the shape of a flag name.
var kebabCase = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// externalAdded holds -external: the ADDITIONAL well-known namespaces parsed
// from the setting, on top of the defaults.
var externalAdded externalSetting

// appPrefix holds -app: the app name whose UPPERCASE_SNAKE prefix every
// app-specific environment variable must carry. Empty skips the prefix
// requirement (the analyzer cannot know the app name on its own).
var appPrefix string

// Analyzer reports urfave/cli v3 flag literals that break the flag standard.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "cliflags",
		Doc:  "reports urfave/cli v3 flag literals that break the opinionated flag standard (env Sources, Value default, kebab-case, no Required)",
		Run:  run,
	}
	a.Flags.Var(&externalAdded, "external",
		"comma-separated additional well-known external env namespaces (e.g. VAULT_,GH_)")
	a.Flags.StringVar(&appPrefix, "app", "",
		"app name whose UPPERCASE_SNAKE prefix app-specific environment variables must carry")
	return a
}

// Registration declares this analyzer to the yze framework.
var Registration = goyze.Registration{
	Name:       "cliflags",
	Categories: []goyze.Category{"cli"},
	URL:        "https://docs.gomatic.dev/yze/cliflags",
	Analyzer:   Analyzer,
}

// run inspects every composite literal for the flag rules.
func run(pass *analysis.Pass) (any, error) {
	known := externals()
	for _, file := range pass.Files {
		meta := metaFlagLiterals(pass, file)
		ast.Inspect(file, func(node ast.Node) bool {
			if lit, ok := node.(*ast.CompositeLit); ok {
				checkFlagLiteral(pass, lit, meta, known)
			}
			return true
		})
	}
	return nil, nil
}

// checkFlagLiteral applies every flag rule to one v3 flag literal. A meta-flag
// override (a member of meta) inverts the env rule: Sources are forbidden
// there instead of required.
func checkFlagLiteral(pass *analysis.Pass, lit *ast.CompositeLit, meta metaLiterals, known namespaces) {
	isFlag, hasZeroDefault := flagType(pass.TypesInfo.TypeOf(lit))
	if !isFlag {
		return
	}
	fields := collectFields(lit)
	name, isResolved := nameOf(pass, fields)
	checkName(pass, fields["Name"], name, isResolved)
	if meta[lit] {
		checkMetaSources(pass, lit, fields, name)
	} else {
		checkSources(pass, lit, fields, name, known)
	}
	if !hasZeroDefault {
		checkValue(pass, lit, fields, name)
	}
	checkRequired(pass, fields, name)
}

// fieldMap indexes a composite literal's keyed elements by field name.
type fieldMap map[string]*ast.KeyValueExpr

// collectFields indexes lit's keyed elements by field name; a struct literal
// spells its keys as plain identifiers. An UNKEYED element is skipped rather
// than exempting the whole literal. The exemption it replaces claimed "field
// absent is meaningless in a positional literal", and that shape does not
// exist in judged code: every v3 flag struct carries unexported fields, so an
// unkeyed literal of one is a compile error outside the framework's own
// package ("too few values in struct literal of type cli.StringFlag",
// v3.10.1). Measured against that module — the only place the shape compiles —
// dropping the exemption left all 1138 findings over its 26 packages
// byte-identical, which is dead defensive code rather than a rule.
func collectFields(lit *ast.CompositeLit) fieldMap {
	fields := fieldMap{}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, isIdent := kv.Key.(*ast.Ident); isIdent {
				fields[key.Name] = kv
			}
		}
	}
	return fields
}

// flagName is a flag's Name value as written in source.
type flagName string

// nameResolved reports whether a flag literal declares a constant Name at all.
// It is a different fact from the name being empty, which is a name.
type nameResolved bool

// nameOf resolves the flag's Name field to its constant value. The second
// result says whether there is a name to judge at all — a literal declaring no
// Name, or declaring it non-constantly, has none. That is a different fact
// from declaring the EMPTY name, and returning "" for both put `Name: ""`
// outside the kebab-case rule it plainly violates.
func nameOf(pass *analysis.Pass, fields fieldMap) (flagName, nameResolved) {
	kv := fields["Name"]
	if kv == nil {
		return "", false
	}
	name, ok := constantString(pass, kv.Value)
	return flagName(name), nameResolved(ok)
}

// constantString resolves expr to its constant string value, following named
// constants, or reports false for anything non-constant.
func constantString(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	value := pass.TypesInfo.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

// checkName reports a flag name that is not kebab-case. isResolved is the
// caller's answer to whether a constant name was found; the empty name IS one,
// and is judged like any other, because urfave registers a flag nothing can
// spell.
func checkName(pass *analysis.Pass, kv *ast.KeyValueExpr, name flagName, isResolved nameResolved) {
	if !isResolved {
		return
	}
	if !kebabCase.MatchString(string(name)) {
		pass.Reportf(kv.Value.Pos(), messageKebab, name)
	}
}

// checkSources reports a flag that binds no environment variable, and checks
// every name it does bind. A Sources field with no cli.EnvVars(...) or
// cli.EnvVar(...) call — or only empty ones — binds nothing.
func checkSources(pass *analysis.Pass, lit *ast.CompositeLit, fields fieldMap, name flagName, known namespaces) {
	calls := envVarCalls(pass, fields["Sources"])
	if !bindsEnv(calls) {
		pass.Reportf(lit.Pos(), messageSources, name)
		return
	}
	for _, call := range calls {
		checkEnvNames(pass, call, known)
	}
}

// envVarCalls finds every cli.EnvVars/cli.EnvVar call within the Sources
// field's value; empty when the field is absent or wires no environment
// source.
func envVarCalls(pass *analysis.Pass, kv *ast.KeyValueExpr) []*ast.CallExpr {
	if kv == nil {
		return nil
	}
	var found []*ast.CallExpr
	ast.Inspect(kv.Value, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && isEnvVarFunc(pass, call.Fun) {
			found = append(found, call)
		}
		return true
	})
	return found
}

// bindsEnv reports whether any found call names at least one variable.
func bindsEnv(calls []*ast.CallExpr) bool {
	for _, call := range calls {
		if len(call.Args) > 0 {
			return true
		}
	}
	return false
}

// isEnvVarFunc reports whether fun resolves to urfave/cli v3's EnvVars (the
// chain form) or EnvVar (the single-source form used inside
// cli.NewValueSourceChain) — both are first-class upstream bindings.
func isEnvVarFunc(pass *analysis.Pass, fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || obj.Pkg() == nil || obj.Pkg().Path() != cliPackage {
		return false
	}
	return obj.Name() == "EnvVars" || obj.Name() == "EnvVar"
}

// checkValue reports a flag with no Value default. The caller exempts value
// types whose zero IS the sensible default (booleans, slices).
func checkValue(pass *analysis.Pass, lit *ast.CompositeLit, fields fieldMap, name flagName) {
	if fields["Value"] == nil {
		pass.Reportf(lit.Pos(), messageValue, name)
	}
}

// checkRequired reports a flag marked Required.
func checkRequired(pass *analysis.Pass, fields fieldMap, name flagName) {
	kv := fields["Required"]
	if kv == nil {
		return
	}
	value := pass.TypesInfo.Types[kv.Value].Value
	if value != nil && value.Kind() == constant.Bool && constant.BoolVal(value) {
		pass.Reportf(kv.Pos(), messageRequired, name)
	}
}
