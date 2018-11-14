// Package merkledag implements the IPFS Merkle DAG data structures.
package merkledag

import (
	"context"
	"fmt"
	"sync"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	ipldcbor "gx/ipfs/QmSbrWdBtHgfYjeRcLotTrPXFkxY4GHMJZVxS6MQvzsYmf/go-ipld-cbor"
	bserv "gx/ipfs/QmWfhv1D18DRSiSm73r4QGcByspzPtxxRTcmHW3axFXZo8/go-blockservice"
)

// TODO: We should move these registrations elsewhere. Really, most of the IPLD
// functionality should go in a `go-ipld` repo but that will take a lot of work
// and design.
func init() {
	ipld.Register(cid.DagProtobuf, DecodeProtobufBlock)
	ipld.Register(cid.Raw, DecodeRawBlock)
	ipld.Register(cid.DagCBOR, ipldcbor.DecodeBlock)
}

// contextKey is a type to use as value for the ProgressTracker contexts.
type contextKey string

const progressContextKey contextKey = "progress"

// NewDAGService constructs a new DAGService (using the default implementation).
// Note that the default implementation is also an ipld.LinkGetter.
func NewDAGService(bs bserv.BlockService) *dagService {
	return &dagService{Blocks: bs}
}

// dagService is an IPFS Merkle DAG service.
// - the root is virtual (like a forest)
// - stores nodes' data in a BlockService
// TODO: should cache Nodes that are in memory, and be
//       able to free some of them when vm pressure is high
type dagService struct {
	Blocks bserv.BlockService
}

// Add adds a node to the dagService, storing the block in the BlockService
func (n *dagService) Add(ctx context.Context, nd ipld.Node) error {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return fmt.Errorf("dagService is nil")
	}

	return n.Blocks.AddBlock(nd)
}

