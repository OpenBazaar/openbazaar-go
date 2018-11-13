package format

import (
	"context"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	"testing"
)

type TestNode struct {
	links   []*Link
	data    []byte
	builder cid.Builder
}

var v0CidPrefix = cid.Prefix{
	Codec:    cid.DagProtobuf,
	MhLength: -1,
	MhType:   mh.SHA2_256,
	Version:  0,
}

func InitNode(d []byte) *TestNode {
	return &TestNode{
		data:    d,
		builder: v0CidPrefix,
	}
}

func (n *TestNode) Resolve([]string) (interface{}, []string, error) {
	return nil, nil, EmptyNodeError
}

func (n *TestNode) Tree(string, int) []string {
	return nil
}

func (n *TestNode) ResolveLink([]string) (*Link, []string, error) {
	return nil, nil, EmptyNodeError
}

func (n *TestNode) Copy() Node {
	return &EmptyNode{}
}

func (n *TestNode) Cid() cid.Cid {
	c, err := n.builder.Sum(n.RawData())
	if err != nil {
		return cid.Cid{}
	}
	return c
}

func (n *TestNode) Links() []*Link {
	return n.links
}

func (n *TestNode) Loggable() map[string]interface{} {
	return nil
}

func (n *TestNode) String() string {
	return string(n.data)
}

func (n *TestNode) RawData() []byte {
	return n.data
}

func (n *TestNode) Size() (uint64, error) {
	return 0, nil
}

func (n *TestNode) Stat() (*NodeStat, error) {
	return &NodeStat{}, nil
}

// AddNodeLink adds a link to another node.
func (n *TestNode) AddNodeLink(name string, that Node) error {

	lnk, err := MakeLink(that)
	if err != nil {
		return err
	}

	lnk.Name = name

	n.AddRawLink(name, lnk)

	return nil
}

func (n *TestNode) AddRawLink(name string, l *Link) error {

	n.links = append(n.links, &Link{
		Name: name,
		Size: l.Size,
		Cid:  l.Cid,
	})
	return nil
}

func TestCopy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	from := newTestDag()

	root := InitNode([]byte("level0"))
	l11 := InitNode([]byte("leve1_node1"))
	l12 := InitNode([]byte("leve1_node2"))
	l21 := InitNode([]byte("leve2_node1"))
	l22 := InitNode([]byte("leve2_node2"))
	l23 := InitNode([]byte("leve2_node3"))

	l11.AddNodeLink(l21.Cid().String(), l21)
	l11.AddNodeLink(l22.Cid().String(), l22)
	l11.AddNodeLink(l23.Cid().String(), l23)
	root.AddNodeLink(l11.Cid().String(), l11)
	root.AddNodeLink(l12.Cid().String(), l12)

	for _, n := range []Node{l23, l22, l21, l12, l11, root} {
		err := from.Add(ctx, n)
		if err != nil {
			t.Fatal(err)
		}
	}
	to := newTestDag()
	err := Copy(ctx, from, to, root.Cid())
	if err != nil {
		t.Error(err)
	}

	r, err := to.Get(ctx, root.Cid())
	if err != nil || len(r.Links()) != 2 {
		t.Error("fail to copy dag")
	}
	l1, err := to.Get(ctx, l11.Cid())
	if err != nil || len(l1.Links()) != 3 {
		t.Error("fail to copy dag")
	}
}
