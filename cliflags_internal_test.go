package cliflags

import (
	"go/ast"
	"go/token"
	"go/types"
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

	for _, entry := range []string{"vault_", "VAULT-", "1VAULT", "VAULT__ADDR", "VAULT_ ADDR", "_VAULT"} {
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
	want.True(isExternal(defaultExternals, "PGHOST"), "a bare namespace owns a single extending segment")
	want.False(isExternal(defaultExternals, "MYAPP_REGION"))
	want.False(isExternal(defaultExternals, "PGGY_BANK"), "a bare namespace owns no two-segment name")
	want.False(isExternal(defaultExternals, "PG"), "the stem alone extends nothing")
	want.False(isExternal(defaultExternals, "AWS"), "a terminated namespace needs a segment after it")
}
