package mixins

import (
	ipld "github.com/ipld/go-ipld-prime"
)

// String can be embedded in a struct to provide all the methods that
// have fixed output for any string-kinded nodes.
// (Mostly this includes all the methods which simply return ErrWrongKind.)
// Other methods will still need to be implemented to finish conforming to Node.
//
// To conserve memory and get a TypeName in errors without embedding,
// write methods on your type with a body that simply initializes this struct
// and immediately uses the relevant method;
// this is more verbose in source, but compiles to a tighter result:
// in memory, there's no embed; and in runtime, the calls will be inlined
// and thus have no cost in execution time.
type String struct {
	TypeName string
}

func (String) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_String
}
func (x String) LookupByString(string) (ipld.Node, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "LookupByString", AppropriateKind: ipld.ReprKindSet_JustMap, ActualKind: ipld.ReprKind_String}
}
func (x String) LookupByNode(key ipld.Node) (ipld.Node, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "LookupByNode", AppropriateKind: ipld.ReprKindSet_JustMap, ActualKind: ipld.ReprKind_String}
}
func (x String) LookupByIndex(idx int) (ipld.Node, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "LookupByIndex", AppropriateKind: ipld.ReprKindSet_JustList, ActualKind: ipld.ReprKind_String}
}
func (x String) LookupBySegment(ipld.PathSegment) (ipld.Node, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "LookupBySegment", AppropriateKind: ipld.ReprKindSet_Recursive, ActualKind: ipld.ReprKind_String}
}
func (String) MapIterator() ipld.MapIterator {
	return nil
}
func (String) ListIterator() ipld.ListIterator {
	return nil
}
func (String) Length() int {
	return -1
}
func (String) IsAbsent() bool {
	return false
}
func (String) IsNull() bool {
	return false
}
func (x String) AsBool() (bool, error) {
	return false, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsBool", AppropriateKind: ipld.ReprKindSet_JustBool, ActualKind: ipld.ReprKind_String}
}
func (x String) AsInt() (int, error) {
	return 0, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsInt", AppropriateKind: ipld.ReprKindSet_JustInt, ActualKind: ipld.ReprKind_String}
}
func (x String) AsFloat() (float64, error) {
	return 0, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsFloat", AppropriateKind: ipld.ReprKindSet_JustFloat, ActualKind: ipld.ReprKind_String}
}
func (x String) AsBytes() ([]byte, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsBytes", AppropriateKind: ipld.ReprKindSet_JustBytes, ActualKind: ipld.ReprKind_String}
}
func (x String) AsLink() (ipld.Link, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsLink", AppropriateKind: ipld.ReprKindSet_JustLink, ActualKind: ipld.ReprKind_String}
}

// StringAssembler has similar purpose as String, but for (you guessed it)
// the NodeAssembler interface rather than the Node interface.
type StringAssembler struct {
	TypeName string
}

func (x StringAssembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "BeginMap", AppropriateKind: ipld.ReprKindSet_JustMap, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) BeginList(sizeHint int) (ipld.ListAssembler, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "BeginList", AppropriateKind: ipld.ReprKindSet_JustList, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) AssignNull() error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignNull", AppropriateKind: ipld.ReprKindSet_JustNull, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) AssignBool(bool) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignBool", AppropriateKind: ipld.ReprKindSet_JustBool, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) AssignInt(int) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignInt", AppropriateKind: ipld.ReprKindSet_JustInt, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) AssignFloat(float64) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignFloat", AppropriateKind: ipld.ReprKindSet_JustFloat, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) AssignBytes([]byte) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignBytes", AppropriateKind: ipld.ReprKindSet_JustBytes, ActualKind: ipld.ReprKind_String}
}
func (x StringAssembler) AssignLink(ipld.Link) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignLink", AppropriateKind: ipld.ReprKindSet_JustLink, ActualKind: ipld.ReprKind_String}
}
