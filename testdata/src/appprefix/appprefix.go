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
			prefixed(), unprefixed(), external(), externalBare(),
			namespaceIsNotAPrefix(), namespaceIsNotASegment(),
			stemInsideASegment(), malformedName(),
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

// externalBare is the bare-namespace side of the same exemption: libpq spells
// its variables as ONE segment extending PG, and PGHOST is one.
func externalBare() cli.Flag {
	return &cli.StringFlag{
		Name:    "db-host",
		Value:   "localhost",
		Sources: cli.EnvVars("PGHOST"),
	}
}

// namespaceIsNotAPrefix differs from externalBare in exactly one place: the
// stem is followed by a SECOND segment. libpq owns no two-segment variable, so
// this is an app variable wearing two of libpq's letters, and the app prefix
// still applies. Matching the namespace by strings.HasPrefix on the whole name
// exempted it — two letters bought silence and acquired none of the property
// the exemption exists for.
func namespaceIsNotAPrefix() cli.Flag {
	return &cli.StringFlag{
		Name:    "bank",
		Value:   "x",
		Sources: cli.EnvVars("PGGY_BANK"), // want `must be prefixed "MY_KILROY_"`
	}
}

// stemInsideASegment differs from a wrapped namespace in exactly one place:
// the final segment CONTAINS the stem instead of starting with it. XPG is no
// more libpq's than BANK is, so the app prefix applies and the wrapped-external
// rule stays quiet.
func stemInsideASegment() cli.Flag {
	return &cli.StringFlag{
		Name:    "xpg",
		Value:   "x",
		Sources: cli.EnvVars("OTHER_XPG"), // want `must be prefixed "MY_KILROY_"`
	}
}

// malformedName is not UPPERCASE_SNAKE_CASE, and the shape rule is the only one
// that judges it: a name that is not a variable name yet has no namespace to
// wrap and no prefix to carry, so exactly one finding lands on it.
func malformedName() cli.Flag {
	return &cli.StringFlag{
		Name:    "debug",
		Value:   "info",
		Sources: cli.EnvVars("my_kilroy_debug"), // want `must be UPPERCASE_SNAKE_CASE`
	}
}

// namespaceIsNotASegment is the underscore-terminated form of the same
// near-miss: DOCKERISH is not DOCKER_, because a terminated namespace owns a
// segment EQUAL to its stem rather than one starting with it.
func namespaceIsNotASegment() cli.Flag {
	return &cli.StringFlag{
		Name:    "socket",
		Value:   "x",
		Sources: cli.EnvVars("DOCKERISH_SOCKET"), // want `must be prefixed "MY_KILROY_"`
	}
}
