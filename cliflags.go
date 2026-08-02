// Package cliflags provides a go/analysis analyzer enforcing the flag shape of
// the opinionated CLI standard on every urfave/cli v3 flag literal. Each rule
// is one line of the go-cli standard:
//
//   - every flag binds an environment variable via Sources: cli.EnvVars(...)
//   - every non-boolean flag carries a sensible default via Value
//   - flag names are kebab-case
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
// the standard's own examples and extended with -external.
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
)

// cliPackage is the import path of the sanctioned CLI framework.
const cliPackage = "github.com/urfave/cli/v3"

// externalPrefix is one well-known external environment-variable namespace,
// e.g. "PG" or "AWS_".
type externalPrefix string

// defaultExternals seeds the namespace list from the go-cli standard's own
// examples (PGHOST/PGPORT, AWS_REGION, DOCKER_*).
var defaultExternals = []externalPrefix{"PG", "AWS_", "DOCKER_"}

// upperSnake is the shape of an app-specific environment variable name.
var upperSnake = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

// kebabCase is the shape of a flag name.
var kebabCase = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// externalExtra holds -external: comma-separated ADDITIONAL well-known
// namespaces, on top of the defaults.
var externalExtra string

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
	a.Flags.StringVar(&externalExtra, "external", "",
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
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if lit, ok := node.(*ast.CompositeLit); ok {
				checkFlagLiteral(pass, lit)
			}
			return true
		})
	}
	return nil, nil
}

// checkFlagLiteral applies every flag rule to one v3 flag literal. An unkeyed
// literal is skipped: it specifies every field positionally, so "field absent"
// is meaningless there, and vet's composites check already forbids unkeyed
// literals of another module's structs.
func checkFlagLiteral(pass *analysis.Pass, lit *ast.CompositeLit) {
	isFlag, isBool := flagType(pass.TypesInfo.TypeOf(lit))
	if !isFlag || !isKeyed(lit) {
		return
	}
	fields := collectFields(lit)
	name := nameOf(pass, fields)
	checkName(pass, fields, name)
	checkSources(pass, lit, fields, name)
	if !isBool {
		checkValue(pass, lit, fields, name)
	}
	checkRequired(pass, fields, name)
}

// flagType reports whether t is a urfave/cli v3 flag type, and whether the
// flag's value type is bool. v3 spells StringFlag/BoolFlag/... as aliases of
// the generic FlagBase, so the unaliased type is a FlagBase instantiation and
// the first type argument is the value type.
func flagType(t types.Type) (isFlag, isBool bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != cliPackage {
		return false, false
	}
	if named.Obj().Name() != "FlagBase" {
		return false, false
	}
	return true, isBoolArgument(named)
}

// isBoolArgument reports whether the FlagBase instantiation's value type is bool.
func isBoolArgument(named *types.Named) bool {
	arguments := named.TypeArgs()
	if arguments == nil || arguments.Len() == 0 {
		return false
	}
	basic, ok := types.Unalias(arguments.At(0)).(*types.Basic)
	return ok && basic.Kind() == types.Bool
}

// isKeyed reports whether every element of lit is a keyed field.
func isKeyed(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		if _, ok := elt.(*ast.KeyValueExpr); !ok {
			return false
		}
	}
	return true
}

// fieldMap indexes a composite literal's keyed elements by field name.
type fieldMap map[string]*ast.KeyValueExpr

// collectFields indexes lit's keyed elements by field name. The caller has
// already established every element is keyed, and a struct literal spells its
// keys as plain identifiers.
func collectFields(lit *ast.CompositeLit) fieldMap {
	fields := fieldMap{}
	for _, elt := range lit.Elts {
		kv, _ := elt.(*ast.KeyValueExpr)
		if key, ok := kv.Key.(*ast.Ident); ok {
			fields[key.Name] = kv
		}
	}
	return fields
}

// flagName is a flag's Name value as written in source.
type flagName string

// nameOf resolves the flag's Name field to its constant value, or "" when the
// literal declares none (or declares it non-constantly).
func nameOf(pass *analysis.Pass, fields fieldMap) flagName {
	kv := fields["Name"]
	if kv == nil {
		return ""
	}
	name, _ := constantString(pass, kv.Value)
	return flagName(name)
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

// checkName reports a flag name that is not kebab-case.
func checkName(pass *analysis.Pass, fields fieldMap, name flagName) {
	kv := fields["Name"]
	if kv == nil || name == "" {
		return
	}
	if !kebabCase.MatchString(string(name)) {
		pass.Reportf(kv.Value.Pos(), messageKebab, name)
	}
}

// checkSources reports a flag that binds no environment variable, and checks
// the names it does bind. A Sources field without a cli.EnvVars(...) call — or
// with an empty one — binds nothing.
func checkSources(pass *analysis.Pass, lit *ast.CompositeLit, fields fieldMap, name flagName) {
	call := envVarsCall(pass, fields["Sources"])
	if call == nil || len(call.Args) == 0 {
		pass.Reportf(lit.Pos(), messageSources, name)
		return
	}
	checkEnvNames(pass, call)
}

// envVarsCall finds the cli.EnvVars call within the Sources field's value, or
// nil when the field is absent or wires no environment source.
func envVarsCall(pass *analysis.Pass, kv *ast.KeyValueExpr) *ast.CallExpr {
	if kv == nil {
		return nil
	}
	var found *ast.CallExpr
	ast.Inspect(kv.Value, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && found == nil && isEnvVarsFunc(pass, call.Fun) {
			found = call
		}
		return found == nil
	})
	return found
}

// isEnvVarsFunc reports whether fun resolves to urfave/cli v3's EnvVars.
func isEnvVarsFunc(pass *analysis.Pass, fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && obj.Name() == "EnvVars" && obj.Pkg() != nil && obj.Pkg().Path() == cliPackage
}

// checkValue reports a flag with no Value default. The caller exempts
// booleans: a boolean's zero value IS its sensible default — the standard's
// own BoolFlag example omits Value.
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
