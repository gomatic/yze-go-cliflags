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

// ONE POLICY, TWO POSITIONS, AND WHY THEY ARE NOT THE SAME TEST. Both matchers
// below answer "does this segment belong to the namespace", and both err toward
// SILENCE — because a false report on an environment variable name is one the
// author cannot act on, and an unactionable finding is answered with a
// baseline. The uncertainty falls on opposite sides at the two positions, so
// erring toward silence makes the LEADING test wide and the inner test narrow.
// An earlier revision unified them on the narrow reading; that is the shape the
// tests below exist to keep out, and the paragraph on leadsNamespace records
// what it cost.

// leadsNamespace reports whether the LEADING segment is the namespace's own,
// which is the unprefixed use the standard requires. Deliberately wide: a bare
// namespace (PG) owns any leading segment starting with its stem, and a
// terminated one (AWS_) owns a leading segment equal to its stem with more to
// follow. Narrowing the bare case to a name of ONE segment was tried and
// REVERTED on measurement: libpq does not spell all of its variables as one
// segment — PGCONNECT_TIMEOUT and PG_COLOR are two — so the narrow reading
// reported real libpq variables and prescribed MYAPP_PGCONNECT_TIMEOUT, a name
// libpq will never read. The cost of being wide is real and is not closable
// here: PGGY_BANK is exempt too, and no prefix match distinguishes it from
// PGCONNECT_TIMEOUT, because "belongs to libpq" is a membership question and
// not a shape. Filed as cliflags.external-name-is-a-namespace-member.
func leadsNamespace(segments []string, namespace externalPrefix) bool {
	stem := strings.TrimSuffix(string(namespace), "_")
	if strings.HasSuffix(string(namespace), "_") {
		return segments[0] == stem && len(segments) > 1
	}
	return strings.HasPrefix(segments[0], stem)
}

// wrapsAt reports whether a NON-LEADING segment is the namespace's own.
// Deliberately narrow, for the mirror-image reason: telling an author that
// MYAPP_PGP_KEY wraps libpq is as unactionable as telling them to prefix
// PGHOST. A terminated namespace owns a segment equal to its stem with more to
// follow (KILROY_AWS_REGION, not KILROY_AWS); a bare one owns a FINAL segment
// that EXTENDS its stem (KILROY_PGHOST, not KILROY_PG and not MYAPP_PGP_KEY).
func wrapsAt(segments []string, at segmentIndex, namespace externalPrefix) bool {
	stem := strings.TrimSuffix(string(namespace), "_")
	last := segmentIndex(len(segments)) - 1
	if strings.HasSuffix(string(namespace), "_") {
		return segments[at] == stem && at < last
	}
	return at == last && segments[at] != stem && strings.HasPrefix(segments[at], stem)
}

// isExternal reports whether name IS a well-known external variable, by asking
// whether its leading segment is the namespace's own.
func isExternal(known namespaces, name envName) bool {
	segments := strings.Split(string(name), "_")
	for _, namespace := range known {
		if leadsNamespace(segments, namespace) {
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

// wrapsNamespace reports whether any non-leading segment is the namespace's
// own. The scan starts at 1 because isExternal owns the leading segment and
// checkEnvName returns before this runs; starting at 0 is INERT rather than
// wrong, so it takes no case. That inertness belongs to the CALL SITE, not to
// this function: a second caller of prefixedExternal that does not pre-filter
// with isExternal would make the start index load-bearing, silently.
func wrapsNamespace(segments []string, namespace externalPrefix) bool {
	for at := segmentIndex(1); at < segmentIndex(len(segments)); at++ {
		if wrapsAt(segments, at, namespace) {
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
