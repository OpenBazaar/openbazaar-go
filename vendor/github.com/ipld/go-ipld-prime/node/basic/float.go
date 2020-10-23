package basicnode

import (
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/mixins"
)

var (
	_ ipld.Node          = plainFloat(0)
	_ ipld.NodePrototype = Prototype__Float{}
	_ ipld.NodeBuilder   = &plainFloat__Builder{}
	_ ipld.NodeAssembler = &plainFloat__Assembler{}
)

func NewFloat(value float64) ipld.Node {
	v := plainFloat(value)
	return &v
}

// plainFloat is a simple boxed float that complies with ipld.Node.
type plainFloat float64

// -- Node interface methods -->

func (plainFloat) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_Float
}
func (plainFloat) LookupByString(string) (ipld.Node, error) {
	return mixins.Float{"float"}.LookupByString("")
}
func (plainFloat) LookupByNode(key ipld.Node) (ipld.Node, error) {
	return mixins.Float{"float"}.LookupByNode(nil)
}
func (plainFloat) LookupByIndex(idx int) (ipld.Node, error) {
	return mixins.Float{"float"}.LookupByIndex(0)
}
func (plainFloat) LookupBySegment(seg ipld.PathSegment) (ipld.Node, error) {
	return mixins.Float{"float"}.LookupBySegment(seg)
}
func (plainFloat) MapIterator() ipld.MapIterator {
	return nil
}
func (plainFloat) ListIterator() ipld.ListIterator {
	return nil
}
func (plainFloat) Length() int {
	return -1
}
func (plainFloat) IsAbsent() bool {
	return false
}
func (plainFloat) IsNull() bool {
	return false
}
func (plainFloat) AsBool() (bool, error) {
	return mixins.Float{"float"}.AsBool()
}
func (plainFloat) AsInt() (int, error) {
	return mixins.Float{"float"}.AsInt()
}
func (n plainFloat) AsFloat() (float64, error) {
	return float64(n), nil
}
func (plainFloat) AsString() (string, error) {
	return mixins.Float{"float"}.AsString()
}
func (plainFloat) AsBytes() ([]byte, error) {
	return mixins.Float{"float"}.AsBytes()
}
func (plainFloat) AsLink() (ipld.Link, error) {
	return mixins.Float{"float"}.AsLink()
}
func (plainFloat) Prototype() ipld.NodePrototype {
	return Prototype__Float{}
}

// -- NodePrototype -->

type Prototype__Float struct{}

func (Prototype__Float) NewBuilder() ipld.NodeBuilder {
	var w plainFloat
	return &plainFloat__Builder{plainFloat__Assembler{w: &w}}
}

// -- NodeBuilder -->

type plainFloat__Builder struct {
	plainFloat__Assembler
}

func (nb *plainFloat__Builder) Build() ipld.Node {
	return nb.w
}
func (nb *plainFloat__Builder) Reset() {
	var w plainFloat
	*nb = plainFloat__Builder{plainFloat__Assembler{w: &w}}
}

// -- NodeAssembler -->

type plainFloat__Assembler struct {
	w *plainFloat
}

func (plainFloat__Assembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return mixins.FloatAssembler{"float"}.BeginMap(0)
}
func (plainFloat__Assembler) BeginList(sizeHint int) (ipld.ListAssembler, error) {
	return mixins.FloatAssembler{"float"}.BeginList(0)
}
func (plainFloat__Assembler) AssignNull() error {
	return mixins.FloatAssembler{"float"}.AssignNull()
}
func (plainFloat__Assembler) AssignBool(bool) error {
	return mixins.FloatAssembler{"float"}.AssignBool(false)
}
func (plainFloat__Assembler) AssignInt(int) error {
	return mixins.FloatAssembler{"float"}.AssignInt(0)
}
func (na *plainFloat__Assembler) AssignFloat(v float64) error {
	*na.w = plainFloat(v)
	return nil
}
func (plainFloat__Assembler) AssignString(string) error {
	return mixins.FloatAssembler{"float"}.AssignString("")
}
func (plainFloat__Assembler) AssignBytes([]byte) error {
	return mixins.FloatAssembler{"float"}.AssignBytes(nil)
}
func (plainFloat__Assembler) AssignLink(ipld.Link) error {
	return mixins.FloatAssembler{"float"}.AssignLink(nil)
}
func (na *plainFloat__Assembler) AssignNode(v ipld.Node) error {
	if v2, err := v.AsFloat(); err != nil {
		return err
	} else {
		*na.w = plainFloat(v2)
		return nil
	}
}
func (plainFloat__Assembler) Prototype() ipld.NodePrototype {
	return Prototype__Float{}
}
