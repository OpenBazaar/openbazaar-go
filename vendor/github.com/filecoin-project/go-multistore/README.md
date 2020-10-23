# go-fil-markets
[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![CircleCI](https://circleci.com/gh/filecoin-project/go-multistore.svg?style=svg)](https://circleci.com/gh/filecoin-project/go-multistore)
[![codecov](https://codecov.io/gh/filecoin-project/go-multistore/branch/master/graph/badge.svg)](https://codecov.io/gh/filecoin-project/go-multistore)
[![GoDoc](https://godoc.org/github.com/filecoin-project/go-multistore?status.svg)](https://godoc.org/github.com/filecoin-project/go-multistore)

This repository provides a mechanism for constructing multiple, isolated, IPFS storage instances (blockstore, filestore, DAGService) on top of a single
go-datastore instance.

### Background reading

You may want to familiarize yourself with various IPFS storage layer components:

- [DataStore](https://github.com/ipfs/go-datastore)
- [BlockStore](https://github.com/ipfs/go-ipfs-blockstore)
- [FileStore](https://github.com/ipfs/go-filestore)
- [BlockService](https://github.com/ipfs/go-blockservice)
- [DAGService](https://github.com/ipfs/go-ipld-format/blob/master/merkledag.go)

## Installation
```bash
go get "github.com/filecoin-project/go-multistore"`
```

## Usage

Initialize multistore:

```golang
var ds datastore.Batching
multiDs, err := multistore.NewMultiDstore(ds)
```

Create new store:

```golang
next := multiDs.Next()
store, err := multiDs.Get(store)

// store will have a blockstore, filestore, and DAGService
```

List existing store indexes:

```golang
indexes := multiDs.List()
```

Delete a store (will delete all data in isolated store without touching the rest of the datastore):

```golang
var index int
err := multiDs.Delete(index)
```

Shutdown (make sure everything is closed):

```golang
multiDs.Close()
```

## Contributing
Issues and PRs are welcome! Please first read the [background reading](#background-reading) and [CONTRIBUTING](./CONTRIBUTING.md) guide, and look over the current code. PRs against master require approval of at least two maintainers. 

Day-to-day discussion takes place in the #fil-components channel of the [Filecoin project chat](https://github.com/filecoin-project/community#chat). Usage or design questions are welcome.

## Project-level documentation
The filecoin-project has a [community repo](https://github.com/filecoin-project/community) with more detail about our resources and policies, such as the [Code of Conduct](https://github.com/filecoin-project/community/blob/master/CODE_OF_CONDUCT.md).

## License
This repository is dual-licensed under Apache 2.0 and MIT terms.

Copyright 2020. Protocol Labs, Inc.
