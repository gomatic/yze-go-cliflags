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
			vault(), prefixedVault(), vaultish(),
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

// vaultish is the does-not-apply case for the added namespace, written against
// the MATCHER and not the description: an added namespace is judged by the same
// theory as a seeded one, so a leading segment that merely STARTS with VAULT is
// not the registered namespace. It differs from vault() in exactly one place —
// the leading segment is VAULTISH rather than VAULT — and no app prefix is set
// in this fixture, so the only rule that could speak stays quiet.
func vaultish() cli.Flag {
	return &cli.StringFlag{
		Name:    "vaultish-addr",
		Value:   "x",
		Sources: cli.EnvVars("VAULTISH_ADDR"),
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
