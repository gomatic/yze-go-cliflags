package cliflags

// Classifying composite-literal types as framework flags: the generic FlagBase
// aliases (StringFlag, BoolFlag, ...) and the non-generic flag structs
// (BoolWithInverseFlag), plus the zero-default exemption their value types
// decide.

import (
	"go/ast"
	"go/types"
	"strings"
)

// flagType reports whether t is a urfave/cli v3 flag type, and whether the
// flag's value type has a zero value that IS its sensible default. v3 spells
// StringFlag/BoolFlag/... as aliases of the generic FlagBase (the first type
// argument is the value type) — but also ships non-generic flag structs
// (BoolWithInverseFlag), which are classified by their Value field instead.
func flagType(t types.Type) (isFlag, hasZeroDefault bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != cliPackage {
		return false, false
	}
	if named.Obj().Name() == "FlagBase" {
		return true, isZeroDefaultArgument(named)
	}
	return structFlagType(named)
}

// structFlagType classifies the framework's non-generic flag structs: an
// exported *Flag-suffixed struct carrying a Value field is a flag literal
// users build (BoolWithInverseFlag), and its Value field's type decides the
// zero-default exemption. Framework interfaces and internal types fall out on
// the struct and field requirements.
func structFlagType(named *types.Named) (isFlag, hasZeroDefault bool) {
	name := named.Obj().Name()
	strct, ok := named.Underlying().(*types.Struct)
	if !ok || !strings.HasSuffix(name, "Flag") || !ast.IsExported(name) {
		return false, false
	}
	value := fieldType(strct, "Value")
	if value == nil {
		return false, false
	}
	return true, isZeroDefaultType(value)
}

// fieldName is a struct field's identifier.
type fieldName string

// fieldType returns the type of the struct's named field, or nil.
func fieldType(strct *types.Struct, name fieldName) types.Type {
	for field := range strct.Fields() {
		if field.Name() == string(name) {
			return field.Type()
		}
	}
	return nil
}

// isZeroDefaultArgument reports whether the FlagBase instantiation's value
// type carries its sensible default in its zero value.
func isZeroDefaultArgument(named *types.Named) bool {
	arguments := named.TypeArgs()
	if arguments == nil || arguments.Len() == 0 {
		return false
	}
	return isZeroDefaultType(arguments.At(0))
}

// isZeroDefaultType reports whether a flag value type's zero value is its
// sensible default: a boolean (false) or a slice (empty) — the standard's own
// BoolFlag example omits Value, and its slice guidance sets no Value either.
func isZeroDefaultType(t types.Type) bool {
	switch resolved := types.Unalias(t).(type) {
	case *types.Slice:
		return true
	case *types.Basic:
		return resolved.Kind() == types.Bool
	}
	return false
}
