package schema

import (
	ipld "github.com/ipld/go-ipld-prime"
)

// Kind is an enum of kind in the IPLD Schema system.
//
// Note that schema.Kind is distinct from ipld.ReprKind!
// Schema kinds include concepts such as "struct" and "enum", which are
// concepts only introduced by the Schema layer, and not present in the
// Data Model layer.
type Kind uint8

const (
	Kind_Invalid Kind = 0
	Kind_Map     Kind = '{'
	Kind_List    Kind = '['
	Kind_Unit    Kind = '1'
	Kind_Bool    Kind = 'b'
	Kind_Int     Kind = 'i'
	Kind_Float   Kind = 'f'
	Kind_String  Kind = 's'
	Kind_Bytes   Kind = 'x'
	Kind_Link    Kind = '/'
	Kind_Struct  Kind = '$'
	Kind_Union   Kind = '^'
	Kind_Enum    Kind = '%'
	// FUTURE: Kind_Any = '?'?
)

func (k Kind) String() string {
	switch k {
	case Kind_Invalid:
		return "Invalid"
	case Kind_Map:
		return "Map"
	case Kind_List:
		return "List"
	case Kind_Unit:
		return "Unit"
	case Kind_Bool:
		return "Bool"
	case Kind_Int:
		return "Int"
	case Kind_Float:
		return "Float"
	case Kind_String:
		return "String"
	case Kind_Bytes:
		return "Bytes"
	case Kind_Link:
		return "Link"
	case Kind_Struct:
		return "Struct"
	case Kind_Union:
		return "Union"
	case Kind_Enum:
		return "Enum"
	default:
		panic("invalid enumeration value!")
	}
}

// ActsLike returns a constant from the ipld.ReprKind enum describing what
// this schema.Kind acts like at the Data Model layer.
//
// Things with similar names are generally conserved
// (e.g. "map" acts like "map");
// concepts added by the schema layer have to be mapped onto something
// (e.g. "struct" acts like "map").
//
// Note that this mapping describes how a typed Node will *act*, programmatically;
// it does not necessarily describe how it will be *serialized*
// (for example, a struct will always act like a map, even if it has a tuple
// representation strategy and thus becomes a list when serialized).
func (k Kind) ActsLike() ipld.ReprKind {
	switch k {
	case Kind_Invalid:
		return ipld.ReprKind_Invalid
	case Kind_Map:
		return ipld.ReprKind_Map
	case Kind_List:
		return ipld.ReprKind_List
	case Kind_Unit:
		return ipld.ReprKind_Bool // maps to 'true'.
	case Kind_Bool:
		return ipld.ReprKind_Bool
	case Kind_Int:
		return ipld.ReprKind_Int
	case Kind_Float:
		return ipld.ReprKind_Float
	case Kind_String:
		return ipld.ReprKind_String
	case Kind_Bytes:
		return ipld.ReprKind_Bytes
	case Kind_Link:
		return ipld.ReprKind_Link
	case Kind_Struct:
		return ipld.ReprKind_Map // clear enough: fields are keys.
	case Kind_Union:
		return ipld.ReprKind_Map // REVIEW: unions are tricky.
	case Kind_Enum:
		return ipld.ReprKind_String // 'AsString' is the one clear thing to define.
	default:
		panic("invalid enumeration value!")
	}
}
