package merkledag

import (
	"context"
	"encoding/json"
	"fmt"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

// Common errors
var (
	ErrNotProtobuf  = fmt.Errorf("expected protobuf dag node")
	ErrLinkNotFound = fmt.Errorf("no link by that name")
)

// ProtoNode represents a node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type ProtoNode struct {
	links []*ipld.Link
	data  []byte

	// cache encoded/marshaled value
	encoded []byte

	cached cid.Cid

	// builder specifies cid version and hashing function
	builder cid.Builder
}

var v0CidPrefix = cid.Prefix{
	Codec:    cid.DagProtobuf,
	MhLength: -1,
	MhType:   mh.SHA2_256,
	Version:  0,
}

var v1CidPrefix = cid.Prefix{
	Codec:    cid.DagProtobuf,
	MhLength: -1,
	MhType:   mh.SHA2_256,
	Version:  1,
}

// V0CidPrefix returns a prefix for CIDv0
func V0CidPrefix() cid.Prefix { return v0CidPrefix }

// V1CidPrefix returns a prefix for CIDv1 with the default settings
func V1CidPrefix() cid.Prefix { return v1CidPrefix }

// PrefixForCidVersion returns the Protobuf prefix for a given CID version
func PrefixForCidVersion(version int) (cid.Prefix, error) {
	switch version {
	case 0:
		return v0CidPrefix, nil
	case 1:
		return v1CidPrefix, nil
	default:
		return cid.Prefix{}, fmt.Errorf("unknown CID version: %d", version)
	}
}

// CidBuilder returns the CID Builder for this ProtoNode, it is never nil
func (n *ProtoNode) CidBuilder() cid.Builder {
	if n.builder == nil {
		n.builder = v0CidPrefix
	}
	return n.builder
}

// SetCidBuilder sets the CID builder if it is non nil, if nil then it
// is reset to the default value
func (n *ProtoNode) SetCidBuilder(builder cid.Builder) {
	if builder == nil {
		n.builder = v0CidPrefix
	} else {
		n.builder = builder.WithCodec(cid.DagProtobuf)
		n.encoded = nil
		n.cached = cid.Undef
	}
}

// LinkSlice is a slice of ipld.Links
type LinkSlice []*ipld.Link

func (ls LinkSlice) Len() int           { return len(ls) }
func (ls LinkSlice) Swap(a, b int)      { ls[a], ls[b] = ls[b], ls[a] }
func (ls LinkSlice) Less(a, b int) bool { return ls[a].Name < ls[b].Name }

// NodeWithData builds a new Protonode with the given data.
func NodeWithData(d []byte) *ProtoNode {
	return &ProtoNode{data: d}
}

// AddNodeLink adds a link to another node.
func (n *ProtoNode) AddNodeLink(name string, that ipld.Node) error {
	n.encoded = nil

	lnk, err := ipld.MakeLink(that)
	if err != nil {
		return err
	}

	lnk.Name = name

	n.AddRawLink(name, lnk)

	return nil
}

// AddRawLink adds a copy of a link to this node
func (n *ProtoNode) AddRawLink(name string, l *ipld.Link) error {
	n.encoded = nil
	n.links = append(n.links, &ipld.Link{
		Name: name,
		Size: l.Size,
		Cid:  l.Cid,
	})

	return nil
}

// RemoveNodeLink removes a link on this node by the given name.
func (n *ProtoNode) RemoveNodeLink(name string) error {
	n.encoded = nil

	ref := n.links[:0]
	found := false

	for _, v := range n.links {
		if v.Name != name {
			ref = append(ref, v)
		} else {
			found = true
		}
	}

	if !found {
		return ipld.ErrNotFound
	}

	n.links = ref

	return nil
}

// GetNodeLink returns a copy of the link with the given name.
func (n *ProtoNode) GetNodeLink(name string) (*ipld.Link, error) {
	for _, l := range n.links {
		if l.Name == name {
			return &ipld.Link{
				Name: l.Name,
				Size: l.Size,
				Cid:  l.Cid,
			}, nil
		}
	}
	return nil, ErrLinkNotFound
}

// GetLinkedProtoNode returns a copy of the ProtoNode with the given name.
func (n *ProtoNode) GetLinkedProtoNode(ctx context.Context, ds ipld.DAGService, name string) (*ProtoNode, error) {
	nd, err := n.GetLinkedNode(ctx, ds, name)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*ProtoNode)
	if !ok {
		return nil, ErrNotProtobuf
	}

	return pbnd, nil
}

// GetLinkedNode returns a copy of the IPLD Node with the given name.
func (n *ProtoNode) GetLinkedNode(ctx context.Context, ds ipld.DAGService, name string) (ipld.Node, error) {
	lnk, err := n.GetNodeLink(name)
	if err != nil {
		return nil, err
	}

	return lnk.GetNode(ctx, ds)
}

