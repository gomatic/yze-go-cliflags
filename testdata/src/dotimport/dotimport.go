// Package dotimport names the framework's meta-flag storage through a DOT
// import, so every assignment target is a bare identifier rather than a
// selector. Identity is the variable — its package and its type — so the
// spelling changes nothing, and the same rules apply here as anywhere.
package dotimport

import (
	. "github.com/urfave/cli/v3"
)

// Configure installs a sole override with no qualifier in sight. It reaches the
// framework, so the Sources requirement is inverted and nothing is reported.
func Configure() {
	HelpFlag = &BoolFlag{Name: "help", Usage: "show help"}
}

// Forge writes an ordinary flag to the same variable and replaces it on the
// very next statement. The write installs nothing, so the literal is judged as
// the ordinary flag it is — the spelling buys no exemption a selector would not
// have bought either.
func Forge() Flag {
	forged := &StringFlag{ // want `must bind an environment variable`
		Name:  "forged",
		Value: "x",
	}
	HelpFlag = forged
	HelpFlag = &BoolFlag{Name: "help"}
	return forged
}

// Indexed writes through an index expression, which names no variable at all,
// so there is nothing for the meta rule to resolve and the ordinary rules hold.
func Indexed(flags []Flag) {
	flags[0] = &BoolFlag{Name: "indexed", Sources: EnvVars("APP_INDEXED")}
}
