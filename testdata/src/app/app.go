// Package app exercises every flag rule against the v3 stub: conformant flags
// are silent; each violation is reported by exactly one rule.
package app

import (
	cli "github.com/urfave/cli/v3"
)

// Environment names resolved through named constants prove the checks read
// constant VALUES, not just literals.
const badEnv = "bad_env"

// nonConstant values name nothing checkable and are left alone.
var (
	dynamicEnv  = "DYNAMIC"
	dynamicName = "dynamic"
	maybe       = true
)

// Command wires the fixture flags.
func Command() *cli.Command {
	return &cli.Command{
		Name: "app",
		Flags: []cli.Flag{
			good(), goodBool(), goodInt(),
			noSources(), fileSources(), emptyEnvVars(), positional(), wrapped(),
			noValue(), camelName(), required(), requiredNonConstant(),
			prefixedExternal(), lowerSnake(), constantEnv(), dynamic(), nameless(),
		},
	}
}

// good is fully conformant: kebab name, unprefixed well-known external
// variable, a Value default.
func good() cli.Flag {
	return &cli.StringFlag{
		Name:    "db-host",
		Usage:   "PostgreSQL host",
		Value:   "localhost",
		Sources: cli.EnvVars("PGHOST"),
	}
}

// goodBool omits Value: a boolean's zero value is its sensible default.
func goodBool() cli.Flag {
	return &cli.BoolFlag{
		Name:    "pretty",
		Sources: cli.EnvVars("JSONL_PRETTY"),
	}
}

// goodInt binds several app variables at once; all are checked, none report.
func goodInt() cli.Flag {
	return &cli.IntFlag{
		Name:    "max-records",
		Value:   1000,
		Sources: cli.EnvVars("JSONL_MAX_RECORDS", "MYAPP_DSN"),
	}
}

// noSources binds no environment variable at all.
func noSources() cli.Flag {
	return &cli.StringFlag{ // want `must bind an environment variable`
		Name:  "output",
		Value: "out.txt",
	}
}

// fileSources wires Sources without an EnvVars call, which binds no
// environment variable either.
func fileSources() cli.Flag {
	return &cli.StringFlag{ // want `must bind an environment variable`
		Name:    "config",
		Value:   "cfg.yaml",
		Sources: cli.Files("cfg.yaml"),
	}
}

// emptyEnvVars calls EnvVars with no names, binding nothing.
func emptyEnvVars() cli.Flag {
	return &cli.StringFlag{ // want `must bind an environment variable`
		Name:    "empty",
		Value:   "x",
		Sources: cli.EnvVars(),
	}
}

// positional writes the literal without field keys: every field is specified
// positionally, so "field absent" is meaningless and vet's composites check
// owns the shape. Nothing is reported here.
func positional() cli.Flag {
	f := cli.StringFlag{"positional", nil, "u", "v", false, cli.ValueSourceChain{}, nil}
	return &f
}

// chain hides the EnvVars call behind a local wrapper.
func chain(names ...string) cli.ValueSourceChain { return cli.EnvVars(names...) }

// wrapped binds through the local wrapper: the binding is invisible at the
// literal, so the flag does not visibly satisfy the standard — write the
// cli.EnvVars call in the literal.
func wrapped() cli.Flag {
	return &cli.StringFlag{ // want `must bind an environment variable`
		Name:    "wrapped",
		Value:   "x",
		Sources: chain("MYAPP_WRAPPED"),
	}
}

// noValue is a non-boolean flag with no default.
func noValue() cli.Flag {
	return &cli.IntFlag{ // want `must carry a sensible default via Value`
		Name:    "batch-size",
		Sources: cli.EnvVars("MYAPP_BATCH_SIZE"),
	}
}

// camelName is not kebab-case.
func camelName() cli.Flag {
	return &cli.StringFlag{
		Name:    "maxRecords", // want `must be kebab-case`
		Value:   "1000",
		Sources: cli.EnvVars("MYAPP_MAX_RECORDS"),
	}
}

// required marks the flag Required — a positional argument in disguise.
func required() cli.Flag {
	return &cli.StringFlag{
		Name:     "tenant",
		Value:    "default",
		Sources:  cli.EnvVars("MYAPP_TENANT"),
		Required: true, // want `must not be Required`
	}
}

// requiredNonConstant sets Required from a non-constant expression, which the
// analyzer cannot judge and leaves alone.
func requiredNonConstant() cli.Flag {
	return &cli.StringFlag{
		Name:     "mode",
		Value:    "auto",
		Sources:  cli.EnvVars("MYAPP_MODE"),
		Required: maybe,
	}
}

// prefixedExternal wraps libpq's own variable under an app prefix; libpq will
// never read the prefixed spelling.
func prefixedExternal() cli.Flag {
	return &cli.StringFlag{
		Name:    "db-name",
		Value:   "postgres",
		Sources: cli.EnvVars("KILROY_PGDATABASE"), // want `use the external name unprefixed`
	}
}

// lowerSnake is not UPPERCASE_SNAKE_CASE.
func lowerSnake() cli.Flag {
	return &cli.StringFlag{
		Name:    "debug-level",
		Value:   "info",
		Sources: cli.EnvVars("kilroy_debug"), // want `must be UPPERCASE_SNAKE_CASE`
	}
}

// constantEnv resolves the bound name through a named constant.
func constantEnv() cli.Flag {
	return &cli.StringFlag{
		Name:    "trace-file",
		Value:   "trace.out",
		Sources: cli.EnvVars(badEnv), // want `must be UPPERCASE_SNAKE_CASE`
	}
}

// nameless declares no Name field at all; the name rules have nothing to
// judge, and the remaining rules still hold.
func nameless() cli.Flag {
	return &cli.StringFlag{
		Value:   "x",
		Sources: cli.EnvVars("MYAPP_NAMELESS"),
	}
}

// dynamic wires non-constant expressions; nothing is checkable and nothing is
// reported beyond what the other rules see.
func dynamic() cli.Flag {
	return &cli.StringFlag{
		Name:    dynamicName,
		Value:   "v",
		Sources: cli.EnvVars(dynamicEnv),
	}
}
