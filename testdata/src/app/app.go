// Package app exercises every flag rule against the v3 stub: conformant flags
// are silent; each violation is reported by exactly one rule.
package app

import (
	"context"

	cli "github.com/urfave/cli/v3"

	"lookalike"
)

// Environment names resolved through named constants prove the checks read
// constant VALUES, not just literals.
const badEnv = "bad_env"

// noName is the EMPTY name reached through a constant: a name the literal
// declares, unlike a Name field that is absent or non-constant.
const noName = ""

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
			good(), goodBool(), goodInt(), goodSlice(), goodMap(), goodDuration(),
			noSources(), fileSources(), emptyEnvVars(), wrapped(), methodWrapped(), forgedEnvVars(), invokedLiteral(),
			noValue(), noMapValue(), camelName(), required(), requiredFalse(), requiredNonConstant(),
			prefixedExternal(), lowerSnake(), constantEnv(), dynamic(), nameless(),
			inverse(), singularEnv(), pgpKey(), prefixedAWS(),
			bareNamespaceTail(), terminatedNamespaceTail(), terminatedStemIsAWholeSegment(),
			emptyName(), emptyConstName(),
			doubleSeparator(), leadingSeparator(), trailingSeparator(),
			kebabPair(), kebabWord(), kebabDigit(), kebabTriple(),
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

// goodSlice omits Value: a slice's zero value — empty — is its sensible
// default, like a boolean's false.
func goodSlice() cli.Flag {
	return &cli.StringSliceFlag{
		Name:    "tags",
		Sources: cli.EnvVars("MYAPP_TAGS"),
	}
}

// goodMap omits Value for the same reason goodSlice does: a map's zero is the
// empty map. It differs from goodSlice in exactly one place — the container —
// so it pins that the exemption follows the reason and not a list of two.
func goodMap() cli.Flag {
	return &cli.StringMapFlag{
		Name:    "labels",
		Sources: cli.EnvVars("MYAPP_LABELS"),
	}
}