func (n *dagService) AddMany(ctx context.Context, nds []ipld.Node) error {
	blks := make([]blocks.Block, len(nds))
	for i, nd := range nds {
		blks[i] = nd
	}
	return n.Blocks.AddBlocks(blks)
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *dagService) Get(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	if n == nil {
		return nil, fmt.Errorf("dagService is nil")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := n.Blocks.GetBlock(ctx, c)
	if err != nil {
		if err == bserv.ErrNotFound {
			return nil, ipld.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get block for %s: %v", c, err)
	}

	return ipld.Decode(b)
}

// GetLinks return the links for the node, the node doesn't necessarily have
// to exist locally.
func (n *dagService) GetLinks(ctx context.Context, c cid.Cid) ([]*ipld.Link, error) {
	if c.Type() == cid.Raw {
		return nil, nil
	}
	node, err := n.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

func (n *dagService) Remove(ctx context.Context, c cid.Cid) error {
	return n.Blocks.DeleteBlock(c)
}

// RemoveMany removes multiple nodes from the DAG. It will likely be faster than
// removing them individually.
//
// This operation is not atomic. If it returns an error, some nodes may or may
// not have been removed.
func (n *dagService) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	// TODO(#4608): make this batch all the way down.
	for _, c := range cids {
		if err := n.Blocks.DeleteBlock(c); err != nil {
			return err
		}
	}
	return nil
}

// GetLinksDirect creates a function to get the links for a node, from
// the node, bypassing the LinkService.  If the node does not exist
// locally (and can not be retrieved) an error will be returned.
func GetLinksDirect(serv ipld.NodeGetter) GetLinks {
	return func(ctx context.Context, c cid.Cid) ([]*ipld.Link, error) {
		nd, err := serv.Get(ctx, c)
		if err != nil {
			if err == bserv.ErrNotFound {
				err = ipld.ErrNotFound
			}
			return nil, err
		}
		return nd.Links(), nil
	}
}

type sesGetter struct {
	bs *bserv.Session
}

// Get gets a single node from the DAG.
func (sg *sesGetter) Get(ctx context.Context, c cid.Cid) (ipld.Node, error) {
	blk, err := sg.bs.GetBlock(ctx, c)
	switch err {
	case bserv.ErrNotFound:
		return nil, ipld.ErrNotFound
	default:
		return nil, err
	case nil:
		// noop
	}

	return ipld.Decode(blk)
}

// GetMany gets many nodes at once, batching the request if possible.
func (sg *sesGetter) GetMany(ctx context.Context, keys []cid.Cid) <-chan *ipld.NodeOption {
	return getNodesFromBG(ctx, sg.bs, keys)
}

// Session returns a NodeGetter using a new session for block fetches.
func (n *dagService) Session(ctx context.Context) ipld.NodeGetter {
	return &sesGetter{bserv.NewSession(ctx, n.Blocks)}
}

// FetchGraph fetches all nodes that are children of the given node
func FetchGraph(ctx context.Context, root cid.Cid, serv ipld.DAGService) error {
	return FetchGraphWithDepthLimit(ctx, root, -1, serv)
}

// FetchGraphWithDepthLimit fetches all nodes that are children to the given
// node down to the given depth. maxDetph=0 means "only fetch root",
// maxDepth=1 means "fetch root and its direct children" and so on...
// maxDepth=-1 means unlimited.
func FetchGraphWithDepthLimit(ctx context.Context, root cid.Cid, depthLim int, serv ipld.DAGService) error {
	var ng ipld.NodeGetter = serv
	ds, ok := serv.(*dagService)
	if ok {
		ng = &sesGetter{bserv.NewSession(ctx, ds.Blocks)}
	}

	set := make(map[string]int)

	// Visit function returns true when:
	// * The element is not in the set and we're not over depthLim
	// * The element is in the set but recorded depth is deeper
	//   than currently seen (if we find it higher in the tree we'll need
	//   to explore deeper than before).
	// depthLim = -1 means we only return true if the element is not in the
	// set.
	visit := func(c cid.Cid, depth int) bool {
		key := string(c.Bytes())
		oldDepth, ok := set[key]

		if (ok && depthLim < 0) || (depthLim >= 0 && depth > depthLim) {
			return false
		}

		if !ok || oldDepth > depth {
			set[key] = depth
			return true
		}
		return false
	}

	v, _ := ctx.Value(progressContextKey).(*ProgressTracker)
	if v == nil {
		return EnumerateChildrenAsyncDepth(ctx, GetLinksDirect(ng), root, 0, visit)
	}

	visitProgress := func(c cid.Cid, depth int) bool {
		if visit(c, depth) {
			v.Increment()
			return true
		}
		return false
	}
	return EnumerateChildrenAsyncDepth(ctx, GetLinksDirect(ng), root, 0, visitProgress)
}

// GetMany gets many nodes from the DAG at once.
//
// This method may not return all requested nodes (and may or may not return an
// error indicating that it failed to do so. It is up to the caller to verify
// that it received all nodes.
func (n *dagService) GetMany(ctx context.Context, keys []cid.Cid) <-chan *ipld.NodeOption {
	return getNodesFromBG(ctx, n.Blocks, keys)
}

func dedupKeys(keys []cid.Cid) []cid.Cid {
	set := cid.NewSet()
	for _, c := range keys {
		set.Add(c)
	}
	if set.Len() == len(keys) {
		return keys
	}
	return set.Keys()
}

func getNodesFromBG(ctx context.Context, bs bserv.BlockGetter, keys []cid.Cid) <-chan *ipld.NodeOption {
	keys = dedupKeys(keys)

	out := make(chan *ipld.NodeOption, len(keys))
	blocks := bs.GetBlocks(ctx, keys)
	var count int

	go func() {
		defer close(out)
		for {
			select {
			case b, ok := <-blocks:
				if !ok {
					if count != len(keys) {
						out <- &ipld.NodeOption{Err: fmt.Errorf("failed to fetch all nodes")}
					}
					return
				}

				nd, err := ipld.Decode(b)
				if err != nil {
					out <- &ipld.NodeOption{Err: err}
					return
				}

				out <- &ipld.NodeOption{Node: nd}
				count++

			case <-ctx.Done():
				out <- &ipld.NodeOption{Err: ctx.Err()}
				return
			}
		}
	}()
	return out
}

// GetLinks is the type of function passed to the EnumerateChildren function(s)
// for getting the children of an IPLD node.
type GetLinks func(context.Context, cid.Cid) ([]*ipld.Link, error)

// GetLinksWithDAG returns a GetLinks function that tries to use the given
// NodeGetter as a LinkGetter to get the children of a given IPLD node. This may
// allow us to traverse the DAG without actually loading and parsing the node in
// question (if we already have the links cached).
func GetLinksWithDAG(ng ipld.NodeGetter) GetLinks {
	return func(ctx context.Context, c cid.Cid) ([]*ipld.Link, error) {
		return ipld.GetLinks(ctx, ng, c)
	}
}

// EnumerateChildren will walk the dag below the given root node and add all
// unseen children to the passed in set.
// TODO: parallelize to avoid disk latency perf hits?
func EnumerateChildren(ctx context.Context, getLinks GetLinks, root cid.Cid, visit func(cid.Cid) bool) error {
	visitDepth := func(c cid.Cid, depth int) bool {
		return visit(c)
	}

	return EnumerateChildrenDepth(ctx, getLinks, root, 0, visitDepth)
}

// EnumerateChildrenDepth walks the dag below the given root and passes the
// current depth to a given visit function. The visit function can be used to
// limit DAG exploration.
func EnumerateChildrenDepth(ctx context.Context, getLinks GetLinks, root cid.Cid, depth int, visit func(cid.Cid, int) bool) error {
	links, err := getLinks(ctx, root)
	if err != nil {
		return err
	}

	for _, lnk := range links {
		c := lnk.Cid
		if visit(c, depth+1) {
			err = EnumerateChildrenDepth(ctx, getLinks, c, depth+1, visit)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ProgressTracker is used to show progress when fetching nodes.
type ProgressTracker struct {
	Total int
	lk    sync.Mutex
}

// DeriveContext returns a new context with value "progress" derived from
// the given one.
func (p *ProgressTracker) DeriveContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, progressContextKey, p)
}

// Increment adds one to the total progress.
func (p *ProgressTracker) Increment() {
	p.lk.Lock()
	defer p.lk.Unlock()
	p.Total++
}

// Value returns the current progress.
func (p *ProgressTracker) Value() int {
	p.lk.Lock()
	defer p.lk.Unlock()
	return p.Total
}

// FetchGraphConcurrency is total number of concurrent fetches that
// 'fetchNodes' will start at a time
var FetchGraphConcurrency = 8

// EnumerateChildrenAsync is equivalent to EnumerateChildren *except* that it
// fetches children in parallel.
//
// NOTE: It *does not* make multiple concurrent calls to the passed `visit` function.
func EnumerateChildrenAsync(ctx context.Context, getLinks GetLinks, c cid.Cid, visit func(cid.Cid) bool) error {
	visitDepth := func(c cid.Cid, depth int) bool {
		return visit(c)
	}

	return EnumerateChildrenAsyncDepth(ctx, getLinks, c, 0, visitDepth)
}

// EnumerateChildrenAsyncDepth is equivalent to EnumerateChildrenDepth *except*
// that it fetches children in parallel (down to a maximum depth in the graph).
//
// NOTE: It *does not* make multiple concurrent calls to the passed `visit` function.
func EnumerateChildrenAsyncDepth(ctx context.Context, getLinks GetLinks, c cid.Cid, startDepth int, visit func(cid.Cid, int) bool) error {
	type cidDepth struct {
		cid   cid.Cid
		depth int
	}

	type linksDepth struct {
		links []*ipld.Link
		depth int
	}

	feed := make(chan *cidDepth)
	out := make(chan *linksDepth)
	done := make(chan struct{})

	var setlk sync.Mutex

	errChan := make(chan error)
	fetchersCtx, cancel := context.WithCancel(ctx)

	defer cancel()

	for i := 0; i < FetchGraphConcurrency; i++ {
		go func() {
			for cdepth := range feed {
				ci := cdepth.cid
				depth := cdepth.depth

				setlk.Lock()
				shouldVisit := visit(ci, depth)
				setlk.Unlock()

				if shouldVisit {
					links, err := getLinks(ctx, ci)
					if err != nil {
						errChan <- err
						return
					}

					outLinks := &linksDepth{
						links: links,
						depth: depth + 1,
					}

					select {
					case out <- outLinks:
					case <-fetchersCtx.Done():
						return
					}
				}
				select {
				case done <- struct{}{}:
				case <-fetchersCtx.Done():
				}
			}
		}()
	}
	defer close(feed)

	send := feed
	var todobuffer []*cidDepth
	var inProgress int

	next := &cidDepth{
		cid:   c,
		depth: startDepth,
	}
	for {
		select {
		case send <- next:
			inProgress++
			if len(todobuffer) > 0 {
				next = todobuffer[0]
				todobuffer = todobuffer[1:]
			} else {
				next = nil
				send = nil
			}
		case <-done:
			inProgress--
			if inProgress == 0 && next == nil {
				return nil
			}
		case linksDepth := <-out:
			for _, lnk := range linksDepth.links {
				cd := &cidDepth{
					cid:   lnk.Cid,
					depth: linksDepth.depth,
				}

				if next == nil {
					next = cd
					send = feed
				} else {
					todobuffer = append(todobuffer, cd)
				}
			}
		case err := <-errChan:
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

var _ ipld.LinkGetter = &dagService{}
var _ ipld.NodeGetter = &dagService{}
var _ ipld.NodeGetter = &sesGetter{}
var _ ipld.DAGService = &dagService{}
