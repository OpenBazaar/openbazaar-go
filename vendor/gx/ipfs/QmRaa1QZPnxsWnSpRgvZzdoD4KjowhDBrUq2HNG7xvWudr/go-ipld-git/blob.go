package ipldgit

import (
	"errors"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	node "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

type Blob []byte

func (b Blob) Cid() cid.Cid {
	c, _ := cid.Prefix{
		MhType:   mh.SHA1,
		MhLength: -1,
		Codec:    cid.GitRaw,
		Version:  1,
	}.Sum([]byte(b))
	return c
}

func (b Blob) Copy() node.Node {
	out := make([]byte, len(b))
	copy(out, b)
	return Blob(out)
}

func (b Blob) Links() []*node.Link {
	return nil
}

func (b Blob) Resolve(_ []string) (interface{}, []string, error) {
	return nil, nil, errors.New("no such link")
}

func (b Blob) ResolveLink(_ []string) (*node.Link, []string, error) {
	return nil, nil, errors.New("no such link")
}

func (b Blob) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"type": "git_blob",
	}
}

func (b Blob) RawData() []byte {
	return []byte(b)
}

func (b Blob) Size() (uint64, error) {
	return uint64(len(b)), nil
}

func (b Blob) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{}, nil
}

func (b Blob) String() string {
	return "[git blob]"
}

func (b Blob) Tree(p string, depth int) []string {
	return nil
}

func (b Blob) GitSha() []byte {
	return cidToSha(b.Cid())
}

var _ node.Node = (Blob)(nil)
