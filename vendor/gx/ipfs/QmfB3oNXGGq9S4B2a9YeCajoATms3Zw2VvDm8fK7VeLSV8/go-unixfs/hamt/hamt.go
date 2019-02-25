// Package hamt implements a Hash Array Mapped Trie over ipfs merkledag nodes.
// It is implemented mostly as described in the wikipedia article on HAMTs,
// however the table size is variable (usually 256 in our usages) as opposed to
// 32 as suggested in the article.  The hash function used is currently
// Murmur3, but this value is configurable (the datastructure reports which
// hash function its using).
//
// The one algorithmic change we implement that is not mentioned in the
// wikipedia article is the collapsing of empty shards.
// Given the following tree: ( '[' = shards, '{' = values )
// [ 'A' ] -> [ 'B' ] -> { "ABC" }
//    |       L-> { "ABD" }
//    L-> { "ASDF" }
// If we simply removed "ABC", we would end up with a tree where shard 'B' only
// has a single child.  This causes two issues, the first, is that now we have
// an extra lookup required to get to "ABD".  The second issue is that now we
// have a tree that contains only "ABD", but is not the same tree that we would
// get by simply inserting "ABD" into a new tree.  To address this, we always
// check for empty shard nodes upon deletion and prune them to maintain a
// consistent tree, independent of insertion order.
package hamt

import (
	"context"
	"fmt"
	"os"
	"sync"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	dag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	bitfield "gx/ipfs/QmTbBs3Y3u5F69XNJzdnnc6SP5GKgcXxCDzx6w8m6piVRT/go-bitfield"
	format "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"
	"gx/ipfs/QmfJHywXQu98UeZtGJBQrPAR6AtmDjjbe3qjTo9piXHPnx/murmur3"
)

const (
	// HashMurmur3 is the multiformats identifier for Murmur3
	HashMurmur3 uint64 = 0x22
)

func (ds *Shard) isValueNode() bool {
	return ds.key != "" && ds.val != nil
}

// A Shard represents the HAMT. It should be initialized with NewShard().
type Shard struct {
	nd *dag.ProtoNode

	bitfield bitfield.Bitfield

	children []*Shard

	tableSize    int
	tableSizeLg2 int

	builder  cid.Builder
	hashFunc uint64

	prefixPadStr string
	maxpadlen    int

	dserv ipld.DAGService

	// leaf node
	key string
	val *ipld.Link
}

// NewShard creates a new, empty HAMT shard with the given size.
func NewShard(dserv ipld.DAGService, size int) (*Shard, error) {
	ds, err := makeShard(dserv, size)
	if err != nil {
		return nil, err
	}

	ds.nd = new(dag.ProtoNode)
	ds.hashFunc = HashMurmur3
	return ds, nil
}

func makeShard(ds ipld.DAGService, size int) (*Shard, error) {
	lg2s, err := logtwo(size)
	if err != nil {
		return nil, err
	}
	maxpadding := fmt.Sprintf("%X", size-1)
	return &Shard{
		tableSizeLg2: lg2s,
		prefixPadStr: fmt.Sprintf("%%0%dX", len(maxpadding)),
		maxpadlen:    len(maxpadding),
		bitfield:     bitfield.NewBitfield(size),
		tableSize:    size,
		dserv:        ds,
	}, nil
}

// NewHamtFromDag creates new a HAMT shard from the given DAG.
func NewHamtFromDag(dserv ipld.DAGService, nd ipld.Node) (*Shard, error) {
	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	fsn, err := format.FSNodeFromBytes(pbnd.Data())
	if err != nil {
		return nil, err
	}

	if fsn.Type() != format.THAMTShard {
		return nil, fmt.Errorf("node was not a dir shard")
	}

	if fsn.HashType() != HashMurmur3 {
		return nil, fmt.Errorf("only murmur3 supported as hash function")
	}

	ds, err := makeShard(dserv, int(fsn.Fanout()))
	if err != nil {
		return nil, err
	}

	ds.nd = pbnd.Copy().(*dag.ProtoNode)
	ds.children = make([]*Shard, len(pbnd.Links()))
	ds.bitfield.SetBytes(fsn.Data())
	ds.hashFunc = fsn.HashType()
	ds.builder = ds.nd.CidBuilder()

	return ds, nil
}

