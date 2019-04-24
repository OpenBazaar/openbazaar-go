package ipldgit

import (
	"encoding/json"
	"errors"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	node "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
)

type Blob struct {
	rawData []byte
	cid     cid.Cid
}

func (b *Blob) Cid() cid.Cid {
	return b.cid
}

func (b *Blob) Copy() node.Node {
	nb := *b
	return &nb
}

func (b *Blob) Links() []*node.Link {
	return nil
}

func (b *Blob) Resolve(_ []string) (interface{}, []string, error) {
	return nil, nil, errors.New("no such link")
}

func (b *Blob) ResolveLink(_ []string) (*node.Link, []string, error) {
	return nil, nil, errors.New("no such link")
}

func (b *Blob) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"type": "git_blob",
	}
}

func (b *Blob) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.rawData)
}

func (b *Blob) RawData() []byte {
	return []byte(b.rawData)
}

func (b *Blob) Size() (uint64, error) {
	return uint64(len(b.rawData)), nil
}

func (b *Blob) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{}, nil
}

func (b *Blob) String() string {
	return "[git blob]"
}

func (b *Blob) Tree(p string, depth int) []string {
	return nil
}

func (b *Blob) GitSha() []byte {
	return cidToSha(b.Cid())
}

var _ node.Node = (*Blob)(nil)
