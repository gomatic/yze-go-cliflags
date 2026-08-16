package cliflags

// The -external setting, which is a flag.Value rather than a string because an
// entry it cannot honour has to be REFUSED at the moment it is applied.

import (
	"regexp"
	"strings"

	errs "github.com/gomatic/go-error"
)

// ErrExternalNamespace reports an -external entry that cannot name an
// environment variable, so registering it would narrow nothing. go-yze's
// ApplyConfig turns a Set error into goyze.ErrInvalidSettingValue, and a
// standalone run fails flag parsing, so the operator is told rather than left
// believing a rule is narrowed.
const ErrExternalNamespace errs.Const = "-external entry is not an environment-variable namespace"

// externalShape is what a well-known external namespace can be: an
// UPPERCASE_SNAKE stem, optionally terminated by the separating underscore —
// PG, AWS_, DOCKER_, GH_. Anything else fails to prefix a name the
// UPPERCASE_SNAKE rule admits, so it narrows nothing.
//
// This validates that an entry CAN match, not that it SHOULD. Whether a
// namespace really belongs to an external tool is the operator's claim and
// nothing here can check it: -external=OTHER_ exempts every OTHER_* variable
// from the app-prefix rule, measured. That is the flag's purpose and it is
// forgeable by design — the discriminator is that it is CONFIGURATION, written
// where docs/r01.md's fifth form (per-analyzer settings) can be inventoried and
// locked, rather than a marker the judged source can type for itself. Measured
// 2026-08-15: no repository under ~/src/github.com configures cliflags at all,
// so the refusal above changes no run in effect today.
var externalShape = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*_?$`)

// externalSetting is the parsed -external setting: the comma-separated
// additional namespaces, each trimmed of the surrounding space the form is
// ordinarily written with. Splitting on a bare comma registered " VAULT_" —
// an entry that can never prefix anything — in silence, so the author who
// wrote "X_, VAULT_" believed a rule was narrowed, the reviewer reading the
// config believed the same, and the rule was unchanged.
type externalSetting struct {
	raw   string
	added namespaces
}

// String reports the setting as written, which is what the flag package prints
// as the default. It is called on a fresh zero value to decide whether there is
// a default to print at all.
func (s externalSetting) String() string { return s.raw }

// Set parses and validates the comma-separated form, refusing the whole
// setting when any entry cannot be a namespace. An empty entry is skipped: a
// trailing comma is punctuation, not a claim about a namespace.
func (s *externalSetting) Set(value string) error {
	added := namespaces{}
	for _, entry := range strings.Split(value, ",") {
		namespace := externalPrefix(strings.TrimSpace(entry))
		if namespace == "" {
			continue
		}
		if !externalShape.MatchString(string(namespace)) {
			return ErrExternalNamespace.With(nil, "entry", entry)
		}
		added = append(added, namespace)
	}
	s.added, s.raw = added, value
	return nil
}
