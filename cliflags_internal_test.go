package cliflags

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectFieldsSkipsAnUnkeyedElement pins what replaced the unkeyed-literal
// exemption. It is written against a constructed literal because no fixture can
// hold one: every v3 flag struct carries unexported fields, so an unkeyed
// literal of one does not compile outside the framework's own package. The
// contract is that an unkeyed element contributes no field rather than
// exempting the literal or dereferencing a nil KeyValueExpr.
func TestCollectFieldsSkipsAnUnkeyedElement(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	keyed := &ast.KeyValueExpr{Key: ast.NewIdent("Name"), Value: ast.NewIdent("x")}
	computed := &ast.KeyValueExpr{Key: &ast.BasicLit{Kind: token.STRING, Value: `"Name"`}, Value: ast.NewIdent("y")}
	lit := &ast.CompositeLit{Elts: []ast.Expr{ast.NewIdent("positional"), keyed, computed}}

	fields := collectFields(lit)
	want.Len(fields, 1, "only the keyed element with an identifier key is indexed")
	want.Same(keyed, fields["Name"])
}

// TestFlagTypeRecognizesOnlyTheFrameworkFlagBase pins the type classifier: a
// basic type, a named type outside the framework, a FlagBase impostor in
// another package, and a framework type that is not FlagBase are none of them
// flags — misclassifying one would impose flag rules on arbitrary structs.
func TestFlagTypeRecognizesOnlyTheFrameworkFlagBase(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	isFlag, isBool := flagType(types.Typ[types.String])
	want.False(isFlag, "a basic type is not a flag")
	want.False(isBool)

	other := types.NewPackage("example.com/other", "other")
	impostor := types.NewNamed(types.NewTypeName(0, other, "FlagBase", nil), types.NewStruct(nil, nil), nil)
	isFlag, _ = flagType(impostor)
	want.False(isFlag, "FlagBase outside the framework package is not a flag")

	framework := types.NewPackage(cliPackage, "cli")
	command := types.NewNamed(types.NewTypeName(0, framework, "Command", nil), types.NewStruct(nil, nil), nil)
	isFlag, _ = flagType(command)
	want.False(isFlag, "a framework type that is not FlagBase is not a flag")

	base := types.NewNamed(types.NewTypeName(0, framework, "FlagBase", nil), types.NewStruct(nil, nil), nil)
	isFlag, isBool = flagType(base)
	want.True(isFlag, "the framework FlagBase is a flag")
	want.False(isBool, "a FlagBase without type arguments defaults to non-boolean")

	isFlag, _ = flagType(types.NewPointer(base))
	want.True(isFlag, "a pointer to a flag type (an elided []*T element) is a flag")
}

// TestStructFlagTypeDemandsAStructWithAValueField pins the non-generic flag
// classifier's guards: a framework interface (RequiredFlag) and a Flag-named
// struct WITHOUT a Value field are not literal-built flags.
func TestStructFlagTypeDemandsAStructWithAValueField(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	framework := types.NewPackage(cliPackage, "cli")

	iface := types.NewNamed(types.NewTypeName(0, framework, "RequiredFlag", nil), types.NewInterfaceType(nil, nil), nil)
	isFlag, _ := flagType(iface)
	want.False(isFlag, "a framework interface is not a flag literal type")

	bare := types.NewNamed(types.NewTypeName(0, framework, "OddFlag", nil), types.NewStruct(nil, nil), nil)
	isFlag, _ = flagType(bare)
	want.False(isFlag, "a Flag-named struct without a Value field is not a flag")

	// Each of the next three deviates from a recognised flag struct in exactly
	// one place — the suffix, the case of the first letter, the field name —
	// so each pins its own conjunct rather than the three together.
	valued := func(name string) *types.Named {
		field := types.NewField(0, framework, "Value", types.Typ[types.String], false)
		strct := types.NewStruct([]*types.Var{field}, nil)
		return types.NewNamed(types.NewTypeName(0, framework, name, nil), strct, nil)
	}

	isFlag, _ = flagType(valued("Config"))
	want.False(isFlag, "a Value-carrying framework struct that is no Flag is not a flag")

	isFlag, _ = flagType(valued("boolFlag"))
	want.False(isFlag, "an unexported framework struct is not a literal users build")

	other := types.NewField(0, framework, "Default", types.Typ[types.String], false)
	misnamed := types.NewNamed(
		types.NewTypeName(0, framework, "OtherFlag", nil),
		types.NewStruct([]*types.Var{other}, nil),
		nil)
	isFlag, _ = flagType(misnamed)
	want.False(isFlag, "a Flag-suffixed struct with no Value field decides no zero default, so it is no flag")

	isFlag, hasZeroDefault := flagType(valued("SomeFlag"))
	want.True(isFlag, "an exported Flag-suffixed framework struct carrying Value is a flag")
	want.False(hasZeroDefault, "a string Value is not a zero-default type")
}

