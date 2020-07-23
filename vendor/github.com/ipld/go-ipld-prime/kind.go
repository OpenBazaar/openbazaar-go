package ipld

// ReprKind represents the primitive kind in the IPLD data model.
// All of these kinds map directly onto serializable data.
//
// Note that ReprKind contains the concept of "map", but not "struct"
// or "object" -- those are a concepts that could be introduced in a
// type system layers, but are *not* present in the data model layer,
// and therefore they aren't included in the ReprKind enum.
type ReprKind uint8

const (
	ReprKind_Invalid ReprKind = 0
	ReprKind_Map     ReprKind = '{'
	ReprKind_List    ReprKind = '['
	ReprKind_Null    ReprKind = '0'
	ReprKind_Bool    ReprKind = 'b'
	ReprKind_Int     ReprKind = 'i'
	ReprKind_Float   ReprKind = 'f'
	ReprKind_String  ReprKind = 's'
	ReprKind_Bytes   ReprKind = 'x'
	ReprKind_Link    ReprKind = '/'
)

func (k ReprKind) String() string {
	switch k {
	case ReprKind_Invalid:
		return "Invalid"
	case ReprKind_Map:
		return "Map"
	case ReprKind_List:
		return "List"
	case ReprKind_Null:
		return "Null"
	case ReprKind_Bool:
		return "Bool"
	case ReprKind_Int:
		return "Int"
	case ReprKind_Float:
		return "Float"
	case ReprKind_String:
		return "String"
	case ReprKind_Bytes:
		return "Bytes"
	case ReprKind_Link:
		return "Link"
	default:
		panic("invalid enumeration value!")
	}
}

// ReprKindSet is a type with a few enumerated consts that are commonly used
// (mostly, in error messages).
type ReprKindSet []ReprKind

var (
	ReprKindSet_Recursive = ReprKindSet{ReprKind_Map, ReprKind_List}
	ReprKindSet_Scalar    = ReprKindSet{ReprKind_Null, ReprKind_Bool, ReprKind_Int, ReprKind_Float, ReprKind_String, ReprKind_Bytes, ReprKind_Link}

	ReprKindSet_JustMap    = ReprKindSet{ReprKind_Map}
	ReprKindSet_JustList   = ReprKindSet{ReprKind_List}
	ReprKindSet_JustNull   = ReprKindSet{ReprKind_Null}
	ReprKindSet_JustBool   = ReprKindSet{ReprKind_Bool}
	ReprKindSet_JustInt    = ReprKindSet{ReprKind_Int}
	ReprKindSet_JustFloat  = ReprKindSet{ReprKind_Float}
	ReprKindSet_JustString = ReprKindSet{ReprKind_String}
	ReprKindSet_JustBytes  = ReprKindSet{ReprKind_Bytes}
	ReprKindSet_JustLink   = ReprKindSet{ReprKind_Link}
)

func (x ReprKindSet) String() string {
	s := ""
	for i := 0; i < len(x)-1; i++ {
		s += x[i].String() + " or "
	}
	s += x[len(x)-1].String()
	return s
}
