package format

import (
	"errors"
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

type EmptyNode struct{}

var EmptyNodeError error = errors.New("dummy node")

func (n *EmptyNode) Resolve([]string) (interface{}, []string, error) {
	return nil, nil, EmptyNodeError
}

func (n *EmptyNode) Tree(string, int) []string {
	return nil
}

func (n *EmptyNode) ResolveLink([]string) (*Link, []string, error) {
	return nil, nil, EmptyNodeError
}

func (n *EmptyNode) Copy() Node {
	return &EmptyNode{}
}

func (n *EmptyNode) Cid() cid.Cid {
	id, err := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.ID,
		MhLength: 0,
	}.Sum(nil)

	if err != nil {
		panic("failed to create an empty cid!")
	}
	return id
}

func (n *EmptyNode) Links() []*Link {
	return nil
}

func (n *EmptyNode) Loggable() map[string]interface{} {
	return nil
}

func (n *EmptyNode) String() string {
	return "[]"
}

func (n *EmptyNode) RawData() []byte {
	return nil
}

func (n *EmptyNode) Size() (uint64, error) {
	return 0, nil
}

func (n *EmptyNode) Stat() (*NodeStat, error) {
	return &NodeStat{}, nil
}

func TestNodeType(t *testing.T) {
	// Type assertion.
	var _ Node = &EmptyNode{}
}
