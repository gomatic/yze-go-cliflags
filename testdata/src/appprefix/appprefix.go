// Package appprefix exercises the -app setting: app-specific variables must
// carry the app-name prefix, while well-known external variables stay exempt.
// The test runs this fixture with -app=my-kilroy, so the required prefix is
// MY_KILROY_.
package appprefix

import (
	cli "github.com/urfave/cli/v3"
)

// Command wires the fixture flags.
func Command() *cli.Command {
	return &cli.Command{
		Name: "appprefix",
		Flags: []cli.Flag{
			prefixed(), unprefixed(), external(),
		},
	}
}

// prefixed carries the app prefix and is silent.
func prefixed() cli.Flag {
	return &cli.StringFlag{
		Name:    "mode",
		Value:   "auto",
		Sources: cli.EnvVars("MY_KILROY_MODE"),
	}
}

// unprefixed is an app-specific variable missing the prefix.
func unprefixed() cli.Flag {
	return &cli.StringFlag{
		Name:    "level",
		Value:   "info",
		Sources: cli.EnvVars("OTHER_MODE"), // want `must be prefixed "MY_KILROY_"`
	}
}

// external is a well-known external variable: exempt from the app prefix.
func external() cli.Flag {
	return &cli.StringFlag{
		Name:    "region",
		Value:   "us-east-1",
		Sources: cli.EnvVars("AWS_REGION"),
	}
}