// SetCidBuilder sets the CID Builder
func (ds *Shard) SetCidBuilder(builder cid.Builder) {
	ds.builder = builder
}

// CidBuilder gets the CID Builder, may be nil if unset
func (ds *Shard) CidBuilder() cid.Builder {
	return ds.builder
}

// Node serializes the HAMT structure into a merkledag node with unixfs formatting
func (ds *Shard) Node() (ipld.Node, error) {
	out := new(dag.ProtoNode)
	out.SetCidBuilder(ds.builder)

	cindex := 0
	// TODO: optimized 'for each set bit'
	for i := 0; i < ds.tableSize; i++ {
		if !ds.bitfield.Bit(i) {
			continue
		}

		ch := ds.children[cindex]
		if ch != nil {
			clnk, err := ch.Link()
			if err != nil {
				return nil, err
			}

			err = out.AddRawLink(ds.linkNamePrefix(i)+ch.key, clnk)
			if err != nil {
				return nil, err
			}
		} else {
			// child unloaded, just copy in link with updated name
			lnk := ds.nd.Links()[cindex]
			label := lnk.Name[ds.maxpadlen:]

			err := out.AddRawLink(ds.linkNamePrefix(i)+label, lnk)
			if err != nil {
				return nil, err
			}
		}
		cindex++
	}

	data, err := format.HAMTShardData(ds.bitfield.Bytes(), uint64(ds.tableSize), HashMurmur3)
	if err != nil {
		return nil, err
	}

	out.SetData(data)

	err = ds.dserv.Add(context.TODO(), out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (ds *Shard) makeShardValue(lnk *ipld.Link) (*Shard, error) {
	lnk2 := *lnk
	s, err := makeShard(ds.dserv, ds.tableSize)
	if err != nil {
		return nil, err
	}

	s.key = lnk.Name[ds.maxpadlen:]
	s.val = &lnk2

	return s, nil
}

func hash(val []byte) []byte {
	h := murmur3.New64()
	h.Write(val)
	return h.Sum(nil)
}

// Set sets 'name' = nd in the HAMT
func (ds *Shard) Set(ctx context.Context, name string, nd ipld.Node) error {
	hv := &hashBits{b: hash([]byte(name))}
	err := ds.dserv.Add(ctx, nd)
	if err != nil {
		return err
	}

	lnk, err := ipld.MakeLink(nd)
	if err != nil {
		return err
	}
	lnk.Name = ds.linkNamePrefix(0) + name

	return ds.modifyValue(ctx, hv, name, lnk)
}

// Remove deletes the named entry if it exists, this operation is idempotent.
func (ds *Shard) Remove(ctx context.Context, name string) error {
	hv := &hashBits{b: hash([]byte(name))}
	return ds.modifyValue(ctx, hv, name, nil)
}

// Find searches for a child node by 'name' within this hamt
func (ds *Shard) Find(ctx context.Context, name string) (*ipld.Link, error) {
	hv := &hashBits{b: hash([]byte(name))}

	var out *ipld.Link
	err := ds.getValue(ctx, hv, name, func(sv *Shard) error {
		out = sv.val
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

type linkType int

const (
	invalidLink linkType = iota
	shardLink
	shardValueLink
)

func (ds *Shard) childLinkType(lnk *ipld.Link) (linkType, error) {
	if len(lnk.Name) < ds.maxpadlen {
		return invalidLink, fmt.Errorf("invalid link name '%s'", lnk.Name)
	}
	if len(lnk.Name) == ds.maxpadlen {
		return shardLink, nil
	}
	return shardValueLink, nil
}

// getChild returns the i'th child of this shard. If it is cached in the
// children array, it will return it from there. Otherwise, it loads the child
// node from disk.
func (ds *Shard) getChild(ctx context.Context, i int) (*Shard, error) {
	if i >= len(ds.children) || i < 0 {
		return nil, fmt.Errorf("invalid index passed to getChild (likely corrupt bitfield)")
	}

	if len(ds.children) != len(ds.nd.Links()) {
		return nil, fmt.Errorf("inconsistent lengths between children array and Links array")
	}

	c := ds.children[i]
	if c != nil {
		return c, nil
	}

	return ds.loadChild(ctx, i)
}

// loadChild reads the i'th child node of this shard from disk and returns it
// as a 'child' interface
func (ds *Shard) loadChild(ctx context.Context, i int) (*Shard, error) {
	lnk := ds.nd.Links()[i]
	lnkLinkType, err := ds.childLinkType(lnk)
	if err != nil {
		return nil, err
	}

	var c *Shard
	if lnkLinkType == shardLink {
		nd, err := lnk.GetNode(ctx, ds.dserv)
		if err != nil {
			return nil, err
		}
		cds, err := NewHamtFromDag(ds.dserv, nd)
		if err != nil {
			return nil, err
		}

		c = cds
	} else {
		s, err := ds.makeShardValue(lnk)
		if err != nil {
			return nil, err
		}
		c = s
	}

	ds.children[i] = c
	return c, nil
}

func (ds *Shard) setChild(i int, c *Shard) {
	ds.children[i] = c
}

// Link returns a merklelink to this shard node
func (ds *Shard) Link() (*ipld.Link, error) {
	if ds.isValueNode() {
		return ds.val, nil
	}

	nd, err := ds.Node()
	if err != nil {
		return nil, err
	}

	err = ds.dserv.Add(context.TODO(), nd)
	if err != nil {
		return nil, err
	}

	return ipld.MakeLink(nd)
}

func (ds *Shard) insertChild(idx int, key string, lnk *ipld.Link) error {
	if lnk == nil {
		return os.ErrNotExist
	}

	i := ds.indexForBitPos(idx)
	ds.bitfield.SetBit(idx)

	lnk.Name = ds.linkNamePrefix(idx) + key
	sv := &Shard{
		key: key,
		val: lnk,
	}

	ds.children = append(ds.children[:i], append([]*Shard{sv}, ds.children[i:]...)...)
	ds.nd.SetLinks(append(ds.nd.Links()[:i], append([]*ipld.Link{nil}, ds.nd.Links()[i:]...)...))
	return nil
}

func (ds *Shard) rmChild(i int) error {
	if i < 0 || i >= len(ds.children) || i >= len(ds.nd.Links()) {
		return fmt.Errorf("hamt: attempted to remove child with out of range index")
	}

	copy(ds.children[i:], ds.children[i+1:])
	ds.children = ds.children[:len(ds.children)-1]

	copy(ds.nd.Links()[i:], ds.nd.Links()[i+1:])
	ds.nd.SetLinks(ds.nd.Links()[:len(ds.nd.Links())-1])

	return nil
}

func (ds *Shard) getValue(ctx context.Context, hv *hashBits, key string, cb func(*Shard) error) error {
	idx := hv.Next(ds.tableSizeLg2)
	if ds.bitfield.Bit(int(idx)) {
		cindex := ds.indexForBitPos(idx)

		child, err := ds.getChild(ctx, cindex)
		if err != nil {
			return err
		}

		if child.isValueNode() {
			if child.key == key {
				return cb(child)
			}
		} else {
			return child.getValue(ctx, hv, key, cb)
		}
	}

	return os.ErrNotExist
}

// EnumLinks collects all links in the Shard.
func (ds *Shard) EnumLinks(ctx context.Context) ([]*ipld.Link, error) {
	var links []*ipld.Link
	var setlk sync.Mutex

	getLinks := makeAsyncTrieGetLinks(ds.dserv, func(sv *Shard) error {
		lnk := sv.val
		lnk.Name = sv.key
		setlk.Lock()
		links = append(links, lnk)
		setlk.Unlock()
		return nil
	})

	cset := cid.NewSet()

	err := dag.EnumerateChildrenAsync(ctx, getLinks, ds.nd.Cid(), cset.Visit)
	return links, err
}

// ForEachLink walks the Shard and calls the given function.
func (ds *Shard) ForEachLink(ctx context.Context, f func(*ipld.Link) error) error {
	return ds.walkTrie(ctx, func(sv *Shard) error {
		lnk := sv.val
		lnk.Name = sv.key

		return f(lnk)
	})
}

// makeAsyncTrieGetLinks builds a getLinks function that can be used with EnumerateChildrenAsync
// to iterate a HAMT shard. It takes an IPLD Dag Service to fetch nodes, and a call back that will get called
// on all links to leaf nodes in a HAMT tree, so they can be collected for an EnumLinks operation
func makeAsyncTrieGetLinks(dagService ipld.DAGService, onShardValue func(shard *Shard) error) dag.GetLinks {

	return func(ctx context.Context, currentCid cid.Cid) ([]*ipld.Link, error) {
		node, err := dagService.Get(ctx, currentCid)
		if err != nil {
			return nil, err
		}
		directoryShard, err := NewHamtFromDag(dagService, node)
		if err != nil {
			return nil, err
		}

		childShards := make([]*ipld.Link, 0, len(directoryShard.children))
		links := directoryShard.nd.Links()
		for idx := range directoryShard.children {
			lnk := links[idx]
			lnkLinkType, err := directoryShard.childLinkType(lnk)

			if err != nil {
				return nil, err
			}
			if lnkLinkType == shardLink {
				childShards = append(childShards, lnk)
			} else {
				sv, err := directoryShard.makeShardValue(lnk)
				if err != nil {
					return nil, err
				}
				err = onShardValue(sv)
				if err != nil {
					return nil, err
				}
			}
		}
		return childShards, nil
	}
}

func (ds *Shard) walkTrie(ctx context.Context, cb func(*Shard) error) error {
	for idx := range ds.children {
		c, err := ds.getChild(ctx, idx)
		if err != nil {
			return err
		}

		if c.isValueNode() {
			if err := cb(c); err != nil {
				return err
			}
		} else {
			if err := c.walkTrie(ctx, cb); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ds *Shard) modifyValue(ctx context.Context, hv *hashBits, key string, val *ipld.Link) error {
	idx := hv.Next(ds.tableSizeLg2)
	if !ds.bitfield.Bit(idx) {
		return ds.insertChild(idx, key, val)
	}

	cindex := ds.indexForBitPos(idx)

	child, err := ds.getChild(ctx, cindex)
	if err != nil {
		return err
	}

	if child.isValueNode() {
		if child.key == key {
			// value modification
			if val == nil {
				ds.bitfield.UnsetBit(idx)
				return ds.rmChild(cindex)
			}

			child.val = val
			return nil
		}

		if val == nil {
			return os.ErrNotExist
		}

		// replace value with another shard, one level deeper
		ns, err := NewShard(ds.dserv, ds.tableSize)
		if err != nil {
			return err
		}
		ns.builder = ds.builder
		chhv := &hashBits{
			b:        hash([]byte(child.key)),
			consumed: hv.consumed,
		}

		err = ns.modifyValue(ctx, hv, key, val)
		if err != nil {
			return err
		}

		err = ns.modifyValue(ctx, chhv, child.key, child.val)
		if err != nil {
			return err
		}

		ds.setChild(cindex, ns)
		return nil
	} else {
		err := child.modifyValue(ctx, hv, key, val)
		if err != nil {
			return err
		}

		if val == nil {
			switch len(child.children) {
			case 0:
				// empty sub-shard, prune it
				// Note: this shouldnt normally ever happen
				//       in the event of another implementation creates flawed
				//       structures, this will help to normalize them.
				ds.bitfield.UnsetBit(idx)
				return ds.rmChild(cindex)
			case 1:
				nchild := child.children[0]
				if nchild.isValueNode() {
					// sub-shard with a single value element, collapse it
					ds.setChild(cindex, nchild)
				}
				return nil
			}
		}

		return nil
	}
}

// indexForBitPos returns the index within the collapsed array corresponding to
// the given bit in the bitset.  The collapsed array contains only one entry
// per bit set in the bitfield, and this function is used to map the indices.
func (ds *Shard) indexForBitPos(bp int) int {
	return ds.bitfield.OnesBefore(bp)
}

// linkNamePrefix takes in the bitfield index of an entry and returns its hex prefix
func (ds *Shard) linkNamePrefix(idx int) string {
	return fmt.Sprintf(ds.prefixPadStr, idx)
}