// Copy returns a copy of the node.
// NOTE: Does not make copies of Node objects in the links.
func (n *ProtoNode) Copy() ipld.Node {
	nnode := new(ProtoNode)
	if len(n.data) > 0 {
		nnode.data = make([]byte, len(n.data))
		copy(nnode.data, n.data)
	}

	if len(n.links) > 0 {
		nnode.links = make([]*ipld.Link, len(n.links))
		copy(nnode.links, n.links)
	}

	nnode.builder = n.builder

	return nnode
}

// RawData returns the protobuf-encoded version of the node.
func (n *ProtoNode) RawData() []byte {
	out, _ := n.EncodeProtobuf(false)
	return out
}

// Data returns the data stored by this node.
func (n *ProtoNode) Data() []byte {
	return n.data
}

// SetData stores data in this nodes.
func (n *ProtoNode) SetData(d []byte) {
	n.encoded = nil
	n.cached = cid.Undef
	n.data = d
}

// UpdateNodeLink return a copy of the node with the link name set to point to
// that. If a link of the same name existed, it is removed.
func (n *ProtoNode) UpdateNodeLink(name string, that *ProtoNode) (*ProtoNode, error) {
	newnode := n.Copy().(*ProtoNode)
	_ = newnode.RemoveNodeLink(name) // ignore error
	err := newnode.AddNodeLink(name, that)
	return newnode, err
}

// Size returns the total size of the data addressed by node,
// including the total sizes of references.
func (n *ProtoNode) Size() (uint64, error) {
	b, err := n.EncodeProtobuf(false)
	if err != nil {
		return 0, err
	}

	s := uint64(len(b))
	for _, l := range n.links {
		s += l.Size
	}
	return s, nil
}

// Stat returns statistics on the node.
func (n *ProtoNode) Stat() (*ipld.NodeStat, error) {
	enc, err := n.EncodeProtobuf(false)
	if err != nil {
		return nil, err
	}

	cumSize, err := n.Size()
	if err != nil {
		return nil, err
	}

	return &ipld.NodeStat{
		Hash:           n.Cid().String(),
		NumLinks:       len(n.links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.data), // includes framing.
		DataSize:       len(n.data),
		CumulativeSize: int(cumSize),
	}, nil
}

// Loggable implements the ipfs/go-log.Loggable interface.
func (n *ProtoNode) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"node": n.String(),
	}
}

// UnmarshalJSON reads the node fields from a JSON-encoded byte slice.
func (n *ProtoNode) UnmarshalJSON(b []byte) error {
	s := struct {
		Data  []byte       `json:"data"`
		Links []*ipld.Link `json:"links"`
	}{}

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	n.data = s.Data
	n.links = s.Links
	return nil
}

// MarshalJSON returns a JSON representation of the node.
func (n *ProtoNode) MarshalJSON() ([]byte, error) {
	out := map[string]interface{}{
		"data":  n.data,
		"links": n.links,
	}

	return json.Marshal(out)
}

// Cid returns the node's Cid, calculated according to its prefix
// and raw data contents.
func (n *ProtoNode) Cid() cid.Cid {
	if n.encoded != nil && n.cached.Defined() {
		return n.cached
	}

	c, err := n.builder.Sum(n.RawData())
	if err != nil {
		// programmer error
		err = fmt.Errorf("invalid CID of length %d: %x: %v", len(n.RawData()), n.RawData(), err)
		panic(err)
	}

	n.cached = c
	return c
}

// String prints the node's Cid.
func (n *ProtoNode) String() string {
	return n.Cid().String()
}

// Multihash hashes the encoded data of this node.
func (n *ProtoNode) Multihash() mh.Multihash {
	// NOTE: EncodeProtobuf generates the hash and puts it in n.cached.
	_, err := n.EncodeProtobuf(false)
	if err != nil {
		// Note: no possibility exists for an error to be returned through here
		panic(err)
	}

	return n.cached.Hash()
}

// Links returns the node links.
func (n *ProtoNode) Links() []*ipld.Link {
	return n.links
}

// SetLinks replaces the node links with the given ones.
func (n *ProtoNode) SetLinks(links []*ipld.Link) {
	n.links = links
}

// Resolve is an alias for ResolveLink.
func (n *ProtoNode) Resolve(path []string) (interface{}, []string, error) {
	return n.ResolveLink(path)
}

// ResolveLink consumes the first element of the path and obtains the link
// corresponding to it from the node. It returns the link
// and the path without the consumed element.
func (n *ProtoNode) ResolveLink(path []string) (*ipld.Link, []string, error) {
	if len(path) == 0 {
		return nil, nil, fmt.Errorf("end of path, no more links to resolve")
	}

	lnk, err := n.GetNodeLink(path[0])
	if err != nil {
		return nil, nil, err
	}

	return lnk, path[1:], nil
}

// Tree returns the link names of the ProtoNode.
// ProtoNodes are only ever one path deep, so anything different than an empty
// string for p results in nothing. The depth parameter is ignored.
func (n *ProtoNode) Tree(p string, depth int) []string {
	if p != "" {
		return nil
	}

	out := make([]string, 0, len(n.links))
	for _, lnk := range n.links {
		out = append(out, lnk.Name)
	}
	return out
}