// noMapValue proves the map exemption is the VALUE rule's alone: a map flag
// binding nothing is still reported for the binding.
func noMapValue() cli.Flag {
	return &cli.StringMapFlag{ // want `must bind an environment variable`
		Name: "annotations",
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

// goodDuration has a named (non-basic, non-slice) value type, whose zero is
// NOT presumed a sensible default — so it carries an explicit Value.
func goodDuration() cli.Flag {
	return &cli.DurationFlag{
		Name:    "shutdown-timeout",
		Value:   cli.Duration(30),
		Sources: cli.EnvVars("MYAPP_SHUTDOWN_TIMEOUT"),
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

// chain hides the EnvVars call behind a local wrapper.
func chain(names ...string) cli.ValueSourceChain { return cli.EnvVars(names...) }

// builder hides the binding behind a method, exercising the selector that
// resolves outside the framework package.
type builder struct{}

func (builder) chain(names ...string) cli.ValueSourceChain { return cli.EnvVars(names...) }

// methodWrapped binds through a method call: invisible at the literal, like
// any local wrapper.
func methodWrapped() cli.Flag {
	return &cli.StringFlag{ // want `must bind an environment variable`
		Name:    "method-wrapped",
		Value:   "x",
		Sources: builder{}.chain("MYAPP_METHOD"),
	}
}

// forger spells the framework's own binder name on a type of this package.
// methodWrapped differs from it in exactly one place — the method's NAME — so
// together they pin that the binding is recognised by the framework package it
// belongs to and not by what it is called.
type forger struct{}

func (forger) EnvVars(names ...string) cli.ValueSourceChain { return cli.EnvVars(names...) }

// forgedEnvVars binds through a method named EnvVars that is not the
// framework's: writing the name acquires none of the framework's binding, so
// the flag is reported exactly like any other that binds nothing visible.
func forgedEnvVars() cli.Flag {
	return &cli.StringFlag{ // want `must bind an environment variable`
		Name:    "forged",
		Value:   "x",
		Sources: forger{}.EnvVars("MYAPP_FORGED"),
	}
}

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

// invokedLiteral wires Sources from an immediately-invoked function literal,
// whose callee is no name at all. It differs from wrapped() in exactly one
// place: the cli.EnvVars call is written HERE rather than behind a named
// helper, so the binding is visible at the literal and nothing is reported.
func invokedLiteral() cli.Flag {
	return &cli.StringFlag{
		Name:    "invoked",
		Value:   "x",
		Sources: func() cli.ValueSourceChain { return cli.EnvVars("MYAPP_INVOKED") }(),
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

// requiredFalse is the other side of the Required boundary: the field is
// present and its constant value is false, which is the flag not being
// required, so nothing is reported.
func requiredFalse() cli.Flag {
	return &cli.StringFlag{
		Name:     "optional",
		Value:    "default",
		Sources:  cli.EnvVars("MYAPP_OPTIONAL"),
		Required: false,
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

// inverse is the framework's non-generic flag struct: every rule applies to it
// exactly as to the FlagBase aliases — no opt-out by flag type.
func inverse() cli.Flag {
	return &cli.BoolWithInverseFlag{ // want `must bind an environment variable`
		Name:     "dryRun", // want `must be kebab-case`
		Required: true,     // want `must not be Required`
	}
}

// singularEnv binds through the equally-first-class cli.EnvVar single-source
// form inside cli.NewValueSourceChain; the binding is real and silent.
func singularEnv() cli.Flag {
	return &cli.StringFlag{
		Name:    "db-port",
		Value:   "5432",
		Sources: cli.NewValueSourceChain(cli.EnvVar("PGPORT")),
	}
}

// pgpKey is an app variable whose middle segment merely starts with a
// namespace stem: PGP is not libpq's namespace, so nothing is reported.
func pgpKey() cli.Flag {
	return &cli.StringFlag{
		Name:    "signing-key",
		Value:   "",
		Sources: cli.EnvVars("MYAPP_PGP_KEY"),
	}
}

// prefixedAWS wraps an underscore-terminated namespace under an app prefix.
func prefixedAWS() cli.Flag {
	return &cli.StringFlag{
		Name:    "region",
		Value:   "us-east-1",
		Sources: cli.EnvVars("KILROY_AWS_REGION"), // want `use the external name unprefixed`
	}
}

// bareNamespaceTail ends in the bare namespace itself rather than a variable
// EXTENDING it. It differs from prefixedExternal in exactly one place — the
// final segment is PG rather than PGDATABASE — and libpq owns no variable
// called PG, so nothing is wrapped and nothing is reported.
func bareNamespaceTail() cli.Flag {
	return &cli.StringFlag{
		Name:    "pg",
		Value:   "x",
		Sources: cli.EnvVars("KILROY_PG"),
	}
}

// terminatedNamespaceTail ends in an underscore-terminated namespace's stem
// with nothing following it. It differs from prefixedAWS in exactly one place —
// the trailing REGION segment is gone — and AWS_ with nothing after it names no
// variable, so nothing is wrapped and nothing is reported.
func terminatedNamespaceTail() cli.Flag {
	return &cli.StringFlag{
		Name:    "aws",
		Value:   "x",
		Sources: cli.EnvVars("KILROY_AWS"),
	}
}

// terminatedStemIsAWholeSegment differs from prefixedAWS in exactly one place:
// the inner segment merely STARTS with the namespace's stem instead of being
// it. AWSX is not AWS_, so nothing is wrapped and nothing is reported — an
// underscore-terminated namespace owns a whole segment, not a run-on.
func terminatedStemIsAWholeSegment() cli.Flag {
	return &cli.StringFlag{
		Name:    "awsx",
		Value:   "x",
		Sources: cli.EnvVars("KILROY_AWSX_REGION"),
	}
}

// emptyName declares the EMPTY name, which matches no kebab shape and names no
// flag anyone can spell. It is a name the literal declares, unlike dynamic's.
func emptyName() cli.Flag {
	return &cli.StringFlag{
		Name:    "", // want `flag name "" must be kebab-case`
		Value:   "x",
		Sources: cli.EnvVars("MYAPP_EMPTY_NAME"),
	}
}

// emptyConstName reaches the same empty name through a constant, proving the
// judgement reads the constant VALUE and not the spelling of the expression.
func emptyConstName() cli.Flag {
	return &cli.StringFlag{
		Name:    noName, // want `flag name "" must be kebab-case`
		Value:   "x",
		Sources: cli.EnvVars("MYAPP_EMPTY_CONST_NAME"),
	}
}

// doubleSeparator, leadingSeparator and trailingSeparator are the violating
// near-misses of the kebab shape, each deviating from a conforming name in
// exactly one place: a doubled separator, a leading one, a trailing one.
func doubleSeparator() cli.Flag {
	return &cli.StringFlag{
		Name:    "a--b", // want `must be kebab-case`
		Value:   "x",
		Sources: cli.EnvVars("MYAPP_DOUBLE"),
	}
}

func leadingSeparator() cli.Flag {
	return &cli.StringFlag{
		Name:    "-a", // want `must be kebab-case`
		Value:   "x",
		Sources: cli.EnvVars("MYAPP_LEADING"),
	}
}

func trailingSeparator() cli.Flag {
	return &cli.StringFlag{
		Name:    "a-", // want `must be kebab-case`
		Value:   "x",
		Sources: cli.EnvVars("MYAPP_TRAILING"),
	}
}

// kebabPair, kebabWord, kebabDigit and kebabTriple are the conforming side of
// the same boundary: a separated pair, a bare word, a digit, and a name whose
// middle segment is a digit. A regex loose enough to admit the four above and
// one tight enough to reject these four are different regexes.
func kebabPair() cli.Flag {
	return &cli.StringFlag{Name: "a-b", Value: "x", Sources: cli.EnvVars("MYAPP_PAIR")}
}

func kebabWord() cli.Flag {
	return &cli.StringFlag{Name: "ab", Value: "x", Sources: cli.EnvVars("MYAPP_WORD")}
}

func kebabDigit() cli.Flag {
	return &cli.StringFlag{Name: "a1", Value: "x", Sources: cli.EnvVars("MYAPP_DIGIT")}
}

func kebabTriple() cli.Flag {
	return &cli.StringFlag{Name: "a-1-b", Value: "x", Sources: cli.EnvVars("MYAPP_TRIPLE")}
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

// configureMetaClean is the conformant shape: an override that binds nothing.
// The Sources requirement is exempt there, so nothing is reported.
func configureMetaClean() {
	cli.VersionFlag = &cli.BoolFlag{Name: "version", Usage: "print version information and exit"}
}

// configureMeta binds environment variables on overrides that reach the
// framework. Each is a defect: urfave's version and help checks run before
// Sources apply, so the binding is inert yet advertised in help output.
func configureMeta() {
	cli.VersionFlag = &cli.BoolFlag{ // want `flag "version" overrides a framework meta-flag`
		Name:    "version",
		Sources: cli.EnvVars("APP_VERSION"),
	}
	cli.HelpFlag = &cli.BoolFlag{ // want `flag "help" overrides a framework meta-flag`
		Name:    "help",
		Sources: cli.EnvVars("APP_HELP"),
	}
}

// configureCompletionClean overrides the third of the framework's Flag
// variables. It differs from configureMetaClean in exactly one place — which
// variable is written — and is exempt for the same reason: v3 triggers
// completion on the literal argument and never reads the variable, so a
// binding there could not work either.
func configureCompletionClean() {
	cli.GenerateShellCompletionFlag = &cli.BoolFlag{Name: "generate-shell-completion", Usage: "emit completions"}
}

// configureCompletionBound binds a variable on that same override, which is the
// same inert-yet-advertised defect the version and help overrides carry.
func configureCompletionBound() {
	cli.GenerateShellCompletionFlag = &cli.BoolFlag{ // want `flag "generate-shell-completion" overrides a framework meta-flag`
		Name:    "generate-shell-completion",
		Sources: cli.EnvVars("APP_COMPLETION"),
	}
}

// configureMetaOverwritten is the does-not-apply case written against the
// MATCHER rather than the description: sneaky is NAMED beside a meta variable
// and never reaches the framework, because the very next statement replaces the
// write with nothing in between. It stays on the ordinary rules and is reported
// like any other flag that binds nothing, while the write that does survive is
// exempt.
func configureMetaOverwritten() {
	sneaky := &cli.StringFlag{ // want `must bind an environment variable`
		Name:  "sneaky",
		Value: "x",
	}
	cli.HelpFlag = sneaky
	cli.HelpFlag = &cli.BoolFlag{Name: "help"}
}

// configureMetaInCase overwrites inside a switch case body. A case body is a
// statement list exactly as a block is, so the replaced write installs nothing
// there either — and a matcher that only knew about blocks would hand the
// exemption back for the price of a switch.
func configureMetaInCase(pick bool) {
	switch pick {
	case true:
		cased := &cli.StringFlag{ // want `must bind an environment variable`
			Name:  "cased",
			Value: "x",
		}
		cli.HelpFlag = cased
		cli.HelpFlag = &cli.BoolFlag{Name: "help"}
	}
}

// configureMetaInSelect is the same shape in a communication clause, the third
// place Go spells a statement list.
func configureMetaInSelect(ch chan int) {
	select {
	case <-ch:
	default:
		selected := &cli.StringFlag{ // want `must bind an environment variable`
			Name:  "selected",
			Value: "x",
		}
		cli.HelpFlag = selected
		cli.HelpFlag = &cli.BoolFlag{Name: "help"}
	}
}

// configureMetaSaveRestore is the canonical save / override / run / restore
// idiom, and it is entirely honest: the override REACHES the framework, runs,
// and is put back. The restore is not adjacent to it, so the override is live
// and exempt, and nothing is reported. A rule that scanned the whole statement
// list marked it dead and demanded an env binding on the help flag — the one
// thing checkMetaSources forbids, so complying with one rule violated another.
func configureMetaSaveRestore(ctx context.Context, cmd *cli.Command) {
	saved := cli.HelpFlag
	cli.HelpFlag = &cli.BoolFlag{Name: "help", Usage: "show help"}
	_ = cmd
	_ = ctx
	cli.HelpFlag = saved
}

// configureMetaOverwrittenLabelled replaces the write on the very next
// statement, which happens to carry a LABEL. A label changes nothing about
// when the assignment runs, so the earlier write is as dead as it is in
// configureMetaOverwritten and its literal is judged the same way. Reading
// only a bare assignment made this one token enough to keep the exemption.
func configureMetaOverwrittenLabelled(again bool) cli.Flag {
	labelled := &cli.StringFlag{ // want `must bind an environment variable`
		Name:  "labelled",
		Value: "x",
	}
	cli.HelpFlag = labelled
retry:
	cli.HelpFlag = &cli.BoolFlag{Name: "help"}
	if again {
		again = false
		goto retry
	}
	return labelled
}

// configureMetaOverwrittenInInit replaces it from an if INIT clause, which
// also runs exactly where the statement sits. It differs from
// configureMetaOverwrittenLabelled in exactly one place — which wrapper hides
// the replacing assignment — so the two pin the unwrapping separately.
func configureMetaOverwrittenInInit(cond bool) cli.Flag {
	inInit := &cli.StringFlag{ // want `must bind an environment variable`
		Name:  "in-init",
		Value: "x",
	}
	cli.HelpFlag = inInit
	if cli.HelpFlag = (&cli.BoolFlag{Name: "help"}); cond {
		_ = cond
	}
	return inInit
}

// configureMetaOverwrittenParenTarget replaces the write through a
// PARENTHESISED target. It differs from configureMetaOverwritten in exactly one
// place — two characters around the left-hand side — and Go compiles the two
// identically, so the earlier write is as dead here as it is there. Matching
// the target by its syntax rather than by the variable it names made those two
// characters enough to keep the exemption, in the same block, on the next line.
func configureMetaOverwrittenParenTarget() cli.Flag {
	parenthesised := &cli.StringFlag{ // want `must bind an environment variable`
		Name:  "parenthesised",
		Value: "x",
	}
	cli.HelpFlag = parenthesised
	(cli.HelpFlag) = &cli.BoolFlag{Name: "help"}
	return parenthesised
}

// configureMetaParenTargetOnly is the same spelling used HONESTLY, as a sole
// override: it installs the literal and nothing replaces it, so the meta rule
// applies and the ordinary Sources requirement does not. The same blindness
// that let the forgery through reported this one.
func configureMetaParenTargetOnly() {
	(cli.VersionFlag) = &cli.BoolFlag{Name: "version", Usage: "print the version"}
}

// configureMetaNotAdjacent puts one statement between the write and its
// replacement. That statement could read the variable, so the analyzer cannot
// say the first write was never observed, and the literal keeps the exemption.
func configureMetaNotAdjacent() cli.Flag {
	spaced := &cli.BoolFlag{Name: "spaced"}
	cli.HelpFlag = spaced
	observe()
	cli.HelpFlag = &cli.BoolFlag{Name: "help"}
	return spaced
}

// observe stands for anything that could read the framework's meta flag.
func observe() {}

// configureLookalikeVar writes another package's variable that is spelled and
// typed exactly like the framework's. Identity is the package the variable
// belongs to, so nothing is exempt and the ordinary Sources rule is reported.
func configureLookalikeVar() {
	lookalike.VersionFlag = &cli.StringFlag{ // want `must bind an environment variable`
		Name:  "lookalike-version",
		Value: "x",
	}
}

// configureMetaIndirect overrides the meta-flags through function-local
// variables: resolution follows a single := or var binding to its &lit, so the
// clean override stays silent (the ordinary Sources rule must NOT fire) and
// the env-bound override is reported exactly like the direct form.
func configureMetaIndirect() {
	clean := &cli.BoolFlag{Name: "version", Usage: "print version information and exit"}
	cli.VersionFlag = clean

	var bound = &cli.BoolFlag{ // want `flag "help" overrides a framework meta-flag`
		Name:    "help",
		Sources: cli.EnvVars("APP_HELP_INDIRECT"),
	}
	cli.HelpFlag = bound
}

// configureMetaDeclaredThenAssigned binds through a declared-then-assigned
// variable: the declaration writes nothing, so the later = is the single write
// and still resolves.
func configureMetaDeclaredThenAssigned() {
	var bound *cli.BoolFlag
	bound = &cli.BoolFlag{ // want `flag "help" overrides a framework meta-flag`
		Name:    "help",
		Sources: cli.EnvVars("APP_HELP_DECLARED"),
	}
	cli.HelpFlag = bound
}

// configureMetaReassigned writes the variable twice, which is outside the
// single-assignment resolution boundary: neither literal is treated as a
// meta-flag override, and both stay silent under the ordinary rules (kebab
// name, env-bound, boolean zero default).
func configureMetaReassigned() {
	again := &cli.BoolFlag{Name: "verbose", Sources: cli.EnvVars("APP_VERBOSE")}
	again = &cli.BoolFlag{Name: "verbose", Sources: cli.EnvVars("APP_VERBOSE")}
	cli.VersionFlag = again
}

// packageMeta is a package-level variable: another file could rebind it, so
// the same-function resolution boundary refuses it — the literal stays on the
// ordinary (conformant here) rules even though it reaches cli.VersionFlag.
var packageMeta = &cli.BoolFlag{Name: "release", Sources: cli.EnvVars("APP_RELEASE")}

// configureMetaPackageVar assigns the unresolvable package-level variable: the
// same-function resolution boundary refuses it, so the literal stays on the
// ordinary rules.
func configureMetaPackageVar() {
	cli.VersionFlag = packageMeta
}

// configureMetaNonLiteral clears the override through right-hand sides that
// resolve to no literal at all — an untyped nil, and a call.
func configureMetaNonLiteral() {
	cli.VersionFlag = nil
	cli.HelpFlag = makeFlag()
}

// twoFlags returns both meta flags at once, so a file can write both variables
// in one statement.
func twoFlags() (cli.Flag, cli.Flag) { return nil, nil }

// configureMetaTuple writes both meta variables from ONE call, which is the
// shape whose left-hand side is longer than its right, and writes it TWICE so
// the pair is adjacent — the two walks that index a right-hand side are the
// per-assignment one and the replacement one, and only an adjacent pair reaches
// the second. Nothing resolves to a literal through a call, so nothing is
// exempt and nothing is reported. The case exists for the index guards that
// make the walk survive it: no rule of this analyzer mentions such a guard, and
// deleting either one panics here with index out of range.
func configureMetaTuple() {
	cli.VersionFlag, cli.HelpFlag = twoFlags()
	cli.VersionFlag, cli.HelpFlag = twoFlags()
}

// makeFlag hides a literal behind a call; values flowing through calls are
// outside the resolution boundary.
func makeFlag() cli.Flag {
	return &cli.BoolFlag{Name: "made", Sources: cli.EnvVars("APP_MADE")}
}

// elidedElements builds flags as elided elements of a pointer-typed slice: the
// &cli.StringFlag is implied by []*cli.StringFlag, and every rule judges the
// elided literal exactly like the spelled-out form.
func elidedElements() []*cli.StringFlag {
	return []*cli.StringFlag{
		{ // want `must bind an environment variable` `must carry a sensible default via Value`
			Name: "badName", // want `must be kebab-case`
		},
		{
			Name:    "well-formed",
			Value:   "x",
			Sources: cli.EnvVars("APP_WELL_FORMED"),
		},
	}
}

// holder proves a selector target outside the framework is not a meta-flag
// override: the ordinary rules stay in force.
type holder struct{ flag cli.Flag }

// metaLookalikes assigns through selectors that are not the framework's own
// Flag storage — a field of THIS package typed cli.Flag, and the framework's
// []Flag list, which is where an app's ordinary flags go. Both stay on the
// ordinary rules, and both bind a variable here, so a matcher that lost either
// the package test or the Flag-type test would report them as inert meta
// bindings instead.
func metaLookalikes(cmd *cli.Command) {
	var h holder
	h.flag = &cli.BoolFlag{Name: "held", Sources: cli.EnvVars("APP_HELD")}
	_ = h
	cmd.Flags = []cli.Flag{&cli.BoolFlag{Name: "listed", Sources: cli.EnvVars("APP_LISTED")}}
}
