package basicnode

import (
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/mixins"
)

var (
	_ ipld.Node          = &plainLink{}
	_ ipld.NodeStyle     = Style__Link{}
	_ ipld.NodeBuilder   = &plainLink__Builder{}
	_ ipld.NodeAssembler = &plainLink__Assembler{}
)

func NewLink(value ipld.Link) ipld.Node {
	return &plainLink{value}
}

// plainLink is a simple box around a Link that complies with ipld.Node.
type plainLink struct {
	x ipld.Link
}

// -- Node interface methods -->

func (plainLink) ReprKind() ipld.ReprKind {
	return ipld.ReprKind_Link
}
func (plainLink) LookupString(string) (ipld.Node, error) {
	return mixins.Link{"link"}.LookupString("")
}
func (plainLink) Lookup(key ipld.Node) (ipld.Node, error) {
	return mixins.Link{"link"}.Lookup(nil)
}
func (plainLink) LookupIndex(idx int) (ipld.Node, error) {
	return mixins.Link{"link"}.LookupIndex(0)
}
func (plainLink) LookupSegment(seg ipld.PathSegment) (ipld.Node, error) {
	return mixins.Link{"link"}.LookupSegment(seg)
}
func (plainLink) MapIterator() ipld.MapIterator {
	return nil
}
func (plainLink) ListIterator() ipld.ListIterator {
	return nil
}
func (plainLink) Length() int {
	return -1
}
func (plainLink) IsUndefined() bool {
	return false
}
func (plainLink) IsNull() bool {
	return false
}
func (plainLink) AsBool() (bool, error) {
	return mixins.Link{"link"}.AsBool()
}
func (plainLink) AsInt() (int, error) {
	return mixins.Link{"link"}.AsInt()
}
func (plainLink) AsFloat() (float64, error) {
	return mixins.Link{"link"}.AsFloat()
}
func (plainLink) AsString() (string, error) {
	return mixins.Link{"link"}.AsString()
}
func (plainLink) AsBytes() ([]byte, error) {
	return mixins.Link{"link"}.AsBytes()
}
func (n *plainLink) AsLink() (ipld.Link, error) {
	return n.x, nil
}
func (plainLink) Style() ipld.NodeStyle {
	return Style__Link{}
}

// -- NodeStyle -->

type Style__Link struct{}

func (Style__Link) NewBuilder() ipld.NodeBuilder {
	var w plainLink
	return &plainLink__Builder{plainLink__Assembler{w: &w}}
}

// -- NodeBuilder -->

type plainLink__Builder struct {
	plainLink__Assembler
}

func (nb *plainLink__Builder) Build() ipld.Node {
	return nb.w
}
func (nb *plainLink__Builder) Reset() {
	var w plainLink
	*nb = plainLink__Builder{plainLink__Assembler{w: &w}}
}

// -- NodeAssembler -->

type plainLink__Assembler struct {
	w *plainLink
}

func (plainLink__Assembler) BeginMap(sizeHint int) (ipld.MapAssembler, error) {
	return mixins.LinkAssembler{"link"}.BeginMap(0)
}
func (plainLink__Assembler) BeginList(sizeHint int) (ipld.ListAssembler, error) {
	return mixins.LinkAssembler{"link"}.BeginList(0)
}
func (plainLink__Assembler) AssignNull() error {
	return mixins.LinkAssembler{"link"}.AssignNull()
}
func (plainLink__Assembler) AssignBool(bool) error {
	return mixins.LinkAssembler{"link"}.AssignBool(false)
}
func (plainLink__Assembler) AssignInt(int) error {
	return mixins.LinkAssembler{"link"}.AssignInt(0)
}
func (plainLink__Assembler) AssignFloat(float64) error {
	return mixins.LinkAssembler{"link"}.AssignFloat(0)
}
func (plainLink__Assembler) AssignString(string) error {
	return mixins.LinkAssembler{"link"}.AssignString("")
}
func (plainLink__Assembler) AssignBytes([]byte) error {
	return mixins.LinkAssembler{"link"}.AssignBytes(nil)
}
func (na *plainLink__Assembler) AssignLink(v ipld.Link) error {
	na.w.x = v
	return nil
}
func (na *plainLink__Assembler) AssignNode(v ipld.Node) error {
	if v2, err := v.AsLink(); err != nil {
		return err
	} else {
		na.w.x = v2
		return nil
	}
}
func (plainLink__Assembler) Style() ipld.NodeStyle {
	return Style__Link{}
}
