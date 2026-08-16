// Package cli is a minimal stub of urfave/cli v3 for analysistest fixtures,
// mirroring the generic FlagBase shape the real module spells its flag types
// with (type StringFlag = FlagBase[string, ...]).
package cli

// ValueSourceChain is a stand-in for the v3 sources chain.
type ValueSourceChain struct{ names []string }

// EnvVars is the stub of the v3 environment-source constructor.
func EnvVars(names ...string) ValueSourceChain { return ValueSourceChain{names: names} }

// Files is a non-environment source, for the Sources-without-EnvVars fixture.
func Files(paths ...string) ValueSourceChain { return ValueSourceChain{names: paths} }

// ValueSource is one source in a chain.
type ValueSource struct{ name string }

// EnvVar is the single-variable environment source.
func EnvVar(key string) ValueSource { return ValueSource{name: key} }

// NewValueSourceChain assembles a chain from single sources.
func NewValueSourceChain(src ...ValueSource) ValueSourceChain {
	names := make([]string, 0, len(src))
	for _, s := range src {
		names = append(names, s.name)
	}
	return ValueSourceChain{names: names}
}

// BoolWithInverseFlag mirrors the framework's non-generic flag struct: no
// FlagBase alias, its own fields, a bool Value — and the unexported fields the
// real one carries, which are what make an unkeyed literal of it a compile
// error in every package but this one.
type BoolWithInverseFlag struct {
	Name        string
	Aliases     []string
	Usage       string
	Value       bool
	Required    bool
	Sources     ValueSourceChain
	Destination *bool

	count      int
	hasBeenSet bool
}

// FlagBase mirrors the generic base every v3 flag type aliases, unexported
// fields included: without them the stub admits an unkeyed literal the real
// module rejects, and a fixture asserting silence there is silent for a reason
// only the stub can produce.
type FlagBase[T any, C any, VC any] struct {
	Name        string
	Aliases     []string
	Usage       string
	Value       T
	Required    bool
	Sources     ValueSourceChain
	Destination *T

	count      int
	hasBeenSet bool
}

// Per-type configs and value creators, as in the real module.
type (
	StringConfig      struct{}
	BoolConfig        struct{}
	IntConfig         struct{}
	StringSliceConfig struct{}
	StringMapConfig   struct{}
	stringValue       struct{}
	boolValue         struct{}
	intValue          struct{}
	stringSliceValue  struct{}
	stringMapValue    struct{}
)

// The flag aliases the fixtures use. StringMapFlag is here because the real
// module ships one: a map-valued flag is the container whose zero is empty
// exactly as a slice's is, and a stub without it cannot see the rule decide.
type (
	StringFlag      = FlagBase[string, StringConfig, stringValue]
	BoolFlag        = FlagBase[bool, BoolConfig, boolValue]
	IntFlag         = FlagBase[int, IntConfig, intValue]
	StringSliceFlag = FlagBase[[]string, StringSliceConfig, stringSliceValue]
	StringMapFlag   = FlagBase[map[string]string, StringMapConfig, stringMapValue]
)

// Duration mirrors a named non-basic value type (the real module uses
// time.Duration).
type Duration int64

type (
	NoConfig      struct{}
	durationValue struct{}
)

// DurationFlag is the alias whose value type is a named type, not a basic one.
type DurationFlag = FlagBase[Duration, NoConfig, durationValue]

// Flag is the v3 flag interface stand-in.
type Flag any

// The package-level Flag variables an app may override. The real module
// declares exactly these three (flag.go:22,28,39) and no other Flag-typed
// package variable; GenerateShellCompletionFlag is here because leaving it out
// is what let the analyzer judge it as an ordinary flag.
var (
	GenerateShellCompletionFlag Flag
	VersionFlag                 Flag
	HelpFlag                    Flag
)

// FlagStringer mirrors a package-level variable of the framework that is NOT a
// flag slot: same package, not Flag-typed.
var FlagStringer func(Flag) string

// Command is a stand-in for the v3 Command type.
type Command struct {
	Name  string
	Flags []Flag
}