// TestIsZeroDefaultTypeCoversEveryEmptyZero names the invariant the doc comment
// asserts and checks it against every value type urfave/cli v3 builds a flag
// alias over: the three whose zero IS the sensible default, and the rest whose
// zero is a value rather than an emptiness. It exists because the alternative
// was to REWORD the claim until the gate stopped reading it as one, which is
// docs/s04.md item 7 arriving on prose — the sentence would have said less than
// its author meant, and nothing would have checked either version.
func TestIsZeroDefaultTypeCoversEveryEmptyZero(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	for _, valueType := range []types.Type{
		types.Typ[types.Bool],
		types.NewSlice(types.Typ[types.String]),
		types.NewMap(types.Typ[types.String], types.Typ[types.String]),
	} {
		want.True(isZeroDefaultType(valueType), valueType.String())
	}

	for _, valueType := range []types.Type{
		types.Typ[types.String], types.Typ[types.Int], types.Typ[types.Int8],
		types.Typ[types.Int64], types.Typ[types.Uint], types.Typ[types.Float32],
		types.Typ[types.Float64],
	} {
		want.False(isZeroDefaultType(valueType), valueType.String())
	}
}

// TestExternalsExtendsTheDefaults pins the -external parsing: the defaults are
// always present, additions append, empty segments are ignored, and the space
// a comma-separated setting is ordinarily written with is trimmed rather than
// registered as part of the namespace.
func TestExternalsExtendsTheDefaults(t *testing.T) {
	original := externalAdded
	t.Cleanup(func() { externalAdded = original })

	externalAdded = externalSetting{}
	assert.Equal(t, defaultExternals, externals(), "no additions yields the defaults")

	require.NoError(t, externalAdded.Set("VAULT_,,GH_"))
	assert.Equal(t,
		append(append(namespaces{}, defaultExternals...), "VAULT_", "GH_"),
		externals(), "additions append and empty segments are ignored")

	require.NoError(t, externalAdded.Set("X_, VAULT_"))
	assert.Equal(t,
		append(append(namespaces{}, defaultExternals...), "X_", "VAULT_"),
		externals(), "the space after a comma is punctuation, not part of the namespace")
	assert.Equal(t, "X_, VAULT_", externalAdded.String(), "the setting reports itself as written")
}

// TestExternalSettingRefusesWhatCannotNarrowAnything pins the refusal: an entry
// that is not an UPPERCASE_SNAKE prefix can never match a name the naming rule
// admits, so it is refused with the package's own sentinel instead of being
// registered in silence. Under go-yze that becomes ErrInvalidSettingValue; a
// registered-but-unmatchable entry is indistinguishable from one that works.
func TestExternalSettingRefusesWhatCannotNarrowAnything(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	// VAULT_ADDR and GOOGLE_APPLICATION_CREDENTIALS are the entries this
	// validator used to ACCEPT: well-formed as prose, unmatchable in fact,
	// because both matchers compare against segments split on underscore and a
	// segment never holds one. Written from the description they read as
	// namespaces; written from the matcher they narrow nothing.
	for _, entry := range []string{
		"vault_", "VAULT-", "1VAULT", "VAULT__ADDR", "VAULT_ ADDR", "_VAULT",
		"VAULT_ADDR", "GOOGLE_APPLICATION_CREDENTIALS", "AWS_S3",
	} {
		var setting externalSetting
		err := setting.Set(entry)
		want.ErrorIs(err, ErrExternalNamespace, entry)
		want.Contains(err.Error(), entry, "the refusal names the entry it refused")
		want.Empty(setting.added, "a refused setting registers nothing")
	}

	var partial externalSetting
	want.ErrorIs(partial.Set("VAULT_,vault_"), ErrExternalNamespace, "one bad entry refuses the setting")
	want.Empty(partial.added, "the whole setting is refused, never half-applied")
}

// TestPrefixedExternalFindsWrappedNamespaces pins the KILROY_PGHOST rule's
// matcher: a wrapped namespace is found with its namespace named, an
// unwrapped name is not, and the leading segment is judged by the same theory
// of a namespace as every other segment.
func TestPrefixedExternalFindsWrappedNamespaces(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	namespace, found := prefixedExternal(defaultExternals, "KILROY_PGHOST")
	want.True(found)
	want.Equal(externalPrefix("PG"), namespace)

	_, found = prefixedExternal(defaultExternals, "MYAPP_MAX_RECORDS")
	want.False(found, "no namespace is wrapped")

	want.True(isExternal(defaultExternals, "AWS_REGION"), "an unprefixed external is itself")
	want.True(isExternal(defaultExternals, "PGHOST"), "a bare namespace owns a leading segment extending it")
	want.True(isExternal(defaultExternals, "PGCONNECT_TIMEOUT"), "libpq spells some of its variables in two segments")
	want.True(isExternal(defaultExternals, "PG_COLOR"), "and some with the stem as a segment of its own")
	want.False(isExternal(defaultExternals, "MYAPP_REGION"))
	want.False(isExternal(defaultExternals, "AWS"), "a terminated namespace needs a segment after it")
	want.False(isExternal(defaultExternals, "AWSREGION"), "a terminated namespace owns no run-on spelling")

	// The leading test is a PREFIX match and this is what that costs: an app
	// variable wearing two of libpq's letters is exempt, and nothing here can
	// tell it from PGCONNECT_TIMEOUT above. Asserted so the hole is visible
	// rather than discovered again — cliflags.external-name-is-a-namespace-member.
	want.True(isExternal(defaultExternals, "PGGY_BANK"), "a known, unclosed hole, asserted rather than hidden")
}

