package fluent

import (
	ipld "github.com/ipld/go-ipld-prime"
)

func Build(ns ipld.NodeStyle, fn func(NodeAssembler)) (ipld.Node, error) {
	nb := ns.NewBuilder()
	fna := WrapAssembler(nb)
	err := Recover(func() {
		fn(fna)
	})
	return nb.Build(), err
}

func MustBuild(ns ipld.NodeStyle, fn func(NodeAssembler)) ipld.Node {
	nb := ns.NewBuilder()
	fn(WrapAssembler(nb))
	return nb.Build()
}
func MustBuildMap(ns ipld.NodeStyle, sizeHint int, fn func(MapAssembler)) ipld.Node {
	return MustBuild(ns, func(fna NodeAssembler) { fna.CreateMap(sizeHint, fn) })
}
func MustBuildList(ns ipld.NodeStyle, sizeHint int, fn func(ListAssembler)) ipld.Node {
	return MustBuild(ns, func(fna NodeAssembler) { fna.CreateList(sizeHint, fn) })
}

func WrapAssembler(na ipld.NodeAssembler) NodeAssembler {
	return &nodeAssembler{na}
}

// NodeAssembler is the same as the interface in the core package, except:
// instead of returning errors, any error will cause panic
// (and you can collect these with `fluent.Recover`);
// and all recursive operations take a function as a parameter,
// within which you will receive another {Map,List,}NodeAssembler.
type NodeAssembler interface {
	CreateMap(sizeHint int, fn func(MapAssembler))
	CreateList(sizeHint int, fn func(ListAssembler))
	AssignNull()
	AssignBool(bool)
	AssignInt(int)
	AssignFloat(float64)
	AssignString(string)
	AssignBytes([]byte)
	AssignLink(ipld.Link)
	AssignNode(ipld.Node)

	Style() ipld.NodeStyle
}

// MapAssembler is the same as the interface in the core package, except:
// instead of returning errors, any error will cause panic
// (and you can collect these with `fluent.Recover`);
// and all recursive operations take a function as a parameter,
// within which you will receive another {Map,List,}NodeAssembler.
type MapAssembler interface {
	AssembleKey() NodeAssembler
	AssembleValue() NodeAssembler

	AssembleEntry(k string) NodeAssembler

	KeyStyle() ipld.NodeStyle
	ValueStyle(k string) ipld.NodeStyle
}

// ListAssembler is the same as the interface in the core package, except:
// instead of returning errors, any error will cause panic
// (and you can collect these with `fluent.Recover`);
// and all recursive operations take a function as a parameter,
// within which you will receive another {Map,List,}NodeAssembler.
type ListAssembler interface {
	AssembleValue() NodeAssembler

	ValueStyle(idx int) ipld.NodeStyle
}

type nodeAssembler struct {
	na ipld.NodeAssembler
}

func (fna *nodeAssembler) CreateMap(sizeHint int, fn func(MapAssembler)) {
	if ma, err := fna.na.BeginMap(sizeHint); err != nil {
		panic(Error{err})
	} else {
		fn(&mapNodeAssembler{ma})
		if err := ma.Finish(); err != nil {
			panic(Error{err})
		}
	}
}
func (fna *nodeAssembler) CreateList(sizeHint int, fn func(ListAssembler)) {
	if la, err := fna.na.BeginList(sizeHint); err != nil {
		panic(Error{err})
	} else {
		fn(&listNodeAssembler{la})
		if err := la.Finish(); err != nil {
			panic(Error{err})
		}
	}
}
func (fna *nodeAssembler) AssignNull() {
	if err := fna.na.AssignNull(); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignBool(v bool) {
	if err := fna.na.AssignBool(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignInt(v int) {
	if err := fna.na.AssignInt(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignFloat(v float64) {
	if err := fna.na.AssignFloat(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignString(v string) {
	if err := fna.na.AssignString(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignBytes(v []byte) {
	if err := fna.na.AssignBytes(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignLink(v ipld.Link) {
	if err := fna.na.AssignLink(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) AssignNode(v ipld.Node) {
	if err := fna.na.AssignNode(v); err != nil {
		panic(Error{err})
	}
}
func (fna *nodeAssembler) Style() ipld.NodeStyle {
	return fna.na.Style()
}

type mapNodeAssembler struct {
	ma ipld.MapAssembler
}

func (fma *mapNodeAssembler) AssembleKey() NodeAssembler {
	return &nodeAssembler{fma.ma.AssembleKey()}
}
func (fma *mapNodeAssembler) AssembleValue() NodeAssembler {
	return &nodeAssembler{fma.ma.AssembleValue()}
}
func (fma *mapNodeAssembler) AssembleEntry(k string) NodeAssembler {
	va, err := fma.ma.AssembleEntry(k)
	if err != nil {
		panic(Error{err})
	}
	return &nodeAssembler{va}
}
func (fma *mapNodeAssembler) KeyStyle() ipld.NodeStyle {
	return fma.ma.KeyStyle()
}
func (fma *mapNodeAssembler) ValueStyle(k string) ipld.NodeStyle {
	return fma.ma.ValueStyle(k)
}

type listNodeAssembler struct {
	la ipld.ListAssembler
}

func (fla *listNodeAssembler) AssembleValue() NodeAssembler {
	return &nodeAssembler{fla.la.AssembleValue()}
}
func (fla *listNodeAssembler) ValueStyle(idx int) ipld.NodeStyle {
	return fla.la.ValueStyle(idx)
}
