// Package metaenv is the fixture for the meta-flag env rule alone: three
// overrides that differ only in what their Sources field holds, so the line
// between "reports an env binding" and "reports Sources" is the only thing the
// package can be failing about.
//
// app.go proves the reported direction from several angles; what it never
// spells is a meta-flag override whose Sources bind NO variable. The rule is
// not "a meta-flag carries no Sources" — a Sources that names nothing is not
// advertised in help output and cannot mislead anybody, so there is nothing
// inert to remove and nothing to report.
package metaenv

import (
	cli "github.com/urfave/cli/v3"
)

// bound is the defect: an override reaching the framework with a variable named
// on it. urfave applies Sources in FlagBase.PostParse, after the version check,
// so the binding never triggers version output while the generated help still
// advertises the variable.
func bound() {
	cli.VersionFlag = &cli.BoolFlag{ // want `flag "version" overrides a framework meta-flag`
		Name:    "version",
		Sources: cli.EnvVars("METAENV_VERSION"),
	}
}

// fileSourced wires Sources on a meta-flag override without binding an
// environment variable. Nothing about it is advertised as a variable, so the
// meta rule is silent — and the ordinary Sources requirement, which this
// override IS exempt from, must not fire in its place.
func fileSourced() {
	cli.HelpFlag = &cli.BoolFlag{
		Name:    "help",
		Sources: cli.Files("help.yaml"),
	}
}

// emptyEnvVars is the closest a silent override gets to the reported one: the
// framework's own env constructor is called, on a meta-flag override, and names
// nothing. It separates "an EnvVars call is present" from "a variable is
// bound", which is the boundary the rule is actually drawn at.
func emptyEnvVars() {
	cli.GenerateShellCompletionFlag = &cli.BoolFlag{
		Name:    "generate-shell-completion",
		Sources: cli.EnvVars(),
	}
}
