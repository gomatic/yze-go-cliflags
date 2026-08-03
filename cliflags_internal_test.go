package cliflags

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
}

// TestExternalsExtendsTheDefaults pins the -external parsing: the defaults are
// always present, additions append, and empty segments are ignored.
func TestExternalsExtendsTheDefaults(t *testing.T) {
	original := externalExtra
	t.Cleanup(func() { externalExtra = original })

	externalExtra = ""
	assert.Equal(t, defaultExternals, externals(), "no additions yields the defaults")

	externalExtra = "VAULT_,,GH_"
	assert.Equal(t,
		append(append([]externalPrefix{}, defaultExternals...), "VAULT_", "GH_"),
		externals(), "additions append and empty segments are ignored")
}

// TestPrefixedExternalFindsWrappedNamespaces pins the KILROY_PGHOST rule's
// matcher: a wrapped namespace is found with its namespace named, an
// unwrapped name is not.
func TestPrefixedExternalFindsWrappedNamespaces(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	namespace, found := prefixedExternal("KILROY_PGHOST")
	want.True(found)
	want.Equal(externalPrefix("PG"), namespace)

	_, found = prefixedExternal("MYAPP_MAX_RECORDS")
	want.False(found, "no namespace is wrapped")

	want.True(hasExternalPrefix("AWS_REGION"), "an unprefixed external is itself")
	want.False(hasExternalPrefix("MYAPP_REGION"))
}
