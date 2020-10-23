package basicnode

import (
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/mixins"
)

var (
	_ ipld.Node          = plainString("")
	_ ipld.NodePrototype = Prototype__String{}
	_ ipld.NodeBuilder   = &plainString__Builder{}
	_ ipld.NodeAssembler = &plainString__Assembler{}
)

func NewString(value string) ipld.Node {
	v := plainString(value)
	return &v
}

// plainString is a simple boxed string that complies with ipld.Node.
// It's useful for many things, such as boxing map keys.
//
// The implementation is a simple typedef of a string;
// handling it as a Node incurs 'runtime.convTstring',
// which is about the best we can do.
type plainString string

// -- Node interface methods -->

func (plainString) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_String
}
func (plainString) LookupByString(string) (ipld.Node, error) {
	return mixins.String{"string"}.LookupByString("")
}
func (plainString) LookupByNode(key ipld.Node) (ipld.Node, error) {
	return mixins.String{"string"}.LookupByNode(nil)
}
func (plainString) LookupByIndex(idx int) (ipld.Node, error) {
	return mixins.String{"string"}.LookupByIndex(0)
}
func (plainString) LookupBySegment(seg ipld.PathSegment) (ipld.Node, error) {
	return mixins.String{"string"}.LookupBySegment(seg)
}
func (plainString) MapIterator() ipld.MapIterator {
	return nil
}
func (plainString) ListIterator() ipld.ListIterator {
	return nil
}
func (plainString) Length() int {
	return -1
}
func (plainString) IsAbsent() bool {
	return false
}
func (plainString) IsNull() bool {
	return false
}
func (plainString) AsBool() (bool, error) {
	return mixins.String{"string"}.AsBool()
}
func (plainString) AsInt() (int, error) {
	return mixins.String{"string"}.AsInt()
}
func (plainString) AsFloat() (float64, error) {
	return mixins.String{"string"}.AsFloat()
}
func (x plainString) AsString() (string, error) {
	return string(x), nil
}
func (plainString) AsBytes() ([]byte, error) {
	return mixins.String{"string"}.AsBytes()
}
func (plainString) AsLink() (ipld.Link, error) {
	return mixins.String{"string"}.AsLink()
}
func (plainString) Prototype() ipld.NodePrototype {
	return Prototype__String{}
}

// -- NodePrototype -->

type Prototype__String struct{}

func (Prototype__String) NewBuilder() ipld.NodeBuilder {
	var w plainString
	return &plainString__Builder{plainString__Assembler{w: &w}}
}

// -- NodeBuilder -->

type plainString__Builder struct {
	plainString__Assembler
}

func (nb *plainString__Builder) Build() ipld.Node {
	return nb.w
}
func (nb *plainString__Builder) Reset() {
	var w plainString
	*nb = plainString__Builder{plainString__Assembler{w: &w}}
}

// -- NodeAssembler -->

type plainString__Assembler struct {
	w *plainString
}

func (plainString__Assembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return mixins.StringAssembler{"string"}.BeginMap(0)
}
func (plainString__Assembler) BeginList(sizeHint int) (ipld.ListAssembler, error) {
	return mixins.StringAssembler{"string"}.BeginList(0)
}
func (plainString__Assembler) AssignNull() error {
	return mixins.StringAssembler{"string"}.AssignNull()
}
func (plainString__Assembler) AssignBool(bool) error {
	return mixins.StringAssembler{"string"}.AssignBool(false)
}
func (plainString__Assembler) AssignInt(int) error {
	return mixins.StringAssembler{"string"}.AssignInt(0)
}
func (plainString__Assembler) AssignFloat(float64) error {
	return mixins.StringAssembler{"string"}.AssignFloat(0)
}
func (na *plainString__Assembler) AssignString(v string) error {
	*na.w = plainString(v)
	return nil
}
func (plainString__Assembler) AssignBytes([]byte) error {
	return mixins.StringAssembler{"string"}.AssignBytes(nil)
}
func (plainString__Assembler) AssignLink(ipld.Link) error {
	return mixins.StringAssembler{"string"}.AssignLink(nil)
}
func (na *plainString__Assembler) AssignNode(v ipld.Node) error {
	if v2, err := v.AsString(); err != nil {
		return err
	} else {
		*na.w = plainString(v2)
		return nil
	}
}
func (plainString__Assembler) Prototype() ipld.NodePrototype {
	return Prototype__String{}
}
