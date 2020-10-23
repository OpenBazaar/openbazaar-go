# go-graphsync

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](http://ipfs.io/)
[![Matrix](https://img.shields.io/badge/matrix-%23ipfs%3Amatrix.org-blue.svg?style=flat-square)](https://matrix.to/#/#ipfs:matrix.org)
[![IRC](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![Discord](https://img.shields.io/discord/475789330380488707?color=blueviolet&label=discord&style=flat-square)](https://discord.gg/24fmuwR)
[![Coverage Status](https://codecov.io/gh/ipfs/go-graphsync/branch/master/graph/badge.svg)](https://codecov.io/gh/ipfs/go-graphsync/branch/master)
[![Build Status](https://circleci.com/gh/ipfs/go-bitswap.svg?style=svg)](https://circleci.com/gh/ipfs/go-graphsync)

> An implementation of the [graphsync protocol](https://github.com/ipld/specs/blob/master/block-layer/graphsync/graphsync.md) in go!

## Table of Contents

- [Background](#background)
- [Install](#install)
- [Usage](#usage)
- [Architecture](#architecture)
- [Contribute](#contribute)
- [License](#license)

## Background

[GraphSync](https://github.com/ipld/specs/blob/master/block-layer/graphsync/graphsync.md) is a protocol for synchronizing IPLD graphs among peers. It allows a host to make a single request to a remote peer for all of the results of traversing an [IPLD selector](https://github.com/ipld/specs/blob/master/block-layer/selectors/selectors.md) on the remote peer's local IPLD graph. 

`go-graphsync` provides an implementation of the Graphsync protocol in go.

### Go-IPLD-Prime

`go-graphsync` relies on `go-ipld-prime` to traverse IPLD Selectors in an IPLD graph. `go-ipld-prime` implements the [IPLD specification](https://github.com/ipld/specs) in go and is an alternative to older implementations such as `go-ipld-format` and `go-ipld-cbor`. In order to use `go-graphsync`, some understanding and use of `go-ipld-prime` concepts is necessary. 

If your existing library (i.e. `go-ipfs` or `go-filecoin`) uses these other older libraries, you can largely use go-graphsync without switching to `go-ipld-prime` across your codebase, but it will require some translations

## Install

`go-graphsync` requires Go >= 1.11 and can be installed using Go modules

## Usage

### Initializing a GraphSync Exchange

```golang
import (
  graphsync "github.com/ipfs/go-graphsync/impl"
  gsnet "github.com/ipfs/go-graphsync/network"
  ipld "github.com/ipld/go-ipld-prime"
)

var ctx context.Context
var host libp2p.Host
var loader ipld.Loader
var storer ipld.Storer

network := gsnet.NewFromLibp2pHost(host)
exchange := graphsync.New(ctx, network, loader, storer)
```

Parameter Notes:

1. `context` is just the parent context for all of GraphSync
2. `network` is a network abstraction provided to Graphsync on top
of libp2p. This allows graphsync to be tested without the actual network
3. `loader` is used to load blocks from content ids from the local block store. It's used when RESPONDING to requests from other clients. It should conform to the IPLD loader interface: https://github.com/ipld/go-ipld-prime/blob/master/linking.go
4. `storer` is used to store incoming blocks to the local block store. It's used when REQUESTING a graphsync query, to store blocks locally once they are validated as part of the correct response. It should conform to the IPLD storer interface: https://github.com/ipld/go-ipld-prime/blob/master/linking.go

### Using GraphSync With An IPFS BlockStore

GraphSync provides two convenience functions in the `storeutil` package for
integrating with BlockStore's from IPFS.

```golang
import (
  graphsync "github.com/ipfs/go-graphsync/impl"
  gsnet "github.com/ipfs/go-graphsync/network"
  storeutil "github.com/ipfs/go-graphsync/storeutil"
  ipld "github.com/ipld/go-ipld-prime"
  blockstore "github.com/ipfs/go-ipfs-blockstore"
)

var ctx context.Context
var host libp2p.Host
var bs blockstore.Blockstore

network := gsnet.NewFromLibp2pHost(host)
loader := storeutil.LoaderForBlockstore(bs)
storer := storeutil.StorerForBlockstore(bs)

exchange := graphsync.New(ctx, network, loader, storer)
```

### Write A Loader An IPFS BlockStore

If you are using a traditional go-ipfs-blockstore, your link loading function looks like this:

```golang
type BlockStore interface {
	Get(lnk cid.Cid) (blocks.Block, error)
}
```

or, more generally:

```golang
type Cid2BlockFn func (lnk cid.Cid) (blocks.Block, error)
```

in `go-ipld-prime`, the signature for a link loader is as follows:

```golang
type Loader func(lnk Link, lnkCtx LinkContext) (io.Reader, error)
```

`go-ipld-prime` intentionally keeps its interfaces as abstract as possible to limit dependencies on other ipfs/filecoin specific packages. An IPLD Link is an abstraction for a CID, and IPLD expects io.Reader's rather than an actual block. IPLD provides a `cidLink` package for working with Links that use CIDs as the underlying data, and it's safe to assume that's the type in use if your code deals only with CIDs. A conversion would look something like this:

```golang
import (
   ipld "github.com/ipld/go-ipld-prime"
   cidLink "github.com/ipld/go-ipld-prime/linking/cid"
)

func LoaderFromCid2BlockFn(cid2BlockFn Cid2BlockFn) ipld.Loader {
	return func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		asCidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("Unsupported Link Type")
		}
		block, err := cid2BlockFn(asCidLink.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(block.RawData()), nil
	}
}
```

### Write A Storer From An IPFS BlockStore

If you are using a traditional go-ipfs-blockstore, your storage function looks like this:

```golang
type BlockStore interface {
	Put(blocks.Block) error
}
```

or, more generally:

```golang
type BlockStoreFn func (blocks.Block) (error)
```

in `go-ipld-prime`, the signature for a link storer is a bit different:

```golang
type StoreCommitter func(Link) error
type Storer func(lnkCtx LinkContext) (io.Writer, StoreCommitter, error)
```

`go-ipld-prime` stores in two parts to support streaming -- the storer is called and returns an IO.Writer and a function to commit changes when finished. Here's how you can write a storer from a traditional block storing signature.

```golang
import (
	blocks "github.com/ipfs/go-block-format"
  ipld "github.com/ipld/go-ipld-prime"
  cidLink "github.com/ipld/go-ipld-prime/linking/cid"
)

func StorerFromBlockStoreFn(blockStoreFn BlockStoreFn) ipld.Storer {
	return func(lnkCtx ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
		var buffer bytes.Buffer
		committer := func(lnk ipld.Link) error {
			asCidLink, ok := lnk.(cidlink.Link)
			if !ok {
				return fmt.Errorf("Unsupported Link Type")
			}
			block := blocks.NewBlockWithCid(buffer.Bytes(), asCidLink.Cid)
			return blockStoreFn(block)
		}
		return &buffer, committer, nil
	}
}
```

### Calling Graphsync

```golang
var exchange graphsync.GraphSync
var ctx context.Context
var p peer.ID
var selector ipld.Node
var rootLink ipld.Link

var responseProgress <-chan graphsync.ResponseProgress
var errors <-chan error

responseProgress, errors = exchange.Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node)
```

Paramater Notes:
1. `ctx` is the context for this request. To cancel an in progress request, cancel the context.
2. `p` is the peer you will send this request to
3. `link` is an IPLD Link, i.e. a CID (cidLink.Link{Cid})
4. `selector` is an IPLD selector node. Recommend using selector builders from go-ipld-prime to construct these

### Response Type

```golang

type ResponseProgress struct {
  Node      ipld.Node // a node which matched the graphsync query
  Path      ipld.Path // the path of that node relative to the traversal start
	LastBlock struct {  // LastBlock stores the Path and Link of the last block edge we had to load. 
		ipld.Path
		ipld.Link
	}
}

```

The above provides both immediate and relevant metadata for matching nodes in a traversal, and is very similar to the information provided by a local IPLD selector traversal in `go-ipld-prime`

## Contribute

PRs are welcome!

Before doing anything heavy, checkout the [Graphsync Architecture](docs/architecture.md)

See our [Contributing Guidelines](https://github.com/ipfs/go-graphsync/blob/master/CONTRIBUTING.md) for more info.

## License

This library is dual-licensed under Apache 2.0 and MIT terms.

Copyright 2019. Protocol Labs, Inc.