// TestPositionAssignmentSeesThroughALabelAndAnInitClause names the unwrapping
// the dead-write rule stands on and checks each wrapper separately. Without it,
// one label or one init clause kept a meta-flag exemption that the identical
// bare assignment would have lost — a laundering route measured against the
// real framework before this test existed.
func TestPositionAssignmentSeesThroughALabelAndAnInitClause(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	assign := &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("x")}, Rhs: []ast.Expr{ast.NewIdent("y")}}
	cond := ast.NewIdent("cond")
	body := &ast.BlockStmt{}

	want.Same(assign, positionAssignment(assign), "a bare assignment is itself")
	want.Same(assign, positionAssignment(&ast.LabeledStmt{Label: ast.NewIdent("L"), Stmt: assign}))
	want.Same(assign, positionAssignment(&ast.IfStmt{Init: assign, Cond: cond, Body: body}))
	want.Same(assign, positionAssignment(&ast.ForStmt{Init: assign, Body: body}))
	want.Same(assign, positionAssignment(&ast.SwitchStmt{Init: assign, Body: body}))
	want.Same(assign, positionAssignment(&ast.TypeSwitchStmt{Init: assign, Body: body}))
	want.Same(assign, positionAssignment(&ast.LabeledStmt{
		Label: ast.NewIdent("L"),
		Stmt:  &ast.IfStmt{Init: assign, Cond: cond, Body: body},
	}), "wrappers nest")

	want.Nil(positionAssignment(&ast.IfStmt{Cond: cond, Body: body}), "an if with no init assigns nothing here")
	want.Nil(positionAssignment(&ast.ExprStmt{X: cond}), "a statement that is no assignment assigns nothing")
	want.Nil(positionAssignment(body), "a nested block runs its statements at their own positions, not here")
}

// TestLeadsNamespaceIsWideAtTheLeadingSegment names the deliberately wide half
// of the namespace policy: what an external tool owns unprefixed. It is wide
// because narrowing it reported PGCONNECT_TIMEOUT and PG_COLOR, real libpq
// variables, and prescribed a name libpq will never read.
func TestLeadsNamespaceIsWideAtTheLeadingSegment(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	bare := func(name string) bool { return leadsNamespace(strings.Split(name, "_"), "PG") }
	want.True(bare("PGHOST"))
	want.True(bare("PGCONNECT_TIMEOUT"), "two segments, and libpq's")
	want.True(bare("PG_COLOR"), "the stem as a segment of its own")
	want.True(bare("PGGY_BANK"), "the cost of a prefix match, asserted rather than hidden")
	want.False(bare("MYAPP_PGHOST"), "a wrapped external does not LEAD with the namespace")
	want.False(bare("WIDGET_NAME"))

	terminated := func(name string) bool { return leadsNamespace(strings.Split(name, "_"), "AWS_") }
	want.True(terminated("AWS_REGION"))
	want.False(terminated("AWS"), "the separator means a segment has to follow")
	want.False(terminated("AWSREGION"), "and that the stem is a whole segment")
	want.False(terminated("MYAPP_AWS_REGION"))
}

// TestExternalShapeMatchesOnlyWhatTheMatcherCanUse names the -external
// validator's shape and checks it against what leadsNamespace and wrapsAt can
// actually consume: one segment, optionally terminated. Written from this
// setting's description instead, it accepted VAULT_ADDR — well-formed prose
// that narrows nothing, which is the failure the validator exists to stop.
func TestExternalShapeMatchesOnlyWhatTheMatcherCanUse(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	for _, entry := range []string{"PG", "AWS_", "DOCKER_", "GH_", "V2_", "X"} {
		want.True(externalShape.MatchString(entry), entry)
	}
	for _, entry := range []string{"VAULT_ADDR", "GOOGLE_APPLICATION_CREDENTIALS", "AWS_S3", "vault_", "1PG", "_PG", "PG-"} {
		want.False(externalShape.MatchString(entry), entry)
	}
}
