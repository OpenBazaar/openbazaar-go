package schema

import (
	ipld "github.com/ipld/go-ipld-prime"
)

type TypeName string // = ast.TypeName

// typesystem.Type is an union interface; each of the `Type*` concrete types
// in this package are one of its members.
//
// Specifically,
//
// 	TypeBool
// 	TypeString
// 	TypeBytes
// 	TypeInt
// 	TypeFloat
// 	TypeMap
// 	TypeList
// 	TypeLink
// 	TypeUnion
// 	TypeStruct
// 	TypeEnum
//
// are all of the kinds of Type.
//
// This is a closed union; you can switch upon the above members without
// including a default case.  The membership is closed by the unexported
// '_Type' method; you may use the BurntSushi/go-sumtype tool to check
// your switches for completeness.
//
// Many interesting properties of each Type are only defined for that specific
// type, so it's typical to use a type switch to handle each type of Type.
// (Your humble author is truly sorry for the word-mash that results from
// attempting to describe the types that describe the typesystem.Type.)
//
// For example, to inspect the kind of fields in a struct: you might
// cast a `Type` interface into `TypeStruct`, and then the `Fields()` on
// that `TypeStruct` can be inspected.  (`Fields()` isn't defined for any
// other kind of Type.)
type Type interface {
	// Unexported marker method to force the union closed.
	_Type()

	// Returns a pointer to the TypeSystem this Type is a member of.
	TypeSystem() *TypeSystem

	// Returns the string name of the Type.  This name is unique within the
	// universe this type is a member of, *unless* this type is Anonymous,
	// in which case a string describing the type will still be returned, but
	// that string will not be required to be unique.
	Name() TypeName

	// Returns the Kind of this Type.
	//
	// The returned value is a 1:1 association with which of the concrete
	// "schema.Type*" structs this interface can be cast to.
	//
	// Note that a schema.Kind is a different enum than ipld.ReprKind;
	// and furthermore, there's no strict relationship between them.
	// schema.TypedNode values can be described by *two* distinct ReprKinds:
	// one which describes how the Node itself will act,
	// and another which describes how the Node presents for serialization.
	// For some combinations of Type and representation strategy, one or both
	// of the ReprKinds can be determined statically; but not always:
	// it can sometimes be necessary to inspect the value quite concretely
	// (e.g., `schema.TypedNode{}.Representation().ReprKind()`) in order to find
	// out exactly how a node will be serialized!  This is because some types
	// can vary in representation kind based on their value (specifically,
	// kinded-representation unions have this property).
	Kind() Kind
}

var (
	_ Type = TypeBool{}
	_ Type = TypeString{}
	_ Type = TypeBytes{}
	_ Type = TypeInt{}
	_ Type = TypeFloat{}
	_ Type = TypeMap{}
	_ Type = TypeList{}
	_ Type = TypeLink{}
	_ Type = TypeUnion{}
	_ Type = TypeStruct{}
	_ Type = TypeEnum{}
)

type anyType struct {
	name     TypeName
	universe *TypeSystem
}

type TypeBool struct {
	anyType
}

type TypeString struct {
	anyType
}

type TypeBytes struct {
	anyType
}

type TypeInt struct {
	anyType
}

type TypeFloat struct {
	anyType
}

type TypeMap struct {
	anyType
	anonymous     bool
	keyType       Type // must be ReprKind==string (e.g. Type==String|Enum).
	valueType     Type
	valueNullable bool
}

type TypeList struct {
	anyType
	anonymous     bool
	valueType     Type
	valueNullable bool
}

type TypeLink struct {
	anyType
	referencedType    Type
	hasReferencedType bool
	// ...?
}

type TypeUnion struct {
	anyType
	style        UnionStyle
	valuesKinded map[ipld.ReprKind]Type // for Style==Kinded
	values       map[string]Type        // for Style!=Kinded (note, key is freetext, not necessarily TypeName of the value)
	typeHintKey  string                 // for Style==Envelope|Inline
	contentKey   string                 // for Style==Envelope
}

type UnionStyle struct{ x string }

var (
	UnionStyle_Kinded   = UnionStyle{"kinded"}
	UnionStyle_Keyed    = UnionStyle{"keyed"}
	UnionStyle_Envelope = UnionStyle{"envelope"}
	UnionStyle_Inline   = UnionStyle{"inline"}
)

type TypeStruct struct {
	anyType
	// n.b. `Fields` is an (order-preserving!) map in the schema-schema;
	//  but it's a list here, with the keys denormalized into the value,
	//   because that's typically how we use it.
	fields         []StructField
	fieldsMap      map[string]StructField // same content, indexed for lookup.
	representation StructRepresentation
}
type StructField struct {
	name     string
	typ      Type
	optional bool
	nullable bool
}

type StructRepresentation interface{ _StructRepresentation() }

func (StructRepresentation_Map) _StructRepresentation()         {}
func (StructRepresentation_Tuple) _StructRepresentation()       {}
func (StructRepresentation_StringPairs) _StructRepresentation() {}
func (StructRepresentation_StringJoin) _StructRepresentation()  {}

type StructRepresentation_Map struct {
	renames   map[string]string
	implicits map[string]interface{}
}
type StructRepresentation_Tuple struct{}
type StructRepresentation_StringPairs struct{ sep1, sep2 string }
type StructRepresentation_StringJoin struct{ sep string }

type TypeEnum struct {
	anyType
	members []string
}
