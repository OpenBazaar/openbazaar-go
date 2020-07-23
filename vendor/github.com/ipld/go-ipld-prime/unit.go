package ipld

var Null Node = nullNode{}

type nullNode struct{}

func (nullNode) ReprKind() ReprKind {
	return ReprKind_Null
}
func (nullNode) LookupString(key string) (Node, error) {
	return nil, ErrWrongKind{TypeName: "null", MethodName: "LookupString", AppropriateKind: ReprKindSet_JustMap, ActualKind: ReprKind_Null}
}
func (nullNode) Lookup(key Node) (Node, error) {
	return nil, ErrWrongKind{TypeName: "null", MethodName: "Lookup", AppropriateKind: ReprKindSet_JustMap, ActualKind: ReprKind_Null}
}
func (nullNode) LookupIndex(idx int) (Node, error) {
	return nil, ErrWrongKind{TypeName: "null", MethodName: "LookupIndex", AppropriateKind: ReprKindSet_JustList, ActualKind: ReprKind_Null}
}
func (nullNode) LookupSegment(seg PathSegment) (Node, error) {
	return nil, ErrWrongKind{TypeName: "null", MethodName: "LookupSegment", AppropriateKind: ReprKindSet_Recursive, ActualKind: ReprKind_Null}
}
func (nullNode) MapIterator() MapIterator {
	return nil
}
func (nullNode) ListIterator() ListIterator {
	return nil
}
func (nullNode) Length() int {
	return -1
}
func (nullNode) IsUndefined() bool {
	return false
}
func (nullNode) IsNull() bool {
	return true
}
func (nullNode) AsBool() (bool, error) {
	return false, ErrWrongKind{TypeName: "null", MethodName: "AsBool", AppropriateKind: ReprKindSet_JustBool, ActualKind: ReprKind_Null}
}
func (nullNode) AsInt() (int, error) {
	return 0, ErrWrongKind{TypeName: "null", MethodName: "AsInt", AppropriateKind: ReprKindSet_JustInt, ActualKind: ReprKind_Null}
}
func (nullNode) AsFloat() (float64, error) {
	return 0, ErrWrongKind{TypeName: "null", MethodName: "AsFloat", AppropriateKind: ReprKindSet_JustFloat, ActualKind: ReprKind_Null}
}
func (nullNode) AsString() (string, error) {
	return "", ErrWrongKind{TypeName: "null", MethodName: "AsString", AppropriateKind: ReprKindSet_JustString, ActualKind: ReprKind_Null}
}
func (nullNode) AsBytes() ([]byte, error) {
	return nil, ErrWrongKind{TypeName: "null", MethodName: "AsBytes", AppropriateKind: ReprKindSet_JustBytes, ActualKind: ReprKind_Null}
}
func (nullNode) AsLink() (Link, error) {
	return nil, ErrWrongKind{TypeName: "null", MethodName: "AsLink", AppropriateKind: ReprKindSet_JustLink, ActualKind: ReprKind_Null}
}
func (nullNode) Style() NodeStyle {
	return nullStyle{}
}

type nullStyle struct{}

func (nullStyle) NewBuilder() NodeBuilder {
	panic("cannot build null nodes") // TODO: okay, fine, we could grind out a simple closing of the loop here.
}

var Undef Node = undefNode{}

type undefNode struct{}

func (undefNode) ReprKind() ReprKind {
	return ReprKind_Null
}
func (undefNode) LookupString(key string) (Node, error) {
	return nil, ErrWrongKind{TypeName: "undef", MethodName: "LookupString", AppropriateKind: ReprKindSet_JustMap, ActualKind: ReprKind_Null}
}
func (undefNode) Lookup(key Node) (Node, error) {
	return nil, ErrWrongKind{TypeName: "undef", MethodName: "Lookup", AppropriateKind: ReprKindSet_JustMap, ActualKind: ReprKind_Null}
}
func (undefNode) LookupIndex(idx int) (Node, error) {
	return nil, ErrWrongKind{TypeName: "undef", MethodName: "LookupIndex", AppropriateKind: ReprKindSet_JustList, ActualKind: ReprKind_Null}
}
func (undefNode) LookupSegment(seg PathSegment) (Node, error) {
	return nil, ErrWrongKind{TypeName: "undef", MethodName: "LookupSegment", AppropriateKind: ReprKindSet_Recursive, ActualKind: ReprKind_Null}
}
func (undefNode) MapIterator() MapIterator {
	return nil
}
func (undefNode) ListIterator() ListIterator {
	return nil
}
func (undefNode) Length() int {
	return -1
}
func (undefNode) IsUndefined() bool {
	return true
}
func (undefNode) IsNull() bool {
	return false
}
func (undefNode) AsBool() (bool, error) {
	return false, ErrWrongKind{TypeName: "undef", MethodName: "AsBool", AppropriateKind: ReprKindSet_JustBool, ActualKind: ReprKind_Null}
}
func (undefNode) AsInt() (int, error) {
	return 0, ErrWrongKind{TypeName: "undef", MethodName: "AsInt", AppropriateKind: ReprKindSet_JustInt, ActualKind: ReprKind_Null}
}
func (undefNode) AsFloat() (float64, error) {
	return 0, ErrWrongKind{TypeName: "undef", MethodName: "AsFloat", AppropriateKind: ReprKindSet_JustFloat, ActualKind: ReprKind_Null}
}
func (undefNode) AsString() (string, error) {
	return "", ErrWrongKind{TypeName: "undef", MethodName: "AsString", AppropriateKind: ReprKindSet_JustString, ActualKind: ReprKind_Null}
}
func (undefNode) AsBytes() ([]byte, error) {
	return nil, ErrWrongKind{TypeName: "undef", MethodName: "AsBytes", AppropriateKind: ReprKindSet_JustBytes, ActualKind: ReprKind_Null}
}
func (undefNode) AsLink() (Link, error) {
	return nil, ErrWrongKind{TypeName: "undef", MethodName: "AsLink", AppropriateKind: ReprKindSet_JustLink, ActualKind: ReprKind_Null}
}
func (undefNode) Style() NodeStyle {
	return undefStyle{}
}

type undefStyle struct{}

func (undefStyle) NewBuilder() NodeBuilder {
	panic("cannot build undef nodes") // this definitely stays true.
}
