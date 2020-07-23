package basicnode

import (
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/mixins"
)

var (
	_ ipld.Node          = plainBytes(nil)
	_ ipld.NodeStyle     = Style__Bytes{}
	_ ipld.NodeBuilder   = &plainBytes__Builder{}
	_ ipld.NodeAssembler = &plainBytes__Assembler{}
)

func NewBytes(value []byte) ipld.Node {
	v := plainBytes(value)
	return &v
}

// plainBytes is a simple boxed byte slice that complies with ipld.Node.
type plainBytes []byte

// -- Node interface methods -->

func (plainBytes) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_Bytes
}
func (plainBytes) LookupString(string) (ipld.Node, error) {
	return mixins.Bytes{"bytes"}.LookupString("")
}
func (plainBytes) Lookup(key ipld.Node) (ipld.Node, error) {
	return mixins.Bytes{"bytes"}.Lookup(nil)
}
func (plainBytes) LookupIndex(idx int) (ipld.Node, error) {
	return mixins.Bytes{"bytes"}.LookupIndex(0)
}
func (plainBytes) LookupSegment(seg ipld.PathSegment) (ipld.Node, error) {
	return mixins.Bytes{"bytes"}.LookupSegment(seg)
}
func (plainBytes) MapIterator() ipld.MapIterator {
	return nil
}
func (plainBytes) ListIterator() ipld.ListIterator {
	return nil
}
func (plainBytes) Length() int {
	return -1
}
func (plainBytes) IsUndefined() bool {
	return false
}
func (plainBytes) IsNull() bool {
	return false
}
func (plainBytes) AsBool() (bool, error) {
	return mixins.Bytes{"bytes"}.AsBool()
}
func (plainBytes) AsInt() (int, error) {
	return mixins.Bytes{"bytes"}.AsInt()
}
func (plainBytes) AsFloat() (float64, error) {
	return mixins.Bytes{"bytes"}.AsFloat()
}
func (plainBytes) AsString() (string, error) {
	return mixins.Bytes{"bytes"}.AsString()
}
func (n plainBytes) AsBytes() ([]byte, error) {
	return []byte(n), nil
}
func (plainBytes) AsLink() (ipld.Link, error) {
	return mixins.Bytes{"bytes"}.AsLink()
}
func (plainBytes) Style() ipld.NodeStyle {
	return Style__Bytes{}
}

// -- NodeStyle -->

type Style__Bytes struct{}

func (Style__Bytes) NewBuilder() ipld.NodeBuilder {
	var w plainBytes
	return &plainBytes__Builder{plainBytes__Assembler{w: &w}}
}

// -- NodeBuilder -->

type plainBytes__Builder struct {
	plainBytes__Assembler
}

func (nb *plainBytes__Builder) Build() ipld.Node {
	return nb.w
}
func (nb *plainBytes__Builder) Reset() {
	var w plainBytes
	*nb = plainBytes__Builder{plainBytes__Assembler{w: &w}}
}

// -- NodeAssembler -->

type plainBytes__Assembler struct {
	w *plainBytes
}

func (plainBytes__Assembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return mixins.BytesAssembler{"bytes"}.BeginMap(0)
}
func (plainBytes__Assembler) BeginList(sizeHint int) (ipld.ListAssembler, error) {
	return mixins.BytesAssembler{"bytes"}.BeginList(0)
}
func (plainBytes__Assembler) AssignNull() error {
	return mixins.BytesAssembler{"bytes"}.AssignNull()
}
func (plainBytes__Assembler) AssignBool(bool) error {
	return mixins.BytesAssembler{"bytes"}.AssignBool(false)
}
func (plainBytes__Assembler) AssignInt(int) error {
	return mixins.BytesAssembler{"bytes"}.AssignInt(0)
}
func (plainBytes__Assembler) AssignFloat(float64) error {
	return mixins.BytesAssembler{"bytes"}.AssignFloat(0)
}
func (plainBytes__Assembler) AssignString(string) error {
	return mixins.BytesAssembler{"bytes"}.AssignString("")
}
func (na *plainBytes__Assembler) AssignBytes(v []byte) error {
	*na.w = plainBytes(v)
	return nil
}
func (plainBytes__Assembler) AssignLink(ipld.Link) error {
	return mixins.BytesAssembler{"bytes"}.AssignLink(nil)
}
func (na *plainBytes__Assembler) AssignNode(v ipld.Node) error {
	if v2, err := v.AsBytes(); err != nil {
		return err
	} else {
		*na.w = plainBytes(v2)
		return nil
	}
}
func (plainBytes__Assembler) Style() ipld.NodeStyle {
	return Style__Bytes{}
}
