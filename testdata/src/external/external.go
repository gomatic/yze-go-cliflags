// Package external exercises the -external setting: the test runs this
// fixture with -external=VAULT_, extending the well-known namespace list.
package external

import (
	cli "github.com/urfave/cli/v3"
)

// Command wires the fixture flags.
func Command() *cli.Command {
	return &cli.Command{
		Name: "external",
		Flags: []cli.Flag{
			vault(), prefixedVault(),
		},
	}
}

// vault uses the extended namespace unprefixed and is silent.
func vault() cli.Flag {
	return &cli.StringFlag{
		Name:    "vault-addr",
		Value:   "https://vault:8200",
		Sources: cli.EnvVars("VAULT_ADDR"),
	}
}

// prefixedVault wraps the extended namespace under an app prefix.
func prefixedVault() cli.Flag {
	return &cli.StringFlag{
		Name:    "vault-token",
		Value:   "",
		Sources: cli.EnvVars("KILROY_VAULT_TOKEN"), // want `use the external name unprefixed`
	}
}
