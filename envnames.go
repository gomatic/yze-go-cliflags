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

// envPrefix is the UPPERCASE_SNAKE prefix -app requires on an app-specific
// environment variable.
type envPrefix string

// segmentIndex addresses one underscore-separated segment of a variable name.
type segmentIndex int

// namespaces is one run's well-known external namespace list: the standard's
// defaults followed by every -external addition the setting accepted.
type namespaces []externalPrefix

// checkEnvNames validates every constant name the EnvVars call binds. A
// non-constant argument names nothing checkable and is left alone.
func checkEnvNames(pass *analysis.Pass, call *ast.CallExpr, known namespaces) {
	for _, arg := range call.Args {
		if name, ok := constantString(pass, arg); ok {
			checkEnvName(pass, arg.Pos(), envName(name), known)
		}
	}
}

// checkEnvName applies the naming rules to one bound environment variable:
// UPPERCASE_SNAKE_CASE shape, well-known external namespaces used unprefixed,
// and (when -app is set) the app-name prefix on app-specific variables.
func checkEnvName(pass *analysis.Pass, at token.Pos, name envName, known namespaces) {
	if !upperSnake.MatchString(string(name)) {
		pass.Reportf(at, messageSnake, name)
		return
	}
	if isExternal(known, name) {
		return
	}
	if namespace, found := prefixedExternal(known, name); found {
		pass.Reportf(at, messagePrefixed, name, namespace)
		return
	}
	checkAppPrefix(pass, at, name)
}

// externals is the well-known namespace list: the standard's defaults plus any
// -external additions, which the setting validated when it was applied.
func externals() namespaces {
	return append(append(namespaces{}, defaultExternals...), externalAdded.added...)
}

// segmentMatches reports whether segments[i] is a variable of the namespace's
// OWN naming. This is the single theory of what a namespace owns, applied at
// every position: an underscore-terminated namespace (AWS_) owns a segment
// equal to its stem with more segments following, and a bare namespace (PG)
// owns a FINAL segment that EXTENDS its stem, because libpq spells its
// variables as one segment. So PGHOST and AWS_REGION are external, PGGY_BANK
// and MYAPP_PGP_KEY are app variables, and KILROY_PGHOST and KILROY_AWS_REGION
// are external namespaces wrapped under an app prefix. Deciding the leading
// segment by strings.HasPrefix and the rest by this rule gave one run two
// theories of the same namespace, and put every name merely STARTING with a
// stem outside the app-prefix rule for the price of two letters.
func segmentMatches(segments []string, at segmentIndex, namespace externalPrefix) bool {
	stem := strings.TrimSuffix(string(namespace), "_")
	last := segmentIndex(len(segments)) - 1
	if strings.HasSuffix(string(namespace), "_") {
		return segments[at] == stem && at < last
	}
	return at == last && segments[at] != stem && strings.HasPrefix(segments[at], stem)
}

// isExternal reports whether name IS a well-known external variable — the
// unprefixed use the standard requires — by asking whether its LEADING segment
// is the namespace's own.
func isExternal(known namespaces, name envName) bool {
	segments := strings.Split(string(name), "_")
	for _, namespace := range known {
		if segmentMatches(segments, 0, namespace) {
			return true
		}
	}
	return false
}

// prefixedExternal reports the well-known namespace that name wraps under an
// app prefix — the KILROY_PGHOST shape the standard forbids, because libpq
// already owns PGHOST and will never read the prefixed spelling.
func prefixedExternal(known namespaces, name envName) (externalPrefix, bool) {
	segments := strings.Split(string(name), "_")
	for _, namespace := range known {
		if wrapsNamespace(segments, namespace) {
			return namespace, true
		}
	}
	return "", false
}

// wrapsNamespace reports whether a NON-LEADING segment is the namespace's own
// variable. The scan starts at 1 because isExternal owns the leading segment
// and checkEnvName returns before this runs; starting at 0 is INERT rather than
// wrong, so it takes no case.
func wrapsNamespace(segments []string, namespace externalPrefix) bool {
	for at := segmentIndex(1); at < segmentIndex(len(segments)); at++ {
		if segmentMatches(segments, at, namespace) {
			return true
		}
	}
	return false
}

// appWant is the prefix -app requires: the app name uppercased, hyphens turned
// into underscores, and one trailing underscore (my-kilroy becomes MY_KILROY_).
func appWant() envPrefix {
	return envPrefix(strings.ToUpper(strings.ReplaceAll(appPrefix, "-", "_")) + "_")
}

// checkAppPrefix reports an app-specific variable missing the app-name prefix.
// Without -app the rule is skipped: knowing the app name takes configuration.
func checkAppPrefix(pass *analysis.Pass, at token.Pos, name envName) {
	if appPrefix == "" {
		return
	}
	want := appWant()
	if !strings.HasPrefix(string(name), string(want)) {
		pass.Reportf(at, messageAppPrefix, name, want)
	}
}
