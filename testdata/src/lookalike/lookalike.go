// Package lookalike declares a variable spelled and typed exactly like the
// framework's meta-flag storage, in a package that is not the framework. It
// exists so a fixture can write cli.VersionFlag's NAME through a selector
// without writing the framework's variable — the one shape that separates
// "identity is the package" from "identity is the spelling".
package lookalike

import (
	cli "github.com/urfave/cli/v3"
)

// VersionFlag is this package's own storage, owned by nobody the framework
// reads.
var VersionFlag cli.Flag
