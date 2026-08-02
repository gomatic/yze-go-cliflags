package cliflags

// The environment-variable naming rules: UPPERCASE_SNAKE_CASE shape, the
// well-known external namespaces (PG*, AWS_*, DOCKER_*, plus -external
// additions) used unprefixed, and the -app app-name prefix on app-specific
// variables.

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// envName is one environment variable name bound by cli.EnvVars.
type envName string

// checkEnvNames validates every constant name the EnvVars call binds. A
// non-constant argument names nothing checkable and is left alone.
func checkEnvNames(pass *analysis.Pass, call *ast.CallExpr) {
	for _, arg := range call.Args {
		if name, ok := constantString(pass, arg); ok {
			checkEnvName(pass, arg.Pos(), envName(name))
		}
	}
}

// checkEnvName applies the naming rules to one bound environment variable:
// UPPERCASE_SNAKE_CASE shape, well-known external namespaces used unprefixed,
// and (when -app is set) the app-name prefix on app-specific variables.
func checkEnvName(pass *analysis.Pass, at token.Pos, name envName) {
	if !upperSnake.MatchString(string(name)) {
		pass.Reportf(at, messageSnake, name)
		return
	}
	if hasExternalPrefix(name) {
		return
	}
	if namespace, found := prefixedExternal(name); found {
		pass.Reportf(at, messagePrefixed, name, namespace)
		return
	}
	checkAppPrefix(pass, at, name)
}

// externals is the well-known namespace list: the standard's defaults plus any
// -external additions.
func externals() []externalPrefix {
	list := defaultExternals
	for _, extra := range strings.Split(externalExtra, ",") {
		if extra != "" {
			list = append(list, externalPrefix(extra))
		}
	}
	return list
}

// hasExternalPrefix reports whether name IS a well-known external variable —
// the unprefixed use the standard requires.
func hasExternalPrefix(name envName) bool {
	for _, namespace := range externals() {
		if strings.HasPrefix(string(name), string(namespace)) {
			return true
		}
	}
	return false
}

// prefixedExternal reports the well-known namespace that name wraps under an
// app prefix — the KILROY_PGHOST shape the standard forbids, because libpq
// already owns PGHOST and will never read the prefixed spelling.
func prefixedExternal(name envName) (externalPrefix, bool) {
	for _, namespace := range externals() {
		if strings.Contains(string(name), "_"+string(namespace)) {
			return namespace, true
		}
	}
	return "", false
}

// checkAppPrefix reports an app-specific variable missing the app-name prefix.
// Without -app the rule is skipped: knowing the app name takes configuration.
func checkAppPrefix(pass *analysis.Pass, at token.Pos, name envName) {
	if appPrefix == "" {
		return
	}
	want := strings.ToUpper(strings.ReplaceAll(appPrefix, "-", "_")) + "_"
	if !strings.HasPrefix(string(name), want) {
		pass.Reportf(at, messageAppPrefix, name, want)
	}
}
