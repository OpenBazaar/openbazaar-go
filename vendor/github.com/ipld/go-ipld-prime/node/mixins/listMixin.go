package mixins

import (
	ipld "github.com/ipld/go-ipld-prime"
)

// List can be embedded in a struct to provide all the methods that
// have fixed output for any int-kinded nodes.
// (Mostly this includes all the methods which simply return ErrWrongKind.)
// Other methods will still need to be implemented to finish conforming to Node.
//
// To conserve memory and get a TypeName in errors without embedding,
// write methods on your type with a body that simply initializes this struct
// and immediately uses the relevant method;
// this is more verbose in source, but compiles to a tighter result:
// in memory, there's no embed; and in runtime, the calls will be inlined
// and thus have no cost in execution time.
type List struct {
	TypeName string
}

func (List) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_List
}
func (x List) LookupByString(string) (ipld.Node, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "LookupByString", AppropriateKind: ipld.ReprKindSet_JustMap, ActualKind: ipld.ReprKind_List}
}
func (x List) LookupByNode(key ipld.Node) (ipld.Node, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "LookupByNode", AppropriateKind: ipld.ReprKindSet_JustMap, ActualKind: ipld.ReprKind_List}
}
func (List) MapIterator() ipld.MapIterator {
	return nil
}
func (List) IsAbsent() bool {
	return false
}
func (List) IsNull() bool {
	return false
}
func (x List) AsBool() (bool, error) {
	return false, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsBool", AppropriateKind: ipld.ReprKindSet_JustBool, ActualKind: ipld.ReprKind_List}
}
func (x List) AsInt() (int, error) {
	return 0, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsInt", AppropriateKind: ipld.ReprKindSet_JustInt, ActualKind: ipld.ReprKind_List}
}
func (x List) AsFloat() (float64, error) {
	return 0, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsFloat", AppropriateKind: ipld.ReprKindSet_JustFloat, ActualKind: ipld.ReprKind_List}
}
func (x List) AsString() (string, error) {
	return "", ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsString", AppropriateKind: ipld.ReprKindSet_JustString, ActualKind: ipld.ReprKind_List}
}
func (x List) AsBytes() ([]byte, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsBytes", AppropriateKind: ipld.ReprKindSet_JustBytes, ActualKind: ipld.ReprKind_List}
}
func (x List) AsLink() (ipld.Link, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AsLink", AppropriateKind: ipld.ReprKindSet_JustLink, ActualKind: ipld.ReprKind_List}
}

// ListAssembler has similar purpose as List, but for (you guessed it)
// the NodeAssembler interface rather than the Node interface.
type ListAssembler struct {
	TypeName string
}

func (x ListAssembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return nil, ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "BeginMap", AppropriateKind: ipld.ReprKindSet_JustMap, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignNull() error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignNull", AppropriateKind: ipld.ReprKindSet_JustNull, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignBool(bool) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignBool", AppropriateKind: ipld.ReprKindSet_JustBool, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignInt(int) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignInt", AppropriateKind: ipld.ReprKindSet_JustInt, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignFloat(float64) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignFloat", AppropriateKind: ipld.ReprKindSet_JustFloat, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignString(string) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignString", AppropriateKind: ipld.ReprKindSet_JustString, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignBytes([]byte) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignBytes", AppropriateKind: ipld.ReprKindSet_JustBytes, ActualKind: ipld.ReprKind_List}
}
func (x ListAssembler) AssignLink(ipld.Link) error {
	return ipld.ErrWrongKind{TypeName: x.TypeName, MethodName: "AssignLink", AppropriateKind: ipld.ReprKindSet_JustLink, ActualKind: ipld.ReprKind_List}
}
